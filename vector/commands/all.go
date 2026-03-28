package commands

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"matrixos/vector/lib/filesystems"
)

// allLockTimeout is the maximum time to wait for the build lock.
var allLockTimeout = 600 * time.Second

// allMailCommand is the mail program used to send build-result emails.
var allMailCommand = "mutt"

// AllCommand orchestrates a full build-and-release cycle:
// seeds → releases → images → janitor → CDN push.
type AllCommand struct {
	BaseCommand
	UI
	SignalGuard
	fs *flag.FlagSet

	// Flags
	forceRelease   bool
	onlyImages     bool
	forceImages    bool
	skipImages     bool
	onBuildServer  bool
	resumeSeeders  bool
	buildName      string
	buildID        string
	skipSeedersRaw string
	onlySeedersRaw string
	disableJanitor bool
	disableMail    bool
	mailUser       string
	cdnPusher      string
	verbose        bool

	// Internal state
	logFile string
}

// NewAllCommand creates a new AllCommand.
func NewAllCommand() *AllCommand {
	return &AllCommand{}
}

func (c *AllCommand) Name() string {
	return "all"
}

func (c *AllCommand) Init(args []string) error {
	if err := c.parseArgs(args); err != nil {
		return err
	}

	if err := c.initBaseConfig(); err != nil {
		return err
	}

	c.StartUI()
	return nil
}

func (c *AllCommand) parseArgs(args []string) error {
	c.fs = flag.NewFlagSet("all", flag.ContinueOnError)

	c.fs.BoolVar(&c.forceRelease, "force-release", false,
		"Force the re-release of the latest built seeds")
	c.fs.BoolVar(&c.onlyImages, "only-images", false,
		"Generate images from the last committed branches, "+
			"skipping seeder and releaser")
	c.fs.BoolVar(&c.forceImages, "force-images", false,
		"Force images creation for all branches after the seeder and releaser")
	c.fs.BoolVar(&c.skipImages, "skip-images", false,
		"Skip images generation for all branches after the seeder and releaser")
	c.fs.BoolVar(&c.onBuildServer, "on-build-server", false,
		"Optimize execution if seeding, release and imaging "+
			"happens on the same machine")
	c.fs.BoolVar(&c.resumeSeeders, "resume", false,
		"Allow seeder to resume seeds (chroots) build from a checkpoint")
	c.fs.StringVar(&c.buildName, "build-name", "matrixOS weekly",
		"Name of the build")
	c.fs.StringVar(&c.buildID, "build-id", "weekly",
		"ID of the build")
	c.fs.StringVar(&c.skipSeedersRaw, "skip-seeders", "",
		"Comma-separated list of seeders to skip (by name)")
	c.fs.StringVar(&c.onlySeedersRaw, "only-seeders", "",
		"Comma-separated allow-list of seeders to accept (by name)")
	c.fs.BoolVar(&c.disableJanitor, "disable-janitor", false,
		"Disable old artifacts cleanup at the end of the build")
	c.fs.BoolVar(&c.disableMail, "disable-send-mail", false,
		"Disable email sending at the end of the build")
	c.fs.StringVar(&c.mailUser, "mail-user", "root",
		"Recipient for build-result emails")
	c.fs.StringVar(&c.cdnPusher, "cdn-pusher", "",
		"Path to an executable that pushes generated artifacts to a CDN")
	c.fs.BoolVar(&c.verbose, "verbose", false, "Show detailed output")

	c.fs.Usage = func() {
		fmt.Printf("Usage: vector build %s [options]\n", c.Name())
		fmt.Println("\nOptions:")
		c.fs.PrintDefaults()
	}
	if err := c.fs.Parse(args); err != nil {
		return err
	}

	if getEuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	if c.buildID == "" {
		return fmt.Errorf("build ID cannot be empty")
	}

	return nil
}

func (c *AllCommand) Run() error {
	return c.RunWithGuard(c.run)
}

func (c *AllCommand) run() error {
	c.SetupPrinters("build:all")
	defer c.FlushPrinters()

	// Acquire build lock.
	unlock, err := c.acquireBuildLock()
	if err != nil {
		return err
	}
	defer unlock()

	// Set up log file.
	logF, err := c.setupLogFile()
	if err != nil {
		return err
	}
	defer logF.Close()

	c.Printf("Logfile at: %s\n", c.logFile)

	// Run the pipeline; send mail on exit.
	runErr := c.runPipeline()

	c.sendMail(runErr)
	return runErr
}

