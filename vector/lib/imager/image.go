package imager

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

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

// NewImageOptions contains device configuration for image creation.
type NewImageOptions struct {
	EfiDevice  string
	BootDevice string
	RootDevice string
	DevicePath string
	ImagePath  string
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
	ExtractReleaseVersion() (string, error)
	BuildImagePathWithReleaseVersion(releaseVersion string) (string, error)
	CreateImage(imageSize string) error
	BuildImagePathWithCompressorExtension() (string, error)
	CompressImage() error
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
	ShowFinalFilesystemInfo() error
	ShowImageTestInfo(artifacts []string)
	RemoveImageFile() error
	ImageLockDir() (string, error)
	ImageLockPath() (string, error)
	ExecuteWithImageLock(fn func() error) error
}

// Image provides image creation and manipulation operations.
type Image struct {
	cfg            config.IConfig
	ostree         cds.IOstree
	fsenc          filesystems.IFsenc
	runner         runner.Func
	stdout         io.Writer
	stderr         io.Writer
	efiDevice      string
	bootDevice     string
	rootDevice     string
	realRootDevice string // if encrypted, devicePath is replaced.
	devicePath     string
	imagePath      string
	mode           ImageMode
	rootfs         string
	ref            string
	encrypted      bool

	// Mount points, set by Mount* methods on success.
	efifsMount  string
	bootfsMount string
	rootfsMount string

	// trackedMounts records every mount point created by this Image
	// so that Cleanup can attempt to unmount them all on failure or signal.
	trackedMountsMu sync.Mutex
	trackedMounts   []string
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

// Cleanup unmounts all mount points tracked by this Image instance
// in reverse order. It is safe to call multiple times.
func (im *Image) Cleanup() {
	im.trackedMountsMu.Lock()
	mounts := slices.Clone(im.trackedMounts)
	im.trackedMounts = nil
	im.trackedMountsMu.Unlock()

	filesystems.CleanupMounts(mounts)
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

	im := &Image{
		cfg:    cfg,
		ostree: ot,
		fsenc:  fsenc,
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
		im.imagePath = opts.ImagePath
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

// ImagesDir returns the directory where generated images are stored.
func (im *Image) ImagesDir() (string, error) {
	v, err := im.cfg.GetItem("Imager.ImagesDir")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.ImagesDir")
	}
	return v, nil
}

func (im *Image) SetBootDevice(device string) { im.bootDevice = device }

func (im *Image) BootDevice() string { return im.bootDevice }

func (im *Image) SetRootDevice(device string) { im.rootDevice = device }

func (im *Image) RootDevice() string { return im.rootDevice }

func (im *Image) SetDevicePath(devicePath string) { im.devicePath = devicePath }

func (im *Image) DevicePath() string { return im.devicePath }

func (im *Image) SetRootfs(rootfs string) { im.rootfs = rootfs }

func (im *Image) SetImagePath(imagePath string) { im.imagePath = imagePath }

func (im *Image) ImagePath() string { return im.imagePath }

// BootRoot returns the boot filesystem mount point (e.g. "/boot").
func (im *Image) BootRoot() (string, error) {
	v, err := im.cfg.GetItem("Imager.BootRoot")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.BootRoot")
	}
	return v, nil
}

// EfiRoot returns the EFI filesystem mount point (e.g. "/efi").
func (im *Image) EfiRoot() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiRoot")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiRoot")
	}
	return v, nil
}

// RelativeEfiBootPath returns the path relative to EfiRoot where the standard ESP
// boot directory is (e.g. "efi/BOOT").
func (im *Image) RelativeEfiBootPath() (string, error) {
	v, err := im.cfg.GetItem("Imager.RelativeEfiBootPath")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.RelativeEfiBootPath")
	}
	return v, nil
}

// EfiExecutable returns the EFI executable name (e.g. "BOOTX64.EFI").
func (im *Image) EfiExecutable() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiExecutable")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiExecutable")
	}
	return v, nil
}

// EfiCertificateFileName returns the SecureBoot PEM certificate file name.
func (im *Image) EfiCertificateFileName() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiCertificateFileName")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiCertificateFileName")
	}
	return v, nil
}

// EfiCertificateFileNameDer returns the SecureBoot DER certificate file name.
func (im *Image) EfiCertificateFileNameDer() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiCertificateFileNameDer")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiCertificateFileNameDer")
	}
	return v, nil
}

// EfiCertificateFileNameKek returns the SecureBoot KEK PEM certificate file name.
func (im *Image) EfiCertificateFileNameKek() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiCertificateFileNameKek")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiCertificateFileNameKek")
	}
	return v, nil
}

// EfiCertificateFileNameKekDer returns the SecureBoot KEK DER certificate file name.
func (im *Image) EfiCertificateFileNameKekDer() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiCertificateFileNameKekDer")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiCertificateFileNameKekDer")
	}
	return v, nil
}

// ReadOnlyVdb returns the read-only VDB path (e.g. "/usr/var-db-pkg").
func (im *Image) ReadOnlyVdb() (string, error) {
	v, err := im.cfg.GetItem("Releaser.ReadOnlyVdb")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Releaser.ReadOnlyVdb")
	}
	return v, nil
}

// DevDir returns the matrixOS dev directory (Root).
func (im *Image) DevDir() (string, error) {
	v, err := im.cfg.GetItem("matrixOS.Root")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid matrixOS.Root")
	}
	return v, nil
}

// LockDir returns the configured image lock directory.
func (im *Image) LockDir() (string, error) {
	v, err := im.cfg.GetItem("Imager.LocksDir")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.LocksDir")
	}
	return v, nil
}

// LockWaitSeconds returns the configured lock wait timeout in seconds.
func (im *Image) LockWaitSeconds() (string, error) {
	v, err := im.cfg.GetItem("Imager.LockWaitSeconds")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.LockWaitSeconds")
	}
	return v, nil
}

// BuildMetadataFile returns the build metadata file path (combining
// ChrootMetadataDir and ChrootMetadataDirBuildFileName).
func (im *Image) BuildMetadataFile() (string, error) {
	metadataDir, err := im.cfg.GetItem("Seeder.ChrootMetadataDir")
	if err != nil {
		return "", err
	}
	if metadataDir == "" {
		return "", errors.New("invalid Seeder.ChrootMetadataDir")
	}
	buildFileName, err := im.cfg.GetItem("Seeder.ChrootMetadataDirBuildFileName")
	if err != nil {
		return "", err
	}
	if buildFileName == "" {
		return "", errors.New("invalid Seeder.ChrootMetadataDirBuildFileName")
	}
	return filepath.Join(metadataDir, buildFileName), nil
}

