package cgroups

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCgroupRoot_Default(t *testing.T) {
	opts := &WorkerPoolOptions{}
	if got := opts.cgroupRoot(); got != defaultCgroupV2Root {
		t.Errorf("got %q, want %q", got, defaultCgroupV2Root)
	}
}

func TestCgroupRoot_Override(t *testing.T) {
	opts := &WorkerPoolOptions{CgroupRoot: "/my/root"}
	if got := opts.cgroupRoot(); got != "/my/root" {
		t.Errorf("got %q, want /my/root", got)
	}
}

// --- NewWorkerPool validation ---

func TestNewWorkerPool_InvalidParallelism(t *testing.T) {
	_, err := NewWorkerPool(&WorkerPoolOptions{Parallelism: 0, NumCPUs: 4})
	if err == nil {
		t.Fatal("expected error for parallelism=0")
	}
}

func TestNewWorkerPool_InvalidNumCPUs(t *testing.T) {
	_, err := NewWorkerPool(&WorkerPoolOptions{Parallelism: 2, NumCPUs: 0})
	if err == nil {
		t.Fatal("expected error for numCPUs=0")
	}
}

func TestNewWorkerPool_NoCgroupV2(t *testing.T) {
	root := t.TempDir() // no cgroup.subtree_control file
	_, err := NewWorkerPool(&WorkerPoolOptions{
		Parallelism: 2, NumCPUs: 4, CgroupRoot: root,
	})
	if err == nil {
		t.Fatal("expected error when cgroup v2 not available")
	}
}

// fakeCgroupRoot creates a temp directory mimicking a cgroup v2 mount point.
// The subtree_control file accepts any writes so both cpuset and cpu paths work.
func fakeCgroupRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(
		filepath.Join(root, "cgroup.subtree_control"),
		[]byte("memory cpuset cpu"), 0644,
	); err != nil {
		t.Fatalf("create fake subtree_control: %v", err)
	}
	return root
}

func TestNewWorkerPool_CreatesWorkers(t *testing.T) {
	root := fakeCgroupRoot(t)
	pool, err := NewWorkerPool(&WorkerPoolOptions{
		Parallelism:       3,
		NumCPUs:           12,
		MemPerWorkerBytes: 4 * 1024 * 1024 * 1024,
		CgroupRoot:        root,
	})
	if err != nil {
		t.Fatalf("NewWorkerPool: %v", err)
	}
	defer pool.Close()

	if len(pool.workers) != 3 {
		t.Fatalf("got %d workers, want 3", len(pool.workers))
	}
	for i, w := range pool.workers {
		if w.fd < 0 {
			t.Errorf("worker %d: fd is negative", i)
		}
		if _, err := os.Stat(w.dir); err != nil {
			t.Errorf("worker %d: dir %s does not exist", i, w.dir)
		}
	}
}

func TestNewWorkerPool_CloseNilsWorkers(t *testing.T) {
	root := fakeCgroupRoot(t)
	pool, err := NewWorkerPool(&WorkerPoolOptions{
		Parallelism:       2,
		NumCPUs:           4,
		MemPerWorkerBytes: 1024 * 1024 * 1024,
		CgroupRoot:        root,
	})
	if err != nil {
		t.Fatalf("NewWorkerPool: %v", err)
	}

	pool.Close()

	if pool.workers != nil {
		t.Error("workers should be nil after Close")
	}
	// SysProcAttr on a closed pool must return nil.
	if got := pool.SysProcAttr(0); got != nil {
		t.Error("SysProcAttr should return nil after Close")
	}
}

// --- SysProcAttr ---

func TestSysProcAttr_NilPool(t *testing.T) {
	var p *WorkerPool
	if got := p.SysProcAttr(0); got != nil {
		t.Error("expected nil for nil pool")
	}
}

func TestSysProcAttr_OutOfBounds(t *testing.T) {
	root := fakeCgroupRoot(t)
	pool, err := NewWorkerPool(&WorkerPoolOptions{
		Parallelism: 1, NumCPUs: 2, MemPerWorkerBytes: 1 << 30, CgroupRoot: root,
	})
	if err != nil {
		t.Fatalf("NewWorkerPool: %v", err)
	}
	defer pool.Close()

	if got := pool.SysProcAttr(5); got != nil {
		t.Error("expected nil for out-of-bounds index")
	}
}

