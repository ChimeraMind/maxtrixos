package ostree

import (
	"io"
	"strings"

	"matrixos/vector/lib/filesystems"
)

// mockOstree implements IOstree for testing commands.
// Only the fields/methods relevant to each test need to be configured;
// everything else returns safe zero values.
type MockOstree struct {
	Root_            string
	RootErr          error
	Ref_             string
	Deployments      []Deployment
	DeploymentsErr   error
	Refs             []string
	RefsErr          error
	Remote_          string
	RemoteErr        error
	SwitchRef        string
	SwitchErr        error
	LastCommit_      string
	LastCommitErr    error
	UpgradeArgs      []string
	UpgradeErr       error
	Packages         []string
	PackagesErr      error
	PackagesByCommit map[string][]string

	LocalRefs_   []string
	LocalRefsErr error

	RemoveFullResult    string
	RemoveFullResultSet bool // when true, return RemoveFullResult even if empty
	RemoveFullErr       error

	BootCommitResult string
	BootCommitErr    error
}

// Config accessors — return zero values (not used in branch/upgrade tests).
func (m *MockOstree) SetStdout(_ io.Writer)                                   {}
func (m *MockOstree) SetStderr(_ io.Writer)                                   {}
func (m *MockOstree) Print(_ string, _ ...interface{})                        {}
func (m *MockOstree) PrintError(_ string, _ ...interface{})                   {}
func (m *MockOstree) Ref() string                                             { return m.Ref_ }
func (m *MockOstree) SetRef(ref string)                                       { m.Ref_ = ref }
func (m *MockOstree) FullBranchSuffix() (string, error)                       { return "-full", nil }
func (m *MockOstree) IsBranchFullSuffixed() (bool, error)                     { return false, nil }
func (m *MockOstree) BranchShortnameToFull(_, _, _, _ string) (string, error) { return "", nil }
func (m *MockOstree) BranchToFull() (string, error)                           { return "", nil }
func (m *MockOstree) RemoveFullFromBranch() (string, error) {
	if m.RemoveFullErr != nil {
		return "", m.RemoveFullErr
	}
	if m.RemoveFullResultSet {
		return m.RemoveFullResult, nil
	}
	// Default: strip -full suffix if present.
	return strings.TrimSuffix(m.Ref_, "-full"), nil
}
func (m *MockOstree) GpgEnabled() (bool, error)                  { return false, nil }
func (m *MockOstree) GpgPrivateKeyPath() (string, error)         { return "", nil }
func (m *MockOstree) GpgPublicKeyPath() (string, error)          { return "", nil }
func (m *MockOstree) GpgOfficialPubKeyPath() (string, error)     { return "", nil }
func (m *MockOstree) OsName() (string, error)                    { return "", nil }
func (m *MockOstree) Arch() (string, error)                      { return "", nil }
func (m *MockOstree) RepoDir() (string, error)                   { return "", nil }
func (m *MockOstree) Sysroot() (string, error)                   { return "", nil }
func (m *MockOstree) Remote() (string, error)                    { return m.Remote_, m.RemoteErr }
func (m *MockOstree) RemoteURL() (string, error)                 { return "", nil }
func (m *MockOstree) AvailableGpgPubKeyPaths() ([]string, error) { return nil, nil }
func (m *MockOstree) GpgBestPubKeyPath() (string, error)         { return "", nil }
func (m *MockOstree) ClientSideGpgArgs() ([]string, error)       { return nil, nil }
func (m *MockOstree) GpgHomeDir() (string, error)                { return "", nil }
func (m *MockOstree) GpgKeyID() (string, error)                  { return "", nil }
func (m *MockOstree) GpgArgs() ([]string, error)                 { return nil, nil }
func (m *MockOstree) SetGpg(_ bool) error                        { return nil }
func (m *MockOstree) SetVerbose(_ bool)                          {}
func (m *MockOstree) SetupEtc(string) error                      { return nil }
func (m *MockOstree) PrepareFilesystemHierarchy(string) error    { return nil }
func (m *MockOstree) ValidateFilesystemHierarchy(string) error   { return nil }
func (m *MockOstree) BootCommit(string) (string, error) {
	if m.BootCommitErr != nil {
		return "", m.BootCommitErr
	}
	if m.BootCommitResult != "" {
		return m.BootCommitResult, nil
	}
	return "abc123commit", nil
}
func (m *MockOstree) InitRepo() error                                 { return nil }
func (m *MockOstree) ListRemotes() ([]string, error)                  { return nil, nil }
func (m *MockOstree) ImportGpgKey(string) error                       { return nil }
func (m *MockOstree) GpgSignFile(string) error                        { return nil }
func (m *MockOstree) GpgKeys() ([]string, error)                      { return nil, nil }
func (m *MockOstree) InitializeSigningGpg() error                     { return nil }
func (m *MockOstree) InitializeRemoteSigningGpg(string, string) error { return nil }
func (m *MockOstree) MaybeInitializeGpg() error                       { return nil }
func (m *MockOstree) MaybeInitializeRemote() error                    { return nil }
func (m *MockOstree) Pull() error                                     { return nil }
func (m *MockOstree) PullWithRemote(string) error                     { return nil }
func (m *MockOstree) Prune() error                                    { return nil }
func (m *MockOstree) GenerateStaticDelta() error                      { return nil }
func (m *MockOstree) UpdateSummary() error                            { return nil }
func (m *MockOstree) AddRemote() error                                { return nil }
func (m *MockOstree) AddRemoteToRootfs(string) error                  { return nil }
func (m *MockOstree) LocalRefs() ([]string, error) {
	return m.LocalRefs_, m.LocalRefsErr
}
func (m *MockOstree) ListContents(string, string) (*[]filesystems.PathInfo, error) {
	return nil, nil
}
func (m *MockOstree) ListEtcChanges(string, string) ([]EtcChange, error) { return nil, nil }
func (m *MockOstree) DeployedRootfs() (string, error)                    { return "", nil }
func (m *MockOstree) BootedRef() (string, error)                         { return "", nil }
func (m *MockOstree) BootedHash() (string, error)                        { return "", nil }
func (m *MockOstree) Deploy(string, []string) error                      { return nil }
func (m *MockOstree) ConfigDiff() (map[string][]string, error)           { return nil, nil }

// Methods with configurable behavior for tests.
func (m *MockOstree) Root() (string, error) {
	if m.Root_ == "" {
		return "/", m.RootErr
	}
	return m.Root_, m.RootErr
}

func (m *MockOstree) ListDeployments() ([]Deployment, error) {
	return m.Deployments, m.DeploymentsErr
}

func (m *MockOstree) RemoteRefs() ([]string, error) {
	return m.Refs, m.RefsErr
}

func (m *MockOstree) Switch() error {
	m.SwitchRef = m.Ref_
	return m.SwitchErr
}

func (m *MockOstree) LastCommit() (string, error) {
	return m.LastCommit_, m.LastCommitErr
}

func (m *MockOstree) Upgrade(args []string) error {
	m.UpgradeArgs = args
	return m.UpgradeErr
}

func (m *MockOstree) ListPackages(commit string) ([]string, error) {
	if m.PackagesByCommit != nil {
		if pkgs, ok := m.PackagesByCommit[commit]; ok {
			return pkgs, m.PackagesErr
		}
	}
	return m.Packages, m.PackagesErr
}