// CreateQcow2 returns whether a QCOW2 image should be created in addition to the raw .img file.
func (im *Image) CreateQcow2() (bool, error) {
	v, err := im.cfg.GetBool("Imager.CreateQcow2")
	if err != nil {
		return false, err
	}
	return v, nil
}

// Productionize returns whether productionization steps should be executed after image creation.
func (im *Image) Productionize() (bool, error) {
	v, err := im.cfg.GetBool("Imager.Productionize")
	if err != nil {
		return false, err
	}
	return v, nil
}

// ImageTests returns whether integration tests should be run after image creation.
func (im *Image) ImageTests() (bool, error) {
	v, err := im.cfg.GetBool("Imager.ImageTests")
	if err != nil {
		return false, err
	}
	return v, nil
}

// --- Helpers ---

// buildImagePath builds the full image file path from a suffix.
func (im *Image) buildImagePath(suffix string) (string, error) {
	outDir, err := im.ImagesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(outDir, suffix), nil
}

// cleanAndStripRef cleans a remote prefix and removes the -full suffix from the stored ref.
func (im *Image) cleanAndStripRef() (string, error) {
	if im.ref == "" {
		return "", errors.New("missing ref, set Ref in NewImageOptions")
	}
	ref := cds.CleanRemoteFromRef(im.ref)
	stripped, err := im.ostree.RemoveFullFromBranch(ref)
	if err != nil {
		return "", err
	}
	if stripped == "" {
		return "", errors.New("invalid ref parameter after cleaning")
	}
	return stripped, nil
}

// refToSuffix converts slashes in a ref to underscores for use in file names.
func refToSuffix(ref string) string {
	return strings.ReplaceAll(ref, "/", "_")
}

// --- Operations ---

func (im *Image) extractSeedName(data []byte) (string, error) {
	// Extract version from SEED_NAME= line.
	var releaseVersion string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "SEED_NAME=") {
			continue
		}

		seedName := strings.TrimPrefix(line, "SEED_NAME=")
		// Version is the part after the last '-'.
		if idx := strings.LastIndex(seedName, "-"); idx >= 0 {
			releaseVersion = seedName[idx+1:]
			im.Print("Extracted release version: %s\n", releaseVersion)
		} else {
			im.PrintWarning("WARNING: SEED_NAME= value has no '-' separator\n")
		}
		break

	}
	if scanner.Err() != nil {
		return releaseVersion, fmt.Errorf("failed to scan build metadata file: %w", scanner.Err())
	}
	return releaseVersion, nil
}

// ExtractReleaseVersion extracts or generates a release version string for an image.
// It attempts to read a build metadata file from the rootfs for the version;
// if unavailable, falls back to the current date (YYYYMMDD).
func (im *Image) ExtractReleaseVersion() (string, error) {
	if im.rootfs == "" {
		return "", errors.New("rootfs not set, call SetRootfs first")
	}

	releaseVersion := time.Now().Format("20060102")
	metadataRelPath, err := im.BuildMetadataFile()
	if err != nil {
		return "", fmt.Errorf(
			"failed to determine build metadata file path: %w", err)
	}
	metadataFile := filepath.Join(im.rootfs, metadataRelPath)

	if filesystems.FileExists(metadataFile) {
		im.PrintError("Build metadata:\n")
		data, err := os.ReadFile(metadataFile)
		if err != nil {
			return "", fmt.Errorf(
				"failed to read build metadata file %s: %w", metadataFile, err)
		}
		im.PrintError("%s", string(data))

		releaseVersion, err = im.extractSeedName(data)
		if err != nil {
			return "", fmt.Errorf(
				"failed to extract release version from build metadata: %w", err)
		}
	} else {
		im.PrintWarning(
			"WARNING! Build metadata file not found: %s\n", metadataFile,
		)
	}

	return releaseVersion, nil
}

// BuildImagePath returns the image file path for the stored ostree ref.
func (im *Image) BuildImagePath() (string, error) {
	if im.ref == "" {
		return "", errors.New("missing ref, set Ref in NewImageOptions")
	}
	ref := cds.CleanRemoteFromRef(im.ref)
	suffix := refToSuffix(ref) + ".img"
	return im.buildImagePath(suffix)
}

// BuildImagePathWithReleaseVersion returns the image file path with an embedded release version.
func (im *Image) BuildImagePathWithReleaseVersion(releaseVersion string) (string, error) {
	if im.ref == "" {
		return "", errors.New("missing ref, set Ref in NewImageOptions")
	}
	if releaseVersion == "" {
		return "", errors.New("missing releaseVersion parameter")
	}
	ref := cds.CleanRemoteFromRef(im.ref)
	suffix := refToSuffix(ref) + "-" + releaseVersion + ".img"
	return im.buildImagePath(suffix)
}

func (im *Image) validateImageModeForCreation() error {
	if im.mode != ModeCreateImageFile {
		return errors.New("invalid image creation mode")
	}
	if im.imagePath == "" {
		return errors.New("missing imagePath parameter")
	}
	return nil
}

// CreateImage creates a sparse image file at imagePath with the given size.
func (im *Image) CreateImage(imageSize string) (retErr error) {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}

	if imageSize == "" {
		return errors.New("missing imageSize parameter")
	}

	imagesDir := filepath.Dir(im.imagePath)
	im.Print(
		"Creating images directory: %s (if it does not exist)\n",
		imagesDir,
	)
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create images directory %s: %w", imagesDir, err)
	}

	// Don't skip removing or sgdisk gets confused due to truncate.
	if err := im.RemoveImageFile(); err != nil {
		return err
	}

	sizeBytes, err := filesystems.ParseHumanSize(imageSize)
	if err != nil {
		return fmt.Errorf("failed to parse image size %q: %w", imageSize, err)
	}

	im.Print("Creating block device image file: %s\n", im.imagePath)
	f, err := os.Create(im.imagePath)
	if err != nil {
		return fmt.Errorf("failed to create image file %s: %w", im.imagePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && retErr == nil {
			retErr = cerr
		}
	}()

	if err := f.Truncate(sizeBytes); err != nil {
		return fmt.Errorf("failed to truncate image file %s to %d bytes: %w", im.imagePath, sizeBytes, err)
	}
	return nil
}

