package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/seeder"
	"matrixos/vector/lib/validation"
)

// --- Test helpers ---

// newTestSeedsCommand creates a SeedsCommand with injected mocks,
// bypassing Init() which requires real config files and root.
func newTestSeedsCommand(sd *seeder.MockSeeder, det *seeder.MockSeederDetector,
	cfg *config.MockConfig, args []string,
) (*SeedsCommand, error) {
	cmd := NewSeedsCommand()
	cmd.sd = sd
	cmd.det = det
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

func defaultSeedsTestConfig(t *testing.T) *config.MockConfig {
	t.Helper()
	return &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.PrivateGitRepoPath": {t.TempDir()},
		},
	}
}

func defaultSeedsTestSeeders(baseDir string) []seeder.SeederInfo {
	seederDir := filepath.Join(baseDir, "00-bedrock")
	return []seeder.SeederInfo{
		{
			Name:        "00-bedrock",
			Dir:         seederDir,
			ChrootExec:  filepath.Join(seederDir, "chroot.sh"),
			PrepperExec: filepath.Join(seederDir, "prepper.sh"),
		},
	}
}

// setupSeedsTestDir creates a temp directory containing the mock seeder
// structure (with a params.sh file) and a mock chroot directory.
// It returns (seedersBaseDir, chrootDir). Both directories are
// automatically cleaned up when the test finishes.
func setupSeedsTestDir(t *testing.T) (string, string) {
	t.Helper()
	baseDir := t.TempDir()
	seederDir := filepath.Join(baseDir, "00-bedrock")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(filepath.Join(seederDir, "params.sh"), []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	chrootDir := filepath.Join(t.TempDir(), "bedrock-20260228")
	if err := os.MkdirAll(chrootDir, 0755); err != nil {
		t.Fatalf("MkdirAll chroot: %v", err)
	}
	return baseDir, chrootDir
}

// requireSeedsTools skips the test if the host does not have
// the executables that VerifySeederEnvironmentSetup checks for.
func requireSeedsTools(t *testing.T) {
	t.Helper()
	tools := []string{
		"chroot", "gpg", "openssl",
		"ostree", "unshare", "wget",
	}
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf(
				"skipping: required tool %q not found", tool,
			)
		}
	}
}

// --- Tests ---

func TestSeedsName(t *testing.T) {
	cmd := NewSeedsCommand()
	if name := cmd.Name(); name != "seeds" {
		t.Errorf("Expected name 'seeds', got %q", name)
	}
}

func TestNewSeedsCommand(t *testing.T) {
	cmd := NewSeedsCommand()
	if cmd == nil {
		t.Fatal("NewSeedsCommand returned nil")
	}
}

func TestSeedsParseArgsNotRoot(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 1000 }
	defer func() { getEuid = origEuid }()

	cmd := NewSeedsCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{})
	if err == nil {
		t.Fatal("Expected error for non-root, got nil")
	}
	if !strings.Contains(err.Error(), "must be run as root") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSeedsParseArgsValid(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewSeedsCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{
		"--chroot-dir", "/some/chroot",
		"--skip-seeders", "a,b",
		"--only-seeders", "c",
		"--resume",
		"--stage3-file", "/tmp/stage3.tar.xz",
		"--verbose",
	})
	if err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"chrootDir", cmd.chrootDir, "/some/chroot"},
		{"resume", cmd.resume, true},
		{"stage3File", cmd.stage3File, "/tmp/stage3.tar.xz"},
		{"verbose", cmd.verbose, true},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}

	if len(cmd.skipSeeders) != 2 {
		t.Errorf("skipSeeders len: got %d, want 2", len(cmd.skipSeeders))
	}
	if len(cmd.onlySeeders) != 1 {
		t.Errorf("onlySeeders len: got %d, want 1", len(cmd.onlySeeders))
	}
}

