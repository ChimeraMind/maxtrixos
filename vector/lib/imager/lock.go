package imager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"matrixos/vector/lib/filesystems"
)

func (im *Imager) ImageLockDir() (string, error) {
	lockDir, err := im.LockDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create lock directory %s: %w", lockDir, err)
	}
	return lockDir, nil
}

func (im *Imager) ImageLockPath() (string, error) {
	if im.ref == "" {
		return "", errors.New("missing ref, set Ref in NewImagerOptions")
	}

	ref, err := im.cleanAndStripRef()
	if err != nil {
		return "", fmt.Errorf("failed to clean ref: %w", err)
	}

	lockDir, err := im.ImageLockDir()
	if err != nil {
		return "", err
	}
	lockFile := filepath.Join(lockDir, ref+".lock")

	lockFileDir := filepath.Dir(lockFile)
	if err := os.MkdirAll(lockFileDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create lock file directory %s: %w", lockFileDir, err)
	}
	return lockFile, nil
}

func (im *Imager) ExecuteWithImageLock(fn func() error) error {
	lockPath, err := im.ImageLockPath()
	if err != nil {
		return fmt.Errorf("failed to get image lock path: %w", err)
	}
	im.Print("Acquiring branch %s lock via %s ...\n", im.ref, lockPath)

	timeoutStr, err := im.LockWaitSeconds()
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

	im.Print("Lock for imager %s, %s acquired!\n", im.ref, lockPath)

	return fn()
}
