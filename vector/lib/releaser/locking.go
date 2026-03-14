package releaser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)
// ReleaseLockDir returns the lock directory, creating it if necessary.
func (r *Releaser) ReleaseLockDir() (string, error) {
	lockDir, err := r.LockDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create lock directory %s: %w", lockDir, err)
	}
	return lockDir, nil
}

// ReleaseLockPath returns the lock file path for the given release name.
func (r *Releaser) ReleaseLockPath(name string) (string, error) {
	if name == "" {
		return "", errors.New("missing release name")
	}
	lockDir, err := r.ReleaseLockDir()
	if err != nil {
		return "", err
	}
	lockFile := filepath.Join(lockDir, name+".lock")

	lockFileDir := filepath.Dir(lockFile)
	if err := os.MkdirAll(lockFileDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create lock file directory %s: %w", lockFileDir, err)
	}
	return lockFile, nil
}

// ExecuteWithReleaseLock acquires an exclusive file lock for the given release name,
// executes fn under that lock, and releases the lock when fn returns.
// If the lock cannot be acquired within the configured timeout, an error is returned.
// If fn panics or the process crashes, the OS closes the file descriptor and
// releases the lock automatically.
func (r *Releaser) ExecuteWithReleaseLock(name string, fn func() error) error {
	lockPath, err := r.ReleaseLockPath(name)
	if err != nil {
		return fmt.Errorf("failed to get release lock path: %w", err)
	}
	r.Print("Acquiring release %s lock via %s ...\n", name, lockPath)

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open lock file %s: %w", lockPath, err)
	}
	defer lockFile.Close()

	timeoutStr, err := r.LockWaitSeconds()
	if err != nil {
		return fmt.Errorf("failed to get lock wait seconds: %w", err)
	}
	timeoutSecs, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid lock wait seconds %q: %w", timeoutStr, err)
	}

	// Try to acquire the exclusive lock with a timeout.
	locked := make(chan error, 1)
	go func() {
		locked <- syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX)
	}()

	select {
	case err := <-locked:
		if err != nil {
			return fmt.Errorf("failed to acquire lock %s: %w", lockPath, err)
		}
	case <-time.After(time.Duration(timeoutSecs) * time.Second):
		return fmt.Errorf("timed out waiting for release lock %s", lockPath)
	}

	r.Print("Lock for releaser %s, %s acquired!\n", name, lockPath)
	return fn()
}
