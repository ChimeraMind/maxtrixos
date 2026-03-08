package imager

import (
	"errors"
	"fmt"
	"io"
	"os"
)

// MockImage implements IImage for testing.
// Only the fields/methods relevant to each test need to be configured;
// everything else returns safe zero values.
type MockImage struct {
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

func (m *MockImage) SetStdout(w io.Writer) { m.stdout = w }
func (m *MockImage) SetStderr(w io.Writer) { m.stderr = w }
func (m *MockImage) Stdout() io.Writer {
	if m.stdout == nil {
		return os.Stdout
	}
	return m.stdout
}
func (m *MockImage) Stderr() io.Writer {
	if m.stderr == nil {
		return os.Stderr
	}
	return m.stderr
}
func (m *MockImage) Print(format string, a ...any) {
	fmt.Fprintf(m.Stdout(), format, a...)
}
func (m *MockImage) PrintWarning(format string, a ...any) {
	fmt.Fprintf(m.Stderr(), format, a...)
}
func (m *MockImage) PrintError(format string, a ...any) {
	fmt.Fprintf(m.Stderr(), format, a...)
}

func (m *MockImage) SetEfiDevice(device string)      { m.EfiDevice_ = device }
func (m *MockImage) EfiDevice() string               { return m.EfiDevice_ }
func (m *MockImage) SetBootDevice(device string)     { m.BootDevice_ = device }
func (m *MockImage) BootDevice() string              { return m.BootDevice_ }
func (m *MockImage) SetRootDevice(device string)     { m.RootDevice_ = device }
func (m *MockImage) RootDevice() string              { return m.RootDevice_ }
func (m *MockImage) SetDevicePath(devicePath string) { m.DevicePath_ = devicePath }
func (m *MockImage) DevicePath() string              { return m.DevicePath_ }
func (m *MockImage) SetImagePath(imagePath string)   { m.ImagePath_ = imagePath }
func (m *MockImage) SetRootfs(rootfs string)         { m.Rootfs_ = rootfs }
func (m *MockImage) Rootfs() string                  { return m.Rootfs_ }
func (m *MockImage) Ref() string                     { return m.Ref_ }
func (m *MockImage) SetRef(ref string)               { m.Ref_ = ref }

func (m *MockImage) EfifsMount() string { return m.EfifsMount_ }
func (m *MockImage) EfiBootDir() (string, error) {
	if m.EfifsMount_ == "" {
		return "", errors.New("EFI filesystem not mounted")
	}
	if m.RelativeEfiBootPath_ == "" {
		return "", errors.New("invalid Imager.RelativeEfiBootPath")
	}
	return m.EfifsMount_ + "/" + m.RelativeEfiBootPath_, nil
}
func (m *MockImage) BootfsMount() string { return m.BootfsMount_ }
func (m *MockImage) RootfsMount() string { return m.RootfsMount_ }

func (m *MockImage) ImagesDir() (string, error) { return m.ImagesDir_, m.ImagesDirErr }
func (m *MockImage) MountDir() (string, error)  { return m.MountDir_, m.MountDirErr }
func (m *MockImage) ImageSize() (string, error) { return m.ImageSize_, m.ImageSizeErr }
func (m *MockImage) EfiPartitionSize() (string, error) {
	return m.EfiPartitionSize_, m.EfiPartitionSizeErr
}
func (m *MockImage) BootPartitionSize() (string, error) {
	return m.BootPartitionSize_, m.BootPartitionSizeErr
}
func (m *MockImage) Compressor() (string, error)        { return m.Compressor_, m.CompressorErr }
func (m *MockImage) EspPartitionType() (string, error)  { return "vfat", nil }
func (m *MockImage) BootPartitionType() (string, error) { return "ext4", nil }
func (m *MockImage) RootPartitionType() (string, error) { return "btrfs", nil }
func (m *MockImage) OsName() (string, error)            { return m.OsName_, m.OsNameErr }
func (m *MockImage) BootRoot() (string, error)          { return m.BootRoot_, m.BootRootErr }
func (m *MockImage) EfiRoot() (string, error)           { return m.EfiRoot_, m.EfiRootErr }
func (m *MockImage) RelativeEfiBootPath() (string, error) {
	return m.RelativeEfiBootPath_, m.RelativeEfiBootErr
}
func (m *MockImage) EfiExecutable() (string, error)                { return "", nil }
func (m *MockImage) EfiCertificateFileName() (string, error)       { return "", nil }
func (m *MockImage) EfiCertificateFileNameDer() (string, error)    { return "", nil }
func (m *MockImage) EfiCertificateFileNameKek() (string, error)    { return "", nil }
func (m *MockImage) EfiCertificateFileNameKekDer() (string, error) { return "", nil }
func (m *MockImage) ReadOnlyVdb() (string, error)                  { return "", nil }
func (m *MockImage) DevDir() (string, error)                       { return "", nil }
func (m *MockImage) HooksDir() (string, error)                     { return "", nil }
func (m *MockImage) LockDir() (string, error)                      { return "", nil }
func (m *MockImage) LockWaitSeconds() (string, error)              { return "", nil }
func (m *MockImage) BuildMetadataFile() (string, error)            { return "", nil }

func (m *MockImage) ExtractReleaseVersion() (string, error) {
	return m.ReleaseVersion_, m.ReleaseVersionErr
}
func (m *MockImage) ImagePath() string {
	return m.ImagePath_
}
func (m *MockImage) BuildImagePath() (string, error) {
	return m.ImagePath_, m.ImagePathErr
}
func (m *MockImage) SetImageMode(mode ImageMode) error {
	m.Mode_ = mode
	return nil
}
func (m *MockImage) ImageMode() ImageMode {
	return m.Mode_
}
func (m *MockImage) BuildImagePathWithReleaseVersion(releaseVersion string) (string, error) {
	return m.ImagePathWithReleaseVersion_, m.ImagePathWithReleaseVerErr
}
func (m *MockImage) CreateImage(imageSize string) error {
	m.CreateImageCalled = true
	return nil
}
func (m *MockImage) CompressedImagePath() (string, error) {
	return m.ImagePathWithCompExt_, m.ImagePathWithCompExtErr
}
func (m *MockImage) CompressImage() error {
	m.CompressImageCalled = true
	return nil
}

func (m *MockImage) ClearPartitionTable() error {
	m.ClearPartitionTableCalled = true
	return nil
}
func (m *MockImage) DatedFsLabel() string { return "IMG-20250101" }
func (m *MockImage) PartitionDevices(efiSize, bootSize, imageSize string) error {
	m.PartitionDevicesCalled = true
	return nil
}
func (m *MockImage) FormatEfifs() error {
	m.FormatEfifsCalled = true
	return nil
}
func (m *MockImage) MountEfifs(mountEfifs string) error {
	m.MountEfifsCalled = true
	m.EfifsMount_ = mountEfifs
	return nil
}
func (m *MockImage) FormatBootfs() error {
	m.FormatBootfsCalled = true
	return nil
}
func (m *MockImage) MountBootfs(mountBootfs string) error {
	m.MountBootfsCalled = true
	m.BootfsMount_ = mountBootfs
	return nil
}
func (m *MockImage) MaybeEncryptRootfs() error {
	m.MaybeEncryptRootfsCalled = true
	return m.MaybeEncryptRootfsErr
}
func (m *MockImage) FormatRootfs() error {
	m.FormatRootfsCalled = true
	return nil
}
func (m *MockImage) RootfsKernelArgs() []string { return nil }
func (m *MockImage) MountRootfs(mountRootfs string) error {
	m.MountRootfsCalled = true
	m.RootfsMount_ = mountRootfs
	return nil
}
func (m *MockImage) GetKernelPath() (string, error) { return "", nil }
func (m *MockImage) SetupPasswords() error {
	m.SetupPasswordsCalled = true
	return nil
}
func (m *MockImage) SetupBootloaderConfig() error {
	m.SetupBootloaderCfgCalled = true
	return nil
}
func (m *MockImage) SetupVmtestConfig() error {
	m.SetupVmtestConfigCalled = true
	return nil
}
func (m *MockImage) InstallSecurebootCerts() error {
	m.InstallSecurebootCalled = true
	return nil
}
func (m *MockImage) InstallMemtest() error {
	m.InstallMemtestCalled = true
	return nil
}
func (m *MockImage) GenerateKernelBootArgs() ([]string, error) {
	return []string{"arg1=val1"}, nil
}
func (m *MockImage) ExtractPackageList() ([]string, error) {
	return m.PackageList_, m.PackageListErr
}
func (m *MockImage) SetupHooks() error {
	m.SetupHooksCalled = true
	return nil
}
func (m *MockImage) InstallBootloader() error {
	m.InstallBootloaderCalled = true
	return m.InstallBootloaderErr
}
func (m *MockImage) Cleanup() {
	m.CleanupCalled = true
}
func (m *MockImage) Build(opts *BuildOptions) error {
	m.BuildCalled = true
	return m.BuildErr
}
func (m *MockImage) TestImage() error {
	m.TestImageCalled = true
	return nil
}
func (m *MockImage) FinalizeFilesystems() error {
	m.FinalizeFsCalled = true
	return nil
}
func (m *MockImage) Qcow2ImagePath() (string, error) {
	return m.Qcow2ImagePath_, m.Qcow2ImagePathErr
}
func (m *MockImage) CreateQcow2Image() error {
	m.CreateQcow2ImageCalled = true
	return nil
}
func (m *MockImage) ShowFinalFilesystemInfo() error {
	m.ShowFinalFsInfoCalled = true
	return nil
}
func (m *MockImage) ShowImageTestInfo(artifacts []string) error {
	m.ShowTestInfoCalled = true
	return nil
}
func (m *MockImage) RemoveImageFile() error         { return nil }
func (m *MockImage) ImageLockDir() (string, error)  { return "", nil }
func (m *MockImage) ImageLockPath() (string, error) { return "", nil }
func (m *MockImage) CreateQcow2() (bool, error)     { return m.CreateQcow2_, m.CreateQcow2Err }
func (m *MockImage) Productionize() (bool, error)   { return m.Productionize_, m.ProductionizeErr }
func (m *MockImage) ImageTests() (bool, error)      { return m.ImageTests_, m.ImageTestsErr }

// ExecuteWithImageLock either calls fn directly or returns the configured error.
func (m *MockImage) ExecuteWithImageLock(fn func() error) error {
	m.ExecuteWithImageLockCalled = true
	if m.ExecuteWithImageLockErr != nil {
		return m.ExecuteWithImageLockErr
	}
	return fn()
}

// DefaultMockImage returns a MockImage pre-configured with sensible defaults for testing.
func DefaultMockImage() *MockImage {
	return &MockImage{
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
