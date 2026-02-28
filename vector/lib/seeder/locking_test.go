package seeder

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

// newLockTestSeeder returns a Seeder whose lock dir points at a temp directory.
func newLockTestSeeder(t *testing.T) *Seeder {
	t.Helper()
	s := newTestSeeder()
	lockDir := filepath.Join(t.TempDir(), "locks")
	s.cfg.(*config.MockConfig).Items["Seeder.LocksDir"] = []string{lockDir}
	s.cfg.(*config.MockConfig).Items["Seeder.LockWaitSeconds"] = []string{"5"}
	return s
}

// ---------- SeederLockDir ----------

func TestSeederLockDir_CreatesDir(t *testing.T) {
	s := newLockTestSeeder(t)

	dir, err := s.SeederLockDir()
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

func TestSeederLockDir_MissingConfig(t *testing.T) {
	s := newTestSeeder()
	// No Seeder.LocksDir in mock config → configItem returns error.
	_, err := s.SeederLockDir()
	if err == nil {
		t.Fatal("expected error when LocksDir is not configured")
	}
}

// ---------- SeederLockPath ----------

func TestSeederLockPath_ReturnsExpectedPath(t *testing.T) {
	s := newLockTestSeeder(t)

	p, err := s.SeederLockPath("bedrock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(s.cfg.(*config.MockConfig).Items["Seeder.LocksDir"][0], "bedrock.lock")
	if p != want {
		t.Fatalf("SeederLockPath() = %q, want %q", p, want)
	}
}

func TestSeederLockPath_EmptyName(t *testing.T) {
	s := newLockTestSeeder(t)

	_, err := s.SeederLockPath("")
	if err == nil {
		t.Fatal("expected error for empty seeder name")
	}
}

// ---------- ExecuteWithSeederLock ----------

func TestExecuteWithSeederLock_RunsFunction(t *testing.T) {
	s := newLockTestSeeder(t)

	called := false
	err := s.ExecuteWithSeederLock("test", func() error {
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

func TestExecuteWithSeederLock_PropagatesFnError(t *testing.T) {
	s := newLockTestSeeder(t)

	want := errors.New("fn failed")
	err := s.ExecuteWithSeederLock("test", func() error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("expected fn error, got %v", err)
	}
}

func TestExecuteWithSeederLock_InvalidTimeout(t *testing.T) {
	s := newLockTestSeeder(t)
	s.cfg.(*config.MockConfig).Items["Seeder.LockWaitSeconds"] = []string{"notanumber"}

	err := s.ExecuteWithSeederLock("test", func() error { return nil })
	if err == nil {
		t.Fatal("expected error for non-numeric timeout")
	}
}

func TestExecuteWithSeederLock_EmptyName(t *testing.T) {
	s := newLockTestSeeder(t)

	err := s.ExecuteWithSeederLock("", func() error { return nil })
	if err == nil {
		t.Fatal("expected error for empty name")
	}
}

func TestExecuteWithSeederLock_MutualExclusion(t *testing.T) {
	s := newLockTestSeeder(t)

	// Hold the lock from outside the Seeder so the inner call blocks.
	lockPath, err := s.SeederLockPath("contended")
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
	s.cfg.(*config.MockConfig).Items["Seeder.LockWaitSeconds"] = []string{"1"}

	err = s.ExecuteWithSeederLock("contended", func() error {
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

func TestExecuteWithSeederLock_SequentialAccess(t *testing.T) {
	s := newLockTestSeeder(t)

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
			err := s.ExecuteWithSeederLock("serial", func() error {
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
