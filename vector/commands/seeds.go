package commands

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"

	"matrixos/vector/lib/cgroups"
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

	// Mutex for concurrent access to results
	mu sync.Mutex

	// Results populated during Run().
	BuiltSeeders []string

	// cgroupRoot overrides the cgroup v2 mount point (for testing).
	cgroupRoot string
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

	// Determine parallelism level.
	parallelism, err := sd.Parallelism()
	if err != nil {
		return fmt.Errorf("failed to read parallelism config: %w", err)
	}
	if parallelism < 1 {
		c.PrintErrf(
			"WARNING: Seeder.Parallelism=%d is invalid, assuming 1.\n",
			parallelism,
		)
		parallelism = 1
	}

	if parallelism > 1 {
		c.Printf("Parallel mode: up to %d seeders at once.\n", parallelism)
	}

	maxMemGiB, err := sd.MaxMemoryGiB()
	if err != nil {
		return fmt.Errorf("failed to read max memory config: %w", err)
	}
	maxCPUs, err := sd.MaxCPUs()
	if err != nil {
		return fmt.Errorf("failed to read max CPUs config: %w", err)
	}

	coresMultiplier, err := sd.CoresMultiplier()
	if err != nil {
		return fmt.Errorf("failed to read cores multiplier config: %w", err)
	}

	if err := c.runSeedsParallel(seeders, &parallelOpts{
		parallelism:     parallelism,
		maxMemGiB:       maxMemGiB,
		maxCPUs:         maxCPUs,
		coresMultiplier: coresMultiplier,
	}); err != nil {
		writerSetup(sd)
		return err
	}

	writerSetup(sd)

	c.Printf("All done:\n")
	for _, info := range seeders {
		c.Printf("  [%s] %s done.\n", info.Name, info.Dir)
	}
	return nil
}

type buildSeedGraphOptions struct {
	seeders      []seeder.SeederInfo
	paramsByName map[string]*seeder.SeederParams
	infoByName   map[string]seeder.SeederInfo
}

// seedGraph holds the shared dependency-graph state for the worker pool.
type seedGraph struct {
	mu         sync.Mutex
	depCount   map[string]int
	dependents map[string][]string
	remaining  int
	stopped    bool
	ready      chan string
}

func (c *SeedsCommand) buildSeedGraph(opts *buildSeedGraphOptions) *seedGraph {
	// Build dependency graph: for each seeder, record which other
	// seeders in the current set it depends on.
	graph := &seedGraph{
		depCount:   make(map[string]int, len(opts.seeders)),
		dependents: make(map[string][]string),
		remaining:  len(opts.seeders),
		ready:      make(chan string, len(opts.seeders)),
	}
	for _, info := range opts.seeders {
		count := 0
		for _, dep := range opts.paramsByName[info.Name].Depends {
			if _, ok := opts.infoByName[dep]; ok {
				count++
				graph.dependents[dep] = append(graph.dependents[dep], info.Name)
			}
			// Dependencies not in the current set are assumed satisfied.
		}
		graph.depCount[info.Name] = count
	}

	// Seed the ready queue with seeders that have zero unsatisfied deps.
	for _, info := range opts.seeders {
		if graph.depCount[info.Name] == 0 {
			graph.ready <- info.Name
		}
	}
	return graph
}

type parallelOpts struct {
	parallelism     int
	maxMemGiB       int
	maxCPUs         int
	coresMultiplier float64
}

