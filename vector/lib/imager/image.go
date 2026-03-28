package imager

import (
	"errors"
	"fmt"
	"io"
	"os"
	"slices"
	"sync"
	"syscall"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/runner"
	"matrixos/vector/lib/validation"

	"matrixos/vector/lib/ostree"
)

type ImageMode int

const (
	ModeFlashToDevice ImageMode = iota
	ModeCreateImageFile
)

// NewImagerOptions contains device configuration for image creation.
type NewImagerOptions struct {
	EfiDevice  string
	BootDevice string
	RootDevice string
	DevicePath string
	Mode       ImageMode
}

// IImager defines the interface for image operations.
// It mirrors all public methods of Imager for testability.
type IImager interface {
	// SetStdout replaces the writer used for informational output.
	SetStdout(w io.Writer)
	// SetStderr replaces the writer used for warnings and errors.
	SetStderr(w io.Writer)
	// Stdout returns the current informational output writer.
	Stdout() io.Writer
	// Stderr returns the current warning/error output writer.
	Stderr() io.Writer

	// Print writes a formatted informational message to stdout.
	Print(format string, args ...any)
	// PrintWarning writes a formatted warning message to stderr.
	PrintWarning(format string, args ...any)
	// PrintError writes a formatted error/diagnostic message to stderr.
	PrintError(format string, args ...any)

	// SetEfiDevice sets the EFI device path.
	SetEfiDevice(device string)
	// EfiDevice returns the EFI device path.
	EfiDevice() string
	// SetBootDevice sets the boot device path.
	SetBootDevice(device string)
	// BootDevice returns the boot device path.
	BootDevice() string
	// SetRootDevice sets the root device path.
	SetRootDevice(device string)
	// RootDevice returns the root device path.
	RootDevice() string
	// SetDevicePath sets the block device path (whole device or loop device).
	SetDevicePath(devicePath string)
	// DevicePath returns the block device path (whole device or loop device).
	DevicePath() string
	// SetImagePath sets the image file path (for ModeCreateImageFile).
	SetImagePath(imagePath string)
	// ImagePath returns the currently stored image file path.
	ImagePath() string
	// BuildImagePath returns the image file path for the stored ostree ref.
	BuildImagePath() (string, error)
	// SetImageMode sets the image creation mode
	// (e.g. flash to device or create image file).
	SetImageMode(mode ImageMode) error
	// ImageMode returns the current image creation mode.
	ImageMode() ImageMode

	// SetRootfs sets the deployed ostree rootfs path.
	SetRootfs(rootfs string)
	// Rootfs returns the deployed ostree rootfs path.
	Rootfs() string
	// SetRef sets the ostree ref.
	SetRef(ref string)
	// Ref returns the ostree ref.
	Ref() string

	// EfifsMount returns the EFI filesystem mount point (set by MountEfifs on success).
	EfifsMount() string
	// EfiBootDir returns the full path to the EFI boot directory on the mounted EFI filesystem.
	EfiBootDir() (string, error)
	// BootfsMount returns the boot filesystem mount point (set by MountBootfs on success).
	BootfsMount() string
	// RootfsMount returns the root filesystem mount point (set by MountRootfs on success).
	RootfsMount() string

	// ImagesDir returns the directory where generated images are stored.
	ImagesDir() (string, error)
	// MountDir returns the directory where image partitions are mounted.
	MountDir() (string, error)
	// ImageSize returns the configured image size (e.g. "32G").
	ImageSize() (string, error)
	// EfiPartitionSize returns the configured EFI partition size (e.g. "200M").
	EfiPartitionSize() (string, error)
	// BootPartitionSize returns the configured boot partition size (e.g. "1G").
	BootPartitionSize() (string, error)
	// Compressor returns the configured compressor command string (e.g. "xz -f -0 -T0").
	Compressor() (string, error)
	// EspPartitionType returns the ESP partition type GUID.
	EspPartitionType() (string, error)
	// BootPartitionType returns the boot partition type GUID.
	BootPartitionType() (string, error)
	// RootPartitionType returns the root partition type GUID.
	RootPartitionType() (string, error)
	// OsName returns the OS name.
	OsName() (string, error)
	// BootRoot returns the boot filesystem mount point (e.g. "/boot").
	BootRoot() (string, error)
	// EfiRoot returns the EFI filesystem mount point (e.g. "/efi").
	EfiRoot() (string, error)
	// RelativeEfiBootPath returns the path relative to EfiRoot where the
	// standard ESP boot directory is (e.g. "efi/BOOT").
	RelativeEfiBootPath() (string, error)
	// EfiExecutable returns the EFI executable name (e.g. "BOOTX64.EFI").
	EfiExecutable() (string, error)
	// EfiCertificateFileName returns the SecureBoot PEM certificate file name.
	EfiCertificateFileName() (string, error)
	// EfiCertificateFileNameDer returns the SecureBoot DER certificate file name.
	EfiCertificateFileNameDer() (string, error)
	// EfiCertificateFileNameKek returns the SecureBoot KEK PEM certificate file name.
	EfiCertificateFileNameKek() (string, error)
	// EfiCertificateFileNameKekDer returns the SecureBoot KEK DER certificate file name.
	EfiCertificateFileNameKekDer() (string, error)
	// ReadOnlyVdb returns the read-only VDB path (e.g. "/usr/var-db-pkg").
	ReadOnlyVdb() (string, error)
	// DevDir returns the matrixOS dev directory (Root).
	DevDir() (string, error)
	// HooksDir returns the directory where image generation hooks are placed.
	HooksDir() (string, error)
	// TestsDir returns the directory where image generation tests are placed.
	TestsDir() (string, error)
	// LockDir returns the configured image lock directory.
	LockDir() (string, error)
	// LockWaitSeconds returns the configured lock wait timeout in seconds.
	LockWaitSeconds() (string, error)
	// BuildMetadataFile returns the build metadata file path.
	BuildMetadataFile() (string, error)
	// CreateQcow2 returns whether a QCOW2 image should be created
	// in addition to the raw .img file.
	CreateQcow2() (bool, error)
	// Productionize returns whether productionization steps should be
	// executed after image creation.
	Productionize() (bool, error)
	// ImageTests returns whether integration tests should be run
	// after image creation.
	ImageTests() (bool, error)

	// Build implements the core image setup logic. It partitions, formats,
	// mounts, deploys ostree, installs the bootloader, and performs
	// post-processing (productionization, compression, signing).
	Build(opts *BuildOptions) error
	// ExtractReleaseVersion extracts or generates a release version string
	// for an image. It attempts to read a build metadata file from the rootfs
	// for the version; if unavailable, falls back to the current date (YYYYMMDD).
	ExtractReleaseVersion() (string, error)
	// BuildImagePathWithReleaseVersion returns the image file path with an
	// embedded release version.
	BuildImagePathWithReleaseVersion(releaseVersion string) (string, error)
	// CreateImage creates a sparse image file at imagePath with the given size.
	CreateImage(imageSize string) error
	// ClearPartitionTable clears the partition table on a device using sgdisk.
	ClearPartitionTable() error
	// DatedFsLabel returns a filesystem label based on the current date (YYYYMMDD).
	DatedFsLabel() string
	// PartitionDevices creates the EFI, boot, and root partitions on a device.
	PartitionDevices(efiSize, bootSize, imageSize string) error
	// FormatEfifs creates a FAT32 filesystem on the EFI partition.
	FormatEfifs() error
	// MountEfifs mounts the EFI partition.
	MountEfifs(mountEfifs string) error
	// FormatBootfs creates a btrfs filesystem on the boot partition.
	FormatBootfs() error
	// MountBootfs mounts the boot partition.
	MountBootfs(mountBootfs string) error
	// MaybeEncryptRootfs encrypts the root partition with LUKS if encryption
	// is enabled in the configuration.
	MaybeEncryptRootfs() error
	// FormatRootfs creates a btrfs filesystem on the root partition.
	FormatRootfs() error
	// RootfsKernelArgs returns the default kernel arguments for the root filesystem.
	RootfsKernelArgs() []string
	// MountRootfs mounts the root partition with btrfs compression options.
	MountRootfs(mountRootfs string) error
	// GetKernelPath returns the kernel version directory name from the deployed rootfs.
	GetKernelPath() (string, error)
	// SetupPasswords sets default passwords for the matrix and root users.
	SetupPasswords() error
	// SetupBootloaderConfig sets up the bootloader configuration.
	SetupBootloaderConfig() error
	// SetupVmtestConfig creates a VM test boot config based on the ostree boot config.
	SetupVmtestConfig() error
	// GetBootloader returns the configured Bootloader implementation.
	GetBootloader() Bootloader
	// InstallSecurebootCerts installs SecureBoot certificates on the EFI partition.
	InstallSecurebootCerts() error
	// InstallMemtest installs the memtest86+ EFI binary to the EFI boot directory.
	InstallMemtest() error
	// GenerateKernelBootArgs generates the kernel boot arguments for the image.
	GenerateKernelBootArgs() ([]string, error)
	// ExtractPackageList returns the list of packages installed in a rootfs.
	ExtractPackageList() ([]string, error)
	// SetupHooks runs image-specific hook scripts.
	SetupHooks() error
	// InstallBootloader installs the bootloader into the image using the
	// configured Bootloader implementation.
	InstallBootloader() error
	// Cleanup unmounts all mount points tracked by this Imager instance in
	// reverse order, detaches loop devices, syncs, and removes temp dirs.
	// It is safe to call multiple times.
	Cleanup()
	// TestImage copies an image to a temp directory and runs test scripts against it.
	TestImage() error
	// FinalizeFilesystems runs fstrim on the root and boot filesystems to
	// improve compression ratios for sparse image files.
	FinalizeFilesystems() error
	// Qcow2ImagePath returns the qcow2 image path for a given .img path.
	Qcow2ImagePath() (string, error)
	// CreateQcow2Image creates a compressed qcow2 image from a raw image.
	CreateQcow2Image() error
	// CompressedImagePath appends the compressor's file extension to the image path.
	CompressedImagePath() (string, error)
	// CompressImage compresses an image file using the configured compressor.
	CompressImage() error
	// ShowFinalFilesystemInfo displays information about the final filesystem layout.
	ShowFinalFilesystemInfo() error
	// ShowImageTestInfo prints information about generated artifacts and
	// how to test them.
	ShowImageTestInfo(artifacts []string) error
	// RemoveImageFile removes an image file and its associated .sha256 and .asc files.
	RemoveImageFile() error
	// ImageLockDir returns the image lock directory, creating it if necessary.
	ImageLockDir() (string, error)
	// ImageLockPath returns the lock file path for the stored ref.
	ImageLockPath() (string, error)
	// ExecuteWithImageLock acquires an exclusive file lock for the given ref,
	// executes fn under that lock, and releases the lock when fn returns.
	ExecuteWithImageLock(fn func() error) error
}

