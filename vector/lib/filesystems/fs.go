package filesystems

import (
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"matrixos/vector/lib/runner"
)

var (
	execRun            runner.Func               = runner.Run
	execOutput         runner.OutputFunc         = runner.Output
	execCombinedOutput runner.CombinedOutputFunc = runner.CombinedOutput
	ExecChrootRun      runner.ChrootRunFunc      = runner.ChrootRun
	ExecChrootOutput   runner.ChrootOutputFunc   = runner.ChrootOutput
	devMapperPrefix                              = "/dev/mapper"
	sysIoctl                                     = unix.Syscall
	sysLsetxattr                                 = unix.Lsetxattr
	sysLgetxattr                                 = unix.Lgetxattr
	sysLlistxattr                                = unix.Llistxattr

	// Mockable paths for block-device sysfs queries.
	sysClassBlockPath     = "/sys/class/block"
	devDiskByLabelPath    = "/dev/disk/by-label"
	devDiskByPartTypePath = "/dev/disk/by-parttypeuuid"
)

// BLKFLSBUF is the ioctl command to flush block device buffers.
// It is commonly 0x1261 on Linux.
const BLKFLSBUF = 0x1261

// PathMode represents the mode of a path.
type PathMode struct {
	Type   string      // E.g., "-", "d", "l"
	SetUID bool        // Set-user-ID bit
	SetGID bool        // Set-group-ID bit
	Sticky bool        // Sticky bit
	Perms  fs.FileMode // Stored as uint32, printed as octal
}

// PathInfo represents the information of a path in an ostree commit.
type PathInfo struct {
	Mode           *PathMode // Mode information of the path
	Uid            uint64    // User ID of the owner
	Gid            uint64    // Group ID of the owner
	Size           uint64    // Size of the file in bytes
	OSTreeChecksum string    // Checksum of the path if regular file
	Path           string    // Full path of the file
	Link           string    // Target of the symlink if Type is "l"
}

// Equals compares two PathInfo entries for metadata equality:
// type, permission bits, uid, gid, size, symlink target and checksums.
func (a *PathInfo) Equals(b *PathInfo) bool {
	if a.Mode.Type != b.Mode.Type {
		return false
	}
	if a.Mode.Perms != b.Mode.Perms {
		return false
	}
	if a.Mode.SetUID != b.Mode.SetUID || a.Mode.SetGID != b.Mode.SetGID || a.Mode.Sticky != b.Mode.Sticky {
		return false
	}
	if a.Uid != b.Uid || a.Gid != b.Gid {
		return false
	}
	if a.Size != b.Size {
		return false
	}
	if a.Link != b.Link {
		return false
	}
	aCksum := "0"
	bCksum := "0"
	if a.Mode.Type == "-" {
		aCksum = a.OSTreeChecksum
	}
	if b.Mode.Type == "-" {
		bCksum = b.OSTreeChecksum
	}
	if aCksum != bCksum {
		return false
	}
	return true
}

// String returns a short human-readable description of a PathInfo.
func (pi *PathInfo) String() string {
	if pi == nil {
		return "(absent)"
	}
	typ := "file"
	switch pi.Mode.Type {
	case "d":
		typ = "dir"
	case "l":
		typ = fmt.Sprintf("link -> %s", pi.Link)
	}
	return fmt.Sprintf("%s %04o uid=%d gid=%d size=%d, csum=%s",
		typ, pi.Mode.Perms, pi.Uid, pi.Gid, pi.Size, pi.OSTreeChecksum)
}

// ListContents lists the contents of a path on the filesystem.
// It walks the directory tree recursively and returns information
// about regular files, directories, and symlinks, ignoring everything else.
func ListContents(path string) ([]*PathInfo, error) {
	if path == "" {
		return nil, fmt.Errorf("missing path parameter")
	}

	otRegFileChecksum := func(p string) string {
		ck, err := OstreeChecksumFileAt(p, OstreeObjectTypeFile, OstreeChecksumFlagsNone)
		if err != nil {
			log.Printf("WARNING: failed to compute OSTree checksum for %s: %v. Using dummy checksum.\n", p, err)
			return "0"
		}
		return ck
	}

	var pis []*PathInfo

	err := filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		mode := info.Mode()
		ft := mode.Type()

		var typeStr string
		var otChksum string
		switch {
		case ft.IsRegular():
			typeStr = "-"
			otChksum = otRegFileChecksum(p)
		case ft.IsDir():
			typeStr = "d"
		case ft&fs.ModeSymlink != 0:
			typeStr = "l"
		default:
			// Ignore anything that is not a regular file, directory, or symlink
			return nil
		}

		pm := &PathMode{
			Type:   typeStr,
			SetUID: mode&fs.ModeSetuid != 0,
			SetGID: mode&fs.ModeSetgid != 0,
			Sticky: mode&fs.ModeSticky != 0,
			Perms:  mode.Perm(),
		}

		pi := &PathInfo{
			Mode:           pm,
			Size:           uint64(info.Size()),
			Path:           p,
			OSTreeChecksum: otChksum,
		}

		// Get UID/GID from the underlying syscall stat
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			pi.Uid = uint64(stat.Uid)
			pi.Gid = uint64(stat.Gid)
		}

		// Resolve symlink target
		if typeStr == "l" {
			target, err := os.Readlink(p)
			if err != nil {
				return err
			}
			pi.Link = target
		}

		pis = append(pis, pi)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return pis, nil
}

