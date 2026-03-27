// Package cgroups provides helpers for managing cgroup v2 hierarchies,
// primarily for memory-limiting parallel worker processes.
package cgroups

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"syscall"
)

const defaultCgroupV2Root = "/sys/fs/cgroup"

// workerCgroup holds the state for a single per-worker cgroup.
type workerCgroup struct {
	dir string
	fd  int
}

// WorkerPool manages a set of per-worker cgroups with memory limits.
// A nil *WorkerPool is safe to call methods on (no-op).
type WorkerPool struct {
	parentDir string
	workers   []*workerCgroup
}

// WorkerPoolOptions contains the resource limits for a worker pool.
type WorkerPoolOptions struct {
	Parallelism       int
	MemPerWorkerBytes uint64
	NumCPUs           int
	// CoresMultiplier scales the number of CPU cores assigned to each
	// worker's cpuset.  Values > 1.0 cause overlapping ranges (cores
	// shared between workers); values < 1.0 reduce each worker's
	// slice.  Defaults to 1.0 (strict partition, no overlap).
	CoresMultiplier float64
	// CgroupRoot overrides the cgroup v2 mount point (default: /sys/fs/cgroup).
	// Useful for testing with a fake cgroup tree.
	CgroupRoot string
}

// cgroupRoot returns the effective cgroup v2 mount point.
func (o *WorkerPoolOptions) cgroupRoot() string {
	if o.CgroupRoot != "" {
		return o.CgroupRoot
	}
	return defaultCgroupV2Root
}

// NewWorkerPool creates a cgroup v2 hierarchy for parallel workers,
// each limited to the specified memory and CPU slice.
func NewWorkerPool(opts *WorkerPoolOptions) (*WorkerPool, error) {
	if opts.Parallelism < 1 {
		return nil, fmt.Errorf("cgroups: parallelism must be >= 1, got %d", opts.Parallelism)
	}
	if opts.NumCPUs < 1 {
		return nil, fmt.Errorf("cgroups: numCPUs must be >= 1, got %d", opts.NumCPUs)
	}

	root := opts.cgroupRoot()

	// Verify cgroup v2 is available.
	subCtl := filepath.Join(root, "cgroup.subtree_control")
	if _, err := os.Stat(subCtl); err != nil {
		return nil, fmt.Errorf("cgroups: cgroup v2 not available (%s): %w", subCtl, err)
	}

	// Try to enable controllers on the root cgroup so they are
	// available for delegation into our child hierarchy.  This is
	// best-effort — the root may already have them, or we may lack
	// permission, both of which are fine.
	enableRootControllers(subCtl)

	parentDir := filepath.Join(root, fmt.Sprintf("vector-seeds-%d", os.Getpid()))
	if err := os.Mkdir(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("cgroups: failed to create parent dir %s: %w", parentDir, err)
	}

	useCpuset, err := enableControllers(parentDir, opts)
	if err != nil {
		os.Remove(parentDir)
		return nil, err
	}

	pool := &WorkerPool{parentDir: parentDir}
	for i := range opts.Parallelism {
		wc, err := createWorkerCgroup(parentDir, i, opts, useCpuset)
		if err != nil {
			pool.Close()
			return nil, err
		}
		pool.workers = append(pool.workers, wc)
	}

	return pool, nil
}

// enableControllers enables cgroup v2 controllers on the parent directory.
// memory is always required.  cpuset is preferred for CPU pinning; cpu
// bandwidth throttling (cpu.max) is only enabled as a fallback.
func enableControllers(parentDir string, opts *WorkerPoolOptions) (useCpuset bool, _ error) {
	ctl := filepath.Join(parentDir, "cgroup.subtree_control")
	if err := os.WriteFile(ctl, []byte("+memory"), 0644); err != nil {
		return useCpuset, fmt.Errorf("cgroups: failed to enable memory controller: %w", err)
	}

	useCpuset = false
	cpusPerWorker := opts.NumCPUs / opts.Parallelism
	if cpusPerWorker >= 1 {
		cpusetErr := os.WriteFile(ctl, []byte("+cpuset"), 0644)
		if cpusetErr == nil {
			useCpuset = true
		} else {
			fmt.Fprintf(
				os.Stderr,
				"WARNING: cpuset controller not available, falling back to cpu bandwidth throttling: %v\n",
				cpusetErr,
			)
		}
	}

	if !useCpuset {
		// cpuset not available — fall back to cpu bandwidth throttling.
		if err := os.WriteFile(ctl, []byte("+cpu"), 0644); err != nil {
			return false, fmt.Errorf("cgroups: failed to enable cpu controller: %w", err)
		}
	}
	return useCpuset, nil
}

// enableRootControllers tries to enable memory, cpu, and cpuset on the
// root cgroup's subtree_control.  Each write is independent and
// best-effort — failures are silently ignored (the controller may
// already be enabled, or we may lack permission).
func enableRootControllers(subtreeControlPath string) {
	for _, ctl := range []string{"+memory", "+cpu", "+cpuset"} {
		_ = os.WriteFile(subtreeControlPath, []byte(ctl), 0644)
	}
}

