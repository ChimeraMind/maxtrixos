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
	sysClassBlockPath = "/sys/class/block"
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

// MountpointToDevice returns the device path for a given mountpoint
// by reading /proc/self/mountinfo.
func MountpointToDevice(mnt string) (string, error) {
	if mnt == "" {
		return "", fmt.Errorf("missing mnt parameter")
	}

	entry, err := findMountByTarget(mnt)
	if err != nil {
		return "", fmt.Errorf("no device found for mountpoint %s", mnt)
	}
	if entry.Source == "" {
		return "", fmt.Errorf("no device found for mountpoint %s", mnt)
	}
	return entry.Source, nil
}

// MountpointToUUID returns the UUID for a given mountpoint by reading
// /proc/self/mountinfo and resolving the device UUID.
func MountpointToUUID(mnt string) (string, error) {
	if mnt == "" {
		return "", fmt.Errorf("missing mnt parameter")
	}

	entry, err := findMountContainingPath(mnt)
	if err != nil {
		return "", fmt.Errorf("no UUID found for mountpoint %s", mnt)
	}
	uuid, err := resolveDeviceAttribute(entry.Source, "UUID")
	if err != nil {
		return "", fmt.Errorf("no UUID found for mountpoint %s: %w", mnt, err)
	}
	return uuid, nil
}

// MountpointToFSType returns the filesystem type for a given mountpoint
// by reading /proc/self/mountinfo.
func MountpointToFSType(mnt string) (string, error) {
	if mnt == "" {
		return "", fmt.Errorf("missing mnt parameter")
	}

	entry, err := findMountContainingPath(mnt)
	if err != nil {
		return "", fmt.Errorf("no FSTYPE found for mountpoint %s", mnt)
	}
	if entry.FSType == "" {
		return "", fmt.Errorf("no FSTYPE found for mountpoint %s", mnt)
	}
	return entry.FSType, nil
}

// CleanupMountsOptions represents the options for cleaning up mounts.
type CleanupMountsOptions struct {
	Mounts []string
	Stdout io.Writer
	Stderr io.Writer
}

// CleanupMounts unmounts a list of mounts in reverse order.
func CleanupMounts(opts CleanupMountsOptions) {
	mounts := opts.Mounts
	if opts.Stdout == nil {
		opts.Stdout = os.Stdout
	}
	if opts.Stderr == nil {
		opts.Stderr = os.Stderr
	}

	DevicesSettle()
	for i := len(mounts) - 1; i >= 0; i-- {
		mnt := mounts[i]
		mounted, _ := isMounted(mnt)
		if !mounted {
			continue
		}
		fmt.Fprintf(opts.Stdout, "Unmounting %s ...\n", mnt)
		if err := Unmount(mnt, 0); err != nil {
			FlushBlockDeviceBuffers(mnt)
			fmt.Fprintf(opts.Stderr, "Unable to umount %s: %v", mnt, err)
			if entry, mntErr := findMountByTarget(mnt); mntErr == nil {
				fmt.Fprintf(opts.Stderr, "%s\n", entry.String())
			}
			fmt.Fprintf(opts.Stderr, "For safety, calling umount -l %s\n", mnt)
			Unmount(mnt, unix.MNT_DETACH)
			continue
		}
	}
	DevicesSettle()
}

// CleanupCryptsetupDevices closes a list of cryptsetup devices.
func CleanupCryptsetupDevices(devices []string) {
	DevicesSettle()
	for _, cd := range devices {
		cdpath, err := GetLuksRootfsDevicePath(cd)
		if err != nil {
			log.Println(err)
			continue
		}
		if _, err := os.Stat(cdpath); os.IsNotExist(err) {
			continue
		}

		fmt.Printf("Closing LUKS device: %s ...\n", cd)
		FlushBlockDeviceBuffers(cdpath)
		cmd := &runner.Cmd{
			Name:   "cryptsetup",
			Args:   []string{"close", cd},
			Stdout: os.Stdout,
			Stderr: os.Stderr,
		}
		if err := execRun(cmd); err != nil {
			fmt.Fprintf(os.Stderr, "Unable to cryptsetup close %s\n", cdpath)
			if entries, mntErr := findMountsBySource(cdpath); mntErr == nil {
				fmt.Fprintf(os.Stderr, "%s\n", formatMountEntries(entries))
			}
			continue
		}
	}
	DevicesSettle()
}

// CleanupLoopDevices detaches a list of loop devices.
func CleanupLoopDevices(devices []string) {
	DevicesSettle()
	for _, ld := range devices {
		if _, err := os.Stat(ld); os.IsNotExist(err) {
			continue
		}
		l := NewLoopFromDevice(ld)
		if l.BackingFile() == "" {
			continue
		}

		fmt.Printf("Cleaning loop device %s ...\n", ld)

		if err := l.Detach(); err != nil {
			FlushBlockDeviceBuffers(ld)
			fmt.Fprintf(os.Stderr, "Unable to close loop device %s\n", ld)
			if entries, mntErr := findMountsBySource(ld); mntErr == nil {
				fmt.Fprintf(os.Stderr, "%s\n", formatMountEntries(entries))
			}
			continue
		}
	}
	DevicesSettle()
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
	stdout      io.Writer
	stderr      io.Writer
}

