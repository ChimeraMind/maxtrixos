package imager

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
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
	Mode       ImageMode
}

// IImage defines the interface for image operations.
// It mirrors all public methods of Image for testability.
type IImage interface {
	// SetStdout replaces the writer used for informational output.
	SetStdout(w io.Writer)
	// SetStderr replaces the writer used for warnings and errors.
	SetStderr(w io.Writer)
	// Stdout returns the current informational output writer.
	Stdout() io.Writer
	// Stderr returns the current warning/error output writer.
	Stderr() io.Writer

	// Operations
	ReleaseVersion(rootfs string) (string, error)
	ImagePath(ref string) (string, error)
	ImagePathWithReleaseVersion(ref, releaseVersion string) (string, error)
	CreateImage(imagePath, imageSize string) error
	ImagePathWithCompressorExtension(imagePath string) (string, error)
	CompressImage(imagePath string) error
	ClearPartitionTable(devicePath string) error
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
	MountRootfs(rootDevice, mountRootfs string) error
	GetKernelPath(ostreeDeployRootfs string) (string, error)
	SetupPasswords(ostreeDeployRootfs string) error
	SetupBootloaderConfig(ref, ostreeDeployRootfs, sysroot, bootdir, efibootdir, efiUUID, bootUUID string) error
	SetupVmtestConfig(bootdir string) error
	InstallSecurebootCerts(ostreeDeployRootfs, mountEfifs, efibootdir string) error
	InstallMemtest(ostreeDeployRootfs, efibootdir string) error
	GenerateKernelBootArgs(ref, efiDevice, bootDevice, physicalRootDevice, rootDevice string, encryptionEnabled bool) ([]string, error)
	PackageList(rootfs string) ([]string, error)
	SetupHooks(ostreeDeployRootfs, ref string) error
	InstallBootloader(ostreeDeployRootfs, mountEfifs, mountBootfs, blockDevice, efibootdir string) ([]string, error)
	TestImage(imagePath, ref string) error
	FinalizeFilesystems(mountRootfs, mountBootfs, mountEfifs string) error
	Qcow2ImagePath(imagePath string) (string, error)
	CreateQcow2Image(imagePath string) error
	ShowFinalFilesystemInfo(blockDevice, mountBootfs, mountEfifs string) error
	ShowTestInfo(artifacts []string)
	RemoveImageFile(imagePath string) error
	ImageLockDir() (string, error)
	ImageLockPath(ref string) (string, error)
	ExecuteWithImageLock(ref string, fn func() error) error
}

// Image provides image creation and manipulation operations.
type Image struct {
	cfg                 config.IConfig
	ostree              ostree.IOstree
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

func (im *Image) SetStdout(w io.Writer) { im.stdout = w }

func (im *Image) SetStderr(w io.Writer) { im.stderr = w }

func (im *Image) Stdout() io.Writer { return im.stdout }

func (im *Image) Stderr() io.Writer { return im.stderr }

func (im *Image) Print(format string, args ...any) {
	fmt.Fprintf(im.stdout, format, args...)
}

func (im *Image) PrintWarning(format string, args ...any) {
	fmt.Fprintf(im.stderr, format, args...)
}

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
func NewImage(cfg config.IConfig, ot ostree.IOstree, fsenc filesystems.IFsenc, opts *NewImageOptions) (*Image, error) {
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

// ImagesOutDir returns the directory where generated images are stored.
func (im *Image) ImagesOutDir() (string, error) {
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

// --- Helpers ---

// parseHumanSize converts a human-readable size string (e.g. "32G", "200M", "1T")
// to bytes. Supports K, M, G, T suffixes (case-insensitive). Without a suffix,
// the value is treated as bytes.
func parseHumanSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errors.New("empty size string")
	}

	multiplier := int64(1)
	suffix := s[len(s)-1]
	switch suffix {
	case 'k', 'K':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'm', 'M':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	case 'g', 'G':
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	case 't', 'T':
		multiplier = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}

	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}
	return n * multiplier, nil
}

// imagePath builds the full image file path from a suffix.
func (im *Image) imagePath(suffix string) (string, error) {
	outDir, err := im.ImagesOutDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(outDir, suffix), nil
}

// cleanAndStripRef cleans a remote prefix and removes the -full suffix from a ref.
func (im *Image) cleanAndStripRef(ref string) (string, error) {
	ref = cds.CleanRemoteFromRef(ref)
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

func extractSeedName(data []byte) (string, error) {
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
			fmt.Fprintf(os.Stderr, "Extracted release version: %s\n", releaseVersion)
		} else {
			fmt.Fprintf(os.Stderr, "WARNING: SEED_NAME= value has no '-' separator\n")
		}
		break

	}
	if scanner.Err() != nil {
		return releaseVersion, fmt.Errorf("failed to scan build metadata file: %w", scanner.Err())
	}
	return releaseVersion, nil
}

