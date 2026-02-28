package imager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

func (im *Image) ImageLockDir() (string, error) {
	lockDir, err := im.LockDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create lock directory %s: %w", lockDir, err)
	}
	return lockDir, nil
}

func (im *Image) ImageLockPath() (string, error) {
	if im.ref == "" {
		return "", errors.New("missing ref, set Ref in NewImageOptions")
	}

	lockDir, err := im.ImageLockDir()
	if err != nil {
		return "", err
	}
	lockFile := filepath.Join(lockDir, im.ref+".lock")

	lockFileDir := filepath.Dir(lockFile)
	if err := os.MkdirAll(lockFileDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create lock file directory %s: %w", lockFileDir, err)
	}
	return lockFile, nil
}

func (im *Image) ExecuteWithImageLock(fn func() error) error {
	lockPath, err := im.ImageLockPath()
	if err != nil {
		return fmt.Errorf("failed to get image lock path: %w", err)
	}
	im.Print("Acquiring branch %s lock via %s ...\n", im.ref, lockPath)

	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open lock file %s: %w", lockPath, err)
	}
	defer lockFile.Close()

	timeoutStr, err := im.LockWaitSeconds()
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
		return fmt.Errorf("timed out waiting for imager lock %s", lockPath)
	}

	im.Print("Lock for imager %s, %s acquired!\n", im.ref, lockPath)

	// Execute the function under the lock.
	// The lock is released when lockFile is closed (deferred above).
	return fn()
}
