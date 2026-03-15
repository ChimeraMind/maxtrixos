package imager

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// MockImager implements IImager for testing.
// Only the fields/methods relevant to each test need to be configured;
// everything else returns safe zero values.
type MockImager struct {
	EfiDevice_  string
	BootDevice_ string
	RootDevice_ string
	DevicePath_ string
	Rootfs_     string
	Ref_        string

	// Mount points stored by Mock Mount* methods
	EfifsMount_  string
	BootfsMount_ string
	RootfsMount_ string

	MountDir_            string
	MountDirErr          error
	ImagesDir_           string
	ImagesDirErr         error
	ImageSize_           string
	ImageSizeErr         error
	EfiPartitionSize_    string
	EfiPartitionSizeErr  error
	BootPartitionSize_   string
	BootPartitionSizeErr error
	Compressor_          string
	CompressorErr        error
	BootRoot_            string
	BootRootErr          error
	EfiRoot_             string
	EfiRootErr           error
	RelativeEfiBootPath_ string
	RelativeEfiBootErr   error
	OsName_              string
	OsNameErr            error

	ImagePath_                   string
	ImagePathErr                 error
	ImagePathWithReleaseVersion_ string
	ImagePathWithReleaseVerErr   error
	ImagePathWithCompExt_        string
	ImagePathWithCompExtErr      error
	Qcow2ImagePath_              string
	Qcow2ImagePathErr            error
	ReleaseVersion_              string
	ReleaseVersionErr            error
	PackageList_                 []string
	PackageListErr               error
	CreateQcow2_                 bool
	CreateQcow2Err               error
	Productionize_               bool
	ProductionizeErr             error
	ImageTests_                  bool
	ImageTestsErr                error
	InstallBootloaderResult      []string
	InstallBootloaderErr         error
	Mode_                        ImageMode

	// Track calls
	ClearPartitionTableCalled bool
	PartitionDevicesCalled    bool
	FormatEfifsCalled         bool
	FormatBootfsCalled        bool
	FormatRootfsCalled        bool
	MaybeEncryptRootfsCalled  bool
	MaybeEncryptRootfsErr     error
	MountRootfsCalled         bool
	MountEfifsCalled          bool
	MountBootfsCalled         bool
	CreateImageCalled         bool
	CompressImageCalled       bool
	CreateQcow2ImageCalled    bool
	TestImageCalled           bool
	SetupBootloaderCfgCalled  bool
	SetupPasswordsCalled      bool
	InstallBootloaderCalled   bool
	CleanupCalled             bool
	SetupVmtestConfigCalled   bool
	InstallSecurebootCalled   bool
	InstallMemtestCalled      bool
	SetupHooksCalled          bool
	FinalizeFsCalled          bool
	ShowFinalFsInfoCalled     bool
	ShowTestInfoCalled        bool

	// ExecuteWithImageLock support
	ExecuteWithImageLockCalled bool
	ExecuteWithImageLockErr    error

	// Build support
	BuildCalled bool
	BuildErr    error

	// I/O writers for Print methods
	stdout io.Writer
	stderr io.Writer
}

func (m *MockImager) SetStdout(w io.Writer) { m.stdout = w }
func (m *MockImager) SetStderr(w io.Writer) { m.stderr = w }
func (m *MockImager) Stdout() io.Writer {
	if m.stdout == nil {
		return os.Stdout
	}
	return m.stdout
}
func (m *MockImager) Stderr() io.Writer {
	if m.stderr == nil {
		return os.Stderr
	}
	return m.stderr
}
func (m *MockImager) Print(format string, a ...any) {
	fmt.Fprintf(m.Stdout(), format, a...)
}
func (m *MockImager) PrintWarning(format string, a ...any) {
	fmt.Fprintf(m.Stderr(), format, a...)
}
func (m *MockImager) PrintError(format string, a ...any) {
	fmt.Fprintf(m.Stderr(), format, a...)
}

func (m *MockImager) SetEfiDevice(device string)      { m.EfiDevice_ = device }
func (m *MockImager) EfiDevice() string               { return m.EfiDevice_ }
func (m *MockImager) SetBootDevice(device string)     { m.BootDevice_ = device }
func (m *MockImager) BootDevice() string              { return m.BootDevice_ }
func (m *MockImager) SetRootDevice(device string)     { m.RootDevice_ = device }
func (m *MockImager) RootDevice() string              { return m.RootDevice_ }
func (m *MockImager) SetDevicePath(devicePath string) { m.DevicePath_ = devicePath }
func (m *MockImager) DevicePath() string              { return m.DevicePath_ }
func (m *MockImager) SetImagePath(imagePath string)   { m.ImagePath_ = imagePath }
func (m *MockImager) SetRootfs(rootfs string)         { m.Rootfs_ = rootfs }
func (m *MockImager) Rootfs() string                  { return m.Rootfs_ }
func (m *MockImager) Ref() string                     { return m.Ref_ }
func (m *MockImager) SetRef(ref string)               { m.Ref_ = ref }

