package commands

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/releaser"
	"matrixos/vector/lib/seeder"
	"matrixos/vector/lib/validation"
)

// ReleasesCommand orchestrates the release workflow across all detected
// seeders — detecting, resolving branches, and committing each seeder's
// chroot filesystem into the ostree repository.  It is the Go port of
// release/release.seeds, wrapping the single-ref ReleaseCommand logic:
// it scans for available seeders, filters them with --skip-seeders /
// --only-seeders, and sequentially releases each seeder under an
// exclusive lock.
type ReleasesCommand struct {
	BaseCommand
	UI
	SignalGuard
	fs *flag.FlagSet

	// Library instances
	sd  seeder.ISeeder
	det seeder.ISeederDetector
	qa  *validation.QA

	// Styled I/O writers
	stdout *styledWriter
	stderr *styledWriter

	// Flags
	releaseStage      string
	skipSeedersRaw    string
	onlySeedersRaw    string
	builtReleasesFile string
	verbose           bool

	// Parsed from flags
	skipSeeders []string
	onlySeeders []string
}

// NewReleasesCommand creates a new ReleasesCommand.
func NewReleasesCommand() *ReleasesCommand {
	return &ReleasesCommand{}
}

func (c *ReleasesCommand) Name() string {
	return "releases"
}