// Imager provides image creation and manipulation operations.
type Imager struct {
	cfg                 config.IConfig
	ostree              ostree.IOstree
	fsenc               filesystems.IFsenc
	runner              runner.Func
	chrootRunner        runner.ChrootRunFunc
	bootloader          Bootloader
	stdout              io.Writer
	stderr              io.Writer
	efiDevice           string
	bootDevice          string
	rootDevice          string
	realRootDevice      string // if encrypted, devicePath is replaced.
	devicePath          string
	imagePath           string
	compressedImagePath string
	mode                ImageMode
	rootfs              string
	ref                 string
	encrypted           bool

	// Mount points, set by Mount* methods on success.
	efifsMount  string
	bootfsMount string
	rootfsMount string

	// QA validation instance.
	qa *validation.QA

	// trackedMounts records every mount point created by this Imager
	// so that Cleanup can attempt to unmount them all on failure or signal.
	trackedMountsMu      sync.Mutex
	trackedMounts        []string
	trackedTmpDirsMu     sync.Mutex
	trackedTmpDirs       []string
	trackedLoopDevicesMu sync.Mutex
	trackedLoopDevices   []*filesystems.Loop
}

func (im *Imager) SetStdout(w io.Writer) { im.stdout = w }

func (im *Imager) SetStderr(w io.Writer) { im.stderr = w }

