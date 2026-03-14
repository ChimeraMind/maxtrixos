package seeder

import (
	"fmt"
	"io"
	"os"
)

// Compile-time interface check.
var _ ISeeder = (*MockSeeder)(nil)

// MockSeeder implements ISeeder for testing commands.
// Only the fields/methods relevant to each test need to be configured;
// everything else returns safe zero values.
type MockSeeder struct {
	stdout io.Writer
	stderr io.Writer

	// Config accessors
	ChrootSeedersDir_              string
	ChrootSeedersDirErr            error
	ChrootBuildArtifactsDir_       string
	ChrootBuildArtifactsDirErr     error
	DisabledSeederFile_            string
	DisabledSeederFileErr          error
	UseLocalGitRepoInsideChroot_   bool
	UseLocalGitRepoInsideChrootErr error
	DeleteDotGitFromGitRepo_       bool
	DeleteDotGitFromGitRepoErr     error
	GitCloneArgs_                  string
	GitCloneArgsErr                error
	ChrootExecName_                string
	ChrootExecNameErr              error
	ParamsExecutableName_          string
	ParamsExecutableNameErr        error
	PrepperExecName_               string
	PrepperExecNameErr             error
	PhasesStateDir_                string
	PhasesStateDirErr              error
	SeederDoneFlagFilePrefix_      string
	SeederDoneFlagFilePrefixErr    error
	PrivateExampleGitRepo_         string
	PrivateExampleGitRepoErr       error
	PrivateGitRepoPath_            string
	PrivateGitRepoPathErr          error
	LockDir_                       string
	LockDirErr                     error
	LockWaitSeconds_               string
	LockWaitSecondsErr             error
	Stage3DownloadUrl_             string
	Stage3DownloadUrlErr           error

	// Execution
	RetryableCmdErr               error
	MaybeInitializePrivateRepoErr error

	// Locking
	SeederLockDir_           string
	SeederLockDirErr         error
	SeederLockPath_          string
	SeederLockPathErr        error
	ExecuteWithSeederLockErr error

	// Track calls
	RetryableCmdCalled               bool
	MaybeInitializePrivateRepoCalled bool
	ExecuteWithSeederLockCalled      bool
}

// DefaultMockSeeder returns a MockSeeder with sensible defaults for tests.
func DefaultMockSeeder() *MockSeeder {
	return &MockSeeder{}
}

func (m *MockSeeder) SetStdout(w io.Writer) { m.stdout = w }
func (m *MockSeeder) SetStderr(w io.Writer) { m.stderr = w }
func (m *MockSeeder) Stdout() io.Writer {
	if m.stdout == nil {
		return os.Stdout
	}
	return m.stdout
}
func (m *MockSeeder) Stderr() io.Writer {
	if m.stderr == nil {
		return os.Stderr
	}
	return m.stderr
}

func (m *MockSeeder) Print(format string, args ...any) {
	fmt.Fprintf(m.Stdout(), format, args...)
}
func (m *MockSeeder) PrintWarning(format string, args ...any) {
	fmt.Fprintf(m.Stderr(), format, args...)
}
func (m *MockSeeder) PrintError(format string, args ...any) {
	fmt.Fprintf(m.Stderr(), format, args...)
}

func (m *MockSeeder) ChrootSeedersDir() (string, error) {
	return m.ChrootSeedersDir_, m.ChrootSeedersDirErr
}
func (m *MockSeeder) ChrootBuildArtifactsDir() (string, error) {
	return m.ChrootBuildArtifactsDir_, m.ChrootBuildArtifactsDirErr
}
func (m *MockSeeder) DisabledSeederFile() (string, error) {
	return m.DisabledSeederFile_, m.DisabledSeederFileErr
}
func (m *MockSeeder) UseLocalGitRepoInsideChroot() (bool, error) {
	return m.UseLocalGitRepoInsideChroot_, m.UseLocalGitRepoInsideChrootErr
}

func (m *MockSeeder) DeleteDotGitFromGitRepo() (bool, error) {
	return m.DeleteDotGitFromGitRepo_, m.DeleteDotGitFromGitRepoErr
}

func (m *MockSeeder) GitCloneArgs() (string, error) {
	return m.GitCloneArgs_, m.GitCloneArgsErr
}

func (m *MockSeeder) ChrootExecName() (string, error) {
	return m.ChrootExecName_, m.ChrootExecNameErr
}
func (m *MockSeeder) ParamsExecutableName() (string, error) {
	return m.ParamsExecutableName_, m.ParamsExecutableNameErr
}
func (m *MockSeeder) PrepperExecName() (string, error) {
	return m.PrepperExecName_, m.PrepperExecNameErr
}
func (m *MockSeeder) PhasesStateDir() (string, error) {
	return m.PhasesStateDir_, m.PhasesStateDirErr
}
func (m *MockSeeder) SeederDoneFlagFilePrefix() (string, error) {
	return m.SeederDoneFlagFilePrefix_, m.SeederDoneFlagFilePrefixErr
}
func (m *MockSeeder) PrivateExampleGitRepo() (string, error) {
	return m.PrivateExampleGitRepo_, m.PrivateExampleGitRepoErr
}
func (m *MockSeeder) PrivateGitRepoPath() (string, error) {
	return m.PrivateGitRepoPath_, m.PrivateGitRepoPathErr
}
func (m *MockSeeder) LockDir() (string, error) {
	return m.LockDir_, m.LockDirErr
}
func (m *MockSeeder) LockWaitSeconds() (string, error) {
	return m.LockWaitSeconds_, m.LockWaitSecondsErr
}
func (m *MockSeeder) Stage3DownloadUrl() (string, error) {
	return m.Stage3DownloadUrl_, m.Stage3DownloadUrlErr
}

func (m *MockSeeder) RetryableCmd(tries int, name string, args ...string) error {
	m.RetryableCmdCalled = true
	return m.RetryableCmdErr
}

func (m *MockSeeder) MaybeInitializePrivateRepo() error {
	m.MaybeInitializePrivateRepoCalled = true
	return m.MaybeInitializePrivateRepoErr
}

func (m *MockSeeder) SeederLockDir() (string, error) {
	return m.SeederLockDir_, m.SeederLockDirErr
}

func (m *MockSeeder) SeederLockPath(name string) (string, error) {
	return m.SeederLockPath_, m.SeederLockPathErr
}

func (m *MockSeeder) ExecuteWithSeederLock(name string, fn func() error) error {
	m.ExecuteWithSeederLockCalled = true
	if m.ExecuteWithSeederLockErr != nil {
		return m.ExecuteWithSeederLockErr
	}
	return fn()
}
