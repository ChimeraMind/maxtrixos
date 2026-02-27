package imager

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"sync"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/runner"
	"matrixos/vector/lib/validation"

	"matrixos/vector/lib/cds"
)

type ImageMode int

const (
	ModeFlashToDevice ImageMode = iota
	ModeCreateImageFile
)

// NewImageOptions contains device configuration for image creation.
type NewImageOptions struct {
	EfiDevice  string
	BootDevice string
	RootDevice string
	DevicePath string
	Mode       ImageMode
}

// IImage defines the interface for image operations.
// It mirrors all public methods of Image for testability.
type IImage interface {
	// I/O writers – override to customise output rendering.
	SetStdout(w io.Writer)
	SetStderr(w io.Writer)
	Stdout() io.Writer
	Stderr() io.Writer

	// Structured output helpers.
	Print(format string, args ...any)
	PrintWarning(format string, args ...any)
	PrintError(format string, args ...any)

	// Device setters
	SetEfiDevice(device string)
	EfiDevice() string
	SetBootDevice(device string)
	BootDevice() string
	SetRootDevice(device string)
	RootDevice() string
	SetDevicePath(devicePath string)
	DevicePath() string
	SetImagePath(imagePath string)
	ImagePath() string
	BuildImagePath() (string, error)
	SetImageMode(mode ImageMode) error
	ImageMode() ImageMode

	SetRootfs(rootfs string)
	Rootfs() string
	SetRef(ref string)
	Ref() string

	// Mount point accessors (set after successful Mount* calls)
	EfifsMount() string
	EfiBootDir() (string, error)
	BootfsMount() string
	RootfsMount() string

	// Config accessors
	ImagesDir() (string, error)
	MountDir() (string, error)
	ImageSize() (string, error)
	EfiPartitionSize() (string, error)
	BootPartitionSize() (string, error)
	Compressor() (string, error)
	EspPartitionType() (string, error)
	BootPartitionType() (string, error)
	RootPartitionType() (string, error)
	OsName() (string, error)
	BootRoot() (string, error)
	EfiRoot() (string, error)
	RelativeEfiBootPath() (string, error)
	EfiExecutable() (string, error)
	EfiCertificateFileName() (string, error)
	EfiCertificateFileNameDer() (string, error)
	EfiCertificateFileNameKek() (string, error)
	EfiCertificateFileNameKekDer() (string, error)
	ReadOnlyVdb() (string, error)
	DevDir() (string, error)
	LockDir() (string, error)
	LockWaitSeconds() (string, error)
	BuildMetadataFile() (string, error)
	CreateQcow2() (bool, error)
	Productionize() (bool, error)
	ImageTests() (bool, error)

	// Operations
	Build(opts *BuildOptions) error
	ExtractReleaseVersion() (string, error)
	BuildImagePathWithReleaseVersion(releaseVersion string) (string, error)
	CreateImage(imageSize string) error
	ClearPartitionTable() error
	DatedFsLabel() string
	PartitionDevices(efiSize, bootSize, imageSize string) error
	FormatEfifs() error
	MountEfifs(mountEfifs string) error
	FormatBootfs() error
	MountBootfs(mountBootfs string) error
	MaybeEncryptRootfs() error
	FormatRootfs() error
	RootfsKernelArgs() []string
	MountRootfs(mountRootfs string) error
	GetKernelPath() (string, error)
	SetupPasswords() error
	SetupBootloaderConfig() error
	SetupVmtestConfig() error
	InstallSecurebootCerts() error
	InstallMemtest() error
	GenerateKernelBootArgs() ([]string, error)
	ExtractPackageList() ([]string, error)
	SetupHooks() error
	InstallBootloader() error
	Cleanup()
	TestImage() error
	FinalizeFilesystems() error
	Qcow2ImagePath() (string, error)
	CreateQcow2Image() error
	CompressedImagePath() (string, error)
	CompressImage() error
	ShowFinalFilesystemInfo() error
	ShowImageTestInfo(artifacts []string) error
	RemoveImageFile() error
	ImageLockDir() (string, error)
	ImageLockPath() (string, error)
	ExecuteWithImageLock(fn func() error) error
}

// Image provides image creation and manipulation operations.
type Image struct {
	cfg                 config.IConfig
	ostree              cds.IOstree
	fsenc               filesystems.IFsenc
	runner              runner.Func
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

	// trackedMounts records every mount point created by this Image
	// so that Cleanup can attempt to unmount them all on failure or signal.
	trackedMountsMu      sync.Mutex
	trackedMounts        []string
	trackedTmpDirsMu     sync.Mutex
	trackedTmpDirs       []string
	trackedLoopDevicesMu sync.Mutex
	trackedLoopDevices   []*filesystems.Loop
}

// SetStdout replaces the writer used for informational output.
// Pass a custom writer to capture or restyle messages from the
// calling command layer.
func (im *Image) SetStdout(w io.Writer) { im.stdout = w }