func (im *Imager) Stdout() io.Writer { return im.stdout }

func (im *Imager) Stderr() io.Writer { return im.stderr }

func (im *Imager) Print(format string, args ...any) {
	fmt.Fprintf(im.stdout, format, args...)
}

func (im *Imager) PrintWarning(format string, args ...any) {
	fmt.Fprintf(im.stderr, format, args...)
}

func (im *Imager) PrintError(format string, args ...any) {
	fmt.Fprintf(im.stderr, format, args...)
}

// trackMount appends a single mount point to the tracked list.
func (im *Imager) trackMount(mnt string) {
	im.trackedMountsMu.Lock()
	defer im.trackedMountsMu.Unlock()
	im.trackedMounts = append(im.trackedMounts, mnt)
}

// trackMounts appends multiple mount points to the tracked list.
func (im *Imager) trackMounts(mnts []string) {
	im.trackedMountsMu.Lock()
	defer im.trackedMountsMu.Unlock()
	im.trackedMounts = append(im.trackedMounts, mnts...)
}

// trackTmpDir appends a single temporary directory to the tracked list.
func (im *Imager) trackTmpDir(tmpDir string) {
	im.trackedTmpDirsMu.Lock()
	defer im.trackedTmpDirsMu.Unlock()
	im.trackedTmpDirs = append(im.trackedTmpDirs, tmpDir)
}

// trackLoopDevice appends a loop device to the tracked list.
func (im *Imager) trackLoopDevice(loop *filesystems.Loop) {
	im.trackedLoopDevicesMu.Lock()
	defer im.trackedLoopDevicesMu.Unlock()
	im.trackedLoopDevices = append(im.trackedLoopDevices, loop)
}

