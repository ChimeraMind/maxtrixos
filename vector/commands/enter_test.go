package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/runner"
	"matrixos/vector/lib/seeder"
)

// newTestEnterCommand creates an EnterCommand with injected mocks,
// bypassing Init() which requires real config files and root.
// It replaces the package-level newSeeder factory so that the
// command uses the provided MockSeeder instead of a real one.
func newTestEnterCommand(
	det *seeder.MockSeederDetector,
	chrootRunner runner.ChrootRunFunc,
	args []string,
) (*EnterCommand, error) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewEnterCommand()
	cmd.det = det
	if det != nil {
		detected, err := det.Detect(nil, nil)
		if err != nil {
			return nil, fmt.Errorf("seeder detection failed: %w", err)
		}
		cmd.detected = detected
	}
	if chrootRunner != nil {
		cmd.chrootRunner = chrootRunner
	}

	if err := cmd.parseArgs(args); err != nil {
		return nil, err
	}
	return cmd, nil
}

// withMockSeeder replaces the package-level newSeeder factory with one
// that returns the given MockSeeder. It returns a cleanup function that
// restores the original factory.
func withMockSeeder(sd *seeder.MockSeeder) func() {
	orig := newSeeder
	newSeeder = func(_ config.IConfig, _ *seeder.NewSeederOptions) (seeder.ISeeder, error) {
		return sd, nil
	}
	return func() { newSeeder = orig }
}

// enterLockTestSetup creates a seeder dir with a params file and returns
// a configured MockSeeder and MockSeederDetector that allow enterChrootWithLock
// to resolve the seeder name for the given chrootDir(s).
// The seeder name used is "00-test".
func enterLockTestSetup(t *testing.T, chrootDirs ...string) (*seeder.MockSeeder, *seeder.MockSeederDetector) {
	t.Helper()
	seederDir := filepath.Join(t.TempDir(), "00-test")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		AllChrootDirs: chrootDirs,
	}
	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{
				Name:       "00-test",
				Dir:        seederDir,
				ChrootExec: filepath.Join(seederDir, "chroot.sh"),
			},
		},
	}
	return sd, det
}

// --- Tests ---

func TestEnterName(t *testing.T) {
	cmd := NewEnterCommand()
	if name := cmd.Name(); name != "enter" {
		t.Errorf("Expected name 'enter', got %q", name)
	}
}

func TestNewEnterCommand(t *testing.T) {
	cmd := NewEnterCommand()
	if cmd == nil {
		t.Fatal("NewEnterCommand returned nil")
	}
	if cmd.chrootRunner == nil {
		t.Fatal("chrootRunner should be set to default")
	}
}