// runPipeline executes the seeds → releases → images → janitor → CDN pipeline.
func (c *AllCommand) runPipeline() error {
	var builtReleases []string

	if !c.onlyImages {
		// --- Seeds ---
		c.Printf("Building new seeds ...\n")
		builtSeeders, err := c.runSeeds()
		if err != nil {
			return fmt.Errorf("seeds build failed: %w", err)
		}

		for _, s := range builtSeeders {
			c.Printf("Seeder built: %s\n", s)
		}

		// --- Releases ---
		var onlySeeders []string
		if len(builtSeeders) > 0 {
			onlySeeders = builtSeeders
			c.Printf(
				"Releasing only for freshly built seeders: %s ...\n",
				strings.Join(builtSeeders, ","))
		} else if c.forceRelease {
			c.Printf(
				"Forcing releases and new images via --force-release.\n")
		} else {
			c.Printf("Nothing to release. Yay.\n")
			return nil
		}

		c.Printf("Releasing newly built seeds ...\n")
		releases, err := c.runReleases(onlySeeders)
		if err != nil {
			return fmt.Errorf("releases failed: %w", err)
		}

		c.Printf("Creating images for the new releases ...\n")

		if c.onBuildServer {
			c.Printf(
				"Executing on a build server, " +
					"stripping remote names ...\n")
			for i, r := range releases {
				if idx := strings.Index(r, ":"); idx >= 0 {
					releases[i] = r[idx+1:]
				}
			}
		}

		for _, r := range releases {
			c.Printf("Built release: %s\n", r)
		}
		builtReleases = releases
	} else {
		c.Printf("Forcing new images only via --only-images ...\n")
	}

	// --- Images ---
	executeImager := false
	var onlyReleases []string

	if c.skipImages {
		c.Printf("Skipping images creation via --skip-images.\n")
	} else if len(builtReleases) > 0 {
		onlyReleases = builtReleases
		c.Printf(
			"Creating new images only for freshly built releases: %s ...\n",
			strings.Join(builtReleases, ","))
		executeImager = true
	} else if c.forceImages {
		c.Printf("Forcing new images via --force-images.\n")
		executeImager = true
	} else if c.onlyImages {
		c.Printf("Creating only images (all) via --only-images.\n")
		executeImager = true
	} else {
		c.Printf("No images to release. Yay?\n")
	}

	if executeImager {
		if err := c.runImages(onlyReleases); err != nil {
			return fmt.Errorf("images build failed: %w", err)
		}
	}

	// --- Janitor ---
	if !c.disableJanitor {
		c.Printf("Running janitor clean ups ...\n")
		if err := c.runJanitor(); err != nil {
			return fmt.Errorf("janitor failed: %w", err)
		}
	}

	// --- CDN Pusher ---
	if err := c.runCDNPusher(builtReleases, executeImager); err != nil {
		return err
	}

	return nil
}

// runSeeds builds all seeds and returns the names
// of successfully built seeders.
func (c *AllCommand) runSeeds() ([]string, error) {
	args := []string{"--verbose"}
	args = append(args, c.seederFilterArgs()...)
	if c.resumeSeeders {
		args = append(args, "--resume")
	}

	cmd := NewSeedsCommand()
	if err := cmd.Init(args); err != nil {
		return nil, err
	}
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return cmd.BuiltSeeders, nil
}

// runReleases releases seeders and returns the built
// release branches. If onlySeeders is non-empty, only
// those seeders are released.
func (c *AllCommand) runReleases(onlySeeders []string) ([]string, error) {
	args := []string{"--verbose"}
	args = append(args, c.seederFilterArgs()...)
	if len(onlySeeders) > 0 {
		args = append(args,
			"--only-seeders="+strings.Join(onlySeeders, ","))
	}

	cmd := NewReleasesCommand()
	if err := cmd.Init(args); err != nil {
		return nil, err
	}
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return cmd.BuiltReleases, nil
}

// runImages builds images for the given release branches.
// If onlyReleases is non-empty, only those branches are imaged.
func (c *AllCommand) runImages(onlyReleases []string) error {
	args := []string{"--local-ostree"}
	if len(onlyReleases) > 0 {
		args = append(args,
			"--only-releases="+strings.Join(onlyReleases, ","))
	}

	cmd := NewImagesCommand()
	if err := cmd.Init(args); err != nil {
		return err
	}
	return cmd.Run()
}

// runJanitor runs artifact cleanup.
func (c *AllCommand) runJanitor() error {
	cmd := NewJanitorCommand()
	if err := cmd.Init(nil); err != nil {
		return err
	}
	return cmd.Run()
}