// createWorkerCgroup creates a single worker cgroup directory with
// memory, cpu, and optional cpuset limits, and returns an open fd.
func createWorkerCgroup(parentDir string, index int, opts *WorkerPoolOptions, useCpuset bool) (*workerCgroup, error) {
	dir := filepath.Join(parentDir, fmt.Sprintf("worker-%d", index))
	if err := os.Mkdir(dir, 0755); err != nil {
		return nil, fmt.Errorf("cgroups: failed to create worker-%d dir: %w", index, err)
	}

	if err := writeMemoryLimit(dir, index, opts.MemPerWorkerBytes); err != nil {
		return nil, err
	}
	if useCpuset {
		// cpuset pins workers to specific cores, which already limits
		// CPU usage and provides cache locality.  cpu.max bandwidth
		// throttling is only needed as a fallback.
		if err := writeCpusetPinning(dir, index, opts); err != nil {
			return nil, err
		}
	} else {
		if err := writeCPUQuota(dir, index, opts); err != nil {
			return nil, err
		}
	}

	fd, err := syscall.Open(dir, syscall.O_RDONLY|syscall.O_DIRECTORY, 0)
	if err != nil {
		return nil, fmt.Errorf("cgroups: failed to open worker-%d cgroup fd: %w", index, err)
	}
	return &workerCgroup{dir: dir, fd: fd}, nil
}

func writeMemoryLimit(dir string, index int, memBytes uint64) error {
	path := filepath.Join(dir, "memory.max")
	if err := os.WriteFile(path, fmt.Appendf(nil, "%d", memBytes), 0644); err != nil {
		return fmt.Errorf("cgroups: failed to write memory.max for worker-%d: %w", index, err)
	}
	return nil
}

func writeCPUQuota(dir string, index int, opts *WorkerPoolOptions) error {
	// CPU bandwidth limit via cpu.max: "$MAX $PERIOD" in µs.
	// e.g. 4 CPUs → "400000 100000" (400ms of CPU time per 100ms).
	// Supports fractional CPUs (e.g. 12 CPUs / 5 workers = 240000).
	const cpuPeriod = 100000
	cpuQuota := opts.NumCPUs * cpuPeriod / opts.Parallelism
	if cpuQuota < cpuPeriod {
		cpuQuota = cpuPeriod // At least 1 CPU worth of bandwidth.
	}
	path := filepath.Join(dir, "cpu.max")
	if err := os.WriteFile(path, fmt.Appendf(nil, "%d %d", cpuQuota, cpuPeriod), 0644); err != nil {
		return fmt.Errorf("cgroups: failed to write cpu.max for worker-%d: %w", index, err)
	}
	return nil
}

func writeCpusetPinning(dir string, index int, opts *WorkerPoolOptions) error {
	// Base cores per worker (strict partition).
	baseCPUs := opts.NumCPUs / opts.Parallelism
	extraCPUs := opts.NumCPUs % opts.Parallelism

	// Apply the multiplier and clamp to total available cores.
	multiplier := opts.CoresMultiplier
	if multiplier <= 0 {
		multiplier = 1.0
	}
	baseForWorker := baseCPUs
	if index < extraCPUs {
		baseForWorker++
	}

	effective := min(
		max(int(math.Round(float64(baseForWorker)*multiplier)), 1),
		opts.NumCPUs,
	)

	// Compute the start position from strict partitioning, then centre
	// the (potentially wider) range around it.
	strictStart := 0
	for i := range index {
		n := baseCPUs
		if i < extraCPUs {
			n++
		}
		strictStart += n
	}
	strictMid := strictStart + baseForWorker/2

	cpuStart := strictMid - effective/2
	cpuEnd := cpuStart + effective - 1

	// Clamp to [0, NumCPUs-1].
	if cpuStart < 0 {
		cpuStart = 0
		cpuEnd = effective - 1
	}
	if cpuEnd >= opts.NumCPUs {
		cpuEnd = opts.NumCPUs - 1
		cpuStart = cpuEnd - effective + 1
		if cpuStart < 0 {
			cpuStart = 0
		}
	}

	path := filepath.Join(dir, "cpuset.cpus")
	if err := os.WriteFile(path, fmt.Appendf(nil, "%d-%d", cpuStart, cpuEnd), 0644); err != nil {
		return fmt.Errorf("cgroups: failed to write cpuset.cpus for worker-%d: %w", index, err)
	}
	return nil
}

// SysProcAttr returns a SysProcAttr configured to spawn a child process
// directly into the given worker's cgroup via clone3(CLONE_INTO_CGROUP),
// or nil if the pool is inactive.
func (p *WorkerPool) SysProcAttr(workerIndex int) *syscall.SysProcAttr {
	if p == nil || workerIndex >= len(p.workers) {
		return nil
	}
	return &syscall.SysProcAttr{
		UseCgroupFD: true,
		CgroupFD:    p.workers[workerIndex].fd,
	}
}

// Close releases all file descriptors and removes the cgroup hierarchy.
func (p *WorkerPool) Close() {
	if p == nil {
		return
	}
	for _, w := range p.workers {
		if w.fd >= 0 {
			syscall.Close(w.fd)
			w.fd = -1
		}
		os.Remove(w.dir)
	}
	p.workers = nil
	os.Remove(p.parentDir)
}