func TestEnterParseArgsNotRoot(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 1000 }
	defer func() { getEuid = origEuid }()

	cmd := NewEnterCommand()
	err := cmd.parseArgs([]string{"/some/chroot"})
	if err == nil {
		t.Fatal("Expected error for non-root, got nil")
	}
	if !strings.Contains(err.Error(), "must be run as root") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterParseArgsNoTargets(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewEnterCommand()
	err := cmd.parseArgs([]string{})
	if err == nil {
		t.Fatal("Expected error for no targets, got nil")
	}
	if !strings.Contains(err.Error(), "no chroot dirs or names specified") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterParseArgsValid(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewEnterCommand()
	if err := cmd.parseArgs([]string{"/some/chroot", "bedrock"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	if len(cmd.targets) != 2 {
		t.Fatalf("Expected 2 targets, got %d", len(cmd.targets))
	}
	if cmd.targets[0] != "/some/chroot" {
		t.Errorf("Expected first target /some/chroot, got %q", cmd.targets[0])
	}
	if cmd.targets[1] != "bedrock" {
		t.Errorf("Expected second target bedrock, got %q", cmd.targets[1])
	}
}

func TestEnterRunWithDirectoryTarget(t *testing.T) {
	chrootDir := t.TempDir()

	var calledWith *runner.ChrootCmd
	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		calledWith = cmd
		return nil
	}

	sd, det := enterLockTestSetup(t, chrootDir)
	restore := withMockSeeder(sd)
	defer restore()

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if calledWith == nil {
		t.Fatal("chrootRunner was not called")
	}
	if calledWith.ChrootDir != chrootDir {
		t.Errorf("Expected ChrootDir %q, got %q", chrootDir, calledWith.ChrootDir)
	}
	if calledWith.Name != "/bin/sh" {
		t.Errorf("Expected command /bin/sh, got %q", calledWith.Name)
	}
	if len(calledWith.Args) != 1 || calledWith.Args[0] != "--login" {
		t.Errorf("Expected args [--login], got %v", calledWith.Args)
	}
	if !sd.SetupChrootMountsCalled {
		t.Error("SetupChrootMounts was not called")
	}
	if !sd.ExecuteWithSeederLockCalled {
		t.Error("ExecuteWithSeederLock was not called (lock should be acquired by default)")
	}
	if sd.ExecuteWithSeederLockName != "00-test" {
		t.Errorf("Expected lock name %q, got %q", "00-test", sd.ExecuteWithSeederLockName)
	}
	if !sd.CleanupCalled {
		t.Error("Cleanup was not called")
	}
}

func TestEnterRunWithNamedTarget(t *testing.T) {
	// Create a chroot directory that will be found by name resolution.
	// The bare name "bedrock" falls through to seedersParams lookup,
	// so the detected seeder must use Name="bedrock" to match.
	chrootsDir := t.TempDir()
	chrootDir := filepath.Join(chrootsDir, "bedrock")
	if err := os.MkdirAll(chrootDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Create seeder structure — use bare name "bedrock" as the seeder name
	// so that seedersParams["bedrock"] exists for the fall-through lookup.
	seederDir := filepath.Join(t.TempDir(), "bedrock")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	var calledWith *runner.ChrootCmd
	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		calledWith = cmd
		return nil
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootName:         "bedrock",
		ChrootsDir:         chrootsDir,
		PreferredChrootDir: filepath.Join(chrootsDir, "bedrock"),
		AllChrootDirs:      []string{filepath.Join(chrootsDir, "bedrock")},
	}
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{
				Name:        "bedrock",
				Dir:         seederDir,
				ChrootExec:  filepath.Join(seederDir, "chroot.sh"),
				PrepperExec: filepath.Join(seederDir, "prepper.sh"),
			},
		},
	}

	// Create params.sh so the file existence check passes.
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{"bedrock"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if calledWith == nil {
		t.Fatal("chrootRunner was not called")
	}
	if calledWith.ChrootDir != chrootDir {
		t.Errorf("Expected ChrootDir %q, got %q", chrootDir, calledWith.ChrootDir)
	}
}

func TestEnterRunNoMatchingNames(t *testing.T) {
	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: nil, // no seeders detected
	}

	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		t.Fatal("chrootRunner should not be called")
		return nil
	}

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{"nonexistent"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no chroot dirs or names found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterRunSetupMountsError(t *testing.T) {
	chrootDir := t.TempDir()

	sd, det := enterLockTestSetup(t, chrootDir)
	sd.SetupChrootMountsErr = fmt.Errorf("mount failure")
	restore := withMockSeeder(sd)
	defer restore()

	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		t.Fatal("chrootRunner should not be called on mount failure")
		return nil
	}

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		err = cmd.run()
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mount failure") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterRunChrootError(t *testing.T) {
	chrootDir := t.TempDir()

	sd, det := enterLockTestSetup(t, chrootDir)
	restore := withMockSeeder(sd)
	defer restore()

	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		return fmt.Errorf("shell exited with error")
	}

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		err = cmd.run()
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "shell exited with error") {
		t.Errorf("Unexpected error: %v", err)
	}
	// Cleanup should still be called even on error.
	if !sd.CleanupCalled {
		t.Error("Cleanup was not called after chroot error")
	}
}