func TestSysProcAttr_Valid(t *testing.T) {
	root := fakeCgroupRoot(t)
	pool, err := NewWorkerPool(&WorkerPoolOptions{
		Parallelism: 2, NumCPUs: 4, MemPerWorkerBytes: 1 << 30, CgroupRoot: root,
	})
	if err != nil {
		t.Fatalf("NewWorkerPool: %v", err)
	}
	defer pool.Close()

	attr := pool.SysProcAttr(0)
	if attr == nil {
		t.Fatal("expected non-nil SysProcAttr")
	}
	if !attr.UseCgroupFD {
		t.Error("UseCgroupFD should be true")
	}
	if attr.CgroupFD < 0 {
		t.Error("CgroupFD should be non-negative")
	}
}

// --- Close nil safety ---

func TestClose_NilPool(t *testing.T) {
	var p *WorkerPool
	p.Close() // must not panic
}

func TestClose_DoubleClose(t *testing.T) {
	root := fakeCgroupRoot(t)
	pool, err := NewWorkerPool(&WorkerPoolOptions{
		Parallelism: 1, NumCPUs: 2, MemPerWorkerBytes: 1 << 30, CgroupRoot: root,
	})
	if err != nil {
		t.Fatalf("NewWorkerPool: %v", err)
	}
	pool.Close()
	pool.Close() // must not panic
}

// --- writeMemoryLimit ---

