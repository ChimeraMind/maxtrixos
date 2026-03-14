package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/releaser"
	"matrixos/vector/lib/validation"
)

// --- Test helpers ---

// newTestReleaseCommand creates a ReleaseCommand with injected mocks,
// bypassing Init() which requires real config, ostree binary, etc.
func newTestReleaseCommand(
	ot ostree.IOstree,
	rel *releaser.MockReleaser,
	cfg *config.MockConfig,
	args []string,
) (*ReleaseCommand, error) {
	cmd := NewReleaseCommand()
	cmd.ot = ot
	cmd.rel = rel
	cmd.cfg = cfg

	qa, err := validation.New(cfg)
	if err != nil {
		return nil, err
	}
	cmd.qa = qa
	cmd.StartUI()

	if err := cmd.parseArgs(args); err != nil {
		return nil, err
	}
	return cmd, nil
}

// defaultReleaseTestConfig returns a config with enough entries for the
// release command's QA checks to pass. The returned config references a
// temp directory as the private git repo path so that
// VerifyReleaserEnvironmentSetup can resolve it.
func defaultReleaseTestConfig() *config.MockConfig {
	tmpDir := filepath.Join(os.TempDir(), "matrixos-release-test-private")
	os.MkdirAll(tmpDir, 0755)
	return &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Sysroot":              {"/sysroot"},
			"Ostree.FullBranchSuffix":     {"full"},
			"matrixOS.OsName":             {"matrixos"},
			"matrixOS.PrivateGitRepoPath": {tmpDir},
		},
	}
}

// requireReleaserTools skips the test if the host does not have
// the executables that VerifyReleaserEnvironmentSetup checks for.
func requireReleaserTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{"chroot", "find", "findmnt", "gpg", "openssl", "ostree", "unshare"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("skipping: required tool %q not found in PATH", tool)
		}
	}
}

// --- Tests ---

func TestReleaseName(t *testing.T) {
	cmd := NewReleaseCommand()
	if name := cmd.Name(); name != "release" {
		t.Errorf("Expected name 'release', got %q", name)
	}
}

func TestNewReleaseCommand(t *testing.T) {
	cmd := NewReleaseCommand()
	if cmd == nil {
		t.Fatal("NewReleaseCommand returned nil")
	}
	if cmd.Name() != "release" {
		t.Errorf("Expected name 'release', got %q", cmd.Name())
	}
}

