package releaser

import (
	"fmt"
	"io"
	"os"
)

// Compile-time interface check.
var _ IRelease = (*MockReleaser)(nil)

// MockReleaser implements IRelease for testing commands.
// Only the fields/methods relevant to each test need to be configured;
// everything else returns safe zero values.
type MockReleaser struct {
	stdout io.Writer
	stderr io.Writer

	ChrootDir_ string
	ImageDir_  string
	Ref_       string

	// Config accessors
	Hostname_                    string
	HostnameErr                  error
	HooksDir_                    string
	HooksDirErr                  error
	DevDir_                      string
	DevDirErr                    error
	UseCpReflink_                bool
	UseCpReflinkErr              error
	ReadOnlyVdb_                 string
	ReadOnlyVdbErr               error
	LockDir_                     string
	LockDirErr                   error
	LockWaitSeconds_             string
	LockWaitSecondsErr           error
	GenerateStaticDeltas_        bool
	GenerateStaticDeltasErr      error
	SecureBootCertPath_          string
	SecureBootCertPathErr        error
	SecureBootKekPath_           string
	SecureBootKekPathErr         error
	PrivateGitRepoPath_          string
	PrivateGitRepoPathErr        error
	DefaultPrivateGitRepoPath_   string
	DefaultPrivateGitRepoPathErr error
	BuildMetadataFile_           string
	BuildMetadataFileErr         error
	ServicesDir_                 string
	ServicesDirErr               error

	// Operation errors
	CheckMatrixOSErr                      error
	SyncFilesystemErr                     error
	PreCleanQAChecksErr                   error
	CleanRootfsErr                        error
	SetupHostnameErr                      error
	SetupServicesErr                      error
	ReleaseHookErr                        error
	PostCleanShrinkErr                    error
	OstreePrepareErr                      error
	MaybeOstreeInitErr                    error
	SymlinkEtcErr                         error
	UnlinkEtcErr                          error
	AddExtraDotDotToUsrEtcPortageErr      error
	RemoveExtraDotDotFromUsrEtcPortageErr error
	ReleaseErr                            error
	SetImageDirErr                        error

	// Locking
	ReleaseLockDir_           string
	ReleaseLockDirErr         error
	ReleaseLockPath_          string
	ReleaseLockPathErr        error
	ExecuteWithReleaseLockErr error

	// Track calls
	CheckMatrixOSCalled                      bool
	SyncFilesystemCalled                     bool
	PreCleanQAChecksCalled                   bool
	CleanRootfsCalled                        bool
	SetupHostnameCalled                      bool
	SetupServicesCalled                      bool
	ReleaseHookCalled                        bool
	PostCleanShrinkCalled                    bool
	OstreePrepareCalled                      bool
	MaybeOstreeInitCalled                    bool
	SymlinkEtcCalled                         bool
	UnlinkEtcCalled                          bool
	AddExtraDotDotToUsrEtcPortageCalled      bool
	RemoveExtraDotDotFromUsrEtcPortageCalled bool
	ReleaseCalled                            bool
	ReleaseOpts                              []CommitOptions
	CleanupCalled                            bool
}

// DefaultMockReleaser returns a MockReleaser with sensible defaults for tests.
func DefaultMockReleaser() *MockReleaser {
	return &MockReleaser{}
}

func (m *MockReleaser) SetStdout(w io.Writer) { m.stdout = w }
func (m *MockReleaser) SetStderr(w io.Writer) { m.stderr = w }
func (m *MockReleaser) Stdout() io.Writer {
	if m.stdout == nil {
		return os.Stdout
	}
	return m.stdout
}
func (m *MockReleaser) Stderr() io.Writer {
	if m.stderr == nil {
		return os.Stderr
	}
	return m.stderr
}

func (m *MockReleaser) Print(format string, args ...any) {
	fmt.Fprintf(m.Stdout(), format, args...)
}
func (m *MockReleaser) PrintWarning(format string, args ...any) {
	fmt.Fprintf(m.Stderr(), format, args...)
}
func (m *MockReleaser) PrintError(format string, args ...any) {
	fmt.Fprintf(m.Stderr(), format, args...)
}

func (m *MockReleaser) SetChrootDir(dir string) { m.ChrootDir_ = dir }
func (m *MockReleaser) ChrootDir() string       { return m.ChrootDir_ }
func (m *MockReleaser) SetImageDir(dir string) error {
	if m.SetImageDirErr != nil {
		return m.SetImageDirErr
	}
	m.ImageDir_ = dir
	return nil
}
func (m *MockReleaser) ImageDir() string  { return m.ImageDir_ }
func (m *MockReleaser) SetRef(ref string) { m.Ref_ = ref }
func (m *MockReleaser) Ref() string       { return m.Ref_ }