// BuildImagePathWithCompressorExtension appends the compressor's file extension to the image path.
// The extension is derived from the first word of the compressor command string.
func (im *Image) BuildImagePathWithCompressorExtension() (string, error) {
	if err := im.validateImageModeForCreation(); err != nil {
		return "", err
	}
	compressor, err := im.Compressor()
	if err != nil {
		return "", fmt.Errorf("failed to get compressor: %w", err)
	}
	if compressor == "" {
		return "", errors.New("missing compressor parameter")
	}
	parts := strings.Fields(compressor)
	if len(parts) == 0 {
		return "", errors.New("invalid compressor parameters: empty command")
	}
	return im.imagePath + "." + parts[0], nil
}

// CompressImage compresses an image file using the configured compressor.
func (im *Image) CompressImage() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}
	compressor, err := im.Compressor()
	if err != nil {
		return fmt.Errorf("failed to get compressor: %w", err)
	}
	if compressor == "" {
		return errors.New("missing compressor parameter")
	}

	imagePathWithExt, err := im.BuildImagePathWithCompressorExtension()
	if err != nil {
		return err
	}

	parts := strings.Fields(compressor)
	if len(parts) == 0 {
		return errors.New("invalid compressor parameters: empty command")
	}
	args := append(parts[1:], im.imagePath)
	if err := im.runner(nil, im.stdout, im.stderr, parts[0], args...); err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}

	if !filesystems.FileExists(imagePathWithExt) {
		return fmt.Errorf("compressed image was not created at the expected path: %s", imagePathWithExt)
	}
	return nil
}

// ClearPartitionTable clears the partition table on a device using sgdisk.
func (im *Image) ClearPartitionTable() error {
	if im.devicePath == "" {
		return errors.New("missing devicePath, not set in NewImageOptions")
	}

	im.Print("Clearing partition table on %s ...\n", im.devicePath)
	if err := im.runner(nil, im.stdout, im.stderr, "sgdisk", "-g", "-o", im.devicePath); err != nil {
		return fmt.Errorf("sgdisk -g -o failed on %s: %w", im.devicePath, err)
	}
	return im.runner(nil, im.stdout, im.stderr, "sgdisk", "-Z", im.devicePath)
}

// DatedFsLabel returns a filesystem label based on the current date (YYYYMMDD).
func (im *Image) DatedFsLabel() string {
	return time.Now().Format("20060102")
}

// PartitionDevices creates the EFI, boot, and root partitions on a device.
func (im *Image) PartitionDevices(efiSize, bootSize, imageSize string) error {
	if efiSize == "" {
		return errors.New("missing efiSize parameter")
	}
	if bootSize == "" {
		return errors.New("missing bootSize parameter")
	}
	if imageSize == "" {
		return errors.New("missing imageSize parameter")
	}
	if im.devicePath == "" {
		return errors.New("missing devicePath, not set in NewImageOptions")
	}

	espPartType, err := im.EspPartitionType()
	if err != nil {
		return err
	}
	bootPartType, err := im.BootPartitionType()
	if err != nil {
		return err
	}
	rootPartType, err := im.RootPartitionType()
	if err != nil {
		return err
	}

	im.Print("Partitioning %s:\n", im.devicePath)
	im.Print(" --> p1 (EFI: %s)\n", efiSize)
	im.Print(" --> p2 (BOOT: %s)\n", bootSize)
	im.Print(" --> p3 (ROOT: Remainder of %s, plus autogrow)\n", imageSize)

	// Create EFI partition.
	epArgs := []string{
		"sgdisk",
		"-n", fmt.Sprintf("1:0:+%s", efiSize),
		"-t", fmt.Sprintf("1:%s", espPartType),
		im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, epArgs[0], epArgs[1:]...); err != nil {
		return fmt.Errorf("sgdisk EFI partition failed: %w", err)
	}

	// Create boot partition.
	bpArgs := []string{
		"sgdisk",
		"-n", fmt.Sprintf("2:0:+%s", bootSize),
		"-t", fmt.Sprintf("2:%s", bootPartType),
		im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, bpArgs[0], bpArgs[1:]...); err != nil {
		return fmt.Errorf("sgdisk boot partition failed: %w", err)
	}

	// Create root partition with -10M padding for systemd-repart.
	rpArgs := []string{
		"sgdisk",
		"-n", "3:0:-10M",
		"-t", fmt.Sprintf("3:%s", rootPartType),
		im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, rpArgs[0], rpArgs[1:]...); err != nil {
		return fmt.Errorf("sgdisk root partition failed: %w", err)
	}

	// Set the auto-grow flag (bit 59) on partition 3.
	agArgs := []string{
		"sgdisk", "-A", "3:set:59", im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, agArgs[0], agArgs[1:]...); err != nil {
		return fmt.Errorf("sgdisk set auto-grow flag failed: %w", err)
	}

	im.Print("Refreshing partition table ...\n")
	args := []string{
		"partprobe", "-s", im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...); err != nil {
		return fmt.Errorf("partprobe failed: %w", err)
	}

	filesystems.DevicesSettle()
	return nil
}

// FormatEfifs creates a FAT32 filesystem on the EFI partition.
func (im *Image) FormatEfifs() error {
	if im.efiDevice == "" {
		return errors.New("missing efiDevice, not set in NewImageOptions")
	}

	im.Print("Creating EFI partition on %s\n", im.efiDevice)
	label := "ME" + im.DatedFsLabel()
	args := []string{
		"mkfs.vfat",
		"-F", "32",
		"-n", label,
		im.efiDevice,
	}
	return im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...)
}

// MountEfifs mounts the EFI partition.
func (im *Image) MountEfifs(mountEfifs string) error {
	if im.efiDevice == "" {
		return errors.New("missing efiDevice, not set in NewImageOptions")
	}
	if mountEfifs == "" {
		return errors.New("missing mountEfifs parameter")
	}

	if !filesystems.DirectoryExists(mountEfifs) {
		im.Print("Creating %s ...\n", mountEfifs)
		if err := os.MkdirAll(mountEfifs, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountEfifs, err)
		}
	}

	im.Print("Mounting %s to %s\n", im.efiDevice, mountEfifs)
	im.trackMount(mountEfifs)
	if err := im.runner(nil, im.stdout, im.stderr, "mount", "-t", "vfat", im.efiDevice, mountEfifs); err != nil {
		return err
	}
	im.efifsMount = mountEfifs
	return nil
}

// EfiBootDir returns the full path to the EFI boot directory on the mounted
// EFI filesystem.
func (im *Image) EfiBootDir() (string, error) {
	if im.efifsMount == "" {
		return "", errors.New("EFI filesystem not mounted")
	}
	relEfiBootPath, err := im.RelativeEfiBootPath()
	if err != nil {
		return "", err
	}
	efibootDir := filepath.Join(im.efifsMount, relEfiBootPath)
	return efibootDir, nil
}