// DevicesSettle waits for udev events to settle.
func DevicesSettle() {
	execRun(&runner.Cmd{
		Name: "udevadm",
		Args: []string{"settle"},
	})
}

// FlushBlockDeviceBuffers flushes the buffers of a block device.
func FlushBlockDeviceBuffers(devPath string) error {
	if devPath == "" {
		return fmt.Errorf("missing devPath parameter")
	}

	f, err := os.Open(devPath)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, _, errno := sysIoctl(unix.SYS_IOCTL, f.Fd(), uintptr(BLKFLSBUF), 0); errno != 0 {
		return fmt.Errorf("ioctl BLKFLSBUF failed: %w", errno)
	}
	return nil
}

// GetLuksRootfsDevicePath returns the device path for a given LUKS name.
func GetLuksRootfsDevicePath(luksName string) (string, error) {
	if luksName == "" {
		return "", fmt.Errorf("missing luksName parameter")
	}
	return filepath.Join(devMapperPrefix, luksName), nil
}

// DeviceUUID returns the UUID of a given device path.
func DeviceUUID(devPath string) (string, error) {
	if devPath == "" {
		return "", fmt.Errorf("missing argument devpath")
	}
	return resolveDeviceAttribute(devPath, "UUID")
}

// DevicePartUUID returns the PARTUUID of a given device path.
func DevicePartUUID(devPath string) (string, error) {
	if devPath == "" {
		return "", fmt.Errorf("missing argument devpath")
	}
	return resolveDeviceAttribute(devPath, "PARTUUID")
}

// PathExists returns true if the path exists (file, directory, or other).
func PathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// FileExists returns true if path exists and is a regular file.
func FileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

// DirectoryExists returns true if path exists and is a directory.
func DirectoryExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// ListSubmounts returns a list of submounts for a given mountpoint
// by reading /proc/self/mountinfo and filtering by prefix.
func ListSubmounts(mnt string) ([]string, error) {
	if mnt == "" {
		return nil, fmt.Errorf("missing argument")
	}
	entries, err := listMountsByPrefix(mnt)
	if err != nil {
		return nil, err
	}

	var submounts []string
	for _, e := range entries {
		submounts = append(submounts, e.Mountpoint)
	}
	return submounts, nil
}

// CheckDirNotFsRoot checks if a directory is the root of the filesystem.
func CheckDirNotFsRoot(mnt string) error {
	if mnt == "" {
		return fmt.Errorf("missing mnt parameter")
	}

	rootStat, err := os.Stat("/")
	if err != nil {
		return err
	}
	mntStat, err := os.Stat(mnt)
	if err != nil {
		return err
	}

	if os.SameFile(rootStat, mntStat) {
		return fmt.Errorf("CRITICAL ERROR: %s IS MAPPED TO HOST ROOT. ABORTING", mnt)
	}
	return nil
}

// CommonRootfsMounts represents the common rootfs mounts that are typically
// set up for a container or chroot environment, such as /dev, /proc, and /run/lock.
type CommonRootfsMounts struct {
	mountPoint  string
	mounting    func(string)
	mounted     func(string)
	mounts      []string
	slaveMounts []string
}

// NewCommonRootfsMounts creates a new CommonRootfsMounts for the given mount point.
func NewCommonRootfsMounts(mnt string, mounting func(string), mounted func(string)) (*CommonRootfsMounts, error) {
	if mnt == "" {
		return nil, fmt.Errorf("missing mnt parameter")
	}

	return &CommonRootfsMounts{
		mountPoint: mnt,
		mounting:   mounting,
		mounted:    mounted,
		slaveMounts: []string{
			"/dev",
			"/dev/pts",
			"/sys",
		},
	}, nil
}

// add adds a mount to the list of mounts to be cleaned up later.
func (m *CommonRootfsMounts) add(mnt string) {
	log.Printf("Mounting: %s\n", mnt)
	m.mounts = append(m.mounts, mnt)
}

// Setup sets up the common rootfs mounts.
func (m *CommonRootfsMounts) Setup() error {
	if _, err := os.Stat(m.mountPoint); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", m.mountPoint)
	}
	if err := CheckDirNotFsRoot(m.mountPoint); err != nil {
		return err
	}

	for _, d := range m.slaveMounts {
		dst := filepath.Join(m.mountPoint, d)
		if err := os.MkdirAll(dst, 0755); err != nil {
			return err
		}
		m.mounting(dst)
		m.add(dst)
		if err := sysMount(d, dst, "", unix.MS_BIND, ""); err != nil {
			return fmt.Errorf("failed to bind mount %s: %w", d, err)
		}
		m.mounted(dst)
		if err := sysMount("", dst, "", unix.MS_SLAVE, ""); err != nil {
			return fmt.Errorf("failed to make slave %s: %w", dst, err)
		}
	}

	chrootDevShm := filepath.Join(m.mountPoint, "dev", "shm")
	if err := os.MkdirAll(chrootDevShm, 0755); err != nil {
		return err
	}
	const devShmFlags = unix.MS_NOSUID | unix.MS_NODEV
	m.mounting(chrootDevShm)
	m.add(chrootDevShm)
	if err := sysMount("devshm", chrootDevShm, "tmpfs", devShmFlags, "mode=1777"); err != nil {
		return fmt.Errorf("failed to mount devshm: %w", err)
	}
	m.mounted(chrootDevShm)

	chrootProc := filepath.Join(m.mountPoint, "proc")
	if err := os.MkdirAll(chrootProc, 0755); err != nil {
		return err
	}
	m.mounting(chrootProc)
	m.add(chrootProc)
	if err := sysMount("proc", chrootProc, "proc", 0, ""); err != nil {
		return fmt.Errorf("failed to mount proc: %w", err)
	}
	m.mounted(chrootProc)

	runLock := filepath.Join(m.mountPoint, "run", "lock")
	if err := os.MkdirAll(runLock, 0755); err != nil {
		return err
	}
	m.mounting(runLock)
	m.add(runLock)
	const runLockFlags = unix.MS_NOSUID | unix.MS_NODEV | unix.MS_NOEXEC | unix.MS_RELATIME
	if err := sysMount("none", runLock, "tmpfs", runLockFlags, "size=5120k"); err != nil {
		return fmt.Errorf("failed to mount run/lock: %w", err)
	}
	m.mounted(runLock)

	return nil
}