func (im *Imager) Cleanup() {
	im.trackedMountsMu.Lock()
	mounts := slices.Clone(im.trackedMounts)
	im.trackedMounts = nil
	im.trackedMountsMu.Unlock()

	copts := filesystems.CleanupMountsOptions{
		Stdout: im.stdout,
		Stderr: im.stderr,
		Mounts: mounts,
	}
	filesystems.CleanupMounts(copts)

	im.trackedLoopDevicesMu.Lock()
	loops := slices.Clone(im.trackedLoopDevices)
	im.trackedLoopDevices = nil
	im.trackedLoopDevicesMu.Unlock()

	for _, loop := range loops {
		if err := loop.Detach(); err != nil {
			fmt.Fprintf(im.stderr, "warning: failed to detach loop device %s: %v\n", loop.Device, err)
		}
	}

	// Sync buffers for image files after umount.
	syscall.Sync()

	im.trackedTmpDirsMu.Lock()
	tmpDirs := slices.Clone(im.trackedTmpDirs)
	im.trackedTmpDirs = nil
	im.trackedTmpDirsMu.Unlock()

	for _, tmpDir := range tmpDirs {
		fmt.Fprintf(im.stdout, "Removing temp dir %s\n", tmpDir)
		if err := os.RemoveAll(tmpDir); err != nil {
			fmt.Fprintf(im.stderr, "Warning: failed to remove temp dir %s: %v\n", tmpDir, err)
		}
	}
}

// NewImager creates a new Imager instance.
func NewImager(cfg config.IConfig, ot ostree.IOstree, fsenc filesystems.IFsenc, opts *NewImagerOptions) (*Imager, error) {
	if cfg == nil {
		return nil, errors.New("missing config parameter")
	}
	if ot == nil {
		return nil, errors.New("missing ostree parameter")
	}
	if fsenc == nil {
		return nil, errors.New("missing fsenc parameter")
	}
	encrypted, err := fsenc.EncryptionEnabled()
	if err != nil {
		return nil, fmt.Errorf("failed to check if encryption is enabled: %w", err)
	}

	qa, err := validation.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize QA: %w", err)
	}

	im := &Imager{
		cfg:          cfg,
		ostree:       ot,
		fsenc:        fsenc,
		qa:           qa,
		runner:       runner.Run,
		chrootRunner: runner.ChrootRun,
		stdout:       os.Stdout,
		stderr:       os.Stderr,
	}
	im.bootloader = NewGrubBootloader(im)
	if opts != nil {
		im.efiDevice = opts.EfiDevice
		im.bootDevice = opts.BootDevice
		im.rootDevice = opts.RootDevice
		im.devicePath = opts.DevicePath
		im.encrypted = encrypted
		im.mode = opts.Mode
	}
	return im, nil
}

func (im *Imager) SetEfiDevice(device string) { im.efiDevice = device }

func (im *Imager) EfiDevice() string { return im.efiDevice }

func (im *Imager) SetBootDevice(device string) { im.bootDevice = device }

func (im *Imager) BootDevice() string { return im.bootDevice }

func (im *Imager) SetRootDevice(device string) { im.rootDevice = device }

func (im *Imager) RootDevice() string { return im.rootDevice }

func (im *Imager) SetDevicePath(devicePath string) { im.devicePath = devicePath }

func (im *Imager) DevicePath() string { return im.devicePath }

func (im *Imager) SetRootfs(rootfs string) { im.rootfs = rootfs }

func (im *Imager) SetImagePath(imagePath string) { im.imagePath = imagePath }

func (im *Imager) ImagePath() string { return im.imagePath }

func (im *Imager) SetImageMode(mode ImageMode) error {
	switch mode {
	case ModeFlashToDevice:
		if im.devicePath == "" {
			return errors.New("devicePath must be set for ModeFlashToDevice")
		}
	case ModeCreateImageFile:
		if im.imagePath == "" {
			return errors.New("imagePath must be set for ModeCreateImageFile")
		}
	default:
		return errors.New("invalid image mode")
	}

	im.mode = mode
	return nil
}

func (im *Imager) ImageMode() ImageMode { return im.mode }

func (im *Imager) Rootfs() string { return im.rootfs }

func (im *Imager) Ref() string { return im.ref }

func (im *Imager) SetRef(ref string) { im.ref = ref }

func (im *Imager) EfifsMount() string { return im.efifsMount }

func (im *Imager) BootfsMount() string { return im.bootfsMount }

func (im *Imager) RootfsMount() string { return im.rootfsMount }