// FormatBootfs creates a btrfs filesystem on the boot partition.
func (im *Image) FormatBootfs() error {
	if im.bootDevice == "" {
		return errors.New("missing bootDevice, not set in NewImageOptions")
	}

	label := "MB" + im.DatedFsLabel()
	im.Print("Creating btrfs on %s (boot)\n", im.bootDevice)
	args := []string{
		"mkfs.btrfs",
		"-f",
		"-L", label,
		im.bootDevice,
	}
	return im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...)
}

// MountBootfs mounts the boot partition.
func (im *Image) MountBootfs(mountBootfs string) error {
	if im.bootDevice == "" {
		return errors.New("missing bootDevice, not set in NewImageOptions")
	}
	if mountBootfs == "" {
		return errors.New("missing mountBootfs parameter")
	}

	if !filesystems.DirectoryExists(mountBootfs) {
		im.Print("Creating %s ...\n", mountBootfs)
		if err := os.MkdirAll(mountBootfs, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountBootfs, err)
		}
	}

	im.Print("Mounting %s to %s\n", im.bootDevice, mountBootfs)
	im.trackMount(mountBootfs)
	if err := im.runner(nil, im.stdout, im.stderr, "mount", im.bootDevice, mountBootfs); err != nil {
		return err
	}
	im.bootfsMount = mountBootfs
	return nil
}

// MaybeEncryptRootfs encrypts the root partition with LUKS if encryption is
// enabled in the configuration.
func (im *Image) MaybeEncryptRootfs() error {
	if !im.encrypted {
		return nil
	}

	// Get the current root device.
	rootDevice := im.RootDevice()
	im.realRootDevice = rootDevice

	encRootfsName, err := im.fsenc.EncryptedRootFsName()
	if err != nil {
		return err
	}
	luksDevice, err := filesystems.GetLuksRootfsDevicePath(encRootfsName)
	if err != nil {
		return err
	}
	if err := im.fsenc.LuksEncrypt(im.rootDevice, luksDevice); err != nil {
		return fmt.Errorf("LUKS encryption failed: %w", err)
	}
	im.SetRootDevice(luksDevice)
	im.Print("New encrypted rootfs partition: %s\n", luksDevice)
	return nil
}

// FormatRootfs creates a btrfs filesystem on the root partition.
func (im *Image) FormatRootfs() error {
	if im.rootDevice == "" {
		return errors.New("missing rootDevice, not set in NewImageOptions")
	}

	label := "MR" + im.DatedFsLabel()
	im.Print("Creating btrfs on %s (root)\n", im.rootDevice)
	args := []string{
		"mkfs.btrfs",
		"-f",
		"-L", label,
		im.rootDevice,
	}
	return im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...)
}

// RootfsKernelArgs returns the default kernel arguments for the root filesystem.
func (im *Image) RootfsKernelArgs() []string {
	return []string{"rootflags=discard=async"}
}

// MountRootfs mounts the root partition with btrfs compression options.
func (im *Image) MountRootfs(mountRootfs string) error {
	if im.rootDevice == "" {
		return errors.New("missing rootDevice, not set in NewImageOptions")
	}
	if mountRootfs == "" {
		return errors.New("missing mountRootfs parameter")
	}

	if !filesystems.DirectoryExists(mountRootfs) {
		im.Print("Creating %s ...\n", mountRootfs)
		if err := os.MkdirAll(mountRootfs, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountRootfs, err)
		}
	}

	compression := "zstd:6"
	btrfsOpts := fmt.Sprintf("compress-force=%s,space_cache=v2,commit=120", compression)
	im.Print("Mounting %s to %s\n", im.rootDevice, mountRootfs)

	im.trackMount(mountRootfs)
	args := []string{
		"mount",
		"-o", btrfsOpts,
		im.rootDevice,
		mountRootfs,
	}
	if err := im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...); err != nil {
		return err
	}
	im.rootfsMount = mountRootfs

	return nil
}

// GetKernelPath returns the kernel version directory name from the deployed rootfs.
func (im *Image) GetKernelPath() (string, error) {
	if im.rootfs == "" {
		return "", errors.New("rootfs not set, call SetRootfs first")
	}

	modulesDir := filepath.Join(im.rootfs, "usr", "lib", "modules")
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return "", fmt.Errorf("failed to read modules directory %s: %w", modulesDir, err)
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no kernel directory found in %s", modulesDir)
	}
	sort.Strings(dirs)
	return dirs[0], nil
}

// SetupPasswords sets default passwords for the matrix and root users.
func (im *Image) SetupPasswords() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}

	shadowFile := filepath.Join(im.rootfs, "etc", "shadow")

	cmd := exec.Command("openssl", "passwd", "-6", "matrix")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("openssl passwd failed: %w", err)
	}
	passHash := strings.TrimSpace(string(out))
	lastChange := fmt.Sprintf("%d", time.Now().Unix()/86400)

	data, err := os.ReadFile(shadowFile)
	if err != nil {
		return fmt.Errorf("failed to read shadow file: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		// Remove existing matrix: and root: lines.
		if strings.HasPrefix(line, "matrix:") || strings.HasPrefix(line, "root:") {
			continue
		}
		lines = append(lines, line)
	}

	shadowEntry := func(user string) string {
		return fmt.Sprintf("%s:%s:%s:0:99999:7:::", user, passHash, lastChange)
	}

	im.Print("Setting the default password of matrix to matrix ...\n")
	lines = append(lines, shadowEntry("matrix"))
	im.Print("Setting the default password of root to matrix ...\n")
	lines = append(lines, shadowEntry("root"))

	return os.WriteFile(shadowFile, []byte(strings.Join(lines, "\n")+"\n"), 0640)
}

