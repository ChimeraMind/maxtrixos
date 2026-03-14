package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/seeder"
	"matrixos/vector/lib/validation"
)

// newSeeder is the factory used to create an ISeeder.
// Tests replace it with a function that returns a mock.
var newSeeder = func(cfg config.IConfig, opts *seeder.NewSeederOptions) (seeder.ISeeder, error) {
	return seeder.NewSeeder(cfg, opts)
}

// SeedsCommand orchestrates the seeder workflow — detecting,
// preparing, and building chroot filesystems using the configured
// seeders.  It is the Go port of build/seeder.
type SeedsCommand struct {
	BaseCommand
	UI
	SignalGuard
	fs *flag.FlagSet

	// Library instances
	det seeder.ISeederDetector
	qa  *validation.QA

	// Flags
	chrootDir        string
	skipSeedersRaw   string
	onlySeedersRaw   string
	resume           bool
	builtRootfsFile  string
	builtSeedersFile string
	stage3File       string
	verbose          bool

	// Parsed from flags
	skipSeeders []string
	onlySeeders []string
}

// NewSeedsCommand creates a new SeedsCommand.
func NewSeedsCommand() *SeedsCommand {
	return &SeedsCommand{}
}

func (c *SeedsCommand) Name() string {
	return "seeds"
}

func (c *SeedsCommand) Init(args []string) error {
	if err := c.parseArgs(args); err != nil {
		return err
	}
	if err := c.initBaseConfig(); err != nil {
		return err
	}

	qa, err := validation.New(c.cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize QA: %w", err)
	}
	c.qa = qa

	det, err := seeder.NewSeederDetector(c.cfg)
	if err != nil {
		return fmt.Errorf(
			"failed to initialize seeder detector: %w", err,
		)
	}
	c.det = det

	c.StartUI()
	return nil
}

// parseArgs parses command-line arguments without initializing config.
func (c *SeedsCommand) parseArgs(args []string) error {
	c.fs = flag.NewFlagSet("seeds", flag.ContinueOnError)

	c.fs.StringVar(&c.chrootDir, "chroot-dir", "",
		"Use the provided directory as chroot directory")
	c.fs.StringVar(&c.skipSeedersRaw, "skip-seeders", "",
		"Comma-separated list of seeders to skip")
	c.fs.StringVar(&c.onlySeedersRaw, "only-seeders", "",
		"Comma-separated allow-list of seeders to accept")
	c.fs.BoolVar(&c.resume, "resume", false,
		"Try resuming seeder work inside chroot")
	c.fs.StringVar(
		&c.builtRootfsFile, "built-rootfs-file", "",
		"Path to write successfully built chroot dirs")
	c.fs.StringVar(
		&c.builtSeedersFile, "built-seeders-file", "",
		"Path to write successfully built seeder names")
	c.fs.StringVar(&c.stage3File, "stage3-file", "",
		"Gentoo stage3 file to unpack (skip download)")
	c.fs.BoolVar(&c.verbose, "verbose", false,
		"Enable verbose mode")

	c.fs.Usage = func() {
		fmt.Printf(
			"Usage: vector build %s [options]\n", c.Name(),
		)
		fmt.Println("\nOptions:")
		c.fs.PrintDefaults()
	}
	if err := c.fs.Parse(args); err != nil {
		return err
	}

	// Must be root.
	if getEuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	c.skipSeeders = SplitCSV(c.skipSeedersRaw)
	c.onlySeeders = SplitCSV(c.onlySeedersRaw)
	return nil
}

// Run delegates to the SignalGuard for cleanup on signals/panics.
func (c *SeedsCommand) Run() error {
	return c.RunWithGuard(c.runSeeds)
}

// updateStdWriters updates the stdout and stderr writers with the given label
// and propagates them to the seeder library.
func (c *SeedsCommand) updateStdWriters(sd seeder.ISeeder, name string) {
	c.SetupPrinters(fmt.Sprintf("seeds:%s", name))
	sd.SetStdout(c.StdoutWriter())
	sd.SetStderr(c.StderrWriter())
	c.det.SetStderr(c.StderrWriter())
}