// Cleanup unmounts all the mounts that were set up by Setup.
func (m *CommonRootfsMounts) Cleanup() error {
	CleanupMounts(m.mounts)
	return nil
}

// BindMount binds a source directory to a destination directory.
func BindMount(src, dst string) error {
	if src == "" {
		return fmt.Errorf("missing src parameter")
	}
	if dst == "" {
		return fmt.Errorf("missing dst parameter")
	}

	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", src)
	}
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", dst)
	}

	if err := CheckDirNotFsRoot(src); err != nil {
		return err
	}
	if err := CheckDirNotFsRoot(dst); err != nil {
		return err
	}

	// log.Printf("Binding %s to %s", src, dst)
	if err := sysMount(src, dst, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("mount bind failed: %w", err)
	}
	if err := sysMount("", dst, "", unix.MS_SLAVE, ""); err != nil {
		return fmt.Errorf("mount make-slave failed: %w", err)
	}
	return nil
}

// BindUmount unmounts a bind mount.
func BindUmount(mnt string) error {
	if mnt == "" {
		return fmt.Errorf("missing mnt parameter")
	}
	if _, err := os.Stat(mnt); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", mnt)
	}
	if err := CheckDirNotFsRoot(mnt); err != nil {
		return err
	}
	CleanupMounts([]string{mnt})
	return nil
}

// BindMountDistdir binds the distfiles directory.
func BindMountDistdir(distfilesDir, rootfs string) (string, error) {
	if distfilesDir == "" {
		return "", fmt.Errorf("missing parameter distfilesDir")
	}
	if rootfs == "" {
		return "", fmt.Errorf("missing rootfs parameter")
	}

	if _, err := os.Stat(distfilesDir); os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist", distfilesDir)
	}
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist", rootfs)
	}

	dstDir := filepath.Join(rootfs, "var", "cache", "distfiles")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return "", err
	}
	return dstDir, BindMount(distfilesDir, dstDir)
}

// BindUmountDistdir unmounts the distfiles directory.
func BindUmountDistdir(rootfs string) error {
	if rootfs == "" {
		return fmt.Errorf("missing rootfs parameter")
	}
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", rootfs)
	}
	dstDir := filepath.Join(rootfs, "var", "cache", "distfiles")
	return BindUmount(dstDir)
}

// BindMountBinpkgs binds the binpkgs directory.
func BindMountBinpkgs(binpkgsDir, rootfs string) (string, error) {
	if binpkgsDir == "" {
		return "", fmt.Errorf("missing parameter binpkgsDir")
	}
	if rootfs == "" {
		return "", fmt.Errorf("missing rootfs parameter")
	}

	if _, err := os.Stat(binpkgsDir); os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist", binpkgsDir)
	}
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return "", fmt.Errorf("%s does not exist", rootfs)
	}

	dstDir := filepath.Join(rootfs, "var", "cache", "binpkgs")
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return "", err
	}
	return dstDir, BindMount(binpkgsDir, dstDir)
}

// BindUmountBinpkgs unmounts the binpkgs directory.
func BindUmountBinpkgs(rootfs string) error {
	if rootfs == "" {
		return fmt.Errorf("missing rootfs parameter")
	}
	if _, err := os.Stat(rootfs); os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist", rootfs)
	}
	dstDir := filepath.Join(rootfs, "var", "cache", "binpkgs")
	return BindUmount(dstDir)
}

// CheckFsCapabilitySupport checks if the filesystem has capability support.
var CheckFsCapabilitySupport = checkFsCapabilitySupport

func checkFsCapabilitySupport(testDir string) (bool, error) {
	if testDir == "" {
		return false, fmt.Errorf("missing parameter testDir")
	}

	tmpBin, err := os.CreateTemp(testDir, ".cap_test.*.bin")
	if err != nil {
		return false, err
	}
	tmpBin.Close()
	defer os.Remove(tmpBin.Name())

	// Use the setcap command (from libcap) rather than raw Lsetxattr.
	// The setcap binary handles VFS capability revision negotiation
	// (v2 vs v3 with rootid for user namespaces) automatically, whereas
	// a raw Lsetxattr with a hardcoded v2 struct will fail when the
	// kernel requires v3.
	if err := execRun(&runner.Cmd{
		Name: "setcap",
		Args: []string{"cap_net_raw+ep", tmpBin.Name()},
	}); err != nil {
		log.Println("WARNING: System/FS does not allow setting capabilities.")
		return false, nil
	}

	tmpCopy := tmpBin.Name() + ".copy"
	defer os.Remove(tmpCopy)

	if err := CopyFilePreserveXattrs(tmpBin.Name(), tmpCopy); err != nil {
		return false, err
	}

	// Verify the capability survived the copy using getcap, matching
	// the bash: getcap "$tmp_copy" | grep -q "cap_net_raw[=+]ep"
	out, err := execCombinedOutput(&runner.Cmd{
		Name: "getcap",
		Args: []string{tmpCopy},
	})
	if err != nil {
		return false, nil
	}

// CopyFile copies a file from src to dst atomically. It writes to a temporary
// file first, syncs it, then renames to the final destination. This ensures the
// destination is never left in a partial state.
func CopyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst + ".tmp")
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	err = destFile.Sync()
	if err != nil {
		return err
	}
	sourceFile.Close()
	destFile.Close()

	return os.Rename(dst+".tmp", dst)
}