// SetupBootloaderConfig sets up the GRUB bootloader configuration.
func (im *Image) SetupBootloaderConfig() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}

	if im.efiDevice == "" {
		return errors.New("missing efiDevice, not set in NewImageOptions")
	}
	if im.bootDevice == "" {
		return errors.New("missing bootDevice, not set in NewImageOptions")
	}

	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}
	if im.rootfsMount == "" {
		return errors.New("missing rootfsMount, call MountRootfs first")
	}

	ref, err := im.cleanAndStripRef()
	if err != nil {
		return fmt.Errorf("failed to clean ref: %w", err)
	}

	efibootDir, err := im.EfiBootDir()
	if err != nil {
		return fmt.Errorf("failed to determine EFI boot directory: %w", err)
	}

	efiDeviceUUID, err := filesystems.DeviceUUID(im.efiDevice)
	if err != nil {
		return fmt.Errorf("unable to get UUID for %s: %w", im.efiDevice, err)
	}

	bootDeviceUUID, err := filesystems.DeviceUUID(im.bootDevice)
	if err != nil {
		return fmt.Errorf("unable to get UUID for %s: %w", im.bootDevice, err)
	}

	// Verify kernel exists.
	if _, err := im.GetKernelPath(); err != nil {
		return fmt.Errorf("failed to determine kernel version: %w", err)
	}

	// Get the boot commit.
	bootCommit, err := im.ostree.BootCommit(im.rootfsMount)
	if err != nil || bootCommit == "" {
		return fmt.Errorf("cannot determine ostree boot commit: %w", err)
	}
	im.Print("Found boot commit: %s\n", bootCommit)

	devDir, err := im.DevDir()
	if err != nil {
		return err
	}

	srcGrubCfg := filepath.Join(devDir, "image", "boot", ref, "grub.cfg")
	resolved, err := filepath.EvalSymlinks(srcGrubCfg)
	if err != nil {
		return fmt.Errorf("failed to resolve grub config path %s: %w", srcGrubCfg, err)
	}
	srcGrubCfg = resolved

	if !filesystems.FileExists(srcGrubCfg) {
		return fmt.Errorf("grub config %s does not exist", srcGrubCfg)
	}
	im.Print("Using grub config from %s\n", srcGrubCfg)

	// Ensure efibootDir exists.
	if err := os.MkdirAll(efibootDir, 0755); err != nil {
		return fmt.Errorf("failed to create efibootDir %s: %w", efibootDir, err)
	}

	dstGrubCfg := filepath.Join(efibootDir, "grub.cfg")
	im.Print("Copying grub: %s -> %s\n", srcGrubCfg, dstGrubCfg)
	if err := filesystems.CopyFile(srcGrubCfg, dstGrubCfg); err != nil {
		return fmt.Errorf("failed to copy grub config: %w", err)
	}

	// Copy GRUB themes if available.
	osName, err := im.OsName()
	if err != nil {
		return err
	}
	themesDir := filepath.Join(
		im.rootfs,
		"usr", "share", "grub",
		"themes", osName+"-theme",
	)
	if filesystems.DirectoryExists(themesDir) {
		im.Print("Copying GRUB themes from %s ...\n", themesDir)
		dstThemesDir := filepath.Join(im.bootfsMount, "grub", "themes")

		if err := os.MkdirAll(dstThemesDir, 0755); err != nil {
			return fmt.Errorf("failed to create themes dir: %w", err)
		}

		dstThemeDir := filepath.Join(dstThemesDir, filepath.Base(themesDir))

		if err := filesystems.CopyDirPreserve(themesDir, dstThemeDir); err != nil {
			return fmt.Errorf("failed to copy themes: %w", err)
		}
	}

	// Write GRUB_CFG environment file.
	efiRoot, err := im.EfiRoot()
	if err != nil {
		return err
	}
	relEfiBootPath, err := im.RelativeEfiBootPath()
	if err != nil {
		return err
	}

	envDir := filepath.Join(im.rootfs, "etc", "environment.d")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment.d dir: %w", err)
	}

	grubCfgEnv := fmt.Sprintf("GRUB_CFG=%s/%s/grub.cfg\n", efiRoot, relEfiBootPath)
	grubCfgEnvPath := filepath.Join(envDir, "99-matrixos-imager-grub.conf")
	if err := os.WriteFile(grubCfgEnvPath, []byte(grubCfgEnv), 0644); err != nil {
		return fmt.Errorf("failed to write grub env config: %w", err)
	}

	// Perform template substitutions in grub.cfg.
	grubData, err := os.ReadFile(dstGrubCfg)
	if err != nil {
		return fmt.Errorf("failed to read grub config for substitution: %w", err)
	}
	grubContent := string(grubData)
	grubContent = strings.ReplaceAll(grubContent, "%BOOTUUID%", bootDeviceUUID)
	grubContent = strings.ReplaceAll(grubContent, "%EFIUUID%", efiDeviceUUID)
	grubContent = strings.ReplaceAll(grubContent, "%OSNAME%", osName)
	if err := os.WriteFile(dstGrubCfg, []byte(grubContent), 0644); err != nil {
		return fmt.Errorf("failed to write substituted grub config: %w", err)
	}

	im.Print("Current grub.cfg:\n")
	im.Print("%s\n", grubContent)

	return nil
}

// SetupVmtestConfig creates a VM test grub config based on the ostree boot config.
func (im *Image) SetupVmtestConfig() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}

	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}

	im.Print("Setting up vmtest grub config based on the ostree boot config in %s ...\n", im.bootfsMount)

	ostreeBootCfg := filepath.Join(im.bootfsMount, "loader", "entries", "ostree-1.conf")
	if !filesystems.FileExists(ostreeBootCfg) {
		return fmt.Errorf("%s does not exist, cannot set up vmtest config", ostreeBootCfg)
	}

	vmtestCfgDir := filepath.Join(im.bootfsMount, ".imager.vmtest", "entries")
	if err := os.MkdirAll(vmtestCfgDir, 0755); err != nil {
		return fmt.Errorf("failed to create vmtest config dir: %w", err)
	}

	vmtestBootCfg := filepath.Join(vmtestCfgDir, "ostree-1.conf")

	consoleParams := "console=ttyS0,115200"
	systemdParams := "systemd.log_color=0"
	envParams := "systemd.setenv=SYSTEMD_COLORS=0 systemd.setenv=SYSTEMD_URLIFY=0"
	bootParams := consoleParams + " " + systemdParams + " " + envParams

	if err := filesystems.CopyFile(ostreeBootCfg, vmtestBootCfg); err != nil {
		return fmt.Errorf("failed to copy vmtest config: %w", err)
	}

	data, err := os.ReadFile(vmtestBootCfg)
	if err != nil {
		return fmt.Errorf("failed to read vmtest config: %w", err)
	}

	content := string(data)
	content = strings.ReplaceAll(content, "splash", "")
	content = strings.ReplaceAll(content, "quiet", bootParams)

	if err := os.WriteFile(vmtestBootCfg, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write vmtest config: %w", err)
	}

	im.Print("Set up vmtest grub config at %s\n", vmtestBootCfg)
	im.Print("Current vmtest grub config:\n")
	im.Print("%s\n", content)

	return nil
}