func (m *MockReleaser) Hostname() (string, error)    { return m.Hostname_, m.HostnameErr }
func (m *MockReleaser) HooksDir() (string, error)    { return m.HooksDir_, m.HooksDirErr }
func (m *MockReleaser) DevDir() (string, error)      { return m.DevDir_, m.DevDirErr }
func (m *MockReleaser) UseCpReflink() (bool, error)  { return m.UseCpReflink_, m.UseCpReflinkErr }
func (m *MockReleaser) ReadOnlyVdb() (string, error) { return m.ReadOnlyVdb_, m.ReadOnlyVdbErr }
func (m *MockReleaser) LockDir() (string, error)     { return m.LockDir_, m.LockDirErr }
func (m *MockReleaser) LockWaitSeconds() (string, error) {
	return m.LockWaitSeconds_, m.LockWaitSecondsErr
}
func (m *MockReleaser) GenerateStaticDeltas() (bool, error) {
	return m.GenerateStaticDeltas_, m.GenerateStaticDeltasErr
}
func (m *MockReleaser) SecureBootCertPath() (string, error) {
	return m.SecureBootCertPath_, m.SecureBootCertPathErr
}
func (m *MockReleaser) SecureBootKekPath() (string, error) {
	return m.SecureBootKekPath_, m.SecureBootKekPathErr
}
func (m *MockReleaser) PrivateGitRepoPath() (string, error) {
	return m.PrivateGitRepoPath_, m.PrivateGitRepoPathErr
}
func (m *MockReleaser) DefaultPrivateGitRepoPath() (string, error) {
	return m.DefaultPrivateGitRepoPath_, m.DefaultPrivateGitRepoPathErr
}
func (m *MockReleaser) BuildMetadataFile() (string, error) {
	return m.BuildMetadataFile_, m.BuildMetadataFileErr
}
func (m *MockReleaser) ServicesDir() (string, error) { return m.ServicesDir_, m.ServicesDirErr }

func (m *MockReleaser) CheckMatrixOS() error {
	m.CheckMatrixOSCalled = true
	return m.CheckMatrixOSErr
}
func (m *MockReleaser) SyncFilesystem() error {
	m.SyncFilesystemCalled = true
	return m.SyncFilesystemErr
}
func (m *MockReleaser) PreCleanQAChecks() error {
	m.PreCleanQAChecksCalled = true
	return m.PreCleanQAChecksErr
}
func (m *MockReleaser) CleanRootfs() error {
	m.CleanRootfsCalled = true
	return m.CleanRootfsErr
}
func (m *MockReleaser) SetupHostname() error {
	m.SetupHostnameCalled = true
	return m.SetupHostnameErr
}
func (m *MockReleaser) SetupServices() error {
	m.SetupServicesCalled = true
	return m.SetupServicesErr
}
func (m *MockReleaser) ReleaseHook() error {
	m.ReleaseHookCalled = true
	return m.ReleaseHookErr
}
func (m *MockReleaser) PostCleanShrink() error {
	m.PostCleanShrinkCalled = true
	return m.PostCleanShrinkErr
}

func (m *MockReleaser) OstreePrepare() error {
	m.OstreePrepareCalled = true
	return m.OstreePrepareErr
}
func (m *MockReleaser) MaybeOstreeInit() error {
	m.MaybeOstreeInitCalled = true
	return m.MaybeOstreeInitErr
}
func (m *MockReleaser) SymlinkEtc() error {
	m.SymlinkEtcCalled = true
	return m.SymlinkEtcErr
}
func (m *MockReleaser) UnlinkEtc() error {
	m.UnlinkEtcCalled = true
	return m.UnlinkEtcErr
}
func (m *MockReleaser) AddExtraDotDotToUsrEtcPortage() error {
	m.AddExtraDotDotToUsrEtcPortageCalled = true
	return m.AddExtraDotDotToUsrEtcPortageErr
}
func (m *MockReleaser) RemoveExtraDotDotFromUsrEtcPortage() error {
	m.RemoveExtraDotDotFromUsrEtcPortageCalled = true
	return m.RemoveExtraDotDotFromUsrEtcPortageErr
}

func (m *MockReleaser) Release(opts CommitOptions) error {
	m.ReleaseCalled = true
	m.ReleaseOpts = append(m.ReleaseOpts, opts)
	return m.ReleaseErr
}

func (m *MockReleaser) ReleaseLockDir() (string, error) {
	return m.ReleaseLockDir_, m.ReleaseLockDirErr
}
func (m *MockReleaser) ReleaseLockPath(name string) (string, error) {
	return m.ReleaseLockPath_, m.ReleaseLockPathErr
}
func (m *MockReleaser) ExecuteWithReleaseLock(name string, fn func() error) error {
	if m.ExecuteWithReleaseLockErr != nil {
		return m.ExecuteWithReleaseLockErr
	}
	return fn()
}

func (m *MockReleaser) Cleanup() {
	m.CleanupCalled = true
}