// ReleaseVersion extracts or generates a release version string for an image.
// It attempts to read a build metadata file from the rootfs for the version;
// if unavailable, falls back to the current date (YYYYMMDD).
func (im *Image) ReleaseVersion(rootfs string) (string, error) {
	if rootfs == "" {
		return "", errors.New("missing rootfs parameter")
	}

	releaseVersion := time.Now().Format("20060102")

	metadataRelPath, err := im.BuildMetadataFile()
	if err != nil {
		return "", fmt.Errorf("failed to determine build metadata file path: %w", err)
	}
	metadataFile := filepath.Join(rootfs, metadataRelPath)

	if filesystems.FileExists(metadataFile) {
		fmt.Fprintf(os.Stderr, "Build metadata:\n")
		data, err := os.ReadFile(metadataFile)
		if err != nil {
			return "", fmt.Errorf("failed to read build metadata file %s: %w", metadataFile, err)
		}
		fmt.Fprint(os.Stderr, string(data))

		releaseVersion, err = extractSeedName(data)
		if err != nil {
			return "", fmt.Errorf("failed to extract release version from build metadata: %w", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "WARNING! Build metadata file not found: %s\n", metadataFile)
	}

	return releaseVersion, nil
}

// ImagePath returns the image file path for a given ostree ref.
func (im *Image) ImagePath(ref string) (string, error) {
	if ref == "" {
		return "", errors.New("missing ref parameter")
	}
	ref = cds.CleanRemoteFromRef(ref)
	suffix := refToSuffix(ref) + ".img"
	return im.imagePath(suffix)
}

// ImagePathWithReleaseVersion returns the image file path with an embedded release version.
func (im *Image) ImagePathWithReleaseVersion(ref, releaseVersion string) (string, error) {
	if ref == "" {
		return "", errors.New("missing ref parameter")
	}
	if releaseVersion == "" {
		return "", errors.New("missing releaseVersion parameter")
	}
	ref = cds.CleanRemoteFromRef(ref)
	suffix := refToSuffix(ref) + "-" + releaseVersion + ".img"
	return im.imagePath(suffix)
}

// CreateImage creates a sparse image file at imagePath with the given size.
func (im *Image) CreateImage(imagePath, imageSize string) (retErr error) {
	if imagePath == "" {
		return errors.New("missing imagePath parameter")
	}
	if imageSize == "" {
		return errors.New("missing imageSize parameter")
	}

	imagesDir := filepath.Dir(imagePath)
	fmt.Fprintf(os.Stdout, "Creating images directory: %s (if it does not exist)\n", imagesDir)
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create images directory %s: %w", imagesDir, err)
	}

	// Don't skip removing or sgdisk gets confused due to truncate.
	if err := im.RemoveImageFile(imagePath); err != nil {
		return err
	}

	sizeBytes, err := parseHumanSize(imageSize)
	if err != nil {
		return fmt.Errorf("failed to parse image size %q: %w", imageSize, err)
	}

	fmt.Fprintf(os.Stdout, "Creating block device image file: %s\n", imagePath)
	f, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("failed to create image file %s: %w", imagePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && retErr == nil {
			retErr = cerr
		}
	}()

	if err := f.Truncate(sizeBytes); err != nil {
		return fmt.Errorf("failed to truncate image file %s to %d bytes: %w", imagePath, sizeBytes, err)
	}
	return nil
}

// ImagePathWithCompressorExtension appends the compressor's file extension to the image path.
// The extension is derived from the first word of the compressor command string.
func (im *Image) ImagePathWithCompressorExtension(imagePath string) (string, error) {
	if imagePath == "" {
		return "", errors.New("missing imagePath parameter")
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
	return imagePath + "." + parts[0], nil
}

// CompressImage compresses an image file using the configured compressor.
func (im *Image) CompressImage(imagePath string) error {
	if imagePath == "" {
		return errors.New("missing imagePath parameter")
	}
	compressor, err := im.Compressor()
	if err != nil {
		return fmt.Errorf("failed to get compressor: %w", err)
	}
	if compressor == "" {
		return errors.New("missing compressor parameter")
	}

	imagePathWithExt, err := im.ImagePathWithCompressorExtension(imagePath)
	if err != nil {
		return err
	}

	parts := strings.Fields(compressor)
	if len(parts) == 0 {
		return errors.New("invalid compressor parameters: empty command")
	}
	args := append(parts[1:], imagePath)
	if err := im.runner(nil, os.Stdout, os.Stderr, parts[0], args...); err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}

	if !filesystems.FileExists(imagePathWithExt) {
		return fmt.Errorf("compressed image was not created at the expected path: %s", imagePathWithExt)
	}
	return nil
}

// ClearPartitionTable clears the partition table on a device using sgdisk.
func (im *Image) ClearPartitionTable(devicePath string) error {
	if devicePath == "" {
		return errors.New("missing devicePath parameter")
	}

	fmt.Fprintf(os.Stdout, "Clearing partition table on %s ...\n", devicePath)
	if err := im.runner(nil, os.Stdout, os.Stderr, "sgdisk", "-g", "-o", devicePath); err != nil {
		return fmt.Errorf("sgdisk -g -o failed on %s: %w", devicePath, err)
	}
	return im.runner(nil, os.Stdout, os.Stderr, "sgdisk", "-Z", devicePath)
}

// DatedFsLabel returns a filesystem label based on the current date (YYYYMMDD).
func (im *Image) DatedFsLabel() string {
	return time.Now().Format("20060102")
}

