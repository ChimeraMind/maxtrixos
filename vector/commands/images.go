package commands

import (
	"flag"
	"fmt"
	"io"

	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/imager"
	"matrixos/vector/lib/releaser"
	"matrixos/vector/lib/validation"
)

// ImagesCommand is a command for building matrixOS images for all detected
// ostree refs.  It is the Go counterpart of the image.releases bash script,
// wrapping the single-ref ImageCommand logic: it scans for available refs
// (local or remote), filters them with --skip-releases / --only-releases,
// and sequentially builds an image for each ref under an exclusive lock.
type ImagesCommand struct {
	BaseCommand
	UI
	SignalGuard
	fs *flag.FlagSet

	// Library instances
	qa *validation.QA

	// Styled I/O writers
	stdout *styledWriter
	stderr *styledWriter

	// Flags
	localOstree         bool
	includeFullBranches bool
	verbose             bool
	skipReleasesRaw     string
	onlyReleasesRaw     string

	// Parsed filter lists
	skipReleases []string
	onlyReleases []string
}

// NewImagesCommand creates a new ImagesCommand
func NewImagesCommand() *ImagesCommand {
	return &ImagesCommand{}
}

func (c *ImagesCommand) Name() string {
	return "images"
}

func (c *ImagesCommand) Init(args []string) error {
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

	c.StartUI()

	return nil
}

// SetStdout creates a fancy styled stdout writer using the UI theme.
func (c *ImagesCommand) SetStdout(label string) *styledWriter {
	c.stdout = c.NewStdoutWriter(fmt.Sprintf("images:%s", label))
	return c.stdout
}

// SetStderr creates a fancy styled stderr writer using the UI theme.
func (c *ImagesCommand) SetStderr(label string) *styledWriter {
	c.stderr = c.NewStderrWriter(fmt.Sprintf("images:%s", label))
	return c.stderr
}

// parseArgs parses the command-line arguments without initializing config or ostree.
func (c *ImagesCommand) parseArgs(args []string) error {
	c.fs = flag.NewFlagSet("images", flag.ContinueOnError)

	c.fs.BoolVar(&c.localOstree, "local-ostree", false,
		"Use the local, already pulled ostree repo instead of fetching from remote")
	c.fs.BoolVar(&c.includeFullBranches, "include-full-branches", false,
		"Include *-full branches in the image creation process")
	c.fs.StringVar(&c.skipReleasesRaw, "skip-releases", "",
		"Comma-separated list of release branches to skip")
	c.fs.StringVar(&c.onlyReleasesRaw, "only-releases", "",
		"Comma-separated allow-list of release branches to accept")
	c.fs.BoolVar(&c.verbose, "verbose", false, "Show detailed output")

	c.fs.Usage = func() {
		fmt.Printf("Usage: vector build %s [options]\n", c.Name())
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

	// Parse the comma-separated filter lists.
	c.skipReleases = SplitCSV(c.skipReleasesRaw)
	c.onlyReleases = SplitCSV(c.onlyReleasesRaw)

	return nil
}

// Run uses the SignalGuard to ensure cleanup functions are executed even
// when the process is terminated by SIGINT or SIGTERM.
func (c *ImagesCommand) Run() error {
	return c.RunWithGuard(c.runImages)
}

// detectReleases detects available OS releases using either local or
// remote listing, then applies the skip/only filters.
func (c *ImagesCommand) detectReleases(w io.Writer) ([]string, error) {
	det, err := releaser.NewReleaseDetector(c.ot)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize release detector: %w", err)
	}
	det.SetStderr(w)

	var refs []string
	if c.localOstree {
		refs, err = det.DetectLocalReleases(c.skipFilter(), c.onlyFilter())
	} else {
		refs, err = det.DetectRemoteReleases(c.skipFilter(), c.onlyFilter())
	}

	return refs, err
}

// skipFilter returns a RefFilterFunc that skips branches present in the
// --skip-releases list.  Returns nil when no skip list is configured.
func (c *ImagesCommand) skipFilter() releaser.RefFilterFunc {
	if len(c.skipReleases) == 0 {
		return nil
	}
	set := make(map[string]bool, len(c.skipReleases))
	for _, s := range c.skipReleases {
		set[s] = true
	}
	return func(ref string) bool {
		return set[ref]
	}
}

// onlyFilter returns a RefFilterFunc that accepts only branches present
// in the --only-releases list.  Returns nil when no only-list is
// configured (i.e. all branches pass).
func (c *ImagesCommand) onlyFilter() releaser.RefFilterFunc {
	if len(c.onlyReleases) == 0 {
		return nil
	}
	set := make(map[string]bool, len(c.onlyReleases))
	for _, s := range c.onlyReleases {
		set[s] = true
	}
	return func(ref string) bool {
		return set[ref]
	}
}