// CopyDirPreserve recursively copies a directory tree from src to dst,
// preserving permissions and extended attributes. Symlinks are recreated
// as symlinks. This is the native Go equivalent of "cp -rp".
func CopyDirPreserve(src, dst string) error {
	srcInfo, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("failed to stat source %s: %w", src, err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("source %s is not a directory", src)
	}

	if err := os.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("failed to create destination %s: %w", dst, err)
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		info, err := d.Info()
		if err != nil {
			return err
		}

		// Handle symlinks.
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			if err != nil {
				return fmt.Errorf("failed to read symlink %s: %w", path, err)
			}
			return os.Symlink(target, dstPath)
		}

		// Handle directories.
		if d.IsDir() {
			if relPath == "." {
				return nil // already created above
			}
			if err := os.MkdirAll(dstPath, info.Mode().Perm()); err != nil {
				return err
			}
			// Preserve sticky, setuid, setgid bits that MkdirAll ignores.
			specialBits := info.Mode() & (os.ModeSticky | os.ModeSetuid | os.ModeSetgid)
			if specialBits != 0 {
				return os.Chmod(dstPath, info.Mode().Perm()|specialBits)
			}
			return nil
		}

		// Handle regular files.
		return CopyFilePreserveXattrs(path, dstPath)
	})
}

// CopyFilePreserveXattrs copies a file from src to dst, preserving permissions
// and extended attributes (xattrs). This is equivalent to "cp -a" for regular files.
func CopyFilePreserveXattrs(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	// Copy extended attributes (includes security.capability)
	attrs, err := sysLlistxattr(src, nil)
	if err != nil || attrs == 0 {
		return nil // no xattrs or not supported
	}
	buf := make([]byte, attrs)
	attrs, err = sysLlistxattr(src, buf)
	if err != nil {
		return nil
	}

	// xattr names are null-terminated strings packed together
	for _, name := range strings.Split(strings.TrimRight(string(buf[:attrs]), "\x00"), "\x00") {
		if name == "" {
			continue
		}
		sz, err := sysLgetxattr(src, name, nil)
		if err != nil {
			continue
		}
		val := make([]byte, sz)
		_, err = sysLgetxattr(src, name, val)
		if err != nil {
			continue
		}
		sysLsetxattr(dst, name, val, 0)
	}
	return nil
}

// CheckHardlinkPreservation verifies that hardlinks are preserved between source and destination.
func CheckHardlinkPreservation(src, dst string) error {
	if src == "" || dst == "" {
		return fmt.Errorf("missing parameter (src: %s, dst: %s)", src, dst)
	}
	log.Printf("Checking hardlink preservation from %s to %s...", src, dst)

	// 1. Walk the source directory to find files with multiple links.
	// 2. Track Inodes to find the first pair of files sharing the same inode.
	// Using a map for O(1) checks.

	// Map from Inode (uint64) -> Path
	seenInodes := make(map[uint64]string)

	var file1Src, file2Src string
	foundPair := false

	// Sentinel error to stop walking early
	errFoundPair := fmt.Errorf("found pair")

	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}

		sys, ok := info.Sys().(*syscall.Stat_t)
		if !ok {
			return nil
		}

		if sys.Nlink > 1 {
			if existingPath, ok := seenInodes[sys.Ino]; ok {
				file1Src = existingPath
				file2Src = path
				foundPair = true
				return errFoundPair
			}
			seenInodes[sys.Ino] = path
		}
		return nil
	})

	if err != nil && err != errFoundPair {
		return fmt.Errorf("error walking source directory: %w", err)
	}

	if !foundPair {
		log.Println("WARNING: no hardlinked file pairs found in source. Cannot verify.")
		return nil
	}

	relPath1, err := filepath.Rel(src, file1Src)
	if err != nil {
		return err
	}
	relPath2, err := filepath.Rel(src, file2Src)
	if err != nil {
		return err
	}

	file1Dst := filepath.Join(dst, relPath1)
	file2Dst := filepath.Join(dst, relPath2)

	info1, err := os.Stat(file1Dst)
	if err != nil {
		return err
	}
	info2, err := os.Stat(file2Dst)
	if err != nil {
		return err
	}

	stat1, ok1 := info1.Sys().(*syscall.Stat_t)
	stat2, ok2 := info2.Sys().(*syscall.Stat_t)

	if !ok1 || !ok2 {
		return fmt.Errorf("could not get inode info")
	}

	if stat1.Ino != stat2.Ino {
		return fmt.Errorf(
			"CRITICAL: hardlinks BROKEN! Files were duplicated.\n  File 1: %s (inode: %d)\n  File 2: %s (inode: %d)",
			file1Dst, stat1.Ino, file2Dst, stat2.Ino,
		)
	}

	log.Printf("SUCCESS: hardlinks preserved (Inode: %d).", stat1.Ino)
	return nil
}