func TestEnterRunEmptyTarget(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	sd := seeder.DefaultMockSeeder()
	restore := withMockSeeder(sd)
	defer restore()

	// Empty target passed directly to run (bypassing parseArgs which would catch it)
	cmd := NewEnterCommand()
	cmd.det = &seeder.MockSeederDetector{}
	cmd.targets = []string{""}

	err := cmd.run()
	if err == nil {
		t.Fatal("Expected error for empty target, got nil")
	}
	if !strings.Contains(err.Error(), "no chroot dirs or names found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterRunUnrecognizedPath(t *testing.T) {
	// Target that looks like a path but doesn't exist as a directory.
	// With no "unrecognized argument" error, the target is silently
	// skipped, resulting in "no chroot dirs or names found".
	sd := seeder.DefaultMockSeeder()
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{}

	cmd, err := newTestEnterCommand(det, nil, []string{"/nonexistent/path/to/chroot"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no chroot dirs or names found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterRunMultipleDirectories(t *testing.T) {
	chrootDir1 := t.TempDir()
	chrootDir2 := t.TempDir()

	var entered []string
	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		entered = append(entered, cmd.ChrootDir)
		return nil
	}

	sd, det := enterLockTestSetup(t, chrootDir1, chrootDir2)
	restore := withMockSeeder(sd)
	defer restore()

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir1, chrootDir2})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if len(entered) != 2 {
		t.Fatalf("Expected 2 chroot entries, got %d", len(entered))
	}
	if entered[0] != chrootDir1 {
		t.Errorf("Expected first entry %q, got %q", chrootDir1, entered[0])
	}
	if entered[1] != chrootDir2 {
		t.Errorf("Expected second entry %q, got %q", chrootDir2, entered[1])
	}
}

func TestEnterDetectionError(t *testing.T) {
	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		DetectErr: fmt.Errorf("detection failed"),
	}

	// Detection now happens during Init (simulated by newTestEnterCommand),
	// so the error is returned at construction time.
	_, err := newTestEnterCommand(det, nil, []string{"somename"})
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "seeder detection failed") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterParamsExecNameError(t *testing.T) {
	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableNameErr = fmt.Errorf("config missing")
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{}

	cmd, err := newTestEnterCommand(det, nil, []string{"somename"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "params executable name") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterParseArgsSkipLockFlag(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewEnterCommand()
	if err := cmd.parseArgs([]string{"--skiplock", "/some/chroot"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	if !cmd.skipLock {
		t.Error("Expected skipLock to be true")
	}
	if len(cmd.targets) != 1 || cmd.targets[0] != "/some/chroot" {
		t.Errorf("Expected targets [/some/chroot], got %v", cmd.targets)
	}
}

func TestEnterParseArgsSkipLockDefault(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewEnterCommand()
	if err := cmd.parseArgs([]string{"/some/chroot"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	if cmd.skipLock {
		t.Error("Expected skipLock to be false by default")
	}
}

func TestEnterRunAcquiresLockByDefault(t *testing.T) {
	chrootDir := t.TempDir()

	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		return nil
	}

	sd, det := enterLockTestSetup(t, chrootDir)
	restore := withMockSeeder(sd)
	defer restore()

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if !sd.ExecuteWithSeederLockCalled {
		t.Error("ExecuteWithSeederLock was not called; lock should be acquired by default")
	}
}

func TestEnterRunSkipsLockWithFlag(t *testing.T) {
	chrootDir := t.TempDir()

	var calledWith *runner.ChrootCmd
	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		calledWith = cmd
		return nil
	}

	sd, det := enterLockTestSetup(t, chrootDir)
	restore := withMockSeeder(sd)
	defer restore()

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{"--skiplock", chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if sd.ExecuteWithSeederLockCalled {
		t.Error("ExecuteWithSeederLock was called; lock should be skipped with --skiplock")
	}
	if calledWith == nil {
		t.Fatal("chrootRunner was not called")
	}
	if calledWith.ChrootDir != chrootDir {
		t.Errorf("Expected ChrootDir %q, got %q", chrootDir, calledWith.ChrootDir)
	}
	if !sd.SetupChrootMountsCalled {
		t.Error("SetupChrootMounts was not called")
	}
}

func TestEnterRunLockAcquisitionError(t *testing.T) {
	chrootDir := t.TempDir()

	sd, det := enterLockTestSetup(t, chrootDir)
	sd.ExecuteWithSeederLockErr = fmt.Errorf("lock timeout")
	restore := withMockSeeder(sd)
	defer restore()

	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		t.Fatal("chrootRunner should not be called when lock fails")
		return nil
	}

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		err = cmd.run()
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "lock timeout") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterRunSkipLockWithMultipleDirectories(t *testing.T) {
	chrootDir1 := t.TempDir()
	chrootDir2 := t.TempDir()

	var entered []string
	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		entered = append(entered, cmd.ChrootDir)
		return nil
	}

	sd, det := enterLockTestSetup(t, chrootDir1, chrootDir2)
	restore := withMockSeeder(sd)
	defer restore()

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{"--skiplock", chrootDir1, chrootDir2})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if len(entered) != 2 {
		t.Fatalf("Expected 2 chroot entries, got %d", len(entered))
	}
	if sd.ExecuteWithSeederLockCalled {
		t.Error("ExecuteWithSeederLock should not be called with --skiplock")
	}
}

func TestEnterRunLockUsesSeederName(t *testing.T) {
	// The chroot dir basename (e.g. "bedrock-20260302") differs from the
	// canonical seeder name ("00-bedrock"). The lock must use the seeder name.
	chrootsDir := t.TempDir()
	chrootDir := filepath.Join(chrootsDir, "bedrock-20260302")
	if err := os.MkdirAll(chrootDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	seederDir := filepath.Join(t.TempDir(), "00-bedrock")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootName:    "bedrock",
		ChrootsDir:    chrootsDir,
		AllChrootDirs: []string{chrootDir},
	}
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{
				Name:       "00-bedrock",
				Dir:        seederDir,
				ChrootExec: filepath.Join(seederDir, "chroot.sh"),
			},
		},
	}

	mockChrootRunner := func(cmd *runner.ChrootCmd) error { return nil }
	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if !sd.ExecuteWithSeederLockCalled {
		t.Fatal("ExecuteWithSeederLock was not called")
	}
	if sd.ExecuteWithSeederLockName != "00-bedrock" {
		t.Errorf("Expected lock name %q, got %q", "00-bedrock", sd.ExecuteWithSeederLockName)
	}
}