func (m *MockImager) EfifsMount() string { return m.EfifsMount_ }
func (m *MockImager) EfiBootDir() (string, error) {
	if m.EfifsMount_ == "" {
		return "", errors.New("EFI filesystem not mounted")
	}
	if m.RelativeEfiBootPath_ == "" {
		return "", errors.New("invalid Imager.RelativeEfiBootPath")
	}
	return m.EfifsMount_ + "/" + m.RelativeEfiBootPath_, nil
}
func (m *MockImager) BootfsMount() string { return m.BootfsMount_ }
func (m *MockImager) RootfsMount() string { return m.RootfsMount_ }

func (m *MockImager) ImagesDir() (string, error) { return m.ImagesDir_, m.ImagesDirErr }
func (m *MockImager) MountDir() (string, error)  { return m.MountDir_, m.MountDirErr }
func (m *MockImager) ImageSize() (string, error) { return m.ImageSize_, m.ImageSizeErr }
func (m *MockImager) EfiPartitionSize() (string, error) {
	return m.EfiPartitionSize_, m.EfiPartitionSizeErr
}
func (m *MockImager) BootPartitionSize() (string, error) {
	return m.BootPartitionSize_, m.BootPartitionSizeErr
}
func (m *MockImager) Compressor() (string, error)        { return m.Compressor_, m.CompressorErr }
func (m *MockImager) EspPartitionType() (string, error)  { return "vfat", nil }
func (m *MockImager) BootPartitionType() (string, error) { return "ext4", nil }
func (m *MockImager) RootPartitionType() (string, error) { return "btrfs", nil }
func (m *MockImager) OsName() (string, error)            { return m.OsName_, m.OsNameErr }
func (m *MockImager) BootRoot() (string, error)          { return m.BootRoot_, m.BootRootErr }
func (m *MockImager) EfiRoot() (string, error)           { return m.EfiRoot_, m.EfiRootErr }
func (m *MockImager) RelativeEfiBootPath() (string, error) {
	return m.RelativeEfiBootPath_, m.RelativeEfiBootErr
}
func (m *MockImager) EfiExecutable() (string, error)                { return "", nil }
func (m *MockImager) EfiCertificateFileName() (string, error)       { return "", nil }
func (m *MockImager) EfiCertificateFileNameDer() (string, error)    { return "", nil }
func (m *MockImager) EfiCertificateFileNameKek() (string, error)    { return "", nil }
func (m *MockImager) EfiCertificateFileNameKekDer() (string, error) { return "", nil }
func (m *MockImager) ReadOnlyVdb() (string, error)                  { return "", nil }
func (m *MockImager) DevDir() (string, error)                       { return "", nil }
func (m *MockImager) HooksDir() (string, error)                     { return "", nil }
func (m *MockImager) LockDir() (string, error)                      { return "", nil }
func (m *MockImager) LockWaitSeconds() (string, error)              { return "", nil }
func (m *MockImager) BuildMetadataFile() (string, error)            { return "", nil }

func (m *MockImager) ExtractReleaseVersion() (string, error) {
	return m.ReleaseVersion_, m.ReleaseVersionErr
}
func (m *MockImager) ImagePath() string {
	return m.ImagePath_
}
func (m *MockImager) BuildImagePath() (string, error) {
	return m.ImagePath_, m.ImagePathErr
}
func (m *MockImager) SetImageMode(mode ImageMode) error {
	m.Mode_ = mode
	return nil
}
func (m *MockImager) ImageMode() ImageMode {
	return m.Mode_
}
func (m *MockImager) BuildImagePathWithReleaseVersion(releaseVersion string) (string, error) {
	return m.ImagePathWithReleaseVersion_, m.ImagePathWithReleaseVerErr
}
func (m *MockImager) CreateImage(imageSize string) error {
	m.CreateImageCalled = true
	return nil
}
func (m *MockImager) CompressedImagePath() (string, error) {
	return m.ImagePathWithCompExt_, m.ImagePathWithCompExtErr
}
func (m *MockImager) CompressImage() error {
	m.CompressImageCalled = true
	return nil
}