// CheckDirIsRoot checks if a directory is the root of the filesystem and exits if it is.
func CheckDirIsRoot(chrootDir string) error {
	if chrootDir == "" {
		return fmt.Errorf("missing chrootDir parameter")
	}
	rootStat, err := os.Stat("/")
	if err != nil {
		return err
	}
	chrootStat, err := os.Stat(chrootDir)
	if err != nil {
		return err
	}
	if os.SameFile(rootStat, chrootStat) {
		return fmt.Errorf("CRITICAL ERROR: %s IS MAPPED TO HOST ROOT. ABORTING", chrootDir)
	}
	return nil
}

// CheckDirsSameFilesystem checks if two directories are on the same filesystem.
func CheckDirsSameFilesystem(src, dst string) (bool, error) {
	if src == "" || dst == "" {
		return false, fmt.Errorf("missing parameters src or dst")
	}
	srcStat, err := os.Stat(src)
	if err != nil {
		return false, err
	}
	dstStat, err := os.Stat(dst)
	if err != nil {
		return false, err
	}
	return srcStat.Sys().(*syscall.Stat_t).Dev == dstStat.Sys().(*syscall.Stat_t).Dev, nil
}

// CreateTempDir creates a temporary directory.
func CreateTempDir(parentDir, prefix string) (string, error) {
	if parentDir == "" {
		return "", fmt.Errorf("missing parentDir parameter")
	}
	if prefix == "" {
		prefix = "tmp"
	}
	// os.MkdirTemp is the replacement for ioutil.TempDir
	tempDir, err := os.MkdirTemp(parentDir, prefix)
	if err != nil {
		return "", fmt.Errorf("failed to create temporary directory: %w", err)
	}
	return tempDir, nil
}

// CreateTempFile creates a temporary file.
func CreateTempFile(parentDir, prefix string) (*os.File, error) {
	if parentDir == "" {
		return nil, fmt.Errorf("missing parentDir parameter")
	}
	if prefix == "" {
		prefix = "tmp"
	}
	// os.CreateTemp is the replacement for ioutil.TempFile
	tempFile, err := os.CreateTemp(parentDir, prefix)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary file: %w", err)
	}
	return tempFile, nil
}

// RemoveFileWithGlobOptions holds options for RemoveFileWithGlob.
type RemoveFileWithGlobOptions struct {
	Stdout io.Writer
	Stderr io.Writer
}

// RemoveFileWithGlob removes files matching a glob pattern.
func RemoveFileWithGlob(target string, opts RemoveFileWithGlobOptions) error {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	matches, err := filepath.Glob(target)
	if err != nil {
		return err
	}

	if len(matches) == 0 {
		fmt.Fprintf(stdout, "Removing (glob): %s does not exist\n", target)
	}

	for _, match := range matches {
		fmt.Fprintf(stdout, "Removing file (glob): %s\n", match)
		err := os.Remove(match)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveDirOptions holds options for RemoveDir.
type RemoveDirOptions struct {
	Stdout io.Writer
	Stderr io.Writer
}

// RemoveDir removes a directory.
func RemoveDir(target string, opts RemoveDirOptions) error {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		fmt.Fprintf(stdout, "Removing: %s does not exist\n", target)
		return nil
	}
	fmt.Fprintf(stdout, "Removing %s\n", target)
	return os.RemoveAll(target)
}

// EmptyDirOptions holds options for EmptyDir.
type EmptyDirOptions struct {
	Stdout io.Writer
	Stderr io.Writer
}

// EmptyDir empties a directory.
func EmptyDir(target string, opts EmptyDirOptions) error {
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		fmt.Fprintf(stdout, "Emptying: %s does not exist\n", target)
		return nil
	}
	fmt.Fprintf(stdout, "Emptying directory %s\n", target)

	entries, err := os.ReadDir(target)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(target, entry.Name())
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

// DirEmpty checks if a directory is empty.
func DirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()

	_, err = f.ReadDir(1)
	if err == io.EOF {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return false, nil
}

// ChrootRun runs a command in a chroot environment using unshare,
// wiring stdin/stdout/stderr.
func ChrootRun(chrootDir, chrootExec string, args ...string) error {
	cmd := runner.ChrootCmd{
		Cmd: runner.Cmd{
			Name:   chrootExec,
			Args:   args,
			Stdin:  os.Stdin,
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		},
		ChrootDir: chrootDir,
	}
	return ExecChrootRun(&cmd)
}

// ChrootOutput runs a command in a chroot environment using unshare
// and returns its standard output.
func ChrootOutput(chrootDir, chrootExec string, args ...string) ([]byte, error) {
	return ExecChrootOutput(&runner.ChrootCmd{
		Cmd:       runner.Cmd{Name: chrootExec, Args: args},
		ChrootDir: chrootDir,
	})
}

// BlockDeviceNthPartition returns the device path of the nth partition
// (1-based) on a block device by scanning sysfs for child partitions.
func BlockDeviceNthPartition(blockDevice string, nth int) (string, error) {
	if blockDevice == "" {
		return "", fmt.Errorf("missing blockDevice parameter")
	}
	if nth <= 0 {
		return "", fmt.Errorf("invalid nth parameter: %d", nth)
	}

	parentBase := filepath.Base(blockDevice)
	parentSysfs := filepath.Join(sysClassBlockPath, parentBase)

	entries, err := os.ReadDir(parentSysfs)
	if err != nil {
		return "", fmt.Errorf("cannot read sysfs for %s: %w", blockDevice, err)
	}

	nthStr := strconv.Itoa(nth)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Partition directories start with the parent device name.
		if !strings.HasPrefix(e.Name(), parentBase) {
			continue
		}
		// Read the partition number from sysfs.
		partFile := filepath.Join(parentSysfs, e.Name(), "partition")
		data, err := readFileBytes(partFile)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == nthStr {
			return filepath.Join(filepath.Dir(blockDevice), e.Name()), nil
		}
	}

	return "", fmt.Errorf("partition %d not found on %s", nth, blockDevice)
}

