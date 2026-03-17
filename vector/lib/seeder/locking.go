package seeder

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"matrixos/vector/lib/filesystems"
)

func (s *Seeder) SeederLockDir() (string, error) {
	lockDir, err := s.LockDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(lockDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create lock directory %s: %w", lockDir, err)
	}
	return lockDir, nil
}

func (s *Seeder) SeederLockPath(name string) (string, error) {
	if name == "" {
		return "", errors.New("missing seeder name")
	}
	lockDir, err := s.SeederLockDir()
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

func (s *Seeder) ExecuteWithSeederLock(name string, fn func() error) error {
	lockPath, err := s.SeederLockPath(name)
	if err != nil {
		return fmt.Errorf("failed to get seeder lock path: %w", err)
	}
	s.Print("Acquiring seeder %s lock via %s ...\n", name, lockPath)

	timeoutStr, err := s.LockWaitSeconds()
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

	s.Print("Lock for seeder %s, %s acquired!\n", name, lockPath)

	return fn()
}