// SetStderr replaces the writer used for warnings and errors.
func (im *Image) SetStderr(w io.Writer) { im.stderr = w }

// Stdout returns the current informational output writer.
func (im *Image) Stdout() io.Writer { return im.stdout }

// Stderr returns the current warning/error output writer.
func (im *Image) Stderr() io.Writer { return im.stderr }

// Print writes a formatted informational message to stdout.
func (im *Image) Print(format string, args ...any) {
	fmt.Fprintf(im.stdout, format, args...)
}

// PrintWarning writes a formatted warning message to stderr.
func (im *Image) PrintWarning(format string, args ...any) {
	fmt.Fprintf(im.stderr, format, args...)
}

// PrintError writes a formatted error/diagnostic message to stderr.
func (im *Image) PrintError(format string, args ...any) {
	fmt.Fprintf(im.stderr, format, args...)
}

// trackMount appends a single mount point to the tracked list.
func (im *Image) trackMount(mnt string) {
	im.trackedMountsMu.Lock()
	defer im.trackedMountsMu.Unlock()
	im.trackedMounts = append(im.trackedMounts, mnt)
}

// trackMounts appends multiple mount points to the tracked list.
func (im *Image) trackMounts(mnts []string) {
	im.trackedMountsMu.Lock()
	defer im.trackedMountsMu.Unlock()
	im.trackedMounts = append(im.trackedMounts, mnts...)
}

// trackTmpDir appends a single temporary directory to the tracked list.
func (im *Image) trackTmpDir(tmpDir string) {
	im.trackedTmpDirsMu.Lock()
	defer im.trackedTmpDirsMu.Unlock()
	im.trackedTmpDirs = append(im.trackedTmpDirs, tmpDir)
}

// trackLoopDevice appends a loop device to the tracked list.
func (im *Image) trackLoopDevice(loop *filesystems.Loop) {
	im.trackedLoopDevicesMu.Lock()
	defer im.trackedLoopDevicesMu.Unlock()
	im.trackedLoopDevices = append(im.trackedLoopDevices, loop)
}

// Cleanup unmounts all mount points tracked by this Image instance
// in reverse order, detaches loop devices, syncs, and removes temp dirs.
// It is safe to call multiple times.
func (im *Image) Cleanup() {
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
	cmd := exec.Command("sync")
	_ = cmd.Run()

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

// NewImage creates a new Image instance.
func NewImage(cfg config.IConfig, ot cds.IOstree, fsenc filesystems.IFsenc, opts *NewImageOptions) (*Image, error) {
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

	im := &Image{
		cfg:    cfg,
		ostree: ot,
		fsenc:  fsenc,
		qa:     qa,
		runner: runner.Run,
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
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

// SetEfiDevice sets the EFI device path.
func (im *Image) SetEfiDevice(device string) { im.efiDevice = device }

// EfiDevice returns the EFI device path.
func (im *Image) EfiDevice() string { return im.efiDevice }

// SetBootDevice sets the boot device path.
func (im *Image) SetBootDevice(device string) { im.bootDevice = device }

// BootDevice returns the boot device path.
func (im *Image) BootDevice() string { return im.bootDevice }

// SetRootDevice sets the root device path.
func (im *Image) SetRootDevice(device string) { im.rootDevice = device }

// RootDevice returns the root device path.
func (im *Image) RootDevice() string { return im.rootDevice }

// SetDevicePath sets the block device path (whole device or loop device).
func (im *Image) SetDevicePath(devicePath string) { im.devicePath = devicePath }

// DevicePath returns the block device path (whole device or loop device).
func (im *Image) DevicePath() string { return im.devicePath }

// SetRootfs sets the deployed ostree rootfs path.
func (im *Image) SetRootfs(rootfs string) { im.rootfs = rootfs }

// SetImagePath sets the image file path (for ModeCreateImageFile).
func (im *Image) SetImagePath(imagePath string) { im.imagePath = imagePath }

// ImagePath returns the currently stored image file path.
func (im *Image) ImagePath() string { return im.imagePath }

// SetImageMode sets the image creation mode
// (e.g. flash to device or create image file).
func (im *Image) SetImageMode(mode ImageMode) error {
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

// ImageMode returns the current image creation mode.
func (im *Image) ImageMode() ImageMode { return im.mode }

// Rootfs returns the deployed ostree rootfs path.
func (im *Image) Rootfs() string { return im.rootfs }

// Ref returns the ostree ref.
func (im *Image) Ref() string { return im.ref }

// SetRef sets the ostree ref.
func (im *Image) SetRef(ref string) { im.ref = ref }

// EfifsMount returns the EFI filesystem mount point (set by MountEfifs on success).
func (im *Image) EfifsMount() string { return im.efifsMount }

// BootfsMount returns the boot filesystem mount point (set by MountBootfs on success).
func (im *Image) BootfsMount() string { return im.bootfsMount }

// RootfsMount returns the root filesystem mount point (set by MountRootfs on success).
func (im *Image) RootfsMount() string { return im.rootfsMount }