// BlockDeviceForPartition returns the parent block device for a partition
// path by resolving the partition's sysfs entry and walking up to the
// parent device.
func BlockDeviceForPartition(partitionPath string) (string, error) {
	if partitionPath == "" {
		return "", fmt.Errorf("missing partitionPath parameter")
	}

	partBase := filepath.Base(partitionPath)
	partSysfs := filepath.Join(sysClassBlockPath, partBase)

	// Verify this is actually a partition by checking the "partition" file.
	partFile := filepath.Join(partSysfs, "partition")
	if _, err := readFileBytes(partFile); err != nil {
		return "", fmt.Errorf("not a partition or cannot read sysfs for %s: %w", partitionPath, err)
	}

	// The real sysfs path for a partition is:
	//   /sys/devices/.../sdX/sdX1
	// Resolving the sysfs symlink and going up one directory gives the parent.
	realPath, err := filepath.EvalSymlinks(partSysfs)
	if err != nil {
		return "", fmt.Errorf("cannot resolve sysfs path for %s: %w", partitionPath, err)
	}

	parentSysfs := filepath.Dir(realPath)
	parentName := filepath.Base(parentSysfs)

	// Verify the parent is a valid block device.
	parentDev := filepath.Join(filepath.Dir(partitionPath), parentName)
	return parentDev, nil
}

// PartitionNumber returns the partition number of a partition device
// by reading its sysfs "partition" attribute.
func PartitionNumber(partitionPath string) (string, error) {
	if partitionPath == "" {
		return "", fmt.Errorf("missing partitionPath parameter")
	}

	partBase := filepath.Base(partitionPath)
	partFile := filepath.Join(sysClassBlockPath, partBase, "partition")
	data, err := readFileBytes(partFile)
	if err != nil {
		return "", fmt.Errorf("cannot read partition number for %s: %w", partitionPath, err)
	}

	return strings.TrimSpace(string(data)), nil
}

// PartitionLabel returns the filesystem label of a partition device.
func PartitionLabel(partitionPath string) (string, error) {
	if partitionPath == "" {
		return "", fmt.Errorf("missing partitionPath parameter")
	}

	label, err := resolveDeviceAttribute(partitionPath, "LABEL")
	if err != nil {
		// No label is not necessarily an error; some partitions have none.
		return "", nil
	}
	return label, nil
}

// PartitionType returns the partition type GUID (uppercased) for a
// partition device.
func PartitionType(partitionPath string) (string, error) {
	if partitionPath == "" {
		return "", fmt.Errorf("missing partitionPath parameter")
	}

	partType, err := resolveDeviceAttribute(partitionPath, "PARTTYPE")
	if err != nil {
		return "", fmt.Errorf("cannot determine partition type for %s: %w", partitionPath, err)
	}
	return strings.ToUpper(partType), nil
}

// PrintDirectoryTree walks a directory tree rooted at root and prints every
// path to w, one per line – equivalent to running "find <root>".
func PrintDirectoryTree(w io.Writer, root string) error {
	if root == "" {
		return fmt.Errorf("missing root parameter")
	}
	return filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			// Report inaccessible paths but keep walking.
			fmt.Fprintf(w, "%s [error: %v]\n", path, err)
			return nil
		}
		fmt.Fprintln(w, path)
		return nil
	})
}

// BlockDevicePartitionInfo holds blkid-style attributes for a single device.
type BlockDevicePartitionInfo struct {
	Device   string // e.g. /dev/loop0p1
	UUID     string // filesystem UUID
	PartUUID string // GPT partition UUID
	Label    string // filesystem label
	FSType   string // filesystem type (from mountinfo)
	PartType string // GPT partition type GUID
}

// String formats the info in a blkid-like output line:
//
//	/dev/loop0p1: UUID="..." PARTUUID="..." LABEL="..." TYPE="..." PARTTYPE="..."
func (bi *BlockDevicePartitionInfo) String() string {
	var parts []string
	if bi.UUID != "" {
		parts = append(parts, fmt.Sprintf("UUID=%q", bi.UUID))
	}
	if bi.PartUUID != "" {
		parts = append(parts, fmt.Sprintf("PARTUUID=%q", bi.PartUUID))
	}
	if bi.Label != "" {
		parts = append(parts, fmt.Sprintf("LABEL=%q", bi.Label))
	}
	if bi.FSType != "" {
		parts = append(parts, fmt.Sprintf("TYPE=%q", bi.FSType))
	}
	if bi.PartType != "" {
		parts = append(parts, fmt.Sprintf("PARTTYPE=%q", bi.PartType))
	}
	return fmt.Sprintf("%s: %s", bi.Device, strings.Join(parts, " "))
}