// runSeeds implements the seeder workflow, mirroring the bash seeder
// script's main() function.
func (c *SeedsCommand) runSeeds() error {
	sd, err := newSeeder(
		c.cfg,
		&seeder.NewSeederOptions{Verbose: c.verbose},
	)
	if err != nil {
		return fmt.Errorf("failed to initialize seeder: %w", err)
	}

	writerSetup := func(tsd seeder.ISeeder) {
		c.updateStdWriters(tsd, "main")
		c.PushCleanup(c.FlushPrinters)
	}
	writerSetup(sd)

	// Ensure private repo is initialized.
	if err := sd.MaybeInitializePrivateRepo(); err != nil {
		return fmt.Errorf(
			"private repo initialization failed: %w", err,
		)
	}

	// Verify seeder environment.
	if err := c.qa.VerifySeederEnvironmentSetup("/"); err != nil {
		return fmt.Errorf(
			"environment verification failed: %w", err,
		)
	}

	c.PushCleanup(sd.Cleanup)

	// Import Gentoo GPG keys.
	if err := sd.ImportGentooGpgKeys(); err != nil {
		return fmt.Errorf("GPG key import failed: %w", err)
	}

	// Detect seeders.
	seeders, err := c.det.Detect(c.skipFilter(), c.onlyFilter())
	if err != nil {
		return fmt.Errorf("seeder detection failed: %w", err)
	}
	if len(seeders) == 0 {
		return fmt.Errorf("no seeders found, nothing to do")
	}

	// Print execution plan.
	c.Printf("Will execute seeders in the following order:\n")
	for _, s := range seeders {
		c.Printf("  %s\n", s.Dir)
	}

	// Initialize output files.
	if err := c.initOutputFiles(); err != nil {
		return err
	}

	// Execute each seeder under its lock.
	for _, info := range seeders {
		err = sd.ExecuteWithSeederLock(
			info.Name,
			func() error { return c.seederWorker(sd, info) },
		)
		if err != nil {
			writerSetup(sd)
			return fmt.Errorf(
				"seeder %s failed: %w", info.Name, err,
			)
		}
	}

	writerSetup(sd)
	c.Printf("Seeds build complete:\n")
	for _, info := range seeders {
		c.Printf("  [%s] %s done.\n", info.Name, info.Dir)
	}
	return nil
}

// seederWorker processes a single seeder: parse params, resolve chroot
// dir, run prepper, set up mounts/DNS/dirs, execute chroot script,
// mark done, and record results.
func (c *SeedsCommand) seederWorker(sd seeder.ISeeder, info seeder.SeederInfo) error {
	sd.Print(
		"[%s] Accepted seeder for execution\n", info.Name,
	)

	c.updateStdWriters(sd, info.Name)
	c.PushCleanup(func() {
		sd.Cleanup()
		c.FlushPrinters()
	})
	// To umount all the mount points.
	defer sd.Cleanup()

	// Parse seeder params.
	paramsName, err := sd.ParamsExecutableName()
	if err != nil {
		return err
	}
	paramsPath := filepath.Join(info.Dir, paramsName)
	if !filesystems.FileExists(paramsPath) {
		return fmt.Errorf("unable to find %s", paramsPath)
	}

	params, err := sd.ParseSeederParams(info.Name, paramsPath)
	if err != nil {
		return fmt.Errorf("failed to parse params: %w", err)
	}

	// Resolve chroot directory.
	chrootDir := params.PreferredChrootDir
	if c.chrootDir != "" {
		sd.PrintWarning(
			"[%s] Overriding chroot dir with --chroot-dir='%s'. This can be dangerous for multiple chroots.\n",
			info.Name,
			c.chrootDir,
		)
		chrootDir = c.chrootDir
	}
	if chrootDir == "" {
		return fmt.Errorf(
			"[%s] no chroot dir specified in params.sh for seeder or --chroot-dir",
			info.Name,
		)
	}

	flagFile, err := sd.SeederDoneFlagFile(info.Name, chrootDir)
	if err != nil {
		return err
	}

	// Check if already done.
	done, err := sd.IsSeederDone(info.Name, chrootDir)
	if err != nil {
		return err
	}

	if done {
		sd.Print(
			"[%s] Already marked as done via %s. Skipping.\n",
			info.Name, flagFile,
		)
		return nil
	}

	// Execute prepper.
	sd.Print(
		"[%s] Executing prepper %s ...\n",
		info.Name, info.PrepperExec,
	)
	prepOpts := &seeder.PrepperOptions{
		ChrootDir:  chrootDir,
		Resume:     c.resume,
		Stage3File: c.stage3File,
	}
	if err := sd.ExecutePrepper(
		info, params, prepOpts,
	); err != nil {
		return fmt.Errorf(
			"[%s] prepper failed: %w", info.Name, err,
		)
	}

	// Setup mounts.
	opts := seeder.SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}
	if err := sd.SetupChrootMounts(opts); err != nil {
		return fmt.Errorf(
			"[%s] mount setup failed: %w", info.Name, err,
		)
	}

	// Setup DNS.
	if err := sd.SetupChrootDNS(chrootDir); err != nil {
		return fmt.Errorf(
			"[%s] DNS setup failed: %w", info.Name, err,
		)
	}

	// Setup chroot dirs.
	if err := sd.SetupChrootDirs(chrootDir); err != nil {
		return fmt.Errorf(
			"[%s] dir setup failed: %w", info.Name, err,
		)
	}

	// Execute seeder inside chroot.
	sd.Print(
		"[%s] Running seeder inside %s ...\n",
		info.Name, chrootDir,
	)
	if err := sd.Seed(chrootDir, info); err != nil {
		return fmt.Errorf(
			"[%s] chroot execution failed: %w", info.Name, err,
		)
	}

	// Mark done.
	sd.Print("[%s] Flagging %s as complete ...\n",
		info.Name, chrootDir,
	)
	if err := sd.MarkSeederDone(info.Name, chrootDir); err != nil {
		return err
	}

	// Record results to output files.
	if err := c.recordResults(info.Name, chrootDir); err != nil {
		return fmt.Errorf(
			"[%s] failed to record results: %w", info.Name, err,
		)
	}

	sd.Print("[%s] SUCCESS: Build complete.\n", info.Name)
	return nil
}