// seederFilterArgs returns the --skip-seeders and --only-seeders flags
// forwarded to both the seeder and releaser sub-processes.
func (c *AllCommand) seederFilterArgs() []string {
	var args []string
	if c.skipSeedersRaw != "" {
		args = append(args, "--skip-seeders="+c.skipSeedersRaw)
	}
	if c.onlySeedersRaw != "" {
		args = append(args, "--only-seeders="+c.onlySeedersRaw)
	}
	return args
}

// sendMail sends a build-result email via mutt if available and enabled.
func (c *AllCommand) sendMail(buildErr error) {
	if c.disableMail {
		c.Printf("Not sending an email with the build results.\n")
		return
	}

	muttExec, err := exec.LookPath(allMailCommand)
	if err != nil {
		c.PrintErrf("mutt not installed, not emailing build status.\n")
		return
	}

	status := "SUCCESSFUL"
	if buildErr != nil {
		status = "FAILED"
	}
	subject := fmt.Sprintf(
		"[%s] %s execution at %s",
		c.buildName,
		status,
		time.Now().Format("20060102"),
	)

	muttArgs := []string{"-s", subject}
	if c.logFile != "" && filesystems.FileExists(c.logFile) {
		muttArgs = append(muttArgs, "-a", c.logFile)
	}
	muttArgs = append(muttArgs, "--", c.mailUser)

	cmd := execCommand(muttExec, muttArgs...)
	cmd.Stdin = strings.NewReader("")
	cmd.Stdout = c.StdoutWriter()
	cmd.Stderr = c.StderrWriter()
	if err := cmd.Run(); err != nil {
		c.PrintErrf("Failed to send mail: %v\n", err)
	}
}

// configItem returns the config value for key, returning an
// error if the lookup fails or the value is empty.
func (c *AllCommand) configItem(key string) (string, error) {
	v, err := c.cfg.GetItem(key)
	if err != nil {
		return "", fmt.Errorf(
			"failed to get %s: %w", key, err)
	}
	if v == "" {
		return "", fmt.Errorf("%s is not set", key)
	}
	return v, nil
}

// acquireBuildLock creates the locks directory and acquires an
// exclusive file lock. The caller must defer the returned unlock
// function.
func (c *AllCommand) acquireBuildLock() (func(), error) {
	baseLocksDir, err := c.configItem("matrixOS.LocksDir")
	if err != nil {
		return nil, err
	}
	locksDir := filepath.Join(
		baseLocksDir, c.buildID+"-builder")
	if err := os.MkdirAll(locksDir, 0755); err != nil {
		return nil, fmt.Errorf(
			"failed to create locks dir: %w", err)
	}
	lockFile := filepath.Join(
		locksDir, c.buildID+"-builder.lock")
	unlock, err := filesystems.AcquireFileLock(
		lockFile, allLockTimeout)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to acquire build lock "+
				"(another builder running?): %w", err)
	}
	return unlock, nil
}

// setupLogFile creates the log directory and opens a timestamped
// log file. The caller must close the returned file.
func (c *AllCommand) setupLogFile() (*os.File, error) {
	baseLogDir, err := c.configItem("matrixOS.LogsDir")
	if err != nil {
		return nil, err
	}
	logDir := filepath.Join(
		baseLogDir, c.buildID+"-builder")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf(
			"failed to create log dir: %w", err)
	}
	c.logFile = filepath.Join(logDir,
		fmt.Sprintf("build-%s.log",
			time.Now().Format("20060102-150405")))

	logF, err := os.Create(c.logFile)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to create log file: %w", err)
	}
	return logF, nil
}

// runCDNPusher executes the CDN pusher script if configured.
func (c *AllCommand) runCDNPusher(
	builtReleases []string, imagesBuilt bool,
) error {
	if c.cdnPusher == "" {
		return nil
	}

	if !filesystems.FileExists(c.cdnPusher) {
		return fmt.Errorf("cdn-pusher %s: file not found", c.cdnPusher)
	}
	info, err := os.Stat(c.cdnPusher)
	if err != nil {
		return fmt.Errorf("cdn-pusher %s: %w", c.cdnPusher, err)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf(
			"ERROR: unable to push to CDN. %s not executable",
			c.cdnPusher)
	}

	imagesFlag := "0"
	if imagesBuilt {
		imagesFlag = "1"
	}

	cmd := execCommand(c.cdnPusher)
	cmd.Env = append(os.Environ(),
		"MATRIXOS_BUILT_RELEASES="+strings.Join(builtReleases, " "),
		"MATRIXOS_BUILT_IMAGES="+imagesFlag,
	)
	cmd.Stdout = c.StdoutWriter()
	cmd.Stderr = c.StderrWriter()
	return cmd.Run()
}