// BlockDeviceInfo collects blkid-style information for all partitions on
// a block device (e.g. /dev/loop0) by querying sysfs and /dev/disk/by-*
// symlinks. The filesystem type is resolved from /proc/self/mountinfo
// when the partition is currently mounted.
func BlockDeviceInfo(blockDevice string) ([]BlockDevicePartitionInfo, error) {
	if blockDevice == "" {
		return nil, fmt.Errorf("missing blockDevice parameter")
	}

	parentBase := filepath.Base(blockDevice)
	parentSysfs := filepath.Join(sysClassBlockPath, parentBase)

	entries, err := os.ReadDir(parentSysfs)
	if err != nil {
		return nil, fmt.Errorf("cannot read sysfs for %s: %w", blockDevice, err)
	}

	var infos []BlockDevicePartitionInfo
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), parentBase) {
			continue
		}
		// Verify it's a partition by checking the "partition" sysfs file.
		partFile := filepath.Join(parentSysfs, e.Name(), "partition")
		if _, err := readFileBytes(partFile); err != nil {
			continue
		}

		partDev := filepath.Join(filepath.Dir(blockDevice), e.Name())
		info := BlockDevicePartitionInfo{Device: partDev}

		// UUID (best-effort, some partitions may not have one).
		if uuid, err := resolveDeviceAttribute(partDev, "UUID"); err == nil {
			info.UUID = uuid
		}
		// PARTUUID
		if partuuid, err := resolveDeviceAttribute(partDev, "PARTUUID"); err == nil {
			info.PartUUID = partuuid
		}
		// LABEL
		if label, err := resolveDeviceAttribute(partDev, "LABEL"); err == nil {
			info.Label = label
		}
		// PARTTYPE
		if partType, err := resolveDeviceAttribute(partDev, "PARTTYPE"); err == nil {
			info.PartType = strings.ToUpper(partType)
		}
		// FSType – resolve via mountinfo if the partition is mounted.
		if mounts, err := findMountsBySource(partDev); err == nil && len(mounts) > 0 {
			info.FSType = mounts[0].FSType
		}

		infos = append(infos, info)
	}

	return infos, nil
}

// PrintBlockDeviceInfo writes blkid-style information for all partitions
// on blockDevice to w.
func PrintBlockDeviceInfo(w io.Writer, blockDevice string) error {
	infos, err := BlockDeviceInfo(blockDevice)
	if err != nil {
		return err
	}
	for _, info := range infos {
		fmt.Fprintln(w, info.String())
	}
	return nil
}

// BlockDeviceNthPartition returns the device path of the nth partition
// (1-based) on a block device by scanning sysfs for child partitions.
func BlockDeviceNthPartition(blockDevice string, nth int) (string, error) {
	if blockDevice == "" {
		return "", fmt.Errorf("missing blockDevice parameter")
	}
	if nth <= 0 {
		return "", fmt.Errorf("invalid nth parameter: %d", nth)
	}

	parentBase := filepath.Base(blockDevice)
	parentSysfs := filepath.Join(sysClassBlockPath, parentBase)

	entries, err := os.ReadDir(parentSysfs)
	if err != nil {
		return "", fmt.Errorf("cannot read sysfs for %s: %w", blockDevice, err)
	}

	nthStr := strconv.Itoa(nth)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Partition directories start with the parent device name.
		if !strings.HasPrefix(e.Name(), parentBase) {
			continue
		}
		// Read the partition number from sysfs.
		partFile := filepath.Join(parentSysfs, e.Name(), "partition")
		data, err := readFileBytes(partFile)
		if err != nil {
			continue
		}
		if strings.TrimSpace(string(data)) == nthStr {
			return filepath.Join(filepath.Dir(blockDevice), e.Name()), nil
		}
	}

	return "", fmt.Errorf("partition %d not found on %s", nth, blockDevice)
}

// BlockDeviceForPartition returns the parent block device for a partition
// path by resolving the partition's sysfs entry and walking up to the
// parent device.
func BlockDeviceForPartition(partitionPath string) (string, error) {
	if partitionPath == "" {
		return "", fmt.Errorf("missing partitionPath parameter")
	}

	partBase := filepath.Base(partitionPath)
	partSysfs := filepath.Join(sysClassBlockPath, partBase)

	// Verify this is actually a partition by checking the "partition" file.
	partFile := filepath.Join(partSysfs, "partition")
	if _, err := readFileBytes(partFile); err != nil {
		return "", fmt.Errorf("not a partition or cannot read sysfs for %s: %w", partitionPath, err)
	}

	// The real sysfs path for a partition is:
	//   /sys/devices/.../sdX/sdX1
	// Resolving the sysfs symlink and going up one directory gives the parent.
	realPath, err := resolveDeviceLink(partSysfs)
	if err != nil {
		return "", fmt.Errorf("cannot resolve sysfs path for %s: %w", partitionPath, err)
	}

	parentSysfs := filepath.Dir(realPath)
	parentName := filepath.Base(parentSysfs)

	// Verify the parent is a valid block device.
	parentDev := filepath.Join(filepath.Dir(partitionPath), parentName)
	return parentDev, nil
}

// PartitionNumber returns the partition number of a partition device
// by reading its sysfs "partition" attribute.
func PartitionNumber(partitionPath string) (string, error) {
	if partitionPath == "" {
		return "", fmt.Errorf("missing partitionPath parameter")
	}

	partBase := filepath.Base(partitionPath)
	partFile := filepath.Join(sysClassBlockPath, partBase, "partition")
	data, err := readFileBytes(partFile)
	if err != nil {
		return "", fmt.Errorf("cannot read partition number for %s: %w", partitionPath, err)
	}

	return strings.TrimSpace(string(data)), nil
}

