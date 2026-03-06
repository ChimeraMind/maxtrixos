package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/runner"
	"matrixos/vector/lib/seeder"
)

// EnterCommand enters a seeded chroot interactively.
type EnterCommand struct {
	BaseCommand
	SignalGuard
	fs *flag.FlagSet

	// Library instances
	det seeder.ISeederDetector

	// Replaceable for testing
	chrootRunner runner.ChrootRunFunc

	// Flags
	skipLock bool

	// Positional arguments (chroot dirs or names)
	targets []string
}

// NewEnterCommand creates a new EnterCommand.
func NewEnterCommand() *EnterCommand {
	return &EnterCommand{
		chrootRunner: runner.ChrootRun,
	}
}

func (c *EnterCommand) Name() string {
	return "enter"
}

func (c *EnterCommand) Init(args []string) error {
	if err := c.parseArgs(args); err != nil {
		return err
	}
	if err := c.initBaseConfig(); err != nil {
		return fmt.Errorf("error reading config: %w", err)
	}

	det, err := seeder.NewSeederDetector(c.cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize seeder detector: %w", err)
	}
	c.det = det

	return nil
}

func (c *EnterCommand) parseArgs(args []string) error {
	c.fs = flag.NewFlagSet("enter", flag.ContinueOnError)

	c.fs.BoolVar(&c.skipLock, "skiplock", false, "Skip acquiring the seeder lock before entering the chroot")

	c.fs.Usage = func() {
		fmt.Println("Usage: vector dev enter [--skiplock] <chroot-dir-or-name> [...]")
		fmt.Println()
		fmt.Println("Enter a seeded chroot interactively.")
		fmt.Println("Specify full paths to chroot directories, or just the chroot name.")
		fmt.Println()
		c.fs.PrintDefaults()
	}

	if err := c.fs.Parse(args); err != nil {
		return err
	}

	if getEuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	c.targets = c.fs.Args()
	if len(c.targets) == 0 {
		c.fs.Usage()
		return fmt.Errorf("no chroot dirs or names specified")
	}

	return nil
}

// Run delegates to the SignalGuard for cleanup on signals/panics.
func (c *EnterCommand) Run() error {
	return c.RunWithGuard(c.run)
}

// run implements the enter workflow.
func (c *EnterCommand) run() error {
	// Classify targets into absolute dirs and bare names.
	var chrootDirs []string
	var chrootNames []string
	for _, target := range c.targets {
		if target == "" {
			continue
		}
		if filesystems.DirectoryExists(target) {
			chrootDirs = append(chrootDirs, target)
			continue
		}
		name := filepath.Base(target)
		if target == name {
			// Bare name — will be resolved against detected seeders.
			chrootNames = append(chrootNames, target)
			continue
		}
		return fmt.Errorf("unable to accept %s, unrecognized argument", target)
	}

	// Resolve bare names by scanning seeder params for SEEDER_CHROOTS_DIR.
	if len(chrootNames) > 0 {
		resolved, err := c.resolveNames(chrootNames)
		if err != nil {
			return err
		}
		chrootDirs = append(chrootDirs, resolved...)
	}

	if len(chrootDirs) == 0 {
		return fmt.Errorf("no chroot dirs or names found")
	}

	for _, d := range chrootDirs {
		fmt.Printf("Found seed: %s\n", d)
	}

	// Enter each chroot.
	for _, chrootDir := range chrootDirs {
		if err := c.enterChroot(chrootDir); err != nil {
			return fmt.Errorf("error entering chroot %s: %w", chrootDir, err)
		}
	}

	return nil
}