func (c *ReleasesCommand) Init(args []string) error {
	if err := c.parseArgs(args); err != nil {
		return err
	}
	if err := c.initBaseConfig(); err != nil {
		return err
	}
	if err := c.initOstree(); err != nil {
		return err
	}

	qa, err := validation.New(c.cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize QA: %w", err)
	}
	c.qa = qa

	sd, err := seeder.NewSeeder(
		c.cfg, &seeder.NewSeederOptions{Verbose: c.verbose},
	)
	if err != nil {
		return fmt.Errorf("failed to initialize seeder: %w", err)
	}
	c.sd = sd

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

// SetStdout creates a styled stdout writer for the releases command.
func (c *ReleasesCommand) SetStdout(label string) *styledWriter {
	c.stdout = c.NewStdoutWriter(
		fmt.Sprintf("releases:%s", label),
	)
	return c.stdout
}

// SetStderr creates a styled stderr writer for the releases command.
func (c *ReleasesCommand) SetStderr(label string) *styledWriter {
	c.stderr = c.NewStderrWriter(
		fmt.Sprintf("releases:%s", label),
	)
	return c.stderr
}

// parseArgs parses command-line arguments without initializing config.
func (c *ReleasesCommand) parseArgs(args []string) error {
	c.fs = flag.NewFlagSet("releases", flag.ContinueOnError)

	c.fs.StringVar(&c.releaseStage, "release-stage", "dev",
		"Release stage: dev or prod")
	c.fs.StringVar(&c.skipSeedersRaw, "skip-seeders", "",
		"Comma-separated list of seeders to skip (by name)")
	c.fs.StringVar(&c.onlySeedersRaw, "only-seeders", "",
		"Comma-separated allow-list of seeders to accept (by name)")
	c.fs.StringVar(&c.builtReleasesFile, "built-releases-file", "",
		"Path to a file where successfully built release branches will be written")
	c.fs.BoolVar(&c.verbose, "verbose", false, "Show detailed output")

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

	// Validate release stage.
	if _, err := releaser.ValidateReleaseStage(c.releaseStage); err != nil {
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
func (c *ReleasesCommand) Run() error {
	return c.RunWithGuard(c.runReleases)
}

// updateStdWriters updates the stdout and stderr writers with the given
// label and propagates them to the seeder library.
func (c *ReleasesCommand) updateStdWriters(name string) (*styledWriter, *styledWriter) {
	stdoutWriter := c.SetStdout(name)
	stderrWriter := c.SetStderr(name)
	c.sd.SetStdout(stdoutWriter)
	c.sd.SetStderr(stderrWriter)
	return stdoutWriter, stderrWriter
}

// runReleases implements the multi-seeder release workflow, mirroring
// the bash release.seeds main().
func (c *ReleasesCommand) runReleases() error {
	writerSetup := func() {
		stdoutWriter, stderrWriter := c.updateStdWriters("main")
		c.PushCleanup(func() {
			stdoutWriter.Flush()
			stderrWriter.Flush()
		})
	}
	writerSetup()

	// Verify releaser environment.
	if err := c.qa.VerifyReleaserEnvironmentSetup("/"); err != nil {
		return fmt.Errorf("environment verification failed: %w", err)
	}

	// Detect seeders.
	seeders, err := c.det.Detect(
		c.skipFilter(), c.onlyFilter(),
	)
	if err != nil {
		return fmt.Errorf("seeder detection failed: %w", err)
	}
	if len(seeders) == 0 {
		return fmt.Errorf("no seeders found, nothing to do")
	}

	// Initialize built-releases output file.
	if err := c.initBuiltReleasesFile(); err != nil {
		return err
	}

	c.sd.Print("Selected release stage: %s\n", c.releaseStage)
	c.sd.Print(
		"Will release seeds in the following order:\n",
	)
	for _, s := range seeders {
		c.sd.Print("  %s\n", s.Name)
	}

	// Release each seeder under its release lock.
	var released []string
	for _, info := range seeders {
		branch, err := c.releaseWorker(info)
		if err != nil {
			writerSetup()
			return fmt.Errorf(
				"seeder %s release failed: %w", info.Name, err,
			)
		}
		released = append(released, branch)
	}

	writerSetup()
	c.sd.Print("SUCCESS: All builds released to ostree.\n")
	for _, b := range released {
		c.sd.Print("  %s\n", b)
	}

	return nil
}

// releaseWorker processes a single seeder under an exclusive release lock:
// 1. Parses seeder params to find the chroot directory.
// 2. Computes the ostree branch from the seeder name and release stage.
// 3. Computes the image directory from the chroot directory.
// 4. Creates a Releaser and runs the full release pipeline.
// 5. Records the released branch.
// Returns the released branch name.
func (c *ReleasesCommand) releaseWorker(info seeder.SeederInfo) (string, error) {
	seederName := info.Name
	// The branch short name is the seeder name without the numeric order prefix
	// (e.g. "00-bedrock" → "bedrock").
	branchShortname := seeder.SeederNameWithoutOrderPrefix(seederName)

	stdoutWriter, stderrWriter := c.updateStdWriters(seederName)
	c.PushCleanup(func() {
		stdoutWriter.Flush()
		stderrWriter.Flush()
	})

	c.sd.Print(
		"Working on seeder %s, ostree branch short name: %s\n",
		seederName, branchShortname,
	)

	// Compute the full ostree branch name from the short name.
	osName, err := c.ot.OsName()
	if err != nil {
		return "", fmt.Errorf("failed to get OS name: %w", err)
	}
	arch, err := c.ot.Arch()
	if err != nil {
		return "", fmt.Errorf("failed to get arch: %w", err)
	}
	branch, err := ostree.BranchShortnameToNormal(
		c.releaseStage, branchShortname, osName, arch,
	)
	if err != nil {
		return "", fmt.Errorf(
			"unable to find ostree branch for %s: %w",
			branchShortname, err,
		)
	}

	c.sd.Print("Determined ostree branch to be: %s\n", branch)

	// Set up release-specific styled writers.
	relStdout := c.NewStdoutWriter(
		fmt.Sprintf("release:%s", c.shortRef(branch)),
	)
	relStderr := c.NewStderrWriter(
		fmt.Sprintf("release:%s", c.shortRef(branch)),
	)

	// Set up ostree for this branch.
	c.ot.SetStdout(relStdout)
	c.ot.SetStderr(relStderr)
	c.ot.SetVerbose(false)
	c.ot.SetRef(branch)

	// Create the releaser instance.
	opts := &releaser.NewReleaserOptions{
		Ref:     branch,
		Verbose: c.verbose,
	}
	rel, err := releaser.NewReleaser(c.cfg, c.ot, opts)
	if err != nil {
		return "", fmt.Errorf(
			"failed to initialize releaser: %w", err,
		)
	}
	rel.SetStdout(relStdout)
	rel.SetStderr(relStderr)

	// Run the entire release pipeline under an exclusive release lock
	// (mirrors bash release_lib.execute_with_release_lock).
	err = rel.ExecuteWithReleaseLock(func() error {
		c.PushCleanup(func() {
			rel.Cleanup()
			relStdout.Flush()
			relStderr.Flush()
		})

		// Locate the chroot directory from seeder params.
		chrootDir, err := c.findChrootDir(info)
		if err != nil {
			return err
		}
		c.sd.Print(
			"Selected chroot dir: %s for seeder: %s\n",
			chrootDir, seederName,
		)
		rel.SetChrootDir(chrootDir)

		// Compute image directory.
		imageDir := chrootDirForImageDir(chrootDir)
		if err := os.MkdirAll(imageDir, 0755); err != nil {
			return fmt.Errorf(
				"failed to create image dir %s: %w", imageDir, err,
			)
		}
		rel.SetImageDir(imageDir)

		// TODO: check if this is really needed.
		// Transfer private git repo into image dir.
		// if err := c.transferMatrixOSPrivate(imageDir); err != nil {
		//	return fmt.Errorf(
		//		"failed to transfer private repo: %w", err,
		//	)
		// }

		if err := rel.Build(); err != nil {
			return err
		}

		rel.Print(
			"Released filesystem to ostree as branch: %s.\n",
			branch,
		)

		// Record the released branch.
		if err := c.recordBuiltRelease(branch); err != nil {
			return fmt.Errorf(
				"failed to record built release: %w", err,
			)
		}

		return nil
	})
	return branch, err
}

// --- Helper methods ---

// findChrootDir locates the chroot directory for a seeder by parsing
// its params.sh file.
func (c *ReleasesCommand) findChrootDir(info seeder.SeederInfo) (string, error) {
	paramsName, err := c.sd.ParamsExecutableName()
	if err != nil {
		return "", err
	}
	paramsPath := filepath.Join(info.Dir, paramsName)
	if !filesystems.FileExists(paramsPath) {
		return "", fmt.Errorf("unable to find %s", paramsPath)
	}

	params, err := c.sd.ParseSeederParams(paramsPath)
	if err != nil {
		return "", fmt.Errorf("failed to parse params: %w", err)
	}

	chrootDir := params.PreferredChrootDir
	if chrootDir == "" {
		return "", fmt.Errorf(
			"no chroot dir specified in params.sh for seeder %s",
			info.Name,
		)
	}
	if !filesystems.DirectoryExists(chrootDir) {
		return "", fmt.Errorf(
			"unable to find chroot dir: %s", chrootDir,
		)
	}
	return chrootDir, nil
}

// chrootDirForImageDir computes the image directory path from a chroot
// directory, mirroring the bash chroot_dir_for_image_dir function.
func chrootDirForImageDir(chrootDir string) string {
	return chrootDir + ".ostree_rootfs"
}

// transferMatrixOSPrivate copies the private git repo into the image
// directory.  Mirrors the bash transfer_matrixos_private function.
func (c *ReleasesCommand) transferMatrixOSPrivate(imageDir string) error {
	if imageDir == "" {
		return fmt.Errorf(
			"missing parameter to transferMatrixOSPrivate",
		)
	}

	srcRepo, err := c.sd.PrivateGitRepoPath()
	if err != nil {
		return fmt.Errorf("failed to get private git repo path: %w", err)
	}
	defaultPath, err := c.sd.DefaultPrivateGitRepoPath()
	if err != nil {
		return fmt.Errorf(
			"failed to get default private git repo path: %w",
			err,
		)
	}
	dstRepo := filepath.Join(imageDir, defaultPath)

	c.sd.Print(
		"Copying %s to %s ... (will be removed before commit)\n",
		srcRepo, dstRepo,
	)

	dstParent := filepath.Dir(dstRepo)
	if err := os.MkdirAll(dstParent, 0755); err != nil {
		return fmt.Errorf(
			"failed to create destination dir %s: %w",
			dstParent, err,
		)
	}

	return filesystems.RsyncCopy(filesystems.RsyncCopyOptions{
		Src:      srcRepo,
		Dst:      dstParent,
		Excludes: []string{".git"},
		Verbose:  c.verbose,
		Stdout:   c.sd.Stdout(),
		Stderr:   c.sd.Stderr(),
	})
}

// skipFilter returns a SeederFilterFunc that skips seeders present in
// --skip-seeders.  Returns nil when no skip list is configured.
func (c *ReleasesCommand) skipFilter() seeder.SeederFilterFunc {
	return makeSkipFilter(c.skipSeeders)
}

// onlyFilter returns a SeederFilterFunc that accepts only seeders in
// --only-seeders.  Returns nil when no allow-list is configured
// (all seeders pass).
func (c *ReleasesCommand) onlyFilter() seeder.SeederFilterFunc {
	return makeOnlyFilter(c.onlySeeders)
}

// initBuiltReleasesFile truncates the built-releases output file if the
// flag was provided.
func (c *ReleasesCommand) initBuiltReleasesFile() error {
	if c.builtReleasesFile == "" {
		return nil
	}
	c.sd.Print(
		"Marking freshly built releases into %s ...\n",
		c.builtReleasesFile,
	)
	if err := os.WriteFile(
		c.builtReleasesFile, []byte{}, 0644,
	); err != nil {
		return fmt.Errorf(
			"failed to init %s: %w",
			c.builtReleasesFile, err,
		)
	}
	return nil
}

// recordBuiltRelease appends the given branch to the built-releases
// output file if the corresponding flag was provided.
func (c *ReleasesCommand) recordBuiltRelease(branch string) error {
	if c.builtReleasesFile == "" {
		return nil
	}
	f, err := os.OpenFile(
		c.builtReleasesFile,
		os.O_APPEND|os.O_WRONLY, 0644,
	)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = fmt.Fprintln(f, branch)
	return err
}