// --- Helper methods ---

// skipFilter returns a SeederFilterFunc that skips seeders present in
// --skip-seeders.  Returns nil when no skip list is configured.
func (c *SeedsCommand) skipFilter() seeder.SeederFilterFunc {
	if len(c.skipSeeders) == 0 {
		return nil
	}
	set := make(map[string]bool, len(c.skipSeeders))
	for _, s := range c.skipSeeders {
		set[s] = true
	}
	return func(name string) bool { return set[name] }
}

// onlyFilter returns a SeederFilterFunc that accepts only seeders in
// --only-seeders.  Returns nil when no allow-list is configured
// (all seeders pass).
func (c *SeedsCommand) onlyFilter() seeder.SeederFilterFunc {
	if len(c.onlySeeders) == 0 {
		return nil
	}
	set := make(map[string]bool, len(c.onlySeeders))
	for _, s := range c.onlySeeders {
		set[s] = true
	}
	return func(name string) bool { return set[name] }
}

// initOutputFiles truncates the built-rootfs and built-seeders output
// files if the corresponding flags were provided.
func (c *SeedsCommand) initOutputFiles() error {
	if c.builtRootfsFile != "" {
		c.Printf(
			"Writing built chroots to %s ...\n",
			c.builtRootfsFile,
		)
		if err := os.WriteFile(
			c.builtRootfsFile, []byte{}, 0644,
		); err != nil {
			return fmt.Errorf(
				"failed to init %s: %w",
				c.builtRootfsFile, err,
			)
		}
	}
	if c.builtSeedersFile != "" {
		c.Printf(
			"Writing built seeders to %s ...\n",
			c.builtSeedersFile,
		)
		if err := os.WriteFile(
			c.builtSeedersFile, []byte{}, 0644,
		); err != nil {
			return fmt.Errorf(
				"failed to init %s: %w",
				c.builtSeedersFile, err,
			)
		}
	}
	return nil
}

// recordBuiltRootfsFile appends the given chrootDir to the built-rootfs
// output file if the corresponding flag was provided.
func (c *SeedsCommand) recordBuiltRootfsFile(chrootDir string) error {
	if c.builtRootfsFile == "" {
		return nil
	}

	f, err := os.OpenFile(
		c.builtRootfsFile,
		os.O_APPEND|os.O_WRONLY, 0644,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, chrootDir)
	return err
}

func (c *SeedsCommand) recordBuiltSeedersFile(seederName string) error {
	if c.builtSeedersFile == "" {
		return nil
	}

	f, err := os.OpenFile(
		c.builtSeedersFile,
		os.O_APPEND|os.O_WRONLY, 0644,
	)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, seederName)
	return err
}

// recordResults appends the seeder name and chroot dir to the output
// files if the corresponding flags were provided.
func (c *SeedsCommand) recordResults(seederName, chrootDir string) error {
	if err := c.recordBuiltRootfsFile(chrootDir); err != nil {
		return fmt.Errorf("failed to record built rootfs: %w", err)
	}

	if err := c.recordBuiltSeedersFile(seederName); err != nil {
		return fmt.Errorf("failed to record built seeder: %w", err)
	}
	return nil
}