// resolveNames maps bare chroot names to full paths by examining
// each detected seeder's params for SEEDER_CHROOTS_DIR.
func (c *EnterCommand) resolveNames(names []string) ([]string, error) {
	sd, err := newSeeder(c.cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize seeder: %w", err)
	}
	c.PushCleanup(sd.Cleanup)
	defer sd.Cleanup()

	paramsName, err := sd.ParamsExecutableName()
	if err != nil {
		return nil, fmt.Errorf("failed to get params executable name: %w", err)
	}

	seeders, err := c.det.Detect(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("seeder detection failed: %w", err)
	}

	// Collect all unique SEEDER_CHROOTS_DIR values.
	seen := make(map[string]bool)
	var chrootsDirs []string
	for _, info := range seeders {
		paramsPath := filepath.Join(info.Dir, paramsName)
		if !filesystems.FileExists(paramsPath) {
			continue
		}
		params, err := sd.ParseSeederParams(info.Name, paramsPath)
		if err != nil {
			// Skip seeders whose params cannot be parsed.
			continue
		}
		if params.ChrootsDir == "" {
			continue
		}
		if !seen[params.ChrootsDir] {
			seen[params.ChrootsDir] = true
			chrootsDirs = append(chrootsDirs, params.ChrootsDir)
		}
	}

	// Look for each name inside the collected chroots dirs.
	var resolved []string
	for _, name := range names {
		for _, dir := range chrootsDirs {
			candidate := filepath.Join(dir, name)
			if filesystems.DirectoryExists(candidate) {
				resolved = append(resolved, candidate)
			}
		}
	}

	return resolved, nil
}

// enterChroot sets up mounts, runs an interactive shell inside the
// chroot, and tears down mounts afterwards.
func (c *EnterCommand) enterChroot(chrootDir string) error {
	sd, err := newSeeder(c.cfg, nil)
	if err != nil {
		return fmt.Errorf("failed to initialize seeder: %w", err)
	}
	c.PushCleanup(sd.Cleanup)
	defer sd.Cleanup()

	fmt.Printf("Entering seed %s: %s\n", filepath.Base(chrootDir), chrootDir)

	if c.skipLock {
		fmt.Println("Skipping seeder lock acquisition (--skiplock).")
		if err := c.enterChrootWorker(sd, chrootDir); err != nil {
			return fmt.Errorf(
				"seeder %s chroot enter failed: %w", filepath.Base(chrootDir), err,
			)
		}
		return nil
	}

	return c.enterChrootWithLock(sd, chrootDir)
}

func (c *EnterCommand) enterChrootWithLock(sd seeder.ISeeder, chrootDir string) error {
	paramsName, err := sd.ParamsExecutableName()
	if err != nil {
		return fmt.Errorf("failed to get params executable name: %w", err)
	}

	seeders, err := c.det.Detect(nil, nil)
	if err != nil {
		return fmt.Errorf("seeder detection failed: %w", err)
	}

	// Find the corresponding seeder chroot dir matching it with chrootDir.
	var seeder *seeder.SeederInfo
	for _, info := range seeders {
		paramsPath := filepath.Join(info.Dir, paramsName)
		if !filesystems.FileExists(paramsPath) {
			continue
		}
		params, err := sd.ParseSeederParams(info.Name, paramsPath)
		if err != nil {
			// Skip seeders whose params cannot be parsed.
			continue
		}
		if slices.Contains(params.AllChrootDirs, chrootDir) {
			seeder = &info
			break
		}
	}
	if seeder == nil {
		return fmt.Errorf(
			"no valid seeder chroot found for chroot dir %s. Try with --skiplock.",
			chrootDir,
		)
	}

	return sd.ExecuteWithSeederLock(
		seeder.Name,
		func() error { return c.enterChrootWorker(sd, chrootDir) },
	)
}

func (c *EnterCommand) enterChrootWorker(sd seeder.ISeeder, chrootDir string) error {
	opts := seeder.SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}

	if err := sd.SetupChrootMounts(opts); err != nil {
		return fmt.Errorf("error setting up mounts: %w", err)
	}

	// Run interactive shell.
	return c.chrootRunner(&runner.ChrootCmd{
		Cmd: runner.Cmd{
			Name:   "/bin/sh",
			Args:   []string{"--login"},
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		},
		ChrootDir: chrootDir,
	})
}