// InstallSecurebootCerts installs SecureBoot certificates on the EFI partition.
func (im *Image) InstallSecurebootCerts() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}
	if im.efifsMount == "" {
		return errors.New("missing efifsMount, call MountEfifs first")
	}
	efibootDir, err := im.EfiBootDir()
	if err != nil {
		return err
	}

	certFileName, err := im.EfiCertificateFileName()
	if err != nil {
		return err
	}
	certDerFileName, err := im.EfiCertificateFileNameDer()
	if err != nil {
		return err
	}
	kekFileName, err := im.EfiCertificateFileNameKek()
	if err != nil {
		return err
	}
	kekDerFileName, err := im.EfiCertificateFileNameKekDer()
	if err != nil {
		return err
	}

	// SecureBoot certificate (db).
	sbCert := filepath.Join(im.rootfs, "etc", "portage", "secureboot.pem")
	if filesystems.FileExists(sbCert) {
		im.Print("Copying SecureBoot cert to EFI partition ...\n")
		if err := filesystems.CopyFile(sbCert, filepath.Join(im.efifsMount, certFileName)); err != nil {
			return fmt.Errorf("failed to copy SecureBoot cert: %w", err)
		}

		im.Print("Generating SecureBoot MOK ...\n")
		if err := im.runner(nil, im.stdout, im.stderr,
			"openssl", "x509", "-in", sbCert,
			"-outform", "DER", "-out", filepath.Join(im.efifsMount, certDerFileName)); err != nil {
			return fmt.Errorf("openssl DER conversion failed: %w", err)
		}
	} else {
		im.PrintWarning("NO SECUREBOOT CERT AT: %s -- ignoring.\n", sbCert)
	}

	// SecureBoot KEK certificate.
	sbKek := filepath.Join(im.rootfs, "etc", "portage", "secureboot-kek.pem")
	if filesystems.FileExists(sbKek) {
		im.Print("Copying SecureBoot KEK cert to EFI partition ...\n")
		if err := filesystems.CopyFile(sbKek, filepath.Join(im.efifsMount, kekFileName)); err != nil {
			return fmt.Errorf("failed to copy SecureBoot KEK cert: %w", err)
		}

		im.Print("Generating SecureBoot KEK DER for convenience ...\n")
		if err := im.runner(nil, im.stdout, im.stderr,
			"openssl", "x509", "-in", sbKek,
			"-outform", "DER", "-out", filepath.Join(im.efifsMount, kekDerFileName)); err != nil {
			return fmt.Errorf("openssl KEK DER conversion failed: %w", err)
		}
	} else {
		im.PrintWarning("NO SECUREBOOT CERT AT: %s -- ignoring.\n", sbKek)
	}

	// Copy the shim binaries.
	shimDir := filepath.Join(im.rootfs, "usr", "share", "shim")
	im.Print("Copying shim for Secureboot from %s to %s ...\n", shimDir, efibootDir)
	return filesystems.CopyDirPreserve(shimDir, efibootDir)
}

// InstallMemtest installs the memtest86+ EFI binary to the EFI boot directory.
func (im *Image) InstallMemtest() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}
	efibootDir, err := im.EfiBootDir()
	if err != nil {
		return err
	}

	memtestBin := filepath.Join(im.rootfs, "usr", "share", "memtest86+", "memtest.efi64")
	if !filesystems.PathExists(memtestBin) {
		im.PrintWarning("WARNING: %s not available, please install memtest86+\n", memtestBin)
		return nil
	}
	return filesystems.CopyFile(memtestBin, filepath.Join(efibootDir, "memtest86plus.efi"))
}

// InstallBootloader installs the GRUB bootloader into the image by running
// grub-install inside a chroot of the deployed rootfs, then replaces the
// unsigned GRUBX64.EFI with the signed version.
// It returns the list of extra mounts created during the process so the caller
// can track them for cleanup.
func (im *Image) InstallBootloader() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}
	if im.efifsMount == "" {
		return errors.New("missing efifsMount, call MountEfifs first")
	}
	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}
	if im.devicePath == "" {
		return errors.New("missing devicePath, not set in NewImageOptions")
	}
	efibootDir, err := im.EfiBootDir()
	if err != nil {
		return fmt.Errorf("failed to determine EFI boot directory: %w", err)
	}

	im.Print("Installing bootloader ...\n")

	efiRoot, err := im.EfiRoot()
	if err != nil {
		return fmt.Errorf("failed to determine EFI root: %w", err)
	}
	bootRoot, err := im.BootRoot()
	if err != nil {
		return fmt.Errorf("failed to determine boot root: %w", err)
	}
	osName, err := im.OsName()
	if err != nil {
		return fmt.Errorf("failed to determine OS name: %w", err)
	}
	efiExe, err := im.EfiExecutable()
	if err != nil {
		return fmt.Errorf("failed to determine EFI executable: %w", err)
	}

	// Bind mount EFI into the chroot.
	efiChrootMount := filepath.Join(im.rootfs, efiRoot)
	if err := os.MkdirAll(efiChrootMount, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", efiChrootMount, err)
	}
	im.trackMount(efiChrootMount)
	err = filesystems.BindMount(im.efifsMount, efiChrootMount)
	if err != nil {
		return fmt.Errorf("failed to bind mount EFI: %w", err)
	}

	// Bind mount boot into the chroot.
	bootChrootMount := filepath.Join(im.rootfs, bootRoot)
	if err := os.MkdirAll(bootChrootMount, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", bootChrootMount, err)
	}
	im.trackMount(bootChrootMount)
	err = filesystems.BindMount(im.bootfsMount, bootChrootMount)
	if err != nil {
		return fmt.Errorf("failed to bind mount boot: %w", err)
	}

	// Setup common rootfs mounts (dev, proc, etc.) without proc for bootloader.
	mounter, err := filesystems.NewCommonRootfsMounts(
		im.rootfs,
		func(tg string) {
			im.trackMount(tg)
		},
		func(tg string) {},
	)
	if err != nil {
		return fmt.Errorf("failed to create common rootfs mounter: %w", err)
	}
	if err := mounter.Setup(); err != nil {
		return fmt.Errorf("failed to setup common rootfs mounts: %w", err)
	}

	// Run grub-install inside the chroot.
	err = filesystems.ChrootRun(im.rootfs, "/usr/bin/grub-install",
		"--target=x86_64-efi",
		"--directory=/usr/lib/grub/x86_64-efi",
		"--efi-directory="+efiRoot,
		"--boot-directory="+bootRoot,
		"--themes="+osName+"-theme",
		"--removable",
		"--modules=ext2 btrfs gzio part_gpt fat part_msdos all_video",
		im.devicePath,
	)

	// Clean up chroot mounts regardless of grub-install result.
	filesystems.BindUmount(bootChrootMount)
	filesystems.BindUmount(efiChrootMount)
	mounter.Cleanup()

	if err != nil {
		return fmt.Errorf("grub-install failed: %w", err)
	}

	// Verify BOOTX64.EFI was created.
	bootx64efi := filepath.Join(efibootDir, efiExe)
	if !filesystems.PathExists(bootx64efi) {
		return fmt.Errorf("%s does not exist after grub-install", bootx64efi)
	}

	// Replace unsigned GRUBX64.EFI with the signed one.
	grubx64efi := filepath.Join(efibootDir, "GRUBX64.EFI")
	im.Print("Removing existing %s as it's not signed ...\n", grubx64efi)
	os.Remove(grubx64efi)

	signedGrubx64efi := filepath.Join(im.rootfs, "usr", "lib", "grub", "grub-x86_64.efi.signed")
	im.Print("Moving %s to %s\n", signedGrubx64efi, grubx64efi)
	if err := os.Rename(signedGrubx64efi, grubx64efi); err != nil {
		return fmt.Errorf("failed to move signed grub binary: %w", err)
	}

	return nil
}