// CommonRootfsMountsOptions represents the options for setting up common rootfs mounts.
type CommonRootfsMountsOptions struct {
	MountPoint string
	Mounting   func(string)
	Mounted    func(string)
	Stdout     io.Writer
	Stderr     io.Writer
}

// NewCommonRootfsMounts creates a new CommonRootfsMounts for the given mount point.
func NewCommonRootfsMounts(opts CommonRootfsMountsOptions) (*CommonRootfsMounts, error) {
	if opts.MountPoint == "" {
		return nil, fmt.Errorf("missing mount point parameter")
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	return &CommonRootfsMounts{
		mountPoint: opts.MountPoint,
		mounting:   opts.Mounting,
		mounted:    opts.Mounted,
		slaveMounts: []string{
			"/dev",
			"/dev/pts",
			"/sys",
		},
		stdout: stdout,
		stderr: stderr,
	}, nil
}

// add adds a mount to the list of mounts to be cleaned up later.
func (m *CommonRootfsMounts) add(mnt string) {
	fmt.Fprintf(m.stdout, "Mounting: %s ...\n", mnt)
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
		if err := Mount(d, dst, "", unix.MS_BIND, ""); err != nil {
			return fmt.Errorf("failed to bind mount %s: %w", d, err)
		}
		m.mounted(dst)
		if err := Mount("", dst, "", unix.MS_SLAVE, ""); err != nil {
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
	if err := Mount("devshm", chrootDevShm, "tmpfs", devShmFlags, "mode=1777"); err != nil {
		return fmt.Errorf("failed to mount devshm: %w", err)
	}
	m.mounted(chrootDevShm)

	chrootProc := filepath.Join(m.mountPoint, "proc")
	if err := os.MkdirAll(chrootProc, 0755); err != nil {
		return err
	}
	m.mounting(chrootProc)
	m.add(chrootProc)
	if err := Mount("proc", chrootProc, "proc", 0, ""); err != nil {
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
	if err := Mount("none", runLock, "tmpfs", runLockFlags, "size=5120k"); err != nil {
		return fmt.Errorf("failed to mount run/lock: %w", err)
	}
	m.mounted(runLock)

	return nil
}

// Cleanup unmounts all the mounts that were set up by Setup.
func (m *CommonRootfsMounts) Cleanup() error {
	opts := CleanupMountsOptions{
		Mounts: m.mounts,
		Stdout: m.stdout,
		Stderr: m.stderr,
	}
	CleanupMounts(opts)
	return nil
}

// BindMountOptions represents the options for creating a bind mount.
type BindMountOptions struct {
	Src      string
	Dst      string
	ReadOnly bool // remount the bind as read-only after setup
	MkdirAll bool // create Dst (and parents) if it does not exist
	Stdout   io.Writer
	Stderr   io.Writer
}

// BindMounter represents a bind mount that tracks its mounted state.
type BindMounter struct {
	src     string
	dst     string
	mounted bool
	stdout  io.Writer
	stderr  io.Writer
}

// NewBindMount creates and performs a bind mount from src to dst.
// When MkdirAll is true the destination path is created automatically.
// When ReadOnly is true the mount is remounted read-only after binding.
func NewBindMount(opts BindMountOptions) (*BindMounter, error) {
	if opts.Src == "" {
		return nil, fmt.Errorf("missing src parameter")
	}
	if opts.Dst == "" {
		return nil, fmt.Errorf("missing dst parameter")
	}

	if _, err := os.Stat(opts.Src); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", opts.Src)
	}
	if opts.MkdirAll {
		if err := os.MkdirAll(opts.Dst, 0755); err != nil {
			return nil, fmt.Errorf("mkdir %s: %w", opts.Dst, err)
		}
	} else if _, err := os.Stat(opts.Dst); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", opts.Dst)
	}

	if err := CheckDirNotFsRoot(opts.Src); err != nil {
		return nil, err
	}
	if err := CheckDirNotFsRoot(opts.Dst); err != nil {
		return nil, err
	}

	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}

	if err := Mount(opts.Src, opts.Dst, "", unix.MS_BIND, ""); err != nil {
		return nil, fmt.Errorf("mount bind failed: %w", err)
	}
	if err := Mount("", opts.Dst, "", unix.MS_SLAVE, ""); err != nil {
		return nil, fmt.Errorf("mount make-slave failed: %w", err)
	}

	if opts.ReadOnly {
		flags := unix.MS_REMOUNT | unix.MS_RDONLY | unix.MS_BIND
		if err := Mount("", opts.Dst, "", uintptr(flags), ""); err != nil {
			// Best-effort unmount on failure.
			_ = Unmount(opts.Dst, 0)
			return nil, fmt.Errorf("remount read-only %s: %w", opts.Dst, err)
		}
	}

	return &BindMounter{
		src:     opts.Src,
		dst:     opts.Dst,
		mounted: true,
		stdout:  stdout,
		stderr:  stderr,
	}, nil
}