// runImages implements the main images building logic, mirroring
// image.releases main().
func (c *ImagesCommand) runImages() error {
	// Set up styled writers for top-level output.
	stdoutWriter := c.SetStdout("all")
	stderrWriter := c.SetStderr("all")

	c.ot.SetStdout(stdoutWriter)
	c.ot.SetStderr(stderrWriter)
	c.ot.SetVerbose(false)

	// Verify imager environment.
	if err := c.qa.VerifyImagerEnvironmentSetup("/"); err != nil {
		return fmt.Errorf("environment verification failed: %w", err)
	}

	// Detect available refs.
	refs, err := c.detectReleases(stderrWriter)
	if err != nil {
		return err
	}

	if len(refs) == 0 {
		fmt.Fprintln(stderrWriter, "No release refs found.")
		return fmt.Errorf("No release refs found, detected or surviving the filters.")
	}

	// Important note: here we have 3 cases.
	// 1) on build server. Branches have no remote: prefix.
	// 2) on "client server", user just building images away from
	//    the mothership (client side). Branches have remote:a/b/c (remote: prefix)
	// 3) on images only build server, where ostree repo is pulled from a remote.
	// Case 2 and 3 are fine, we use the remote prefix and happy days.
	// For case 1, we detect this from the absence of the remote: prefix and set remote="local".
	var released []string
	for _, ref := range refs {
		// Skip full-suffixed branches unless explicitly included.
		c.ot.SetRef(ref)
		isFull, err := c.ot.IsBranchFullSuffixed()
		if err != nil {
			return fmt.Errorf("failed to check full branch suffix for %s: %w", ref, err)
		}
		if isFull && !c.includeFullBranches {
			fmt.Fprintf(stdoutWriter,
				"Skipping full branch: %s (use --include-full-branches to include)\n", ref)
			continue
		}

		fmt.Fprintf(stdoutWriter, "Working on release branch: %s ...\n", ref)
		if err := c.imageWorker(ref); err != nil {
			return fmt.Errorf("image build failed for ref %s: %w", ref, err)
		}
		released = append(released, ref)
	}

	fmt.Fprintf(stdoutWriter, "Successfully built images for %d releases:\n",
		len(released))
	for _, r := range released {
		fmt.Fprintf(stdoutWriter, "  %s\n", r)
	}

	return nil
}

// imageWorker creates a per-ref imager instance, acquires an image lock,
// and delegates to the ImageCommand-style image building pipeline.
// This mirrors the bash `image_worker()` function in image.releases.
func (c *ImagesCommand) imageWorker(ref string) error {
	// Create per-ref styled writers.
	stdoutWriter := c.NewStdoutWriter(fmt.Sprintf("image:%s", c.shortRef(ref)))
	stderrWriter := c.NewStderrWriter(fmt.Sprintf("image:%s", c.shortRef(ref)))

	// Set up ostree for this ref.
	c.ot.SetStdout(stdoutWriter)
	c.ot.SetStderr(stderrWriter)
	c.ot.SetVerbose(false)

	// Create the fsenc instance.
	fsenc, err := filesystems.NewFsenc(
		c.cfg,
		func(mapperName string) {
			fmt.Fprintf(stdoutWriter, "Opening encrypted rootfs as %s ...\n", mapperName)
		},
		func(mapperName string) {
			fmt.Fprintf(stdoutWriter, "Closing encrypted rootfs as %s ...\n", mapperName)
		},
	)
	if err != nil {
		return fmt.Errorf("failed to initialize fsenc: %w", err)
	}

	// Validate LUKS variables.
	if err := fsenc.ValidateLuksVariables(); err != nil {
		return fmt.Errorf("LUKS validation failed: %w", err)
	}

	// Initialize the imager for this ref.
	opts := imager.NewImageOptions{}
	im, err := imager.NewImage(c.cfg, c.ot, fsenc, &opts)
	if err != nil {
		return fmt.Errorf("failed to initialize imager: %w", err)
	}
	im.SetStdout(stdoutWriter)
	im.SetStderr(stderrWriter)

	// Handle refs that contain the remote prefix (e.g. "origin:matrixos/...").
	rr, err := c.resolveRefRemote(ref, im.PrintWarning)
	if err != nil {
		return err
	}
	im.SetRef(rr.Ref)
	c.ot.SetRef(rr.Ref)

	// Fail fast on bad params.
	if err := failFastChecks(c.ot, im); err != nil {
		return err
	}

	buildOpts := &imager.BuildOptions{}

	// Execute the build under an exclusive image lock.
	return im.ExecuteWithImageLock(func() error {
		c.PushCleanup(im.Cleanup)
		c.PushCleanup(fsenc.Cleanup)
		c.PushCleanup(func() {
			stdoutWriter.Flush()
			stderrWriter.Flush()
		})
		defer c.RunCleanups()

		if err := c.initGpg(); err != nil {
			return err
		}

		// Initialize ostree for this ref.
		if c.localOstree {
			if err := c.showLocalRefs(im); err != nil {
				return err
			}
		} else {
			if err := c.initializeRemoteOstree(im); err != nil {
				return err
			}
		}

		return im.Build(buildOpts)
	})
}

// initializeRemoteOstree sets up the ostree remote and pulls
// the specified ref.  Mirrors image.go initializeRemoteOstree().
func (c *ImagesCommand) initializeRemoteOstree(im imager.IImage) error {
	remote, err := c.ot.Remote()
	if err != nil {
		return err
	}

	if err := c.showRemoteRefs(im); err != nil {
		return err
	}

	im.Print("\n%s%sPulling ostree ref %s:%s ...%s\n",
		c.cBold, c.iconDownload, remote, c.ot.Ref(), c.cReset)
	if err := c.ot.Pull(); err != nil {
		return fmt.Errorf("ostree pull failed: %w", err)
	}
	return nil
}

// showLocalRefs prints the local ostree refs to the provided printf function.
func (c *ImagesCommand) showLocalRefs(im imager.IImage) error {
	refs, err := c.ot.LocalRefs()
	if err != nil {
		return fmt.Errorf("failed to list local refs: %w", err)
	}
	im.Print("Local refs:\n")
	for _, r := range refs {
		im.Print("  %s\n", r)
	}
	return nil
}

// showRemoteRefs prints the remote ostree refs to the provided printf function.
func (c *ImagesCommand) showRemoteRefs(im imager.IImage) error {
	refs, err := c.ot.RemoteRefs()
	if err != nil {
		return fmt.Errorf("failed to list remote refs: %w", err)
	}
	im.Print("Remote refs:\n")
	for _, r := range refs {
		im.Print("  %s\n", r)
	}
	return nil
}