// GenerateKernelBootArgs generates the kernel boot arguments for the image.
func (im *Image) GenerateKernelBootArgs() ([]string, error) {
	ref, err := im.cleanAndStripRef()
	if err != nil {
		return nil, fmt.Errorf("failed to clean ref: %w", err)
	}
	if im.efiDevice == "" {
		return nil, errors.New("missing efiDevice, not set in NewImageOptions")
	}
	if im.bootDevice == "" {
		return nil, errors.New("missing bootDevice, not set in NewImageOptions")
	}
	if im.rootDevice == "" {
		return nil, errors.New("missing rootDevice, not set in NewImageOptions")
	}

	// if we are encrypting, use the realRootDevice
	rootDevice := im.rootDevice
	if im.encrypted {
		if im.realRootDevice == "" {
			return nil, errors.New("missing realRootDevice for encrypted image")
		}
		rootDevice = im.realRootDevice
	}

	bootArgs := im.RootfsKernelArgs()

	// Root device UUID for LUKS.
	rootDeviceUUID, err := filesystems.DeviceUUID(rootDevice)
	if err != nil {
		return nil, fmt.Errorf("unable to get device UUID for %s: %w", rootDevice, err)
	}
	if im.encrypted {
		bootArgs = append(bootArgs, fmt.Sprintf("rd.luks.uuid=%s", rootDeviceUUID))
	}

	// EFI partition mount via systemd.
	efiRoot, err := im.EfiRoot()
	if err != nil {
		return nil, err
	}
	efiPartUUID, err := filesystems.DevicePartUUID(im.efiDevice)
	if err != nil {
		return nil, fmt.Errorf("unable to get PARTUUID of EFI partition: %w", err)
	}
	bootArgs = append(bootArgs, fmt.Sprintf("systemd.mount-extra=PARTUUID=%s:%s:auto:defaults", efiPartUUID, efiRoot))

	// Boot partition mount via systemd.
	bootRoot, err := im.BootRoot()
	if err != nil {
		return nil, err
	}
	bootPartUUID, err := filesystems.DevicePartUUID(im.bootDevice)
	if err != nil {
		return nil, fmt.Errorf("unable to get PARTUUID of boot partition: %w", err)
	}
	bootArgs = append(bootArgs, fmt.Sprintf("systemd.mount-extra=PARTUUID=%s:%s:auto:defaults", bootPartUUID, bootRoot))

	// Read additional kernel cmdline params from the image boot directory.
	devDir, err := im.DevDir()
	if err != nil {
		return nil, err
	}
	cmdlineFile := filepath.Join(devDir, "image", "boot", ref, "cmdline.conf")
	if filesystems.FileExists(cmdlineFile) {
		im.Print("Reading additional kernel args from %s ...\n", cmdlineFile)
		data, err := os.ReadFile(cmdlineFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read cmdline file: %w", err)
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			bootArgs = append(bootArgs, line)
		}
	} else {
		im.PrintWarning("WARNING: no additional kernel cmdline params available, %s does not exist.\n", cmdlineFile)
	}

	return bootArgs, nil
}

// ExtractPackageList returns the list of packages installed in a rootfs.
func (im *Image) ExtractPackageList() ([]string, error) {
	if im.rootfs == "" {
		return nil, errors.New("rootfs not set, call SetRootfs first")
	}

	roVdb, err := im.ReadOnlyVdb()
	if err != nil {
		return nil, err
	}

	vdb := filepath.Join(im.rootfs, roVdb)
	if !filesystems.DirectoryExists(vdb) {
		im.PrintError("%s does not exist. cannot generate pkglist\n", vdb)
		return nil, nil
	}

	var pkgList []string
	categories, err := os.ReadDir(vdb)
	if err != nil {
		return nil, fmt.Errorf("failed to read vdb directory %s: %w", vdb, err)
	}
	for _, cat := range categories {
		if !cat.IsDir() {
			continue
		}
		catPath := filepath.Join(vdb, cat.Name())
		pkgs, err := os.ReadDir(catPath)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read category directory %s: %w", catPath, err)
		}
		for _, pkg := range pkgs {
			pkgList = append(pkgList, filepath.Join(cat.Name(), pkg.Name()))
		}
	}

	im.Print("Generated package list:\n")
	for _, pkg := range pkgList {
		im.Print(">> %s\n", pkg)
	}
	return pkgList, nil
}

// SetupHooks runs image-specific hook scripts.
func (im *Image) SetupHooks() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}

	ref, err := im.cleanAndStripRef()
	if err != nil {
		return fmt.Errorf("failed to clean ref: %w", err)
	}

	devDir, err := im.DevDir()
	if err != nil {
		return err
	}

	hooksSrcDir := filepath.Join(devDir, "image", "hooks")
	if !filesystems.DirectoryExists(hooksSrcDir) {
		im.PrintError("hooks source dir %s does not exist\n", hooksSrcDir)
		return nil
	}

	hookExec := filepath.Join(hooksSrcDir, ref+".sh")
	if !filesystems.FileExists(hookExec) {
		im.PrintError("hook script %s does not exist\n", hookExec)
		return nil
	}

	info, err := os.Stat(hookExec)
	if err != nil {
		return fmt.Errorf("failed to stat hook script: %w", err)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("hook script %s is not executable", hookExec)
	}

	cmd := exec.Command(hookExec)
	cmd.Stdout = im.stdout
	cmd.Stderr = im.stderr
	cmd.Env = append(os.Environ(),
		"MATRIXOS_DEV_DIR="+devDir,
		"ROOTFS="+im.rootfs,
		"REF="+ref,
	)
	return cmd.Run()
}