// PartitionLabel returns the filesystem label of a partition device
// by scanning /dev/disk/by-label/ for a symlink pointing to the device.
func PartitionLabel(partitionPath string) (string, error) {
	if partitionPath == "" {
		return "", fmt.Errorf("missing partitionPath parameter")
	}

	label, err := resolveDeviceAttribute(partitionPath, devDiskByLabelPath)
	if err != nil {
		// No label is not necessarily an error; some partitions have none.
		return "", nil
	}
	return label, nil
}

// PartitionType returns the partition type GUID (uppercased) for a
// partition device by scanning /dev/disk/by-parttypeuuid/ for a symlink
// pointing to the device.
func PartitionType(partitionPath string) (string, error) {
	if partitionPath == "" {
		return "", fmt.Errorf("missing partitionPath parameter")
	}

	partType, err := resolveDeviceAttribute(partitionPath, devDiskByPartTypePath)
	if err != nil {
		return "", fmt.Errorf("cannot determine partition type for %s: %w", partitionPath, err)
	}
	return strings.ToUpper(partType), nil
}

// PrintDirectoryTree walks a directory tree rooted at root and prints every
// path to w, one per line – equivalent to running "find <root>".
func PrintDirectoryTree(w io.Writer, root string) error {
	if root == "" {
		return fmt.Errorf("missing root parameter")
	}
	return filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			// Report inaccessible paths but keep walking.
			fmt.Fprintf(w, "%s [error: %v]\n", path, err)
			return nil
		}
		fmt.Fprintln(w, path)
		return nil
	})
}

// BlockDevicePartitionInfo holds blkid-style attributes for a single device.
type BlockDevicePartitionInfo struct {
	Device   string // e.g. /dev/loop0p1
	UUID     string // filesystem UUID
	PartUUID string // GPT partition UUID
	Label    string // filesystem label
	FSType   string // filesystem type (from mountinfo)
	PartType string // GPT partition type GUID
}

// String formats the info in a blkid-like output line:
//
//	/dev/loop0p1: UUID="..." PARTUUID="..." LABEL="..." TYPE="..." PARTTYPE="..."
func (bi *BlockDevicePartitionInfo) String() string {
	var parts []string
	if bi.UUID != "" {
		parts = append(parts, fmt.Sprintf("UUID=%q", bi.UUID))
	}
	if bi.PartUUID != "" {
		parts = append(parts, fmt.Sprintf("PARTUUID=%q", bi.PartUUID))
	}
	if bi.Label != "" {
		parts = append(parts, fmt.Sprintf("LABEL=%q", bi.Label))
	}
	if bi.FSType != "" {
		parts = append(parts, fmt.Sprintf("TYPE=%q", bi.FSType))
	}
	if bi.PartType != "" {
		parts = append(parts, fmt.Sprintf("PARTTYPE=%q", bi.PartType))
	}
	return fmt.Sprintf("%s: %s", bi.Device, strings.Join(parts, " "))
}

// BlockDeviceInfo collects blkid-style information for all partitions on
// a block device (e.g. /dev/loop0) by querying sysfs and /dev/disk/by-*
// symlinks. The filesystem type is resolved from /proc/self/mountinfo
// when the partition is currently mounted.
func BlockDeviceInfo(blockDevice string) ([]BlockDevicePartitionInfo, error) {
	if blockDevice == "" {
		return nil, fmt.Errorf("missing blockDevice parameter")
	}

	parentBase := filepath.Base(blockDevice)
	parentSysfs := filepath.Join(sysClassBlockPath, parentBase)

	entries, err := os.ReadDir(parentSysfs)
	if err != nil {
		return nil, fmt.Errorf("cannot read sysfs for %s: %w", blockDevice, err)
	}

	var infos []BlockDevicePartitionInfo
	for _, e := range entries {
		if !e.IsDir() || !strings.HasPrefix(e.Name(), parentBase) {
			continue
		}
		// Verify it's a partition by checking the "partition" sysfs file.
		partFile := filepath.Join(parentSysfs, e.Name(), "partition")
		if _, err := readFileBytes(partFile); err != nil {
			continue
		}

		partDev := filepath.Join(filepath.Dir(blockDevice), e.Name())
		info := BlockDevicePartitionInfo{Device: partDev}

		// UUID (best-effort, some partitions may not have one).
		if uuid, err := resolveDeviceAttribute(partDev, devDiskByUUIDPath); err == nil {
			info.UUID = uuid
		}
		// PARTUUID
		if partuuid, err := resolveDeviceAttribute(partDev, devDiskByPartUUIDPath); err == nil {
			info.PartUUID = partuuid
		}
		// LABEL
		if label, err := resolveDeviceAttribute(partDev, devDiskByLabelPath); err == nil {
			info.Label = label
		}
		// PARTTYPE
		if partType, err := resolveDeviceAttribute(partDev, devDiskByPartTypePath); err == nil {
			info.PartType = strings.ToUpper(partType)
		}
		// FSType – resolve via mountinfo if the partition is mounted.
		if mounts, err := findMountsBySource(partDev); err == nil && len(mounts) > 0 {
			info.FSType = mounts[0].FSType
		}

		infos = append(infos, info)
	}

	return infos, nil
}

// PrintBlockDeviceInfo writes blkid-style information for all partitions
// on blockDevice to w.
func PrintBlockDeviceInfo(w io.Writer, blockDevice string) error {
	infos, err := BlockDeviceInfo(blockDevice)
	if err != nil {
		return err
	}
	for _, info := range infos {
		fmt.Fprintln(w, info.String())
	}
	return nil
}