func TestSeedsParseArgsDefaults(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewSeedsCommand()
	cmd.StartUI()
	if err := cmd.parseArgs([]string{}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	if cmd.verbose {
		t.Error("Expected verbose false by default")
	}
	if cmd.resume {
		t.Error("Expected resume false by default")
	}
}

func TestSeedsNoSeedersFound(t *testing.T) {
	requireSeedsTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	sd := seeder.DefaultMockSeeder()
	det := &seeder.MockSeederDetector{
		Detect_: nil, // no seeders
	}
	cfg := defaultSeedsTestConfig(t)

	cmd, err := newTestSeedsCommand(sd, det, cfg, []string{})
	if err != nil {
		t.Fatalf("newTestSeedsCommand: %v", err)
	}

	err = cmd.runSeeds()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no seeders found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSeedsDetectionError(t *testing.T) {
	requireSeedsTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	sd := seeder.DefaultMockSeeder()
	det := &seeder.MockSeederDetector{
		DetectErr: fmt.Errorf("scan failed"),
	}
	cfg := defaultSeedsTestConfig(t)

	cmd, err := newTestSeedsCommand(sd, det, cfg, []string{})
	if err != nil {
		t.Fatalf("newTestSeedsCommand: %v", err)
	}

	err = cmd.runSeeds()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "seeder detection failed") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSeedsGpgImportError(t *testing.T) {
	requireSeedsTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	sd := seeder.DefaultMockSeeder()
	sd.ImportGentooGpgKeysErr = fmt.Errorf("gpg failed")
	det := &seeder.MockSeederDetector{
		Detect_: defaultSeedsTestSeeders(t.TempDir()),
	}
	cfg := defaultSeedsTestConfig(t)

	cmd, err := newTestSeedsCommand(sd, det, cfg, []string{})
	if err != nil {
		t.Fatalf("newTestSeedsCommand: %v", err)
	}

	err = cmd.runSeeds()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "GPG key import failed") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSeedsPrivateRepoError(t *testing.T) {
	requireSeedsTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	sd := seeder.DefaultMockSeeder()
	sd.MaybeInitializePrivateRepoErr = fmt.Errorf("clone failed")
	det := &seeder.MockSeederDetector{
		Detect_: defaultSeedsTestSeeders(t.TempDir()),
	}
	cfg := defaultSeedsTestConfig(t)

	cmd, err := newTestSeedsCommand(sd, det, cfg, []string{})
	if err != nil {
		t.Fatalf("newTestSeedsCommand: %v", err)
	}

	err = cmd.runSeeds()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "private repo") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestSeedsFullPipeline(t *testing.T) {
	requireSeedsTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	baseDir, chrootDir := setupSeedsTestDir(t)
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootName:         "bedrock-20260228",
		ChrootsDir:         filepath.Dir(chrootDir),
		PreferredChrootDir: chrootDir,
	}
	det := &seeder.MockSeederDetector{
		Detect_: defaultSeedsTestSeeders(baseDir),
	}
	cfg := defaultSeedsTestConfig(t)

	cmd, err := newTestSeedsCommand(sd, det, cfg, []string{})
	if err != nil {
		t.Fatalf("newTestSeedsCommand: %v", err)
	}

	err = cmd.runSeeds()
	if err != nil {
		t.Fatalf("runSeeds failed: %v", err)
	}

	// Verify pipeline steps were called.
	if !sd.ImportGentooGpgKeysCalled {
		t.Error("ImportGentooGpgKeys not called")
	}
	if !sd.MaybeInitializePrivateRepoCalled {
		t.Error("MaybeInitializePrivateRepo not called")
	}
	if !sd.ExecuteWithSeederLockCalled {
		t.Error("ExecuteWithSeederLock not called")
	}
	if !sd.ExecutePrepperCalled {
		t.Error("ExecutePrepper not called")
	}
	if !sd.SetupChrootMountsCalled {
		t.Error("SetupChrootMounts not called")
	}
	if !sd.SetupChrootDNSCalled {
		t.Error("SetupChrootDNS not called")
	}
	if !sd.SetupChrootDirsCalled {
		t.Error("SetupChrootDirs not called")
	}
	if !sd.SeedCalled {
		t.Error("Seed not called")
	}
	if !sd.MarkSeederDoneCalled {
		t.Error("MarkSeederDone not called")
	}
}

func TestSeedsWorkerPrepperError(t *testing.T) {
	requireSeedsTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	baseDir, chrootDir := setupSeedsTestDir(t)
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootName:         "bedrock-20260228",
		ChrootsDir:         filepath.Dir(chrootDir),
		PreferredChrootDir: chrootDir,
	}
	sd.ExecutePrepperErr = fmt.Errorf("prepper failed")
	det := &seeder.MockSeederDetector{
		Detect_: defaultSeedsTestSeeders(baseDir),
	}
	cfg := defaultSeedsTestConfig(t)

	cmd, err := newTestSeedsCommand(sd, det, cfg, []string{})
	if err != nil {
		t.Fatalf("newTestSeedsCommand: %v", err)
	}

	err = cmd.runSeeds()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "prepper failed") {
		t.Errorf("Unexpected error: %v", err)
	}
	// Mount should not have been called.
	if sd.SetupChrootMountsCalled {
		t.Error("SetupChrootMounts should not be called after prepper error")
	}
}

