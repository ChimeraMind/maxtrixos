package releaser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"matrixos/vector/lib/filesystems"
)

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

func (r *Releaser) ReleaseLockPath() (string, error) {
	name := r.ref
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

func (r *Releaser) ExecuteWithReleaseLock(fn func() error) error {
	lockPath, err := r.ReleaseLockPath()
	if err != nil {
		return fmt.Errorf("failed to get release lock path: %w", err)
	}
	r.Print("Acquiring release %s lock via %s ...\n", r.ref, lockPath)

	timeoutStr, err := r.LockWaitSeconds()
	if err != nil {
		return fmt.Errorf("failed to get lock wait seconds: %w", err)
	}
	timeoutSecs, err := strconv.Atoi(timeoutStr)
	if err != nil {
		return fmt.Errorf("invalid lock wait seconds %q: %w", timeoutStr, err)
	}

	unlock, err := filesystems.AcquireFileLock(lockPath, time.Duration(timeoutSecs)*time.Second)
	if err != nil {
		return err
	}
	defer unlock()

	r.Print("Lock for releaser %s, %s acquired!\n", r.ref, lockPath)
	return fn()
}
