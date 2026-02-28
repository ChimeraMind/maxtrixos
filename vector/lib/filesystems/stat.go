package filesystems

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// NormalizeTimestamps recursively sets the access and modification times of
// every file, symlink, and directory under root (including root itself) to t.
//
// Symlinks are updated without following them (equivalent to touch -h).
// Directories are updated in depth-first (deepest first) order so that
// touching children does not alter already-normalised parent timestamps.
func NormalizeTimestamps(root string, t time.Time) error {
	ts := []unix.Timespec{
		unix.NsecToTimespec(t.UnixNano()), // atime
		unix.NsecToTimespec(t.UnixNano()), // mtime
	}

	var dirs []string
	if err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			dirs = append(dirs, path)
			return nil
		}
		// AT_SYMLINK_NOFOLLOW: set the timestamp on the symlink itself,
		// not its target. For regular files this flag is harmless.
		return unix.UtimesNanoAt(unix.AT_FDCWD, path, ts, unix.AT_SYMLINK_NOFOLLOW)
	}); err != nil {
		return fmt.Errorf("timestamp normalization failed: %w", err)
	}

	// Walk in reverse so deeper directories are touched first.
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := unix.UtimesNanoAt(unix.AT_FDCWD, dirs[i], ts, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return fmt.Errorf("timestamp normalization of %s failed: %w", dirs[i], err)
		}
	}
	return nil
}