// PartitionDevices creates the EFI, boot, and root partitions on a device.
func (im *Image) PartitionDevices(efiSize, bootSize, imageSize, devicePath string) error {
	if efiSize == "" {
		return errors.New("missing efiSize parameter")
	}
	if bootSize == "" {
		return errors.New("missing bootSize parameter")
	}
	if imageSize == "" {
		return errors.New("missing imageSize parameter")
	}
	if devicePath == "" {
		return errors.New("missing devicePath parameter")
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

	fmt.Fprintf(os.Stdout, "Partitioning %s:\n", devicePath)
	fmt.Fprintf(os.Stdout, " --> p1 (EFI: %s)\n", efiSize)
	fmt.Fprintf(os.Stdout, " --> p2 (BOOT: %s)\n", bootSize)
	fmt.Fprintf(os.Stdout, " --> p3 (ROOT: Remainder of %s, plus autogrow)\n\n", imageSize)

	// Create EFI partition.
	if err := im.runner(nil, os.Stdout, os.Stderr, "sgdisk",
		"-n", fmt.Sprintf("1:0:+%s", efiSize),
		"-t", fmt.Sprintf("1:%s", espPartType),
		devicePath); err != nil {
		return fmt.Errorf("sgdisk EFI partition failed: %w", err)
	}

	// Create boot partition.
	if err := im.runner(nil, os.Stdout, os.Stderr, "sgdisk",
		"-n", fmt.Sprintf("2:0:+%s", bootSize),
		"-t", fmt.Sprintf("2:%s", bootPartType),
		devicePath); err != nil {
		return fmt.Errorf("sgdisk boot partition failed: %w", err)
	}

	// Create root partition with -10M padding for systemd-repart.
	if err := im.runner(nil, os.Stdout, os.Stderr, "sgdisk",
		"-n", "3:0:-10M",
		"-t", fmt.Sprintf("3:%s", rootPartType),
		devicePath); err != nil {
		return fmt.Errorf("sgdisk root partition failed: %w", err)
	}

	// Set the auto-grow flag (bit 59) on partition 3.
	if err := im.runner(nil, os.Stdout, os.Stderr, "sgdisk",
		"-A", "3:set:59",
		devicePath); err != nil {
		return fmt.Errorf("sgdisk set auto-grow flag failed: %w", err)
	}

	fmt.Fprintln(os.Stdout, "Refreshing partition table ...")
	if err := im.runner(nil, os.Stdout, os.Stderr, "partprobe", "-s", devicePath); err != nil {
		return fmt.Errorf("partprobe failed: %w", err)
	}

	filesystems.DevicesSettle()
	return nil
}

// FormatEfifs creates a FAT32 filesystem on the EFI partition.
func (im *Image) FormatEfifs(efiDevice string) error {
	if efiDevice == "" {
		return errors.New("missing efiDevice parameter")
	}

	fmt.Fprintf(os.Stdout, "Creating EFI partition on %s\n", efiDevice)
	label := "ME" + im.DatedFsLabel()
	return im.runner(nil, os.Stdout, os.Stderr, "mkfs.vfat", "-F", "32", "-n", label, efiDevice)
}

// MountEfifs mounts the EFI partition.
func (im *Image) MountEfifs(efiDevice, mountEfifs string) error {
	if efiDevice == "" {
		return errors.New("missing efiDevice parameter")
	}
	if mountEfifs == "" {
		return errors.New("missing mountEfifs parameter")
	}

	if !filesystems.DirectoryExists(mountEfifs) {
		fmt.Fprintf(os.Stdout, "Creating %s ...\n", mountEfifs)
		if err := os.MkdirAll(mountEfifs, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountEfifs, err)
		}
	}

	fmt.Fprintf(os.Stdout, "Mounting %s to %s\n", efiDevice, mountEfifs)
	return im.runner(nil, os.Stdout, os.Stderr, "mount", "-t", "vfat", efiDevice, mountEfifs)
}

// FormatBootfs creates a btrfs filesystem on the boot partition.
func (im *Image) FormatBootfs(bootDevice string) error {
	if bootDevice == "" {
		return errors.New("missing bootDevice parameter")
	}

	label := "MB" + im.DatedFsLabel()
	fmt.Fprintf(os.Stdout, "Creating btrfs on %s (boot)\n", bootDevice)
	return im.runner(nil, os.Stdout, os.Stderr, "mkfs.btrfs", "-f", "-L", label, bootDevice)
}

// MountBootfs mounts the boot partition.
func (im *Image) MountBootfs(bootDevice, mountBootfs string) error {
	if bootDevice == "" {
		return errors.New("missing bootDevice parameter")
	}
	if mountBootfs == "" {
		return errors.New("missing mountBootfs parameter")
	}

	if !filesystems.DirectoryExists(mountBootfs) {
		fmt.Fprintf(os.Stdout, "Creating %s ...\n", mountBootfs)
		if err := os.MkdirAll(mountBootfs, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountBootfs, err)
		}
	}

	fmt.Fprintf(os.Stdout, "Mounting %s to %s\n", bootDevice, mountBootfs)
	return im.runner(nil, os.Stdout, os.Stderr, "mount", bootDevice, mountBootfs)
}

// FormatRootfs creates a btrfs filesystem on the root partition.
func (im *Image) FormatRootfs(rootDevice string) error {
	if rootDevice == "" {
		return errors.New("missing rootDevice parameter")
	}

	label := "MR" + im.DatedFsLabel()
	fmt.Fprintf(os.Stdout, "Creating btrfs on %s (root)\n", rootDevice)
	return im.runner(nil, os.Stdout, os.Stderr, "mkfs.btrfs", "-f", "-L", label, rootDevice)
}

// RootfsKernelArgs returns the default kernel arguments for the root filesystem.
func (im *Image) RootfsKernelArgs() []string {
	return []string{"rootflags=discard=async"}
}