func TestSeedsWorkerMountError(t *testing.T) {
	requireSeedsTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	baseDir, chrootDir := setupSeedsTestDir(t)
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootName:         "bedrock-20260228",
		ChrootsDir:         filepath.Dir(chrootDir),
		PreferredChrootDir: chrootDir,
	}
	sd.SetupChrootMountsErr = fmt.Errorf("mount failed")
	det := &seeder.MockSeederDetector{
		Detect_: defaultSeedsTestSeeders(baseDir),
	}
	cfg := defaultSeedsTestConfig(t)

	cmd, err := newTestSeedsCommand(sd, det, cfg, []string{})
	if err != nil {
		t.Fatalf("newTestSeedsCommand: %v", err)
	}

	err = cmd.runSeeds()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mount setup failed") {
		t.Errorf("Unexpected error: %v", err)
	}
	// Chroot execution should not have been attempted.
	if sd.SeedCalled {
		t.Error("Seed should not be called after mount error")
	}
}

func TestSeedsWorkerSkipsDoneSeeder(t *testing.T) {
	requireSeedsTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	baseDir, chrootDir := setupSeedsTestDir(t)
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootName:         "bedrock-20260228",
		ChrootsDir:         filepath.Dir(chrootDir),
		PreferredChrootDir: chrootDir,
	}
	sd.IsSeederDone_ = true // Already done
	det := &seeder.MockSeederDetector{
		Detect_: defaultSeedsTestSeeders(baseDir),
	}
	cfg := defaultSeedsTestConfig(t)

	cmd, err := newTestSeedsCommand(sd, det, cfg, []string{})
	if err != nil {
		t.Fatalf("newTestSeedsCommand: %v", err)
	}

	err = cmd.runSeeds()
	if err != nil {
		t.Fatalf("runSeeds failed: %v", err)
	}

	// The prepper and chroot should NOT have been called.
	if sd.ExecutePrepperCalled {
		t.Error("ExecutePrepper should not be called for done seeder")
	}
	if sd.SeedCalled {
		t.Error("Seed should not be called for done seeder")
	}
}

func TestSeedsBuildSubcommand(t *testing.T) {
	bc := NewBuildCommand()
	err := bc.Init([]string{"seeds", "--help"})
	// --help causes flag.Parse to return ErrHelp, which is acceptable.
	// What matters is "seeds" is recognized.
	if err != nil && strings.Contains(
		err.Error(), "unknown subcommand",
	) {
		t.Errorf(
			"Expected 'seeds' to be a known subcommand, got: %v",
			err,
		)
	}
}

func TestSeedsOutputFiles(t *testing.T) {
	requireSeedsTools(t)
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	tmp := t.TempDir()
	rootfsFile := filepath.Join(tmp, "rootfs.txt")
	seedersFile := filepath.Join(tmp, "seeders.txt")

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	baseDir, chrootDir := setupSeedsTestDir(t)
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootName:         "bedrock-20260228",
		ChrootsDir:         filepath.Dir(chrootDir),
		PreferredChrootDir: chrootDir,
	}

	det := &seeder.MockSeederDetector{
		Detect_: defaultSeedsTestSeeders(baseDir),
	}
	cfg := defaultSeedsTestConfig(t)

	cmd, err := newTestSeedsCommand(sd, det, cfg, []string{
		"--built-rootfs-file", rootfsFile,
		"--built-seeders-file", seedersFile,
	})
	if err != nil {
		t.Fatalf("newTestSeedsCommand: %v", err)
	}

	if err := cmd.runSeeds(); err != nil {
		t.Fatalf("runSeeds failed: %v", err)
	}

	rootfsData, err := os.ReadFile(rootfsFile)
	if err != nil {
		t.Fatalf("read rootfs file: %v", err)
	}
	if !strings.Contains(string(rootfsData), "bedrock-20260228") {
		t.Errorf(
			"rootfs file should contain chroot dir, got: %s",
			string(rootfsData),
		)
	}

	seedersData, err := os.ReadFile(seedersFile)
	if err != nil {
		t.Fatalf("read seeders file: %v", err)
	}
	if !strings.Contains(string(seedersData), "00-bedrock") {
		t.Errorf(
			"seeders file should contain seeder name, got: %s",
			string(seedersData),
		)
	}
}