// TestImage copies an image to a temp directory and runs test scripts against it.
func (im *Image) TestImage() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}

	ref, err := im.cleanAndStripRef()
	if err != nil {
		return fmt.Errorf("failed to clean ref: %w", err)
	}

	devDir, err := im.DevDir()
	if err != nil {
		return err
	}

	testDir := filepath.Join(devDir, "image", "tests", ref)
	if !filesystems.DirectoryExists(testDir) {
		im.PrintError("test dir %s does not exist, skipping test\n", testDir)
		return nil
	}

	mountDir, err := im.MountDir()
	if err != nil {
		return err
	}

	imageTempDir, err := filesystems.CreateTempDir(mountDir, refToSuffix(ref))
	if err != nil {
		return fmt.Errorf("failed to create temp dir for testing: %w", err)
	}
	defer os.RemoveAll(imageTempDir)

	imageName := filepath.Base(im.imagePath)
	testImagePath := filepath.Join(imageTempDir, imageName)
	im.Print("Copying image to %s for testing ...\n", testImagePath)
	if err := filesystems.CopyFileReflink(im.imagePath, testImagePath); err != nil {
		return fmt.Errorf("failed to copy image for testing: %w", err)
	}

	logsDir, err := im.cfg.GetItem("matrixOS.LogsDir")
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(testDir)
	if err != nil {
		return fmt.Errorf("failed to read test dir: %w", err)
	}
	for _, entry := range entries {
		ts := filepath.Join(testDir, entry.Name())
		info, err := os.Stat(ts)
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
			im.PrintError("Skipping non-executable test script %s\n", ts)
			continue
		}

		im.Print("Running test script %s ...\n", ts)
		cmd := exec.Command(ts)
		cmd.Stdout = im.stdout
		cmd.Stderr = im.stderr
		cmd.Env = append(os.Environ(),
			"MATRIXOS_DEV_DIR="+devDir,
			"MATRIXOS_LOGS_DIR="+logsDir,
			"IMAGE_PATH="+testImagePath,
			"REF="+ref,
		)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("test script %s failed: %w", ts, err)
		}
	}
	return nil
}

// FinalizeFilesystems runs fstrim on the root and boot filesystems to improve
// compression ratios for sparse image files.
func (im *Image) FinalizeFilesystems() error {
	if im.rootfsMount == "" {
		return errors.New("missing rootfsMount, call MountRootfs first")
	}
	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}
	if im.efifsMount == "" {
		return errors.New("missing efifsMount, call MountEfifs first")
	}

	// fstrim may fail on USB sticks, so errors are intentionally ignored.
	filesystems.FstrimAll(
		im.runner, im.stdout, im.stderr,
		im.rootfsMount, im.bootfsMount,
	)

	return nil
}

// Qcow2ImagePath returns the qcow2 image path for a given .img path.
func (im *Image) Qcow2ImagePath() (string, error) {
	if err := im.validateImageModeForCreation(); err != nil {
		return "", err
	}
	return im.imagePath + ".qcow2", nil
}

// CreateQcow2Image creates a compressed qcow2 image from a raw image.
func (im *Image) CreateQcow2Image() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}
	qcow2Path, _ := im.Qcow2ImagePath()
	return im.runner(nil, im.stdout, im.stderr,
		"qemu-img", "convert", "-c", "-O", "qcow2", "-p", im.imagePath, qcow2Path)
}

// ShowFinalFilesystemInfo displays information about the final filesystem layout.
func (im *Image) ShowFinalFilesystemInfo() error {
	if im.devicePath == "" {
		return errors.New("missing devicePath, not set in NewImageOptions")
	}
	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}
	if im.efifsMount == "" {
		return errors.New("missing efifsMount, call MountEfifs first")
	}

	im.Print("Final boot partition directory tree:\n")
	if err := filesystems.PrintDirectoryTree(im.stdout, im.bootfsMount); err != nil {
		im.PrintWarning("WARNING: failed to list boot directory tree: %v\n", err)
	}

	im.Print("Final EFI partition directory tree:\n")
	if err := filesystems.PrintDirectoryTree(im.stdout, im.efifsMount); err != nil {
		im.PrintWarning("WARNING: failed to list EFI directory tree: %v\n", err)
	}

	im.Print("Block devices on %s:\n", im.devicePath)
	if err := filesystems.PrintBlockDeviceInfo(im.stdout, im.devicePath); err != nil {
		im.PrintWarning("WARNING: failed to get block device info: %v\n", err)
	}

	im.Print("Filesystem setup complete!\n")
	return nil
}

// ShowImageTestInfo prints information about generated artifacts and how to test them.
func (im *Image) ShowImageTestInfo(artifacts []string) {
	if err := im.validateImageModeForCreation(); err != nil {
		im.PrintError("show_test_info: invalid image mode: %v\n", err)
		return
	}

	if len(artifacts) != 0 {
		im.Print("Generated artifacts:\n")
		for _, a := range artifacts {
			im.Print(">> %s\n", a)
		}
	}

	im.Print("\n")
	im.Print("How to test:\n")
	im.Print("$ vector dev vm -image IMAGE_PATH -memory 8G -interactive\n")
	im.Print("\n")
	im.Print("To move to a USB stick:\n")
	im.Print("    dd if=IMAGE_PATH of=/dev/sdX bs=4M conv=sparse,sync status=progress\n")
	im.Print("\n")
	im.Print("\nImage creation complete! > %s\n", im.ImagePath())
}

// RemoveImageFile removes an image file and its associated .sha256 and .asc files.
func (im *Image) RemoveImageFile() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}

	im.Print("Removing %s ...\n", im.imagePath)
	for _, path := range []string{im.imagePath, im.imagePath + ".sha256", im.imagePath + ".asc"} {
		os.Remove(path) // Ignore errors (file may not exist).
	}
	return nil
}

// ImageLockDir returns the image lock directory, creating it if necessary.
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

// ImageLockPath returns the lock file path for the stored ref.
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

// ExecuteWithImageLock acquires an exclusive file lock for the given ref,
// executes fn under that lock, and releases the lock when fn returns.
// If the lock cannot be acquired within the configured timeout, an error is returned.
// If fn panics or the process crashes, the OS closes the file descriptor and
// releases the lock automatically.
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
