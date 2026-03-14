package releaser

import (
	"bytes"
	"sync"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/ostree"
)

// newTestReleaser builds a Releaser with mock dependencies for unit tests.
// It bypasses NewReleaser to avoid validation side-effects.
func newTestReleaser() *Releaser {
	return &Releaser{
		cfg:    &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}},
		ostree: &ostree.MockOstree{},
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
}

func TestTrackMount_AppendsInOrder(t *testing.T) {
	r := newTestReleaser()

	r.trackMount("/mnt/a")
	r.trackMount("/mnt/b")
	r.trackMount("/mnt/c")

	if got := len(r.trackedMounts); got != 3 {
		t.Fatalf("expected 3 tracked mounts, got %d", got)
	}
	want := []string{"/mnt/a", "/mnt/b", "/mnt/c"}
	for i, w := range want {
		if r.trackedMounts[i] != w {
			t.Errorf("trackedMounts[%d] = %q, want %q", i, r.trackedMounts[i], w)
		}
	}
}

func TestTrackMount_ConcurrentSafety(t *testing.T) {
	r := newTestReleaser()
	var wg sync.WaitGroup

	const n = 100
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			r.trackMount("/mnt/x")
		}()
	}
	wg.Wait()

	if got := len(r.trackedMounts); got != n {
		t.Fatalf("expected %d tracked mounts, got %d", n, got)
	}
}

func TestCleanup_ClearsTrackedMounts(t *testing.T) {
	r := newTestReleaser()
	r.trackMount("/mnt/a")
	r.trackMount("/mnt/b")

	r.Cleanup()

	if got := len(r.trackedMounts); got != 0 {
		t.Fatalf("expected 0 tracked mounts after Cleanup, got %d", got)
	}
}

func TestCleanup_Idempotent(t *testing.T) {
	r := newTestReleaser()
	r.trackMount("/mnt/a")

	r.Cleanup()
	r.Cleanup() // second call should not panic

	if got := len(r.trackedMounts); got != 0 {
		t.Fatalf("expected 0 tracked mounts after double Cleanup, got %d", got)
	}
}

func TestCleanup_EmptyMountsIsNoOp(t *testing.T) {
	r := newTestReleaser()

	// Should not panic when there are no mounts to clean up.
	r.Cleanup()

	if r.trackedMounts != nil {
		t.Fatal("expected trackedMounts to be nil after Cleanup with no mounts")
	}
}