// runSeedsParallel builds seeders concurrently, respecting dependency
// order and the configured parallelism limit. Each seeder gets its own
// ISeeder instance so stdout/stderr and mount tracking are isolated.
func (c *SeedsCommand) runSeedsParallel(seeders []seeder.SeederInfo, opts *parallelOpts) error {
	parallelism := opts.parallelism
	// Parse params for every seeder to extract dependencies.
	paramsByName, err := c.parseSeedersParams(seeders)
	if err != nil {
		return err
	}

	// Build lookup structures.
	infoByName := make(map[string]seeder.SeederInfo, len(seeders))
	for _, info := range seeders {
		infoByName[info.Name] = info
	}

	// Build dependency graph: for each seeder, record which other
	// seeders in the current set it depends on.
	bsOpts := &buildSeedGraphOptions{
		seeders:      seeders,
		paramsByName: paramsByName,
		infoByName:   infoByName,
	}
	graph := c.buildSeedGraph(bsOpts)

	// Create per-worker cgroups for memory and CPU limiting.
	cgPool, err := c.createWorkerCgroups(opts, parallelism)
	if err != nil {
		return err
	}

	// Context cancelled on SIGINT/SIGTERM via PushCleanup.
	ctx, cancel := context.WithCancel(context.Background())
	c.PushCleanup(cancel)
	defer cancel()

	wpOpts := &runWorkerPoolOptions{
		parallelism:  parallelism,
		infoByName:   infoByName,
		paramsByName: paramsByName,
		graph:        graph,
		cgPool:       cgPool,
	}
	if err := c.runWorkerPool(ctx, wpOpts); err != nil {
		return err
	}
	return c.runPostBuild(seeders, paramsByName)
}