// Dst returns the destination mountpoint path.
func (b *BindMounter) Dst() string {
	return b.dst
}

// Unmount unmounts the bind mount. It is safe to call multiple times.
func (b *BindMounter) Unmount() error {
	if !b.mounted {
		return nil
	}
	if _, err := os.Stat(b.dst); os.IsNotExist(err) {
		b.mounted = false
		return fmt.Errorf("%s does not exist", b.dst)
	}
	if err := CheckDirNotFsRoot(b.dst); err != nil {
		return err
	}
	opts := CleanupMountsOptions{
		Mounts: []string{b.dst},
		Stdout: b.stdout,
		Stderr: b.stderr,
	}
	CleanupMounts(opts)
	b.mounted = false
	return nil
}

// BindMountDistdirOptions represents the options for binding
// the distfiles directory.
type BindMountDistdirOptions struct {
	DistfilesDir string
	Rootfs       string
	Stdout       io.Writer
	Stderr       io.Writer
}

// BindMountDistdir binds the distfiles directory.
func BindMountDistdir(opts BindMountDistdirOptions) (*BindMounter, error) {
	if opts.DistfilesDir == "" {
		return nil, fmt.Errorf("missing parameter distfilesDir")
	}
	if opts.Rootfs == "" {
		return nil, fmt.Errorf("missing rootfs parameter")
	}

	if _, err := os.Stat(opts.DistfilesDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", opts.DistfilesDir)
	}
	if _, err := os.Stat(opts.Rootfs); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", opts.Rootfs)
	}

	dstDir := filepath.Join(
		opts.Rootfs, "var", "cache", "distfiles",
	)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return nil, err
	}
	return NewBindMount(BindMountOptions{
		Src:    opts.DistfilesDir,
		Dst:    dstDir,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
	})
}

// BindMountBinpkgsOptions represents the options for binding
// the binpkgs directory.
type BindMountBinpkgsOptions struct {
	BinpkgsDir string
	Rootfs     string
	Stdout     io.Writer
	Stderr     io.Writer
}

// BindMountBinpkgs binds the binpkgs directory.
func BindMountBinpkgs(opts BindMountBinpkgsOptions) (*BindMounter, error) {
	if opts.BinpkgsDir == "" {
		return nil, fmt.Errorf("missing parameter binpkgsDir")
	}
	if opts.Rootfs == "" {
		return nil, fmt.Errorf("missing rootfs parameter")
	}

	if _, err := os.Stat(opts.BinpkgsDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", opts.BinpkgsDir)
	}
	if _, err := os.Stat(opts.Rootfs); os.IsNotExist(err) {
		return nil, fmt.Errorf("%s does not exist", opts.Rootfs)
	}

	dstDir := filepath.Join(
		opts.Rootfs, "var", "cache", "binpkgs",
	)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return nil, err
	}
	return NewBindMount(BindMountOptions{
		Src:    opts.BinpkgsDir,
		Dst:    dstDir,
		Stdout: opts.Stdout,
		Stderr: opts.Stderr,
	})
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

	if err := setFileCap(tmpBin.Name(), unix.CAP_NET_RAW); err != nil {
		log.Println("WARNING: System/FS does not allow setting capabilities.")
		return false, nil
	}

	tmpCopy := tmpBin.Name() + ".copy"
	defer os.Remove(tmpCopy)

	if err := CopyFilePreserveXattrs(tmpBin.Name(), tmpCopy); err != nil {
		return false, err
	}

	hasCap, err := getFileCap(tmpCopy, unix.CAP_NET_RAW)
	if err != nil {
		// xattr not present on copy means FS does not preserve capabilities
		return false, nil
	}
	return hasCap, nil
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

// CheckActiveMounts checks for active mounts under a given directory
// by reading /proc/self/mountinfo.
func CheckActiveMounts(chrootDir string) error {
	if chrootDir == "" {
		return fmt.Errorf("missing chrootDir parameter")
	}

	entries, _ := listMountsByPrefix(chrootDir)
	if len(entries) == 0 {
		return nil
	}

	var foundMounts []string
	for _, e := range entries {
		foundMounts = append(foundMounts, e.Mountpoint)
	}

	return fmt.Errorf(
		"cannot operate sync to %s. Active mounts detected:\n- %s\nPlease umount manually.",
		chrootDir, strings.Join(foundMounts, "\n- "),
	)
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

// RemoveFileWithGlob removes files matching a glob pattern.
func RemoveFileWithGlob(target string) error {
	matches, err := filepath.Glob(target)
	if err != nil {
		return err
	}
	for _, match := range matches {
		err := os.Remove(match)
		if err != nil {
			return err
		}
	}
	return nil
}

// RemoveDir removes a directory.
func RemoveDir(target string) error {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		log.Printf("Removing: %s does not exist\n", target)
		return nil
	}
	log.Printf("Removing %s\n", target)
	return os.RemoveAll(target)
}

// EmptyDir empties a directory.
func EmptyDir(target string) error {
	if _, err := os.Stat(target); os.IsNotExist(err) {
		log.Printf("Emptying: %s does not exist\n", target)
		return nil
	}
	log.Printf("Emptying directory %s\n", target)

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