func TestWriteMemoryLimit(t *testing.T) {
	dir := t.TempDir()
	if err := writeMemoryLimit(dir, 0, 16*1024*1024*1024); err != nil {
		t.Fatalf("writeMemoryLimit: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "memory.max"))
	if err != nil {
		t.Fatalf("read memory.max: %v", err)
	}
	want := "17179869184" // 16 GiB
	if string(got) != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// --- writeCPUQuota ---

func TestWriteCPUQuota(t *testing.T) {
	tests := []struct {
		name    string
		numCPUs int
		par     int
		want    string
	}{
		{"4cpus_2workers", 4, 2, "200000 100000"},
		{"12cpus_5workers", 12, 5, "240000 100000"},
		{"1cpu_4workers_clamp", 1, 4, "100000 100000"},
		{"8cpus_1worker", 8, 1, "800000 100000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			opts := &WorkerPoolOptions{NumCPUs: tt.numCPUs, Parallelism: tt.par}
			if err := writeCPUQuota(dir, 0, opts); err != nil {
				t.Fatalf("writeCPUQuota: %v", err)
			}
			got, _ := os.ReadFile(filepath.Join(dir, "cpu.max"))
			if string(got) != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// --- writeCpusetPinning ---

func TestWriteCpusetPinning(t *testing.T) {
	tests := []struct {
		name       string
		numCPUs    int
		par        int
		multiplier float64
		wantByIdx  map[int]string // worker index -> expected cpuset.cpus content
	}{
		{
			name:    "12cpus_2workers_1x",
			numCPUs: 12, par: 2, multiplier: 1.0,
			wantByIdx: map[int]string{0: "0-5", 1: "6-11"},
		},
		{
			name:    "12cpus_3workers_1x",
			numCPUs: 12, par: 3, multiplier: 1.0,
			wantByIdx: map[int]string{0: "0-3", 1: "4-7", 2: "8-11"},
		},
		{
			name:    "12cpus_2workers_1.5x",
			numCPUs: 12, par: 2, multiplier: 1.5,
			wantByIdx: map[int]string{0: "0-8", 1: "3-11"},
		},
		{
			name:    "12cpus_3workers_2x",
			numCPUs: 12, par: 3, multiplier: 2.0,
			wantByIdx: map[int]string{0: "0-7", 1: "2-9", 2: "4-11"},
		},
		{
			name:    "12cpus_2workers_0.5x",
			numCPUs: 12, par: 2, multiplier: 0.5,
			wantByIdx: map[int]string{0: "2-4", 1: "8-10"},
		},
		{
			name:    "2cpus_2workers_1x",
			numCPUs: 2, par: 2, multiplier: 1.0,
			wantByIdx: map[int]string{0: "0-0", 1: "1-1"},
		},
		{
			name:    "10cpus_3workers_1x_uneven",
			numCPUs: 10, par: 3, multiplier: 1.0,
			wantByIdx: map[int]string{0: "0-3", 1: "4-6", 2: "7-9"},
		},
		{
			name:    "multiplier_zero_becomes_1x",
			numCPUs: 12, par: 2, multiplier: 0,
			wantByIdx: map[int]string{0: "0-5", 1: "6-11"},
		},
		{
			name:    "huge_multiplier_clamps_to_all",
			numCPUs: 12, par: 2, multiplier: 10.0,
			wantByIdx: map[int]string{0: "0-11", 1: "0-11"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := &WorkerPoolOptions{
				NumCPUs:         tt.numCPUs,
				Parallelism:     tt.par,
				CoresMultiplier: tt.multiplier,
			}
			for idx, want := range tt.wantByIdx {
				dir := t.TempDir()
				if err := writeCpusetPinning(dir, idx, opts); err != nil {
					t.Fatalf("worker %d: %v", idx, err)
				}
				got, _ := os.ReadFile(filepath.Join(dir, "cpuset.cpus"))
				if string(got) != want {
					t.Errorf("worker %d: got %q, want %q", idx, got, want)
				}
			}
		})
	}
}

// --- enableControllers ---

func TestEnableControllers_PrefersCpuset(t *testing.T) {
	dir := t.TempDir()
	ctl := filepath.Join(dir, "cgroup.subtree_control")
	os.WriteFile(ctl, []byte(""), 0644)

	opts := &WorkerPoolOptions{NumCPUs: 8, Parallelism: 2}
	useCpuset, err := enableControllers(dir, opts)
	if err != nil {
		t.Fatalf("enableControllers: %v", err)
	}
	if !useCpuset {
		t.Error("expected cpuset to be enabled when cpusPerWorker >= 1")
	}
}

func TestEnableControllers_FallsBackToCPU(t *testing.T) {
	dir := t.TempDir()
	ctl := filepath.Join(dir, "cgroup.subtree_control")
	os.WriteFile(ctl, []byte(""), 0644)

	// With only 1 CPU for 2 workers, cpusPerWorker=0, so cpuset won't be tried.
	opts := &WorkerPoolOptions{NumCPUs: 1, Parallelism: 2}
	useCpuset, err := enableControllers(dir, opts)
	if err != nil {
		t.Fatalf("enableControllers: %v", err)
	}
	if useCpuset {
		t.Error("expected cpu fallback when cpusPerWorker < 1")
	}
}

// --- enableRootControllers ---

func TestEnableRootControllers_BestEffort(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cgroup.subtree_control")
	os.WriteFile(path, []byte(""), 0644)

	// Should not panic or error.
	enableRootControllers(path)

	// Non-existent path: all writes silently fail.
	enableRootControllers(filepath.Join(dir, "nonexistent"))
}

// --- Integration: NewWorkerPool with cpuset ---

func TestNewWorkerPool_CpusetFiles(t *testing.T) {
	root := fakeCgroupRoot(t)
	pool, err := NewWorkerPool(&WorkerPoolOptions{
		Parallelism:       2,
		NumCPUs:           8,
		MemPerWorkerBytes: 2 * 1024 * 1024 * 1024,
		CoresMultiplier:   1.5,
		CgroupRoot:        root,
	})
	if err != nil {
		t.Fatalf("NewWorkerPool: %v", err)
	}
	defer pool.Close()

	for i, w := range pool.workers {
		mem, err := os.ReadFile(filepath.Join(w.dir, "memory.max"))
		if err != nil {
			t.Errorf("worker %d: missing memory.max: %v", i, err)
		}
		if string(mem) != "2147483648" {
			t.Errorf("worker %d: memory.max = %q, want 2147483648", i, mem)
		}

		cpus, err := os.ReadFile(filepath.Join(w.dir, "cpuset.cpus"))
		if err != nil {
			t.Errorf("worker %d: missing cpuset.cpus: %v", i, err)
		}
		if len(cpus) == 0 {
			t.Errorf("worker %d: cpuset.cpus is empty", i)
		}
	}
}

// --- Integration: NewWorkerPool with cpu.max fallback ---

func TestNewWorkerPool_CPUQuotaFallback(t *testing.T) {
	root := fakeCgroupRoot(t)
	// 1 CPU for 2 workers -> cpusPerWorker=0 -> cpuset skipped -> cpu.max used.
	pool, err := NewWorkerPool(&WorkerPoolOptions{
		Parallelism:       2,
		NumCPUs:           1,
		MemPerWorkerBytes: 1 << 30,
		CgroupRoot:        root,
	})
	if err != nil {
		t.Fatalf("NewWorkerPool: %v", err)
	}
	defer pool.Close()

	for i, w := range pool.workers {
		cpuMax, err := os.ReadFile(filepath.Join(w.dir, "cpu.max"))
		if err != nil {
			t.Errorf("worker %d: missing cpu.max: %v", i, err)
		}
		if string(cpuMax) != "100000 100000" {
			t.Errorf("worker %d: cpu.max = %q, want \"100000 100000\"", i, cpuMax)
		}
	}
}
