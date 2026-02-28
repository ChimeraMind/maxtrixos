package releaser

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/ostree"
)

// newOstreeTestReleaser returns a Releaser wired to a MockOstree with a
// real temporary imageDir. The caller can customise the mock after creation.
func newOstreeTestReleaser(tb testing.TB) (*Releaser, *ostree.MockOstree) {
	tb.Helper()
	mock := &ostree.MockOstree{}
	r := &Releaser{
		cfg:    &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}},
		ostree: mock,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
	r.imageDir = tb.(*testing.T).TempDir()
	return r, mock
}

// ---------------------------------------------------------------------------
// SymlinkEtc
// ---------------------------------------------------------------------------

func TestSymlinkEtc_CreatesSymlink(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	if err := r.SymlinkEtc(); err != nil {
		t.Fatalf("SymlinkEtc: %v", err)
	}

	target, err := os.Readlink(filepath.Join(r.imageDir, "etc"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if target != "usr/etc" {
		t.Errorf("symlink target = %q, want %q", target, "usr/etc")
	}
}

func TestSymlinkEtc_FailsIfAlreadyExists(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	// Create a real directory so the symlink creation fails.
	if err := os.MkdirAll(filepath.Join(r.imageDir, "etc"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := r.SymlinkEtc(); err == nil {
		t.Fatal("expected error when etc already exists")
	}
}

// ---------------------------------------------------------------------------
// UnlinkEtc
// ---------------------------------------------------------------------------

func TestUnlinkEtc_RemovesSymlink(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	// Create the symlink first.
	if err := os.Symlink("usr/etc", filepath.Join(r.imageDir, "etc")); err != nil {
		t.Fatal(err)
	}

	if err := r.UnlinkEtc(); err != nil {
		t.Fatalf("UnlinkEtc: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(r.imageDir, "etc")); !os.IsNotExist(err) {
		t.Error("expected etc to be removed")
	}
}

func TestUnlinkEtc_FailsIfMissing(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	if err := r.UnlinkEtc(); err == nil {
		t.Fatal("expected error when etc does not exist")
	}
}

// ---------------------------------------------------------------------------
// AddExtraDotDotToUsrEtcPortage
// ---------------------------------------------------------------------------

func TestAddExtraDotDotToUsrEtcPortage_Success(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	// Layout: imageDir/usr/etc/portage -> ../portage-data
	// After adding "../", becomes: ../../portage-data
	// That resolves relative to imageDir/usr/etc/ → imageDir/portage-data
	usrEtc := filepath.Join(r.imageDir, "usr", "etc")
	if err := os.MkdirAll(usrEtc, 0755); err != nil {
		t.Fatal(err)
	}

	// Create the target that the *adjusted* symlink (../../portage-data) will point to.
	if err := os.MkdirAll(filepath.Join(r.imageDir, "portage-data"), 0755); err != nil {
		t.Fatal(err)
	}

	origTarget := "../portage-data"
	if err := os.Symlink(origTarget, filepath.Join(usrEtc, "portage")); err != nil {
		t.Fatal(err)
	}

	if err := r.AddExtraDotDotToUsrEtcPortage(); err != nil {
		t.Fatalf("AddExtraDotDotToUsrEtcPortage: %v", err)
	}

	got, err := os.Readlink(filepath.Join(usrEtc, "portage"))
	if err != nil {
		t.Fatal(err)
	}
	want := "../" + origTarget
	if got != want {
		t.Errorf("symlink = %q, want %q", got, want)
	}
}

func TestAddExtraDotDotToUsrEtcPortage_BrokenSymlink(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	usrEtc := filepath.Join(r.imageDir, "usr", "etc")
	if err := os.MkdirAll(usrEtc, 0755); err != nil {
		t.Fatal(err)
	}

	// Symlink to a target that won't exist after adding "../".
	if err := os.Symlink("nonexistent", filepath.Join(usrEtc, "portage")); err != nil {
		t.Fatal(err)
	}

	err := r.AddExtraDotDotToUsrEtcPortage()
	if err == nil {
		t.Fatal("expected error for broken symlink")
	}
	if !strings.Contains(err.Error(), "broken") {
		t.Errorf("error = %q, want it to mention 'broken'", err)
	}
}

func TestAddExtraDotDotToUsrEtcPortage_NoSymlink(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	// Don't create the symlink at all.
	err := r.AddExtraDotDotToUsrEtcPortage()
	if err == nil {
		t.Fatal("expected error when symlink is missing")
	}
}

// ---------------------------------------------------------------------------
// RemoveExtraDotDotFromUsrEtcPortage
// ---------------------------------------------------------------------------

func TestRemoveExtraDotDotFromUsrEtcPortage_Success(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	usrEtc := filepath.Join(r.imageDir, "usr", "etc")
	if err := os.MkdirAll(usrEtc, 0755); err != nil {
		t.Fatal(err)
	}

	// Starts with the extra "../" prefix.
	if err := os.Symlink("../../../portage-target", filepath.Join(usrEtc, "portage")); err != nil {
		t.Fatal(err)
	}

	if err := r.RemoveExtraDotDotFromUsrEtcPortage(); err != nil {
		t.Fatalf("RemoveExtraDotDotFromUsrEtcPortage: %v", err)
	}

	got, err := os.Readlink(filepath.Join(usrEtc, "portage"))
	if err != nil {
		t.Fatal(err)
	}
	want := "../../portage-target"
	if got != want {
		t.Errorf("symlink = %q, want %q", got, want)
	}
}

func TestRemoveExtraDotDotFromUsrEtcPortage_NoPrefix(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	usrEtc := filepath.Join(r.imageDir, "usr", "etc")
	if err := os.MkdirAll(usrEtc, 0755); err != nil {
		t.Fatal(err)
	}

	// Target that does not have the "../" prefix — should remain unchanged.
	if err := os.Symlink("portage-target", filepath.Join(usrEtc, "portage")); err != nil {
		t.Fatal(err)
	}

	if err := r.RemoveExtraDotDotFromUsrEtcPortage(); err != nil {
		t.Fatalf("RemoveExtraDotDotFromUsrEtcPortage: %v", err)
	}

	got, err := os.Readlink(filepath.Join(usrEtc, "portage"))
	if err != nil {
		t.Fatal(err)
	}
	if got != "portage-target" {
		t.Errorf("symlink = %q, want %q", got, "portage-target")
	}
}

func TestRemoveExtraDotDotFromUsrEtcPortage_NoSymlink(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	err := r.RemoveExtraDotDotFromUsrEtcPortage()
	if err == nil {
		t.Fatal("expected error when symlink is missing")
	}
}

// ---------------------------------------------------------------------------
// OstreePrepare
// ---------------------------------------------------------------------------

func TestOstreePrepare_Success(t *testing.T) {
	r, _ := newOstreeTestReleaser(t)

	if err := r.OstreePrepare(); err != nil {
		t.Fatalf("OstreePrepare: %v", err)
	}
}

func TestOstreePrepare_PrepareError(t *testing.T) {
	r, mock := newOstreeTestReleaser(t)
	mock.PrepareFilesystemHierarchyErr = errors.New("prepare boom")

	err := r.OstreePrepare()
	if err == nil || err.Error() != "prepare boom" {
		t.Fatalf("expected 'prepare boom', got %v", err)
	}
}

func TestOstreePrepare_ValidateError(t *testing.T) {
	r, mock := newOstreeTestReleaser(t)
	mock.ValidateFilesystemHierarchyErr = errors.New("validate boom")

	err := r.OstreePrepare()
	if err == nil || err.Error() != "validate boom" {
		t.Fatalf("expected 'validate boom', got %v", err)
	}
}

// ---------------------------------------------------------------------------
// MaybeOstreeInit
// ---------------------------------------------------------------------------

func TestMaybeOstreeInit_AlreadyExists(t *testing.T) {
	r, mock := newOstreeTestReleaser(t)

	repoDir := t.TempDir()
	mock.RepoDir_ = repoDir
	// Create the objects directory so the check passes.
	if err := os.MkdirAll(filepath.Join(repoDir, "objects"), 0755); err != nil {
		t.Fatal(err)
	}

	if err := r.MaybeOstreeInit(); err != nil {
		t.Fatalf("MaybeOstreeInit: %v", err)
	}

	if mock.InitRepoCalled {
		t.Error("InitRepo should NOT be called when repo already exists")
	}
}

func TestMaybeOstreeInit_CreatesNew(t *testing.T) {
	r, mock := newOstreeTestReleaser(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	mock.RepoDir_ = repoDir

	if err := r.MaybeOstreeInit(); err != nil {
		t.Fatalf("MaybeOstreeInit: %v", err)
	}

	if !mock.InitRepoCalled {
		t.Error("InitRepo should be called for a new repo")
	}
	if !mock.SetGpgCalled {
		t.Error("SetGpg should be called after init")
	}
}

func TestMaybeOstreeInit_RepoDirError(t *testing.T) {
	r, mock := newOstreeTestReleaser(t)
	mock.RepoDirErr = errors.New("repodir boom")

	err := r.MaybeOstreeInit()
	if err == nil || err.Error() != "repodir boom" {
		t.Fatalf("expected 'repodir boom', got %v", err)
	}
}

func TestMaybeOstreeInit_InitRepoError(t *testing.T) {
	r, mock := newOstreeTestReleaser(t)
	mock.RepoDir_ = filepath.Join(t.TempDir(), "repo")
	mock.InitRepoErr = errors.New("init boom")

	err := r.MaybeOstreeInit()
	if err == nil || err.Error() != "init boom" {
		t.Fatalf("expected 'init boom', got %v", err)
	}
}

func TestMaybeOstreeInit_GpgEnabledError(t *testing.T) {
	r, mock := newOstreeTestReleaser(t)
	mock.RepoDir_ = filepath.Join(t.TempDir(), "repo")
	mock.GpgEnabledErr = errors.New("gpg boom")

	err := r.MaybeOstreeInit()
	if err == nil || err.Error() != "gpg boom" {
		t.Fatalf("expected 'gpg boom', got %v", err)
	}
}

func TestMaybeOstreeInit_SetGpgError(t *testing.T) {
	r, mock := newOstreeTestReleaser(t)
	mock.RepoDir_ = filepath.Join(t.TempDir(), "repo")
	mock.SetGpgErr = errors.New("setgpg boom")

	err := r.MaybeOstreeInit()
	if err == nil || err.Error() != "setgpg boom" {
		t.Fatalf("expected 'setgpg boom', got %v", err)
	}
}

// ---------------------------------------------------------------------------
// Release
// ---------------------------------------------------------------------------

// releaseTestSetup configures a Releaser and MockOstree for a successful
// Release() call. Individual tests can break one thing at a time.
func releaseTestSetup(t *testing.T) (*Releaser, *ostree.MockOstree) {
	t.Helper()
	r, mock := newOstreeTestReleaser(t)

	repoDir := filepath.Join(t.TempDir(), "repo")
	mock.RepoDir_ = repoDir
	mock.OsName_ = "matrixOS"
	mock.FancyOsName_ = "Matrix OS"
	mock.GpgArgs_ = []string{"--gpg-sign=ABC"}

	cfgMock := r.cfg.(*config.MockConfig)
	cfgMock.Items["Seeder.ChrootMetadataDir"] = []string{"usr/share/matrixos"}
	cfgMock.Bools["Releaser.GenerateStaticDeltas"] = true

	return r, mock
}

func TestRelease_Success(t *testing.T) {
	r, mock := releaseTestSetup(t)
	opts := CommitOptions{Branch: "matrixos/stable"}

	if err := r.Release(opts); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if !mock.CommitCalled {
		t.Error("Commit should be called")
	}
	if !mock.PruneCalled {
		t.Error("Prune should be called")
	}
	if !mock.GenerateStaticDeltaCalled {
		t.Error("GenerateStaticDelta should be called")
	}
	if !mock.UpdateSummaryCalled {
		t.Error("UpdateSummary should be called")
	}

	// Validate commit options.
	if mock.CommitOpts.Branch != "matrixos/stable" {
		t.Errorf("commit branch = %q, want %q", mock.CommitOpts.Branch, "matrixos/stable")
	}
	if !strings.Contains(mock.CommitOpts.Subject, "matrixOS") {
		t.Errorf("subject should contain OS name, got %q", mock.CommitOpts.Subject)
	}
	if !strings.Contains(mock.CommitOpts.Body, "Matrix OS") {
		t.Errorf("body should contain fancy OS name, got %q", mock.CommitOpts.Body)
	}
}

func TestRelease_WithConsume(t *testing.T) {
	r, mock := releaseTestSetup(t)
	opts := CommitOptions{Branch: "matrixos/stable", Consume: true}

	if err := r.Release(opts); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if !mock.CommitOpts.Consume {
		t.Error("commit opts should have Consume=true")
	}
}

func TestRelease_MissingBranch(t *testing.T) {
	r, _ := releaseTestSetup(t)
	opts := CommitOptions{}

	err := r.Release(opts)
	if err == nil || !strings.Contains(err.Error(), "missing branch") {
		t.Fatalf("expected 'missing branch' error, got %v", err)
	}
}

func TestRelease_EtcExists(t *testing.T) {
	r, _ := releaseTestSetup(t)

	// Create /etc in imageDir — this should cause an error.
	if err := os.MkdirAll(filepath.Join(r.imageDir, "etc"), 0755); err != nil {
		t.Fatal(err)
	}

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || !strings.Contains(err.Error(), "illegal") {
		t.Fatalf("expected 'illegal' error, got %v", err)
	}
}

func TestRelease_RepoDirError(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.RepoDirErr = errors.New("repodir boom")

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || err.Error() != "repodir boom" {
		t.Fatalf("expected 'repodir boom', got %v", err)
	}
}

func TestRelease_OsNameError(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.OsNameErr = errors.New("osname boom")

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || err.Error() != "osname boom" {
		t.Fatalf("expected 'osname boom', got %v", err)
	}
}

func TestRelease_FancyOsNameError(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.FancyOsNameErr = errors.New("fancy boom")

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || err.Error() != "fancy boom" {
		t.Fatalf("expected 'fancy boom', got %v", err)
	}
}

func TestRelease_GpgArgsError(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.GpgArgsErr = errors.New("gpg boom")

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || err.Error() != "gpg boom" {
		t.Fatalf("expected 'gpg boom', got %v", err)
	}
}

func TestRelease_CommitError(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.CommitErr = errors.New("commit boom")

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || !strings.Contains(err.Error(), "commit boom") {
		t.Fatalf("expected wrapped 'commit boom', got %v", err)
	}
}

func TestRelease_PruneError(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.PruneErr = errors.New("prune boom")

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || !strings.Contains(err.Error(), "prune boom") {
		t.Fatalf("expected wrapped 'prune boom', got %v", err)
	}
}

func TestRelease_StaticDeltaError(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.GenerateStaticDeltaErr = errors.New("delta boom")

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || !strings.Contains(err.Error(), "delta boom") {
		t.Fatalf("expected wrapped 'delta boom', got %v", err)
	}
}

func TestRelease_UpdateSummaryError(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.UpdateSummaryErr = errors.New("summary boom")

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || !strings.Contains(err.Error(), "summary boom") {
		t.Fatalf("expected wrapped 'summary boom', got %v", err)
	}
}

func TestRelease_SkipsStaticDeltas(t *testing.T) {
	r, mock := releaseTestSetup(t)
	cfgMock := r.cfg.(*config.MockConfig)
	cfgMock.Bools["Releaser.GenerateStaticDeltas"] = false

	opts := CommitOptions{Branch: "matrixos/stable"}
	if err := r.Release(opts); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if mock.GenerateStaticDeltaCalled {
		t.Error("GenerateStaticDelta should NOT be called when disabled")
	}
	if !mock.UpdateSummaryCalled {
		t.Error("UpdateSummary should still be called")
	}
}

func TestRelease_WithParentBranch(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.LastCommit_ = "abc123parent"

	opts := CommitOptions{
		Branch:       "matrixos/stable",
		ParentBranch: "matrixos/testing",
	}

	if err := r.Release(opts); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if mock.Ref_ != "matrixos/testing" {
		t.Errorf("SetRef = %q, want %q", mock.Ref_, "matrixos/testing")
	}
	if mock.CommitOpts.Parent != "abc123parent" {
		t.Errorf("commit parent = %q, want %q", mock.CommitOpts.Parent, "abc123parent")
	}
	if !strings.Contains(mock.CommitOpts.Body, "matrixos/testing") {
		t.Errorf("body should contain parent branch, got %q", mock.CommitOpts.Body)
	}
}

func TestRelease_ParentBranchRevParseError(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.LastCommitErr = errors.New("revparse boom")

	opts := CommitOptions{
		Branch:       "matrixos/stable",
		ParentBranch: "matrixos/testing",
	}

	err := r.Release(opts)
	if err == nil || !strings.Contains(err.Error(), "revparse boom") {
		t.Fatalf("expected wrapped 'revparse boom', got %v", err)
	}
}

func TestRelease_ReadsMetadataFile(t *testing.T) {
	r, mock := releaseTestSetup(t)

	// Create a build metadata file in imageDir.
	metaDir := filepath.Join(r.imageDir, "usr/share/matrixos")
	if err := os.MkdirAll(metaDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(metaDir, "build"), []byte("build-42"), 0644); err != nil {
		t.Fatal(err)
	}

	opts := CommitOptions{Branch: "matrixos/stable"}
	if err := r.Release(opts); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if !strings.Contains(mock.CommitOpts.Body, "build-42") {
		t.Errorf("body should contain metadata, got %q", mock.CommitOpts.Body)
	}
}

func TestRelease_MissingMetadataUsesDefault(t *testing.T) {
	r, mock := releaseTestSetup(t)

	opts := CommitOptions{Branch: "matrixos/stable"}
	if err := r.Release(opts); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if !strings.Contains(mock.CommitOpts.Body, "not available") {
		t.Errorf("body should contain 'not available' when metadata missing, got %q", mock.CommitOpts.Body)
	}
}

func TestRelease_BuildMetadataFileError(t *testing.T) {
	r, _ := releaseTestSetup(t)
	r.cfg = &config.ErrConfig{Err: errors.New("cfg boom")}

	opts := CommitOptions{Branch: "matrixos/stable"}
	err := r.Release(opts)
	if err == nil || !strings.Contains(err.Error(), "cfg boom") {
		t.Fatalf("expected 'cfg boom', got %v", err)
	}
}

func TestRelease_BodyContainsNoneWhenNoParent(t *testing.T) {
	r, mock := releaseTestSetup(t)

	opts := CommitOptions{Branch: "matrixos/stable"}
	if err := r.Release(opts); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if !strings.Contains(mock.CommitOpts.Body, "none") {
		t.Errorf("body should contain 'none' when no parent branch, got %q", mock.CommitOpts.Body)
	}
}

func TestRelease_GpgArgsPassedToCommit(t *testing.T) {
	r, mock := releaseTestSetup(t)
	mock.GpgArgs_ = []string{"--gpg-sign=KEY1", "--gpg-homedir=/tmp"}

	opts := CommitOptions{Branch: "matrixos/stable"}
	if err := r.Release(opts); err != nil {
		t.Fatalf("Release: %v", err)
	}

	if len(mock.CommitOpts.GpgArgs) != 2 {
		t.Fatalf("gpgArgs len = %d, want 2", len(mock.CommitOpts.GpgArgs))
	}
	if mock.CommitOpts.GpgArgs[0] != "--gpg-sign=KEY1" {
		t.Errorf("gpgArgs[0] = %q, want %q", mock.CommitOpts.GpgArgs[0], "--gpg-sign=KEY1")
	}
}
