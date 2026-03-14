package filesystems

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/sys/unix"

	"matrixos/vector/lib/runner"
)

// Mount wraps unix.Mount and can be replaced in tests.
var Mount = unix.Mount

// Unmount wraps unix.Unmount and can be replaced in tests.
var Unmount = unix.Unmount

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
		if err := Mount("", dst, "", unix.MS_SLAVE, ""); err != nil {
			return fmt.Errorf("failed to make slave %s: %w", dst, err)
		}
		m.mounted(dst)
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
	src       string
	dst       string
	readOnly  bool
	mounted   bool
	mountedMu sync.Mutex
	stdout    io.Writer
	stderr    io.Writer
}

// NewBindMount validates the bind mount parameters and returns a
// BindMounter ready to be mounted. Call Mount() to perform the actual
// mount. This separation allows the caller to register Unmount()
// cleanup before any mount happens, preventing mount-point leaks.
//
// When MkdirAll is true the destination path is created automatically.
// When ReadOnly is true Mount() remounts the bind read-only after binding.
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

	return &BindMounter{
		src:      opts.Src,
		dst:      opts.Dst,
		readOnly: opts.ReadOnly,
		stdout:   stdout,
		stderr:   stderr,
	}, nil
}

// Mount performs the bind mount. It is safe to register Unmount()
// as a cleanup before calling Mount() so that partial mounts are
// always cleaned up.
func (b *BindMounter) Mount() error {
	// protect against accidental double mount.
	b.mountedMu.Lock()
	defer b.mountedMu.Unlock()
	if b.mounted {
		return fmt.Errorf("bind mount already mounted: %s -> %s", b.src, b.dst)
	}
	b.mounted = true

	if _, err := os.Stat(b.dst); errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%s does not exist", b.dst)
	}

	if err := Mount(b.src, b.dst, "", unix.MS_BIND, ""); err != nil {
		return fmt.Errorf("mount bind failed: %w", err)
	}
	if err := Mount("", b.dst, "", unix.MS_SLAVE, ""); err != nil {
		return fmt.Errorf("mount make-slave failed: %w", err)
	}

	if b.readOnly {
		flags := unix.MS_REMOUNT | unix.MS_RDONLY | unix.MS_BIND
		if err := Mount("", b.dst, "", uintptr(flags), ""); err != nil {
			return fmt.Errorf("remount read-only %s: %w", b.dst, err)
		}
	}
	return nil
}

// Dst returns the destination mountpoint path.
func (b *BindMounter) Dst() string {
	return b.dst
}

// Unmount unmounts the bind mount. It is safe to call multiple times.
func (b *BindMounter) Unmount() error {
	// Protect against accidental coding errors.
	b.mountedMu.Lock()
	defer b.mountedMu.Unlock()
	if !b.mounted {
		fmt.Fprintf(
			b.stderr,
			"Warning: unmount bind mount not marked as mounted: %s -> %s\n",
			b.src,
			b.dst,
		)
		return nil
	}
	b.mounted = false

	if _, err := os.Stat(b.dst); os.IsNotExist(err) {
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

// BindMountDistdir creates a bind mounter for the distfiles directory.
// Call Mount() on the returned BindMounter to perform the actual mount.
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

// BindMountBinpkgs creates a bind mounter for the binpkgs directory.
// Call Mount() on the returned BindMounter to perform the actual mount.
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