// MountRootfs mounts the root partition with btrfs compression options.
func (im *Image) MountRootfs(rootDevice, mountRootfs string) error {
	if rootDevice == "" {
		return errors.New("missing rootDevice parameter")
	}
	if mountRootfs == "" {
		return errors.New("missing mountRootfs parameter")
	}

	if !filesystems.DirectoryExists(mountRootfs) {
		fmt.Fprintf(os.Stdout, "Creating %s ...\n", mountRootfs)
		if err := os.MkdirAll(mountRootfs, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountRootfs, err)
		}
	}

	compression := "zstd:6"
	btrfsOpts := fmt.Sprintf("compress-force=%s,space_cache=v2,commit=120", compression)
	fmt.Fprintf(os.Stdout, "Mounting %s to %s\n", rootDevice, mountRootfs)
	return im.runner(nil, os.Stdout, os.Stderr, "mount", "-o", btrfsOpts, rootDevice, mountRootfs)
}

// GetKernelPath returns the kernel version directory name from the deployed rootfs.
func (im *Image) GetKernelPath(ostreeDeployRootfs string) (string, error) {
	if ostreeDeployRootfs == "" {
		return "", errors.New("missing ostreeDeployRootfs parameter")
	}

	modulesDir := filepath.Join(ostreeDeployRootfs, "usr", "lib", "modules")
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
func (im *Image) SetupPasswords(ostreeDeployRootfs string) error {
	if ostreeDeployRootfs == "" {
		return errors.New("missing ostreeDeployRootfs parameter")
	}

	shadowFile := filepath.Join(ostreeDeployRootfs, "etc", "shadow")

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

	fmt.Fprintln(os.Stdout, "Setting the default password of matrix to matrix ...")
	lines = append(lines, shadowEntry("matrix"))
	fmt.Fprintln(os.Stdout, "Setting the default password of root to matrix ...")
	lines = append(lines, shadowEntry("root"))

	return os.WriteFile(shadowFile, []byte(strings.Join(lines, "\n")+"\n"), 0640)
}

// SetupBootloaderConfig sets up the GRUB bootloader configuration.
func (im *Image) SetupBootloaderConfig(ref, ostreeDeployRootfs, sysroot, bootdir, efibootdir, efiUUID, bootUUID string) error {
	if ref == "" {
		return errors.New("missing ref parameter")
	}
	ref, err := im.cleanAndStripRef(ref)
	if err != nil {
		return fmt.Errorf("failed to clean ref: %w", err)
	}
	if ostreeDeployRootfs == "" {
		return errors.New("missing ostreeDeployRootfs parameter")
	}
	if sysroot == "" {
		return errors.New("missing sysroot parameter")
	}
	if bootdir == "" {
		return errors.New("missing bootdir parameter")
	}
	if efibootdir == "" {
		return errors.New("missing efibootdir parameter")
	}
	if efiUUID == "" {
		return errors.New("missing efiUUID parameter")
	}
	if bootUUID == "" {
		return errors.New("missing bootUUID parameter")
	}

	// Verify kernel exists.
	if _, err := im.GetKernelPath(ostreeDeployRootfs); err != nil {
		return fmt.Errorf("failed to determine kernel version: %w", err)
	}

	// Get the boot commit.
	bootCommit, err := im.ostree.BootCommit(sysroot)
	if err != nil || bootCommit == "" {
		return fmt.Errorf("cannot determine ostree boot commit: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Found boot commit: %s\n", bootCommit)

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
	fmt.Fprintf(os.Stdout, "Using grub config from %s\n", srcGrubCfg)

	// Ensure efibootdir exists.
	if err := os.MkdirAll(efibootdir, 0755); err != nil {
		return fmt.Errorf("failed to create efibootdir %s: %w", efibootdir, err)
	}

	dstGrubCfg := filepath.Join(efibootdir, "grub.cfg")
	fmt.Fprintf(os.Stdout, "Copying grub: %s -> %s\n", srcGrubCfg, dstGrubCfg)
	if err := filesystems.CopyFile(srcGrubCfg, dstGrubCfg); err != nil {
		return fmt.Errorf("failed to copy grub config: %w", err)
	}

	// Copy GRUB themes if available.
	osName, err := im.OsName()
	if err != nil {
		return err
	}
	themesDir := filepath.Join(ostreeDeployRootfs, "usr", "share", "grub", "themes", osName+"-theme")
	if filesystems.DirectoryExists(themesDir) {
		fmt.Fprintf(os.Stdout, "Copying GRUB themes from %s ...\n", themesDir)
		dstThemesDir := filepath.Join(bootdir, "grub", "themes")
		if err := os.MkdirAll(dstThemesDir, 0755); err != nil {
			return fmt.Errorf("failed to create themes dir: %w", err)
		}
		if err := im.runner(nil, os.Stdout, os.Stderr, "cp", "-v", "-rp", themesDir, dstThemesDir+"/"); err != nil {
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
	envDir := filepath.Join(ostreeDeployRootfs, "etc", "environment.d")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment.d dir: %w", err)
	}
	grubCfgEnv := fmt.Sprintf("GRUB_CFG=%s/%s/grub.cfg\n", efiRoot, relEfiBootPath)
	if err := os.WriteFile(filepath.Join(envDir, "99-matrixos-imager-grub.conf"), []byte(grubCfgEnv), 0644); err != nil {
		return fmt.Errorf("failed to write grub env config: %w", err)
	}

	// Perform template substitutions in grub.cfg.
	grubData, err := os.ReadFile(dstGrubCfg)
	if err != nil {
		return fmt.Errorf("failed to read grub config for substitution: %w", err)
	}
	grubContent := string(grubData)
	grubContent = strings.ReplaceAll(grubContent, "%BOOTUUID%", bootUUID)
	grubContent = strings.ReplaceAll(grubContent, "%EFIUUID%", efiUUID)
	grubContent = strings.ReplaceAll(grubContent, "%OSNAME%", osName)
	if err := os.WriteFile(dstGrubCfg, []byte(grubContent), 0644); err != nil {
		return fmt.Errorf("failed to write substituted grub config: %w", err)
	}

	fmt.Fprintln(os.Stdout, "Current grub.cfg:")
	fmt.Fprintln(os.Stdout, grubContent)
	fmt.Fprintln(os.Stdout, "EOF")

	return nil
}

// SetupVmtestConfig creates a VM test grub config based on the ostree boot config.
func (im *Image) SetupVmtestConfig(bootdir string) error {
	if bootdir == "" {
		return errors.New("missing bootdir parameter")
	}

	fmt.Fprintf(os.Stdout, "Setting up vmtest grub config based on the ostree boot config in %s ...\n", bootdir)

	ostreeBootCfg := filepath.Join(bootdir, "loader", "entries", "ostree-1.conf")
	if !filesystems.FileExists(ostreeBootCfg) {
		return fmt.Errorf("%s does not exist, cannot set up vmtest config", ostreeBootCfg)
	}

	vmtestCfgDir := filepath.Join(bootdir, ".imager.vmtest", "entries")
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

	fmt.Fprintf(os.Stdout, "Set up vmtest grub config at %s\n", vmtestBootCfg)
	fmt.Fprintln(os.Stdout, "Current vmtest grub config:")
	fmt.Fprintln(os.Stdout, content)
	fmt.Fprintln(os.Stdout, "EOF")

	return nil
}

// InstallSecurebootCerts installs SecureBoot certificates on the EFI partition.
func (im *Image) InstallSecurebootCerts(ostreeDeployRootfs, mountEfifs, efibootdir string) error {
	if ostreeDeployRootfs == "" {
		return errors.New("missing ostreeDeployRootfs parameter")
	}
	if mountEfifs == "" {
		return errors.New("missing mountEfifs parameter")
	}
	if efibootdir == "" {
		return errors.New("missing efibootdir parameter")
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
	sbCert := filepath.Join(ostreeDeployRootfs, "etc", "portage", "secureboot.pem")
	if filesystems.FileExists(sbCert) {
		fmt.Fprintln(os.Stdout, "Copying SecureBoot cert to EFI partition ...")
		if err := filesystems.CopyFile(sbCert, filepath.Join(mountEfifs, certFileName)); err != nil {
			return fmt.Errorf("failed to copy SecureBoot cert: %w", err)
		}

		fmt.Fprintln(os.Stdout, "Generating SecureBoot MOK ...")
		if err := im.runner(nil, os.Stdout, os.Stderr,
			"openssl", "x509", "-in", sbCert,
			"-outform", "DER", "-out", filepath.Join(mountEfifs, certDerFileName)); err != nil {
			return fmt.Errorf("openssl DER conversion failed: %w", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "NO SECUREBOOT CERT AT: %s -- ignoring.\n", sbCert)
	}

	// SecureBoot KEK certificate.
	sbKek := filepath.Join(ostreeDeployRootfs, "etc", "portage", "secureboot-kek.pem")
	if filesystems.FileExists(sbKek) {
		fmt.Fprintln(os.Stdout, "Copying SecureBoot KEK cert to EFI partition ...")
		if err := filesystems.CopyFile(sbKek, filepath.Join(mountEfifs, kekFileName)); err != nil {
			return fmt.Errorf("failed to copy SecureBoot KEK cert: %w", err)
		}

		fmt.Fprintln(os.Stdout, "Generating SecureBoot KEK DER for convenience ...")
		if err := im.runner(nil, os.Stdout, os.Stderr,
			"openssl", "x509", "-in", sbKek,
			"-outform", "DER", "-out", filepath.Join(mountEfifs, kekDerFileName)); err != nil {
			return fmt.Errorf("openssl KEK DER conversion failed: %w", err)
		}
	} else {
		fmt.Fprintf(os.Stderr, "NO SECUREBOOT CERT AT: %s -- ignoring.\n", sbKek)
	}

	// Copy the shim binaries.
	shimDir := filepath.Join(ostreeDeployRootfs, "usr", "share", "shim")
	fmt.Fprintf(os.Stdout, "Copying shim for Secureboot from %s to %s ...\n", shimDir, efibootdir)
	return im.runner(nil, os.Stdout, os.Stderr, "cp", "-v", shimDir+"/.", efibootdir+"/")
}

// InstallMemtest installs the memtest86+ EFI binary to the EFI boot directory.
func (im *Image) InstallMemtest(ostreeDeployRootfs, efibootdir string) error {
	if ostreeDeployRootfs == "" {
		return errors.New("missing ostreeDeployRootfs parameter")
	}
	if efibootdir == "" {
		return errors.New("missing efibootdir parameter")
	}

	memtestBin := filepath.Join(ostreeDeployRootfs, "usr", "share", "memtest86+", "memtest.efi64")
	if !filesystems.PathExists(memtestBin) {
		fmt.Fprintf(os.Stderr, "WARNING: %s not available, please install memtest86+\n", memtestBin)
		return nil
	}
	return filesystems.CopyFile(memtestBin, filepath.Join(efibootdir, "memtest86plus.efi"))
}

// InstallBootloader installs the GRUB bootloader into the image by running
// grub-install inside a chroot of the deployed rootfs, then replaces the
// unsigned GRUBX64.EFI with the signed version.
// It returns the list of extra mounts created during the process so the caller
// can track them for cleanup.
func (im *Image) InstallBootloader(ostreeDeployRootfs, mountEfifs, mountBootfs, blockDevice, efibootdir string) ([]string, error) {
	if ostreeDeployRootfs == "" {
		return nil, errors.New("missing ostreeDeployRootfs parameter")
	}
	if mountEfifs == "" {
		return nil, errors.New("missing mountEfifs parameter")
	}
	if mountBootfs == "" {
		return nil, errors.New("missing mountBootfs parameter")
	}
	if blockDevice == "" {
		return nil, errors.New("missing blockDevice parameter")
	}
	if efibootdir == "" {
		return nil, errors.New("missing efibootdir parameter")
	}

	fmt.Fprintln(os.Stdout, "Installing bootloader ...")

	efiRoot, err := im.EfiRoot()
	if err != nil {
		return nil, err
	}
	bootRoot, err := im.BootRoot()
	if err != nil {
		return nil, err
	}
	osName, err := im.OsName()
	if err != nil {
		return nil, err
	}
	efiExe, err := im.EfiExecutable()
	if err != nil {
		return nil, err
	}

	var extraMounts []string

	// Bind mount EFI into the chroot.
	efiChrootMount := filepath.Join(ostreeDeployRootfs, efiRoot)
	if err := os.MkdirAll(efiChrootMount, 0755); err != nil {
		return extraMounts, fmt.Errorf("failed to create %s: %w", efiChrootMount, err)
	}
	mnt, err := filesystems.BindMount(mountEfifs, efiChrootMount)
	if err != nil {
		return extraMounts, fmt.Errorf("failed to bind mount EFI: %w", err)
	}
	extraMounts = append(extraMounts, mnt)

	// Bind mount boot into the chroot.
	bootChrootMount := filepath.Join(ostreeDeployRootfs, bootRoot)
	if err := os.MkdirAll(bootChrootMount, 0755); err != nil {
		return extraMounts, fmt.Errorf("failed to create %s: %w", bootChrootMount, err)
	}
	mnt, err = filesystems.BindMount(mountBootfs, bootChrootMount)
	if err != nil {
		return extraMounts, fmt.Errorf("failed to bind mount boot: %w", err)
	}
	extraMounts = append(extraMounts, mnt)

	// Setup common rootfs mounts (dev, proc, etc.) without proc for bootloader.
	chrootMounts, err := filesystems.SetupCommonRootfsMounts(ostreeDeployRootfs)
	if err != nil {
		return extraMounts, fmt.Errorf("failed to setup common rootfs mounts: %w", err)
	}
	extraMounts = append(extraMounts, chrootMounts...)

	// Run grub-install inside the chroot.
	err = filesystems.ChrootRun(ostreeDeployRootfs, "/usr/bin/grub-install",
		"--target=x86_64-efi",
		"--directory=/usr/lib/grub/x86_64-efi",
		"--efi-directory="+efiRoot,
		"--boot-directory="+bootRoot,
		"--themes="+osName+"-theme",
		"--removable",
		"--modules=ext2 btrfs gzio part_gpt fat part_msdos all_video",
		blockDevice,
	)

	// Clean up chroot mounts regardless of grub-install result.
	filesystems.BindUmount(bootChrootMount)
	filesystems.BindUmount(efiChrootMount)
	filesystems.UnsetupCommonRootfsMounts(ostreeDeployRootfs)

	if err != nil {
		return nil, fmt.Errorf("grub-install failed: %w", err)
	}

	// Verify BOOTX64.EFI was created.
	bootx64efi := filepath.Join(efibootdir, efiExe)
	if !filesystems.PathExists(bootx64efi) {
		return nil, fmt.Errorf("%s does not exist after grub-install", bootx64efi)
	}

	// Replace unsigned GRUBX64.EFI with the signed one.
	grubx64efi := filepath.Join(efibootdir, "GRUBX64.EFI")
	fmt.Fprintf(os.Stdout, "Removing existing %s as it's not signed ...\n", grubx64efi)
	os.Remove(grubx64efi)

	signedGrubx64efi := filepath.Join(ostreeDeployRootfs, "usr", "lib", "grub", "grub-x86_64.efi.signed")
	fmt.Fprintf(os.Stdout, "Moving %s to %s\n", signedGrubx64efi, grubx64efi)
	if err := os.Rename(signedGrubx64efi, grubx64efi); err != nil {
		return nil, fmt.Errorf("failed to move signed grub binary: %w", err)
	}

	return nil, nil
}

// GenerateKernelBootArgs generates the kernel boot arguments for the image.
func (im *Image) GenerateKernelBootArgs(ref, efiDevice, bootDevice, physicalRootDevice, rootDevice string, encryptionEnabled bool) ([]string, error) {
	ref, err := im.cleanAndStripRef(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to clean ref: %w", err)
	}
	if efiDevice == "" {
		return nil, errors.New("missing efiDevice parameter")
	}
	if bootDevice == "" {
		return nil, errors.New("missing bootDevice parameter")
	}
	if physicalRootDevice == "" {
		return nil, errors.New("missing physicalRootDevice parameter")
	}
	if rootDevice == "" {
		return nil, errors.New("missing rootDevice parameter")
	}

	bootArgs := im.RootfsKernelArgs()

	// Root device UUID for LUKS.
	rootDeviceUUID, err := filesystems.DeviceUUID(physicalRootDevice)
	if err != nil {
		return nil, fmt.Errorf("unable to get device UUID for %s: %w", physicalRootDevice, err)
	}
	if encryptionEnabled {
		bootArgs = append(bootArgs, fmt.Sprintf("rd.luks.uuid=%s", rootDeviceUUID))
	}

	// EFI partition mount via systemd.
	efiRoot, err := im.EfiRoot()
	if err != nil {
		return nil, err
	}
	efiPartUUID, err := filesystems.DevicePartUUID(efiDevice)
	if err != nil {
		return nil, fmt.Errorf("unable to get PARTUUID of EFI partition: %w", err)
	}
	bootArgs = append(bootArgs, fmt.Sprintf("systemd.mount-extra=PARTUUID=%s:%s:auto:defaults", efiPartUUID, efiRoot))

	// Boot partition mount via systemd.
	bootRoot, err := im.BootRoot()
	if err != nil {
		return nil, err
	}
	bootPartUUID, err := filesystems.DevicePartUUID(bootDevice)
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
		fmt.Fprintf(os.Stdout, "Reading additional kernel cmdline params from %s ...\n", cmdlineFile)
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
		fmt.Fprintf(os.Stderr, "WARNING: no additional kernel cmdline params available, %s does not exist.\n", cmdlineFile)
	}

	return bootArgs, nil
}

// PackageList returns the list of packages installed in a rootfs.
func (im *Image) PackageList(rootfs string) ([]string, error) {
	if rootfs == "" {
		return nil, errors.New("missing rootfs parameter")
	}

	roVdb, err := im.ReadOnlyVdb()
	if err != nil {
		return nil, err
	}

	vdb := filepath.Join(strings.TrimRight(rootfs, "/"), roVdb)
	if !filesystems.DirectoryExists(vdb) {
		fmt.Fprintf(os.Stderr, "%s does not exist. cannot generate pkglist\n", vdb)
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
			return nil, fmt.Errorf("failed to read category directory %s: %w", catPath, err)
		}
		for _, pkg := range pkgs {
			pkgList = append(pkgList, filepath.Join(cat.Name(), pkg.Name()))
		}
	}

	fmt.Fprintln(os.Stdout, "Generated package list:")
	for _, pkg := range pkgList {
		fmt.Fprintf(os.Stdout, ">> %s\n", pkg)
	}
	return pkgList, nil
}

// SetupHooks runs image-specific hook scripts.
func (im *Image) SetupHooks(ostreeDeployRootfs, ref string) error {
	if ostreeDeployRootfs == "" {
		return errors.New("missing ostreeDeployRootfs parameter")
	}
	if ref == "" {
		return errors.New("missing ref parameter")
	}

	ref, err := im.cleanAndStripRef(ref)
	if err != nil {
		return fmt.Errorf("failed to clean ref: %w", err)
	}

	devDir, err := im.DevDir()
	if err != nil {
		return err
	}

	hooksSrcDir := filepath.Join(devDir, "image", "hooks")
	if !filesystems.DirectoryExists(hooksSrcDir) {
		fmt.Fprintf(os.Stderr, "hooks source dir %s does not exist\n", hooksSrcDir)
		return nil
	}

	hookExec := filepath.Join(hooksSrcDir, ref+".sh")
	if !filesystems.FileExists(hookExec) {
		fmt.Fprintf(os.Stderr, "hook script %s does not exist\n", hookExec)
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"MATRIXOS_DEV_DIR="+devDir,
		"ROOTFS="+ostreeDeployRootfs,
		"REF="+ref,
	)
	return cmd.Run()
}

// TestImage copies an image to a temp directory and runs test scripts against it.
func (im *Image) TestImage(imagePath, ref string) error {
	if imagePath == "" {
		return errors.New("missing imagePath parameter")
	}
	if ref == "" {
		return errors.New("missing ref parameter")
	}

	ref, err := im.cleanAndStripRef(ref)
	if err != nil {
		return fmt.Errorf("failed to clean ref: %w", err)
	}

	devDir, err := im.DevDir()
	if err != nil {
		return err
	}

	testDir := filepath.Join(devDir, "image", "tests", ref)
	if !filesystems.DirectoryExists(testDir) {
		fmt.Fprintf(os.Stderr, "test dir %s does not exist, skipping test\n", testDir)
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

	imageName := filepath.Base(imagePath)
	testImagePath := filepath.Join(imageTempDir, imageName)
	fmt.Fprintf(os.Stdout, "Copying image to %s for testing ...\n", testImagePath)
	if err := im.runner(nil, os.Stdout, os.Stderr, "cp", "--reflink=auto", "-v", imagePath, testImagePath); err != nil {
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
			fmt.Fprintf(os.Stderr, "Skipping non-executable test script %s\n", ts)
			continue
		}

		fmt.Fprintf(os.Stdout, "Running test script %s ...\n", ts)
		cmd := exec.Command(ts)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
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
func (im *Image) FinalizeFilesystems(mountRootfs, mountBootfs, mountEfifs string) error {
	if mountRootfs == "" {
		return errors.New("missing mountRootfs parameter")
	}
	if mountBootfs == "" {
		return errors.New("missing mountBootfs parameter")
	}
	if mountEfifs == "" {
		return errors.New("missing mountEfifs parameter")
	}

	fmt.Fprintf(os.Stdout, "Executing fstrim on %s\n", mountRootfs)
	// fstrim may fail on USB sticks, so ignore errors.
	im.runner(nil, os.Stdout, os.Stderr, "fstrim", "-v", mountRootfs)

	fmt.Fprintf(os.Stdout, "Executing fstrim on %s\n", mountBootfs)
	im.runner(nil, os.Stdout, os.Stderr, "fstrim", "-v", mountBootfs)

	return nil
}

// Qcow2ImagePath returns the qcow2 image path for a given .img path.
func (im *Image) Qcow2ImagePath(imagePath string) (string, error) {
	if imagePath == "" {
		return "", errors.New("missing imagePath parameter")
	}
	return imagePath + ".qcow2", nil
}

// CreateQcow2Image creates a compressed qcow2 image from a raw image.
func (im *Image) CreateQcow2Image(imagePath string) error {
	if imagePath == "" {
		return errors.New("missing imagePath parameter")
	}
	qcow2Path, _ := im.Qcow2ImagePath(imagePath)
	return im.runner(nil, os.Stdout, os.Stderr,
		"qemu-img", "convert", "-c", "-O", "qcow2", "-p", imagePath, qcow2Path)
}

// ShowFinalFilesystemInfo displays information about the final filesystem layout.
func (im *Image) ShowFinalFilesystemInfo(blockDevice, mountBootfs, mountEfifs string) error {
	if blockDevice == "" {
		return errors.New("missing blockDevice parameter")
	}
	if mountBootfs == "" {
		return errors.New("missing mountBootfs parameter")
	}
	if mountEfifs == "" {
		return errors.New("missing mountEfifs parameter")
	}

	fmt.Fprintln(os.Stdout, "Final boot partition directory tree:")
	if err := filesystems.PrintDirectoryTree(os.Stdout, mountBootfs); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to list boot directory tree: %v\n", err)
	}

	fmt.Fprintln(os.Stdout, "Final EFI partition directory tree:")
	if err := filesystems.PrintDirectoryTree(os.Stdout, mountEfifs); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to list EFI directory tree: %v\n", err)
	}

	fmt.Fprintf(os.Stdout, "Block devices on %s:\n", blockDevice)
	if err := filesystems.PrintBlockDeviceInfo(os.Stdout, blockDevice); err != nil {
		fmt.Fprintf(os.Stderr, "WARNING: failed to get block device info: %v\n", err)
	}

	fmt.Fprintln(os.Stdout, "Filesystem setup complete!")
	return nil
}

// ShowTestInfo prints information about generated artifacts and how to test them.
func (im *Image) ShowTestInfo(artifacts []string) {
	if len(artifacts) == 0 {
		fmt.Fprintln(os.Stderr, "show_test_info: missing artifacts array parameter")
		return
	}

	fmt.Fprintln(os.Stdout, "Generated artifacts:")
	for _, a := range artifacts {
		fmt.Fprintf(os.Stdout, ">> %s\n", a)
	}

	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "How to test:")
	fmt.Fprintln(os.Stdout, "$ vector dev vm -image IMAGE_PATH -memory 8G -interactive")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "To move to a USB stick:")
	fmt.Fprintln(os.Stdout, "    dd if=IMAGE_PATH of=/dev/sdX bs=4M conv=sparse,sync status=progress")
	fmt.Fprintln(os.Stdout)
}

// RemoveImageFile removes an image file and its associated .sha256 and .asc files.
func (im *Image) RemoveImageFile(imagePath string) error {
	if imagePath == "" {
		return errors.New("missing imagePath parameter")
	}

	fmt.Fprintf(os.Stdout, "Removing %s ...\n", imagePath)
	for _, path := range []string{imagePath, imagePath + ".sha256", imagePath + ".asc"} {
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

// ImageLockPath returns the lock file path for a given ref.
func (im *Image) ImageLockPath(ref string) (string, error) {
	if ref == "" {
		return "", errors.New("missing ref parameter")
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

// ExecuteWithImageLock acquires an exclusive file lock for the given ref,
// executes fn under that lock, and releases the lock when fn returns.
// If the lock cannot be acquired within the configured timeout, an error is returned.
// If fn panics or the process crashes, the OS closes the file descriptor and
// releases the lock automatically.
func (im *Image) ExecuteWithImageLock(ref string, fn func() error) error {
	lockPath, err := im.ImageLockPath(ref)
	if err != nil {
		return fmt.Errorf("failed to get image lock path: %w", err)
	}
	fmt.Fprintf(os.Stdout, "Acquiring branch %s lock via %s ...\n", ref, lockPath)

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

	fmt.Fprintf(os.Stdout, "Lock for imager %s, %s acquired!\n", ref, lockPath)

	// Execute the function under the lock.
	// The lock is released when lockFile is closed (deferred above).
	return fn()
}
