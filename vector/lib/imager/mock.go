package imager

// MockImage implements IImage for testing.
// Only the fields/methods relevant to each test need to be configured;
// everything else returns safe zero values.
type MockImage struct {
	MountDir_            string
	MountDirErr          error
	ImagesOutDir_        string
	ImagesOutDirErr      error
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
	BlockDeviceForPartPath_      string
	BlockDeviceForPartPathErr    error
	BlockDeviceNthPartPaths      map[int]string
	BlockDeviceNthPartPathErr    error
	InstallBootloaderResult      []string
	InstallBootloaderErr         error

	// Track calls
	ClearPartitionTableCalled bool
	PartitionDevicesCalled    bool
	FormatEfifsCalled         bool
	FormatBootfsCalled        bool
	FormatRootfsCalled        bool
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
}

func (m *MockImage) ImagesOutDir() (string, error) { return m.ImagesOutDir_, m.ImagesOutDirErr }
func (m *MockImage) MountDir() (string, error)     { return m.MountDir_, m.MountDirErr }
func (m *MockImage) ImageSize() (string, error)    { return m.ImageSize_, m.ImageSizeErr }
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
func (m *MockImage) LockDir() (string, error)                      { return "", nil }
func (m *MockImage) LockWaitSeconds() (string, error)              { return "", nil }
func (m *MockImage) BuildMetadataFile() (string, error)            { return "", nil }

func (m *MockImage) ReleaseVersion(rootfs string) (string, error) {
	return m.ReleaseVersion_, m.ReleaseVersionErr
}
func (m *MockImage) ImagePath(ref string) (string, error) {
	return m.ImagePath_, m.ImagePathErr
}
func (m *MockImage) ImagePathWithReleaseVersion(ref, releaseVersion string) (string, error) {
	return m.ImagePathWithReleaseVersion_, m.ImagePathWithReleaseVerErr
}
func (m *MockImage) CreateImage(imagePath, imageSize string) error {
	m.CreateImageCalled = true
	return nil
}
func (m *MockImage) ImagePathWithCompressorExtension(imagePath, compressor string) (string, error) {
	return m.ImagePathWithCompExt_, m.ImagePathWithCompExtErr
}
func (m *MockImage) CompressImage(imagePath, compressor string) error {
	m.CompressImageCalled = true
	return nil
}
func (m *MockImage) BlockDeviceNthPartitionPath(blockDevice string, nth int) (string, error) {
	if m.BlockDeviceNthPartPaths != nil {
		if p, ok := m.BlockDeviceNthPartPaths[nth]; ok {
			return p, m.BlockDeviceNthPartPathErr
		}
	}
	return "", m.BlockDeviceNthPartPathErr
}
func (m *MockImage) BlockDeviceForPartitionPath(partitionPath string) (string, error) {
	return m.BlockDeviceForPartPath_, m.BlockDeviceForPartPathErr
}
func (m *MockImage) PartitionNumber(partitionPath string) (string, error) { return "1", nil }
func (m *MockImage) PartitionLabel(partitionPath string) (string, error)  { return "EFI", nil }
func (m *MockImage) ClearPartitionTable(devicePath string) error {
	m.ClearPartitionTableCalled = true
	return nil
}
func (m *MockImage) GetPartitionType(devicePath string) (string, error) { return "gpt", nil }
func (m *MockImage) DatedFsLabel() string                               { return "IMG-20250101" }
func (m *MockImage) PartitionDevices(efiSize, bootSize, imageSize, devicePath string) error {
	m.PartitionDevicesCalled = true
	return nil
}
func (m *MockImage) FormatEfifs(efiDevice string) error {
	m.FormatEfifsCalled = true
	return nil
}
func (m *MockImage) MountEfifs(efiDevice, mountEfifs string) error {
	m.MountEfifsCalled = true
	return nil
}
func (m *MockImage) FormatBootfs(bootDevice string) error {
	m.FormatBootfsCalled = true
	return nil
}
func (m *MockImage) MountBootfs(bootDevice, mountBootfs string) error {
	m.MountBootfsCalled = true
	return nil
}
func (m *MockImage) FormatRootfs(rootDevice string) error {
	m.FormatRootfsCalled = true
	return nil
}
func (m *MockImage) RootfsKernelArgs() []string { return nil }
func (m *MockImage) MountRootfs(rootDevice, mountRootfs string) error {
	m.MountRootfsCalled = true
	return nil
}
func (m *MockImage) GetKernelPath(ostreeDeployRootfs string) (string, error) { return "", nil }
func (m *MockImage) SetupPasswords(ostreeDeployRootfs string) error {
	m.SetupPasswordsCalled = true
	return nil
}
func (m *MockImage) SetupBootloaderConfig(ref, ostreeDeployRootfs, sysroot, bootdir, efibootdir, efiUUID, bootUUID string) error {
	m.SetupBootloaderCfgCalled = true
	return nil
}
func (m *MockImage) SetupVmtestConfig(bootdir string) error {
	m.SetupVmtestConfigCalled = true
	return nil
}
func (m *MockImage) InstallSecurebootCerts(ostreeDeployRootfs, mountEfifs, efibootdir string) error {
	m.InstallSecurebootCalled = true
	return nil
}
func (m *MockImage) InstallMemtest(ostreeDeployRootfs, efibootdir string) error {
	m.InstallMemtestCalled = true
	return nil
}
func (m *MockImage) GenerateKernelBootArgs(ref, efiDevice, bootDevice, physicalRootDevice, rootDevice string, encryptionEnabled bool) ([]string, error) {
	return []string{"arg1=val1"}, nil
}
func (m *MockImage) PackageList(rootfs string) ([]string, error) {
	return m.PackageList_, m.PackageListErr
}
func (m *MockImage) SetupHooks(ostreeDeployRootfs, ref string) error {
	m.SetupHooksCalled = true
	return nil
}
func (m *MockImage) InstallBootloader(ostreeDeployRootfs, mountEfifs, mountBootfs, blockDevice, efibootdir string) ([]string, error) {
	m.InstallBootloaderCalled = true
	return m.InstallBootloaderResult, m.InstallBootloaderErr
}
func (m *MockImage) TestImage(imagePath, ref string) error {
	m.TestImageCalled = true
	return nil
}
func (m *MockImage) FinalizeFilesystems(mountRootfs, mountBootfs, mountEfifs string) error {
	m.FinalizeFsCalled = true
	return nil
}
func (m *MockImage) Qcow2ImagePath(imagePath string) (string, error) {
	return m.Qcow2ImagePath_, m.Qcow2ImagePathErr
}
func (m *MockImage) CreateQcow2Image(imagePath string) error {
	m.CreateQcow2ImageCalled = true
	return nil
}
func (m *MockImage) ShowFinalFilesystemInfo(blockDevice, mountBootfs, mountEfifs string) error {
	m.ShowFinalFsInfoCalled = true
	return nil
}
func (m *MockImage) ShowTestInfo(artifacts []string) {
	m.ShowTestInfoCalled = true
}
func (m *MockImage) RemoveImageFile(imagePath string) error   { return nil }
func (m *MockImage) ImageLockDir() (string, error)            { return "", nil }
func (m *MockImage) ImageLockPath(ref string) (string, error) { return "", nil }

// ExecuteWithImageLock either calls fn directly or returns the configured error.
func (m *MockImage) ExecuteWithImageLock(ref string, fn func() error) error {
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