func (m *MockImager) ClearPartitionTable() error {
	m.ClearPartitionTableCalled = true
	return nil
}
func (m *MockImager) DatedFsLabel() string { return "IMG-20250101" }
func (m *MockImager) PartitionDevices(efiSize, bootSize, imageSize string) error {
	m.PartitionDevicesCalled = true
	return nil
}
func (m *MockImager) FormatEfifs() error {
	m.FormatEfifsCalled = true
	return nil
}
func (m *MockImager) MountEfifs(mountEfifs string) error {
	m.MountEfifsCalled = true
	m.EfifsMount_ = mountEfifs
	return nil
}
func (m *MockImager) FormatBootfs() error {
	m.FormatBootfsCalled = true
	return nil
}
func (m *MockImager) MountBootfs(mountBootfs string) error {
	m.MountBootfsCalled = true
	m.BootfsMount_ = mountBootfs
	return nil
}
func (m *MockImager) MaybeEncryptRootfs() error {
	m.MaybeEncryptRootfsCalled = true
	return m.MaybeEncryptRootfsErr
}
func (m *MockImager) FormatRootfs() error {
	m.FormatRootfsCalled = true
	return nil
}
func (m *MockImager) RootfsKernelArgs() []string { return nil }
func (m *MockImager) MountRootfs(mountRootfs string) error {
	m.MountRootfsCalled = true
	m.RootfsMount_ = mountRootfs
	return nil
}
func (m *MockImager) GetKernelPath() (string, error) { return "", nil }
func (m *MockImager) SetupPasswords() error {
	m.SetupPasswordsCalled = true
	return nil
}
func (m *MockImager) SetupBootloaderConfig() error {
	m.SetupBootloaderCfgCalled = true
	return nil
}
func (m *MockImager) SetupVmtestConfig() error {
	m.SetupVmtestConfigCalled = true
	return nil
}
func (m *MockImager) InstallSecurebootCerts() error {
	m.InstallSecurebootCalled = true
	return nil
}
func (m *MockImager) InstallMemtest() error {
	m.InstallMemtestCalled = true
	return nil
}
func (m *MockImager) GenerateKernelBootArgs() ([]string, error) {
	return []string{"arg1=val1"}, nil
}
func (m *MockImager) ExtractPackageList() ([]string, error) {
	return m.PackageList_, m.PackageListErr
}
func (m *MockImager) SetupHooks() error {
	m.SetupHooksCalled = true
	return nil
}
func (m *MockImager) InstallBootloader() error {
	m.InstallBootloaderCalled = true
	return m.InstallBootloaderErr
}
func (m *MockImager) Cleanup() {
	m.CleanupCalled = true
}
func (m *MockImager) Build(opts *BuildOptions) error {
	m.BuildCalled = true
	return m.BuildErr
}
func (m *MockImager) TestImage() error {
	m.TestImageCalled = true
	return nil
}
func (m *MockImager) FinalizeFilesystems() error {
	m.FinalizeFsCalled = true
	return nil
}
func (m *MockImager) Qcow2ImagePath() (string, error) {
	return m.Qcow2ImagePath_, m.Qcow2ImagePathErr
}
func (m *MockImager) CreateQcow2Image() error {
	m.CreateQcow2ImageCalled = true
	return nil
}
func (m *MockImager) ShowFinalFilesystemInfo() error {
	m.ShowFinalFsInfoCalled = true
	return nil
}
func (m *MockImager) ShowImageTestInfo(artifacts []string) error {
	m.ShowTestInfoCalled = true
	return nil
}
func (m *MockImager) RemoveImageFile() error         { return nil }
func (m *MockImager) ImageLockDir() (string, error)  { return "", nil }
func (m *MockImager) ImageLockPath() (string, error) { return "", nil }
func (m *MockImager) CreateQcow2() (bool, error)     { return m.CreateQcow2_, m.CreateQcow2Err }
func (m *MockImager) Productionize() (bool, error)   { return m.Productionize_, m.ProductionizeErr }
func (m *MockImager) ImageTests() (bool, error)      { return m.ImageTests_, m.ImageTestsErr }

// ExecuteWithImageLock either calls fn directly or returns the configured error.
func (m *MockImager) ExecuteWithImageLock(fn func() error) error {
	m.ExecuteWithImageLockCalled = true
	if m.ExecuteWithImageLockErr != nil {
		return m.ExecuteWithImageLockErr
	}
	return fn()
}

// DefaultMockImager returns a MockImager pre-configured with sensible defaults for testing.
func DefaultMockImager() *MockImager {
	return &MockImager{
		MountDir_:            "/tmp/test-mount",
		ImageSize_:           "8G",
		EfiPartitionSize_:    "512M",
		BootPartitionSize_:   "1G",
		Compressor_:          "xz",
		BootRoot_:            "/boot",
		EfiRoot_:             "/efi",
		RelativeEfiBootPath_: "EFI/BOOT",
		OsName_:              "matrixos",
		ImagePath_:           "/tmp/test.img",
		ReleaseVersion_:      "1.0.0",
		PackageList_:         []string{"app-misc/foo-1.0", "sys-apps/bar-2.0"},
	}
}