// createWorkerCgroups sets up per-worker cgroup v2 resource limits.
// Returns nil when parallelism <= 1 (no isolation needed).
func (c *SeedsCommand) createWorkerCgroups(opts *parallelOpts, parallelism int) (*cgroups.WorkerPool, error) {
	if parallelism <= 1 {
		return nil, nil
	}
	var si syscall.Sysinfo_t
	if err := syscall.Sysinfo(&si); err != nil {
		return nil, fmt.Errorf("failed to query system memory: %w", err)
	}
	totalBytes := si.Totalram * uint64(si.Unit)
	if opts.maxMemGiB > 0 {
		configBytes := uint64(opts.maxMemGiB) * 1024 * 1024 * 1024
		if configBytes < totalBytes {
			totalBytes = configBytes
		}
	}
	memPerWorker := totalBytes / uint64(parallelism)
	numCPUs := runtime.NumCPU()
	if opts.maxCPUs > 0 && opts.maxCPUs < numCPUs {
		numCPUs = opts.maxCPUs
	}
	cgPool, err := cgroups.NewWorkerPool(&cgroups.WorkerPoolOptions{
		Parallelism:       parallelism,
		MemPerWorkerBytes: memPerWorker,
		NumCPUs:           numCPUs,
		CoresMultiplier:   opts.coresMultiplier,
		CgroupRoot:        c.cgroupRoot,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create worker cgroups: %w", err)
	}
	c.PushCleanup(cgPool.Close)
	cpusPerWorker := numCPUs / parallelism
	if opts.coresMultiplier > 0 && opts.coresMultiplier != 1.0 {
		c.Printf("Worker cgroups: %d workers × %d GiB RAM, %d CPUs each (%.1fx oversubscription)\n",
			parallelism, memPerWorker/(1024*1024*1024), cpusPerWorker, opts.coresMultiplier)
	} else {
		c.Printf("Worker cgroups: %d workers × %d GiB RAM, %d CPUs each\n",
			parallelism, memPerWorker/(1024*1024*1024), cpusPerWorker)
	}
	return cgPool, nil
}

type runWorkerPoolOptions struct {
	parallelism  int
	infoByName   map[string]seeder.SeederInfo
	paramsByName map[string]*seeder.SeederParams
	graph        *seedGraph
	cgPool       *cgroups.WorkerPool
}

// runPostBuild runs post-build scripts sequentially for every seeder
// that has a PostBuildExec configured. Called after all parallel builds
// complete successfully.
func (c *SeedsCommand) runPostBuild(seeders []seeder.SeederInfo, paramsByName map[string]*seeder.SeederParams) error {
	c.Printf("Running post-build hooks sequentially ...\n")
	for _, info := range seeders {
		if info.PostBuildExec == "" {
			c.Printf("  [%s] No post-build script, skipping.\n", info.Name)
			continue
		}

		params := paramsByName[info.Name]
		chrootDir, err := c.resolveChrootDir(info.Name, params)
		if err != nil {
			return err
		}

		c.Printf("  [%s] Running post-build in %s ...\n", info.Name, chrootDir)

		sopts := &seeder.NewSeederOptions{
			Verbose: c.verbose,
			Stdout:  c.UI.NewStdoutWriter(fmt.Sprintf("seeds:%s:post", info.Name)),
			Stderr:  c.UI.NewStderrWriter(fmt.Sprintf("seeds:%s:post", info.Name)),
		}
		postSD, err := newSeeder(c.cfg, sopts)
		if err != nil {
			return fmt.Errorf("[%s] failed to create seeder for post-build: %w", info.Name, err)
		}
		c.PushCleanup(postSD.Cleanup)

		if err := postSD.SetupChrootDNS(chrootDir); err != nil {
			return fmt.Errorf("[%s] post-build DNS setup failed: %w", info.Name, err)
		}

		postOpts := &seeder.SeedOptions{
			ChrootDir: chrootDir,
			Info:      info,
		}
		if err := postSD.PostBuild(postOpts); err != nil {
			return fmt.Errorf("[%s] post-build failed: %w", info.Name, err)
		}
		postSD.Cleanup()

		c.Printf("  [%s] Post-build complete.\n", info.Name)
	}
	return nil
}

// runWorkerPool spawns a fixed pool of worker goroutines that pull
// seeders from the graph's ready channel.  Workers stop when the
// context is cancelled, an error occurs, or all seeders are done.
func (c *SeedsCommand) runWorkerPool(ctx context.Context, opts *runWorkerPoolOptions) error {
	var firstErr error
	var errMu sync.Mutex

	setError := func(err error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
		// Signal all workers to stop by closing the ready channel.
		opts.graph.mu.Lock()
		if !opts.graph.stopped {
			opts.graph.stopped = true
			close(opts.graph.ready)
		}
		opts.graph.mu.Unlock()
	}

	// Fixed worker pool: each goroutine pulls from ready until the
	// channel is closed or the context is cancelled.
	var wg sync.WaitGroup
	for i := 0; i < opts.parallelism; i++ {
		workerIdx := i
		wg.Go(func() {
			sysProcAttr := opts.cgPool.SysProcAttr(workerIdx)
			for {

				var seederName string
				select {
				case <-ctx.Done():
					return
				case name, ok := <-opts.graph.ready:
					if !ok {
						return
					}
					seederName = name
				}

				info := opts.infoByName[seederName]

				// Create an isolated ISeeder for this worker.
				sopts := &seeder.NewSeederOptions{
					Verbose: c.verbose,
					Stdout:  c.UI.NewStdoutWriter(fmt.Sprintf("seeds:%s", seederName)),
					Stderr:  c.UI.NewStderrWriter(fmt.Sprintf("seeds:%s", seederName)),
				}
				workerSD, err := newSeeder(c.cfg, sopts)
				if err != nil {
					setError(fmt.Errorf("[%s] failed to create seeder: %w", seederName, err))
					return
				}

				c.PushCleanup(workerSD.Cleanup)

				// Run the worker under its file lock.
				err = workerSD.ExecuteWithSeederLock(seederName, func() error {
					swOpts := &seederWorkerOptions{
						sd:          workerSD,
						info:        info,
						params:      opts.paramsByName[seederName],
						sysProcAttr: sysProcAttr,
					}
					return c.seederWorker(ctx, swOpts)
				})
				workerSD.Cleanup()

				if err != nil {
					setError(fmt.Errorf("seeder %s failed: %w", seederName, err))
					return
				}

				// Notify dependents and possibly close ready.
				opts.graph.mu.Lock()
				if !opts.graph.stopped {
					opts.graph.remaining--
					for _, dep := range opts.graph.dependents[seederName] {
						opts.graph.depCount[dep]--
						if opts.graph.depCount[dep] == 0 {
							opts.graph.ready <- dep
						}
					}
					if opts.graph.remaining == 0 {
						opts.graph.stopped = true
						close(opts.graph.ready)
					}
				}
				opts.graph.mu.Unlock()
			}
		})
	}

	wg.Wait()
	return firstErr
}

type seederWorkerOptions struct {
	sd          seeder.ISeeder
	info        seeder.SeederInfo
	params      *seeder.SeederParams
	sysProcAttr *syscall.SysProcAttr
}

// seederWorker processes a single seeder: resolve chroot dir, run
// prepper, set up DNS/dirs, execute chroot script, mark done, and
// record results.
// When params is nil it parses them from disk (sequential path).
func (c *SeedsCommand) seederWorker(ctx context.Context, swOpts *seederWorkerOptions) error {
	sd := swOpts.sd
	info := swOpts.info
	params := swOpts.params

	sd.Print(
		"[%s] Accepted seeder for execution\n", info.Name,
	)

	// Resolve chroot directory.
	chrootDir, err := c.resolveChrootDir(info.Name, params)
	if err != nil {
		return err
	}
	if c.chrootDir != "" {
		sd.PrintWarning(
			"[%s] Overriding chroot dir with --chroot-dir='%s'. This can be dangerous for multiple chroots.\n",
			info.Name,
			c.chrootDir,
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

	if err := ctx.Err(); err != nil {
		return err
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

	if err := ctx.Err(); err != nil {
		return err
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

	if err := ctx.Err(); err != nil {
		return err
	}

	// Execute seeder inside chroot.
	sd.Print(
		"[%s] Running seeder inside %s ...\n",
		info.Name, chrootDir,
	)
	opts := &seeder.SeedOptions{
		ChrootDir:   chrootDir,
		Info:        info,
		SysProcAttr: swOpts.sysProcAttr,
	}
	if err := sd.Seed(opts); err != nil {
		return fmt.Errorf(
			"[%s] chroot execution failed: %w", info.Name, err,
		)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Mark done.
	sd.Print("[%s] Flagging %s as complete ...\n",
		info.Name, chrootDir,
	)
	if err := sd.MarkSeederDone(info.Name, chrootDir); err != nil {
		return err
	}

	// Record results (thread-safe).
	if err := c.recordResults(info.Name, chrootDir); err != nil {
		return fmt.Errorf(
			"[%s] failed to record results: %w", info.Name, err,
		)
	}

	sd.Print("[%s] SUCCESS: Build complete.\n", info.Name)
	return nil
}

// parseSeedersParams parses the params file for each seeder and returns
// a map of seeder name to parsed params.
func (c *SeedsCommand) parseSeedersParams(seeders []seeder.SeederInfo) (map[string]*seeder.SeederParams, error) {
	paramsByName := make(map[string]*seeder.SeederParams, len(seeders))
	for _, info := range seeders {
		opts := &seeder.NewSeederOptions{Verbose: c.verbose}
		sd, err := newSeeder(c.cfg, opts)
		if err != nil {
			return nil, fmt.Errorf("[%s] failed to create seeder: %w", info.Name, err)
		}

		paramsName, err := sd.ParamsExecutableName()
		if err != nil {
			return nil, fmt.Errorf("[%s] failed to get params name: %w", info.Name, err)
		}
		paramsPath := filepath.Join(info.Dir, paramsName)
		if !filesystems.FileExists(paramsPath) {
			return nil, fmt.Errorf("[%s] unable to find %s", info.Name, paramsPath)
		}

		params, err := sd.ParseSeederParams(info.Name, paramsPath)
		if err != nil {
			return nil, fmt.Errorf("[%s] failed to parse params: %w", info.Name, err)
		}
		paramsByName[info.Name] = params
	}
	return paramsByName, nil
}

// --- Helper methods ---

// resolveChrootDir returns the chroot directory for a seeder, preferring
// the --chroot-dir flag override when set.
func (c *SeedsCommand) resolveChrootDir(name string, params *seeder.SeederParams) (string, error) {
	chrootDir := params.PreferredChrootDir
	if c.chrootDir != "" {
		chrootDir = c.chrootDir
	}
	if chrootDir == "" {
		return "", fmt.Errorf(
			"[%s] no chroot dir specified in params or --chroot-dir",
			name,
		)
	}
	return chrootDir, nil
}

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
	c.BuiltSeeders = append(c.BuiltSeeders, seederName)

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
// It is safe for concurrent use.
func (c *SeedsCommand) recordResults(seederName, chrootDir string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.recordBuiltRootfsFile(chrootDir); err != nil {
		return fmt.Errorf("failed to record built rootfs: %w", err)
	}

	if err := c.recordBuiltSeedersFile(seederName); err != nil {
		return fmt.Errorf("failed to record built seeder: %w", err)
	}
	return nil
}
