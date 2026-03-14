package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/ostree"
)

// --- Test helpers ---

// newTestReleaseCommand creates a ReleaseCommand with injected mocks,
// bypassing Init() which requires real config, ostree binary, etc.
func newTestReleaseCommand(
	ot ostree.IOstree,
	cfg *config.MockConfig,
	args []string,
) (*ReleaseCommand, error) {
	cmd := NewReleaseCommand()
	cmd.ot = ot
	cmd.cfg = cfg
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

	// Use a branch shortname (no slashes).
	cmd, err := newTestReleaseCommand(ot, cfg, []string{
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