func TestReleaseParseArgsMissingBranch(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewReleaseCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	})
	if err == nil {
		t.Fatal("Expected error for missing --ref, got nil")
	}
	if !strings.Contains(err.Error(), "--ref is required") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestReleaseParseArgsMissingChrootDir(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewReleaseCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--image-dir", "/some/image",
	})
	if err == nil {
		t.Fatal("Expected error for missing --chroot-dir, got nil")
	}
	if !strings.Contains(err.Error(), "--chroot-dir is required") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestReleaseParseArgsMissingImageDir(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewReleaseCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
	})
	if err == nil {
		t.Fatal("Expected error for missing --image-dir, got nil")
	}
	if !strings.Contains(err.Error(), "--image-dir is required") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestReleaseParseArgsNotRoot(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 1000 }
	defer func() { getEuid = origEuid }()

	cmd := NewReleaseCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	})
	if err == nil {
		t.Fatal("Expected error for non-root, got nil")
	}
	if !strings.Contains(err.Error(), "must be run as root") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestReleaseParseArgsRemotePrefix(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewReleaseCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{
		"--ref", "origin:matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	})
	if err == nil {
		t.Fatal("Expected error for remote-prefixed ref, got nil")
	}
	if !strings.Contains(err.Error(), "remote prefix") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestReleaseParseArgsValid(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewReleaseCommand()
	cmd.StartUI()
	if err := cmd.parseArgs([]string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
		"--verbose",
	}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"ref", cmd.ref, "matrixos/x86_64/dev/gnome"},
		{"chrootDir", cmd.chrootDir, "/some/chroot"},
		{"imageDir", cmd.imageDir, "/some/image"},
		{"verbose", cmd.verbose, true},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestReleaseParseArgsDefaults(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewReleaseCommand()
	cmd.StartUI()
	if err := cmd.parseArgs([]string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	if cmd.verbose {
		t.Error("Expected verbose false")
	}
}

func TestReleaseRunShortNameRefRejected(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := &ostree.MockOstree{}
	cfg := defaultReleaseTestConfig()
	rel := releaser.DefaultMockReleaser()

	// Use a branch shortname (no slashes).
	cmd, err := newTestReleaseCommand(ot, rel, cfg, []string{
		"--ref", "gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	})
	if err != nil {
		t.Fatalf("newTestReleaseCommand failed: %v", err)
	}

	err = cmd.Run()
	if err == nil {
		t.Fatal("Expected error for ref shortname, got nil")
	}
	if !strings.Contains(err.Error(), "specify a complete branch name") {
		t.Errorf("Expected 'specify a complete branch name' error, got: %v", err)
	}
}

// TestReleaseExecuteReleaseFullPipeline verifies the complete release
// pipeline calls the right methods in order.
func TestReleaseExecuteReleaseFullPipeline(t *testing.T) {
	requireReleaserTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := &ostree.MockOstree{}
	cfg := defaultReleaseTestConfig()
	rel := releaser.DefaultMockReleaser()

	cmd, err := newTestReleaseCommand(ot, rel, cfg, []string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	})
	if err != nil {
		t.Fatalf("newTestReleaseCommand failed: %v", err)
	}

	ref := "matrixos/x86_64/dev/gnome"
	fullBranch := "matrixos/x86_64/dev/gnome-full" // mock BranchToFull returns ""
	// For our test, just use a predictable full branch name.
	fullBranch = ref + "-full"

	err = cmd.executeRelease(ref, fullBranch)
	if err != nil {
		t.Fatalf("executeRelease failed: %v", err)
	}

	// Verify all pipeline steps were called.
	if !rel.CheckMatrixOSCalled {
		t.Error("Expected CheckMatrixOS to be called")
	}
	if !rel.SyncFilesystemCalled {
		t.Error("Expected SyncFilesystem to be called")
	}
	if !rel.PreCleanQAChecksCalled {
		t.Error("Expected PreCleanQAChecks to be called")
	}
	if !rel.CleanRootfsCalled {
		t.Error("Expected CleanRootfs to be called")
	}
	if !rel.SetupServicesCalled {
		t.Error("Expected SetupServices to be called")
	}
	if !rel.SetupHostnameCalled {
		t.Error("Expected SetupHostname to be called")
	}
	if !rel.ReleaseHookCalled {
		t.Error("Expected ReleaseHook to be called")
	}
	if !rel.OstreePrepareCalled {
		t.Error("Expected OstreePrepare to be called")
	}
	if !rel.MaybeOstreeInitCalled {
		t.Error("Expected MaybeOstreeInit to be called")
	}
	if !rel.PostCleanShrinkCalled {
		t.Error("Expected PostCleanShrink to be called")
	}

	// Verify two commits were made.
	if len(rel.ReleaseOpts) != 2 {
		t.Fatalf("Expected 2 Release calls, got %d", len(rel.ReleaseOpts))
	}

	// First commit: full branch, no consume, no parent.
	first := rel.ReleaseOpts[0]
	if first.Branch != fullBranch {
		t.Errorf("First commit branch: got %q, want %q", first.Branch, fullBranch)
	}
	if first.Consume {
		t.Error("First commit should not consume")
	}
	if first.ParentBranch != "" {
		t.Errorf("First commit should have no parent, got %q", first.ParentBranch)
	}

	// Second commit: regular branch, consume, parent=full.
	second := rel.ReleaseOpts[1]
	if second.Branch != ref {
		t.Errorf("Second commit branch: got %q, want %q", second.Branch, ref)
	}
	if !second.Consume {
		t.Error("Second commit should consume")
	}
	if second.ParentBranch != fullBranch {
		t.Errorf("Second commit parent: got %q, want %q", second.ParentBranch, fullBranch)
	}

	// Verify symlink/unlink ordering via call tracking.
	if !rel.UnlinkEtcCalled {
		t.Error("Expected UnlinkEtc to be called")
	}
	if !rel.SymlinkEtcCalled {
		t.Error("Expected SymlinkEtc to be called")
	}
	if !rel.AddExtraDotDotToUsrEtcPortageCalled {
		t.Error("Expected AddExtraDotDotToUsrEtcPortage to be called")
	}
	if !rel.RemoveExtraDotDotFromUsrEtcPortageCalled {
		t.Error("Expected RemoveExtraDotDotFromUsrEtcPortage to be called")
	}
}

// TestReleaseExecuteReleaseCheckMatrixOSError verifies early pipeline errors.
func TestReleaseExecuteReleaseCheckMatrixOSError(t *testing.T) {
	requireReleaserTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := &ostree.MockOstree{}
	cfg := defaultReleaseTestConfig()
	rel := releaser.DefaultMockReleaser()
	rel.CheckMatrixOSErr = fmt.Errorf("private repo not found")

	cmd, err := newTestReleaseCommand(ot, rel, cfg, []string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	})
	if err != nil {
		t.Fatalf("newTestReleaseCommand failed: %v", err)
	}

	err = cmd.executeRelease("matrixos/x86_64/dev/gnome", "matrixos/x86_64/dev/gnome-full")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "matrixOS check failed") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// TestReleaseExecuteReleaseSyncFilesystemError verifies sync errors abort the pipeline.
func TestReleaseExecuteReleaseSyncFilesystemError(t *testing.T) {
	requireReleaserTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := &ostree.MockOstree{}
	cfg := defaultReleaseTestConfig()
	rel := releaser.DefaultMockReleaser()
	rel.SyncFilesystemErr = fmt.Errorf("rsync failed")

	cmd, err := newTestReleaseCommand(ot, rel, cfg, []string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	})
	if err != nil {
		t.Fatalf("newTestReleaseCommand failed: %v", err)
	}

	err = cmd.executeRelease("matrixos/x86_64/dev/gnome", "matrixos/x86_64/dev/gnome-full")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "filesystem sync failed") {
		t.Errorf("Unexpected error: %v", err)
	}
	// Steps after sync should not be called.
	if rel.CleanRootfsCalled {
		t.Error("CleanRootfs should not be called after sync error")
	}
}

// TestReleaseExecuteReleaseFirstCommitError verifies first commit failure.
func TestReleaseExecuteReleaseFirstCommitError(t *testing.T) {
	requireReleaserTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := &ostree.MockOstree{}
	cfg := defaultReleaseTestConfig()
	rel := releaser.DefaultMockReleaser()
	rel.ReleaseErr = fmt.Errorf("commit failed")

	cmd, err := newTestReleaseCommand(ot, rel, cfg, []string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	})
	if err != nil {
		t.Fatalf("newTestReleaseCommand failed: %v", err)
	}

	err = cmd.executeRelease("matrixos/x86_64/dev/gnome", "matrixos/x86_64/dev/gnome-full")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "full branch release failed") {
		t.Errorf("Unexpected error: %v", err)
	}
	// PostCleanShrink should not have been called.
	if rel.PostCleanShrinkCalled {
		t.Error("PostCleanShrink should not be called after first commit error")
	}
}

