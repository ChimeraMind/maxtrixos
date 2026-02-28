package imager

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
)

// --- ImageLockDir Tests ---

func TestImageLockDir(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		im := newTestImage(cfg, &ostree.MockOstree{})

		result, err := im.ImageLockDir()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != lockDir {
			t.Errorf("got %q, want %q", result, lockDir)
		}
		// Verify directory was created.
		if _, err := os.Stat(lockDir); os.IsNotExist(err) {
			t.Error("lock directory should have been created")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		_, err := im.ImageLockDir()
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

// --- ImageLockPath Tests ---

func TestImageLockPath(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.ref = "matrixos/amd64/gnome"

		result, err := im.ImageLockPath()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		expected := filepath.Join(lockDir, "matrixos/amd64/gnome.lock")
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("EmptyRef", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		_, err := im.ImageLockPath()
		if err == nil {
			t.Error("should error for empty ref")
		}
	})
}

// --- ExecuteWithImageLock Tests ---

func TestExecuteWithImageLock(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"5"}
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.ref = "test/ref"

		called := false
		err := im.ExecuteWithImageLock(func() error {
			called = true
			return nil
		})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !called {
			t.Error("fn should have been called")
		}
	})

	t.Run("FnErrorPropagated", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"5"}
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.ref = "test/ref"

		fnErr := errors.New("fn failed")
		err := im.ExecuteWithImageLock(func() error {
			return fnErr
		})
		if err == nil {
			t.Fatal("expected error from fn")
		}
		if !errors.Is(err, fnErr) {
			t.Errorf("got error %v, want %v", err, fnErr)
		}
	})

	t.Run("EmptyRef", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		err := im.ExecuteWithImageLock(func() error { return nil })
		if err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("InvalidLockWaitSeconds", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"notanumber"}
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.ref = "test/ref"

		err := im.ExecuteWithImageLock(func() error { return nil })
		if err == nil {
			t.Error("should error for invalid lock wait seconds")
		}
		if !strings.Contains(err.Error(), "invalid lock wait seconds") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.ref = "test/ref"
		err := im.ExecuteWithImageLock(func() error { return nil })
		if err == nil {
			t.Error("should error from broken config")
		}
	})

	t.Run("LockIsExclusive", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"5"}
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.ref = "exclusive/ref"

		// Acquire the lock in the callback and verify a second goroutine blocks.
		started := make(chan struct{})
		proceed := make(chan struct{})
		done := make(chan error, 1)

		go func() {
			done <- im.ExecuteWithImageLock(func() error {
				close(started) // signal we hold the lock
				<-proceed      // wait until test says to release
				return nil
			})
		}()

		<-started // first goroutine holds the lock

		// Try to acquire the same lock with a very short timeout.
		cfg2 := baseImageConfig()
		cfg2.Items["Imager.LocksDir"] = []string{lockDir}
		cfg2.Items["Imager.LockWaitSeconds"] = []string{"1"}
		im2 := newTestImage(cfg2, &ostree.MockOstree{})
		im2.ref = "exclusive/ref"

		err := im2.ExecuteWithImageLock(func() error {
			return nil
		})
		if err == nil {
			t.Error("second lock acquisition should have timed out")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("expected timeout error, got: %v", err)
		}

		close(proceed) // release the first lock
		if err := <-done; err != nil {
			t.Fatalf("first goroutine errored: %v", err)
		}
	})

	t.Run("LockReleasedAfterFn", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"5"}
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.ref = "release/ref"

		// First call acquires and releases the lock.
		err := im.ExecuteWithImageLock(func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("first call error: %v", err)
		}

		// Second call should succeed since the lock was released.
		err = im.ExecuteWithImageLock(func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("second call should succeed after lock release: %v", err)
		}
	})
}