func TestEnterRunLockNoMatchingSeeder(t *testing.T) {
	// chrootDir exists on disk but is NOT in any seeder's AllChrootDirs.
	// The DirectoryExists fallback accepts it at classification time,
	// but enterChrootWithLock can't find a matching seeder, triggering
	// "no valid seeder chroot found" error suggesting --skiplock.
	chrootDir := t.TempDir()

	seederDir := filepath.Join(t.TempDir(), "00-bedrock")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		AllChrootDirs: []string{"/some/other/dir"},
	}
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{
				Name:       "00-bedrock",
				Dir:        seederDir,
				ChrootExec: filepath.Join(seederDir, "chroot.sh"),
			},
		},
	}

	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		t.Fatal("chrootRunner should not be called when seeder cannot be resolved")
		return nil
	}

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		err = cmd.run()
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no valid seeder chroot found") {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "--skiplock") {
		t.Errorf("Expected error to suggest --skiplock, got: %v", err)
	}
}

// --- New tests for seeder-name-based chroot resolution ---

func TestEnterRunNewSeederError(t *testing.T) {
	// When newSeeder returns an error, run() should propagate it.
	orig := newSeeder
	newSeeder = func(_ config.IConfig, _ *seeder.NewSeederOptions) (seeder.ISeeder, error) {
		return nil, fmt.Errorf("seeder factory boom")
	}
	defer func() { newSeeder = orig }()

	det := &seeder.MockSeederDetector{}
	cmd, err := newTestEnterCommand(det, nil, []string{"/some/dir"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to initialize seeder") {
		t.Errorf("Unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "seeder factory boom") {
		t.Errorf("Expected wrapped cause, got: %v", err)
	}
}

func TestEnterRunMakeSeederParamsError(t *testing.T) {
	// When makeSeederParams fails (ParamsExecutableName error), run()
	// should return "failed to make params map".
	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableNameErr = fmt.Errorf("params name unavailable")
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{}
	cmd, err := newTestEnterCommand(det, nil, []string{"somename"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to make params map") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterMakeSeederParamsSkipsUnparseable(t *testing.T) {
	// makeSeederParams should skip seeders whose params cannot be parsed
	// and still return a valid (possibly empty) map.
	seederDir := filepath.Join(t.TempDir(), "00-broken")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParamsErr = fmt.Errorf("parse error")
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{Name: "00-broken", Dir: seederDir},
		},
	}

	cmd, err := newTestEnterCommand(det, nil, []string{"somename"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	// run() still proceeds; the broken seeder is just absent from the map.
	// "somename" gets added to chrootNames but can't be resolved, so we
	// get "no chroot dirs or names found".
	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no chroot dirs or names found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterMakeSeederParamsSkipsMissingParamsFile(t *testing.T) {
	// makeSeederParams should skip seeders whose params file doesn't exist.
	seederDir := filepath.Join(t.TempDir(), "00-noparams")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	// Deliberately do NOT create params.sh.

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{Name: "00-noparams", Dir: seederDir},
		},
	}

	cmd, err := newTestEnterCommand(det, nil, []string{"somename"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	// The seeder is skipped due to missing params file, so "somename"
	// gets added to chrootNames but can't be resolved.
	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no chroot dirs or names found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterRunSeederNameTargetPreferredChrootDir(t *testing.T) {
	// A target that is a seeder name (containing /) resolves via
	// PreferredChrootDir from the seeder params. The basename is extracted
	// and resolved through resolveNames using ChrootsDir.
	chrootsDir := t.TempDir()
	chrootDir := filepath.Join(chrootsDir, "bedrock")
	if err := os.MkdirAll(chrootDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	seederDir := filepath.Join(t.TempDir(), "sub", "00-bedrock")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var calledWith *runner.ChrootCmd
	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		calledWith = cmd
		return nil
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootsDir:         chrootsDir,
		PreferredChrootDir: chrootDir,
		AllChrootDirs:      []string{chrootDir},
	}
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{
				Name:       "sub/00-bedrock",
				Dir:        seederDir,
				ChrootExec: filepath.Join(seederDir, "chroot.sh"),
			},
		},
	}

	// Target contains '/' and is NOT an existing directory, so it hits the
	// seedersParams lookup. The seeder name "sub/00-bedrock" matches.
	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{"sub/00-bedrock"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if calledWith == nil {
		t.Fatal("chrootRunner was not called")
	}
	if calledWith.ChrootDir != chrootDir {
		t.Errorf("Expected ChrootDir %q, got %q", chrootDir, calledWith.ChrootDir)
	}
}

func TestEnterRunSeederNameTargetLatestAvailableChrootDir(t *testing.T) {
	// When PreferredChrootDir is empty, LatestAvailableChrootDir is used.
	// The basename is extracted and resolved through resolveNames using ChrootsDir.
	chrootsDir := t.TempDir()
	chrootDir := filepath.Join(chrootsDir, "server")
	if err := os.MkdirAll(chrootDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	seederDir := filepath.Join(t.TempDir(), "sub", "00-server")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var calledWith *runner.ChrootCmd
	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		calledWith = cmd
		return nil
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootsDir:               chrootsDir,
		PreferredChrootDir:       "",
		LatestAvailableChrootDir: chrootDir,
		AllChrootDirs:            []string{chrootDir},
	}
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{
				Name:       "sub/00-server",
				Dir:        seederDir,
				ChrootExec: filepath.Join(seederDir, "chroot.sh"),
			},
		},
	}

	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{"sub/00-server"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if calledWith == nil {
		t.Fatal("chrootRunner was not called")
	}
	if calledWith.ChrootDir != chrootDir {
		t.Errorf("Expected ChrootDir %q, got %q", chrootDir, calledWith.ChrootDir)
	}
}

func TestEnterRunSeederNameTargetNoChrootDirs(t *testing.T) {
	// Target matches a seeder name in seedersParams, but both
	// PreferredChrootDir and LatestAvailableChrootDir are empty →
	// no names are added, so resolution finds nothing.
	seederDir := filepath.Join(t.TempDir(), "sub", "00-empty")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		PreferredChrootDir:       "",
		LatestAvailableChrootDir: "",
	}
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{
				Name: "sub/00-empty",
				Dir:  seederDir,
			},
		},
	}

	cmd, err := newTestEnterCommand(det, nil, []string{"sub/00-empty"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no chroot dirs or names found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterRunSeederNameTargetNotInParams(t *testing.T) {
	// Target contains '/' but is NOT found in seedersParams →
	// "unrecognized argument" error.
	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{} // no seeders detected

	cmd, err := newTestEnterCommand(det, nil, []string{"sub/nonexistent"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no chroot dirs or names found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterRunLockWithPreferredChrootDirMatch(t *testing.T) {
	// enterChrootWithLock should match via PreferredChrootDir as a
	// last-ditch attempt when AllChrootDirs doesn't contain chrootDir.
	// chrootDir exists on disk, so the DirectoryExists fallback accepts it
	// at classification time even without being in AllChrootDirs.
	chrootDir := t.TempDir()

	seederDir := filepath.Join(t.TempDir(), "00-bedrock")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		PreferredChrootDir: chrootDir,
		AllChrootDirs:      []string{}, // empty — forces PreferredChrootDir fallback in lock
	}
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{
				Name:       "00-bedrock",
				Dir:        seederDir,
				ChrootExec: filepath.Join(seederDir, "chroot.sh"),
			},
		},
	}

	mockChrootRunner := func(cmd *runner.ChrootCmd) error { return nil }
	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if !sd.ExecuteWithSeederLockCalled {
		t.Fatal("ExecuteWithSeederLock was not called")
	}
	if sd.ExecuteWithSeederLockName != "00-bedrock" {
		t.Errorf("Expected lock name %q, got %q", "00-bedrock", sd.ExecuteWithSeederLockName)
	}
}

func TestEnterRunSkipLockWorkerError(t *testing.T) {
	// With --skiplock, enterChrootWorker error should be returned
	// wrapped with the seeder name.
	chrootDir := t.TempDir()

	sd, det := enterLockTestSetup(t, chrootDir)
	sd.SetupChrootMountsErr = fmt.Errorf("mount exploded")
	restore := withMockSeeder(sd)
	defer restore()

	cmd, err := newTestEnterCommand(det, nil, []string{"--skiplock", chrootDir})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		err = cmd.run()
	})

	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "chroot enter failed") {
		t.Errorf("Expected wrapped chroot enter error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mount exploded") {
		t.Errorf("Expected root cause, got: %v", err)
	}
}

func TestEnterResolveNamesEmptyChrootsDir(t *testing.T) {
	// resolveNames should skip seeders whose ChrootsDir is empty,
	// resulting in no resolution for the bare name.
	seederDir := filepath.Join(t.TempDir(), "00-empty")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootsDir: "", // empty — should be skipped
	}
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{Name: "00-empty", Dir: seederDir},
		},
	}

	cmd, err := newTestEnterCommand(det, nil, []string{"somename"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	if !strings.Contains(err.Error(), "no chroot dirs or names found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterResolveNamesSeederNotInSps(t *testing.T) {
	// resolveNames should skip seeders that are not in the
	// pre-computed SeedersParams map (e.g. due to parse failure).
	seederDir := filepath.Join(t.TempDir(), "00-broken")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	// ParseSeederParams returns error → makeSeederParams skips it,
	// so sps won't contain "00-broken". resolveNames must also skip it.
	sd.ParseSeederParamsErr = fmt.Errorf("bad params")
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{Name: "00-broken", Dir: seederDir},
		},
	}

	cmd, err := newTestEnterCommand(det, nil, []string{"somename"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	err = cmd.run()
	if err == nil {
		t.Fatal("Expected error, got nil")
	}
	// "somename" gets added to chrootNames but can't be resolved
	// because the seeder's params were unparseable.
	if !strings.Contains(err.Error(), "no chroot dirs or names found") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestEnterRunMixedTargetsSeederNameAndDirectory(t *testing.T) {
	// Mix of a direct directory target and a seeder-name target (with /).
	// The seeder-name target resolves via PreferredChrootDir basename + ChrootsDir.
	chrootDir1 := t.TempDir()
	chrootsDir := t.TempDir()
	chrootDir2 := filepath.Join(chrootsDir, "server")
	if err := os.MkdirAll(chrootDir2, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	seederDir := filepath.Join(t.TempDir(), "sub", "00-server")
	if err := os.MkdirAll(seederDir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	paramsPath := filepath.Join(seederDir, "params.sh")
	if err := os.WriteFile(paramsPath, []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	var entered []string
	mockChrootRunner := func(cmd *runner.ChrootCmd) error {
		entered = append(entered, cmd.ChrootDir)
		return nil
	}

	sd := seeder.DefaultMockSeeder()
	sd.ParamsExecutableName_ = "params.sh"
	sd.ParseSeederParams_ = &seeder.SeederParams{
		ChrootsDir:         chrootsDir,
		PreferredChrootDir: chrootDir2,
		AllChrootDirs:      []string{chrootDir1, chrootDir2},
	}
	restore := withMockSeeder(sd)
	defer restore()

	det := &seeder.MockSeederDetector{
		Detect_: []seeder.SeederInfo{
			{
				Name:       "sub/00-server",
				Dir:        seederDir,
				ChrootExec: filepath.Join(seederDir, "chroot.sh"),
			},
		},
	}

	// First target is a real dir, second is a seeder name (resolved via params).
	cmd, err := newTestEnterCommand(det, mockChrootRunner, []string{chrootDir1, "sub/00-server"})
	if err != nil {
		t.Fatalf("newTestEnterCommand: %v", err)
	}

	captureStdout(t, func() {
		if err := cmd.run(); err != nil {
			t.Fatalf("run failed: %v", err)
		}
	})

	if len(entered) != 2 {
		t.Fatalf("Expected 2 chroot entries, got %d", len(entered))
	}
	if entered[0] != chrootDir1 {
		t.Errorf("Expected first entry %q, got %q", chrootDir1, entered[0])
	}
	if entered[1] != chrootDir2 {
		t.Errorf("Expected second entry (from PreferredChrootDir) %q, got %q", chrootDir2, entered[1])
	}
}
