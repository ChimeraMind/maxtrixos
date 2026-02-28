package filesystems

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// lstatTimes returns atime and mtime for path without following symlinks.
func lstatTimes(t *testing.T, path string) (atime, mtime time.Time) {
	t.Helper()
	var stat unix.Stat_t
	if err := unix.Lstat(path, &stat); err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	return time.Unix(stat.Atim.Sec, stat.Atim.Nsec), time.Unix(stat.Mtim.Sec, stat.Mtim.Nsec)
}

func TestNormalizeTimestamps_RegularFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(f, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	target := time.Unix(1, 0)
	if err := NormalizeTimestamps(dir, target); err != nil {
		t.Fatal(err)
	}

	_, mtime := lstatTimes(t, f)
	if !mtime.Equal(target) {
		t.Errorf("file mtime = %v, want %v", mtime, target)
	}
	_, dirMtime := lstatTimes(t, dir)
	if !dirMtime.Equal(target) {
		t.Errorf("dir mtime = %v, want %v", dirMtime, target)
	}
}

func TestNormalizeTimestamps_Symlink(t *testing.T) {
	dir := t.TempDir()
	realFile := filepath.Join(dir, "real")
	if err := os.WriteFile(realFile, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink("real", link); err != nil {
		t.Fatal(err)
	}

	target := time.Unix(1, 0)
	if err := NormalizeTimestamps(dir, target); err != nil {
		t.Fatal(err)
	}

	// The symlink itself should have the normalised timestamp.
	_, linkMtime := lstatTimes(t, link)
	if !linkMtime.Equal(target) {
		t.Errorf("symlink mtime = %v, want %v", linkMtime, target)
	}
	// The real file behind the symlink should also be normalised.
	_, realMtime := lstatTimes(t, realFile)
	if !realMtime.Equal(target) {
		t.Errorf("real file mtime = %v, want %v", realMtime, target)
	}
}

func TestNormalizeTimestamps_NestedDirs(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(sub, "deep.txt")
	if err := os.WriteFile(f, []byte("deep"), 0644); err != nil {
		t.Fatal(err)
	}

	target := time.Unix(1, 0)
	if err := NormalizeTimestamps(root, target); err != nil {
		t.Fatal(err)
	}

	// Check every directory in the chain plus the file.
	for _, p := range []string{
		root,
		filepath.Join(root, "a"),
		filepath.Join(root, "a", "b"),
		sub,
		f,
	} {
		_, mtime := lstatTimes(t, p)
		if !mtime.Equal(target) {
			t.Errorf("%s mtime = %v, want %v", p, mtime, target)
		}
	}
}

func TestNormalizeTimestamps_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	target := time.Unix(42, 0)
	if err := NormalizeTimestamps(dir, target); err != nil {
		t.Fatal(err)
	}

	_, mtime := lstatTimes(t, dir)
	if !mtime.Equal(target) {
		t.Errorf("empty dir mtime = %v, want %v", mtime, target)
	}
}

func TestNormalizeTimestamps_BrokenSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "broken")
	if err := os.Symlink("nonexistent", link); err != nil {
		t.Fatal(err)
	}

	target := time.Unix(1, 0)
	if err := NormalizeTimestamps(dir, target); err != nil {
		t.Fatal(err)
	}

	// Even a broken symlink should have its own timestamp set.
	_, linkMtime := lstatTimes(t, link)
	if !linkMtime.Equal(target) {
		t.Errorf("broken symlink mtime = %v, want %v", linkMtime, target)
	}
}

func TestNormalizeTimestamps_NonexistentRoot(t *testing.T) {
	err := NormalizeTimestamps("/tmp/does-not-exist-normalize-test", time.Unix(1, 0))
	if err == nil {
		t.Fatal("expected error for nonexistent root")
	}
}

func TestNormalizeTimestamps_CustomTime(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "file")
	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	target := time.Date(2020, 6, 15, 12, 0, 0, 0, time.UTC)
	if err := NormalizeTimestamps(dir, target); err != nil {
		t.Fatal(err)
	}

	_, mtime := lstatTimes(t, f)
	if !mtime.Equal(target) {
		t.Errorf("file mtime = %v, want %v", mtime, target)
	}
}