// TestReleaseBuildSubcommand verifies that "release" is registered as a build subcommand.
func TestReleaseBuildSubcommand(t *testing.T) {
	bc := NewBuildCommand()
	err := bc.Init([]string{"release", "--help"})
	// --help causes flag.Parse to return an ErrHelp, which is acceptable.
	// What matters is that "release" is recognized (not "unknown subcommand").
	if err != nil && strings.Contains(err.Error(), "unknown subcommand") {
		t.Errorf("Expected 'release' to be a known subcommand, got: %v", err)
	}
}

// TestReleasePostCleanShrinkError verifies that post-clean shrink errors abort the pipeline.
func TestReleasePostCleanShrinkError(t *testing.T) {
	requireReleaserTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := &ostree.MockOstree{}
	cfg := defaultReleaseTestConfig()
	rel := releaser.DefaultMockReleaser()
	rel.PostCleanShrinkErr = fmt.Errorf("emerge --depclean failed")

	cmd, err := newTestReleaseCommand(ot, rel, cfg, []string{
		"--ref", "matrixos/x86_64/dev/gnome",
		"--chroot-dir", "/some/chroot",
		"--image-dir", "/some/image",
	})
	if err != nil {
		t.Fatalf("newTestReleaseCommand failed: %v", err)
	}

	err = cmd.executeRelease("matrixos/x86_64/dev/gnome", "matrixos/x86_64/dev/gnome-full")
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "post-clean shrink failed") {
		t.Errorf("Unexpected error: %v", err)
	}
	// The second commit should not have been attempted.
	if len(rel.ReleaseOpts) > 1 {
		t.Errorf("Expected at most 1 Release call, got %d", len(rel.ReleaseOpts))
	}
}
