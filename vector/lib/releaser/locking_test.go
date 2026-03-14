package releaser

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"matrixos/vector/lib/config"
)

// newLockTestReleaser returns a Releaser whose lock dir points at a temp directory.
func newLockTestReleaser(t *testing.T) *Releaser {
	t.Helper()
	r := newTestReleaser()
	lockDir := filepath.Join(t.TempDir(), "locks")
	r.cfg.(*config.MockConfig).Items["Releaser.LocksDir"] = []string{lockDir}
	r.cfg.(*config.MockConfig).Items["Releaser.LockWaitSeconds"] = []string{"5"}
	return r
}

// ---------- ReleaseLockDir ----------

func TestReleaseLockDir_CreatesDir(t *testing.T) {
	r := newLockTestReleaser(t)

	dir, err := r.ReleaseLockDir()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("lock directory not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatal("lock path exists but is not a directory")
	}
}

func TestReleaseLockDir_MissingConfig(t *testing.T) {
	r := newTestReleaser()
	// No Releaser.LocksDir in mock config → configItem returns error.
	_, err := r.ReleaseLockDir()
	if err == nil {
		t.Fatal("expected error when LocksDir is not configured")
	}
}

// ---------- ReleaseLockPath ----------

func TestReleaseLockPath_ReturnsExpectedPath(t *testing.T) {
	r := newLockTestReleaser(t)

	p, err := r.ReleaseLockPath("myrelease")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(r.cfg.(*config.MockConfig).Items["Releaser.LocksDir"][0], "myrelease.lock")
	if p != want {
		t.Fatalf("ReleaseLockPath() = %q, want %q", p, want)
	}
}

func TestReleaseLockPath_EmptyName(t *testing.T) {
	r := newLockTestReleaser(t)

	_, err := r.ReleaseLockPath("")
	if err == nil {
		t.Fatal("expected error for empty release name")
	}
}

func TestReleaseLockPath_NestedName(t *testing.T) {
	r := newLockTestReleaser(t)

	p, err := r.ReleaseLockPath("origin/matrixos")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// The intermediate directory for origin/ should exist.
	if _, statErr := os.Stat(filepath.Dir(p)); statErr != nil {
		t.Fatalf("parent dir of nested lock was not created: %v", statErr)
	}
}

// ---------- ExecuteWithReleaseLock ----------

func TestExecuteWithReleaseLock_RunsFunction(t *testing.T) {
	r := newLockTestReleaser(t)

	called := false
	err := r.ExecuteWithReleaseLock("test", func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("fn was not called")
	}
}

func TestExecuteWithReleaseLock_PropagatesFnError(t *testing.T) {
	r := newLockTestReleaser(t)

	want := errors.New("fn failed")
	err := r.ExecuteWithReleaseLock("test", func() error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected fn error, got %v", err)
	}
}

func TestExecuteWithReleaseLock_InvalidTimeout(t *testing.T) {
	r := newLockTestReleaser(t)
	r.cfg.(*config.MockConfig).Items["Releaser.LockWaitSeconds"] = []string{"notanumber"}

	err := r.ExecuteWithReleaseLock("test", func() error { return nil })
	if err == nil {
		t.Fatal("expected error for non-numeric timeout")
	}
}

func TestExecuteWithReleaseLock_EmptyName(t *testing.T) {
	r := newLockTestReleaser(t)

	err := r.ExecuteWithReleaseLock("", func() error { return nil })
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestExecuteWithReleaseLock_MutualExclusion(t *testing.T) {
	r := newLockTestReleaser(t)

	// Hold the lock from outside the Releaser so the inner call blocks.
	lockPath, err := r.ReleaseLockPath("contended")
	if err != nil {
		t.Fatalf("unexpected error getting lock path: %v", err)
	}
	externalFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("failed to open lock file externally: %v", err)
	}
	if err := syscall.Flock(int(externalFile.Fd()), syscall.LOCK_EX); err != nil {
		externalFile.Close()
		t.Fatalf("failed to acquire external lock: %v", err)
	}

	// Use a very short timeout so we don't slow down the test suite.
	r.cfg.(*config.MockConfig).Items["Releaser.LockWaitSeconds"] = []string{"1"}

	err = r.ExecuteWithReleaseLock("contended", func() error {
		t.Fatal("fn should never run while lock is held externally")
		return nil
	})

	externalFile.Close()

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !containsString(err.Error(), "timed out") {
		t.Fatalf("expected timeout message, got: %v", err)
	}
}

func TestExecuteWithReleaseLock_SequentialAccess(t *testing.T) {
	r := newLockTestReleaser(t)

	var order []int
	var mu sync.Mutex

	appendOrder := func(v int) {
		mu.Lock()
		defer mu.Unlock()
		order = append(order, v)
	}

	// Two goroutines contend for the same lock. Both must complete,
	// proving the lock is released after each fn returns.
	var wg sync.WaitGroup
	var started atomic.Int32

	for i := 1; i <= 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			started.Add(1)
			err := r.ExecuteWithReleaseLock("serial", func() error {
				appendOrder(id)
				time.Sleep(10 * time.Millisecond) // hold briefly
				return nil
			})
			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", id, err)
			}
		}(i)
	}
	wg.Wait()

	mu.Lock()
	defer mu.Unlock()
	if len(order) != 2 {
		t.Fatalf("expected 2 executions, got %d", len(order))
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
