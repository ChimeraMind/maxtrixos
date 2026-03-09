package commands

import (
	"os"
	"strings"
	"testing"
	"time"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/imager"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/validation"
)

// --- Test helpers ---

// newTestFlashCommand creates a FlashCommand with injected mocks,
// bypassing Init() which requires real config, ostree binary, etc.
func newTestFlashCommand(
	ot ostree.IOstree,
	im *imager.MockImager,
	fsenc *filesystems.MockFsenc,
	cfg *config.MockConfig,
	args []string,
) (*FlashCommand, error) {
	cmd := NewFlashCommand()
	cmd.ot = ot
	cmd.im = im
	cmd.fsenc = fsenc
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

func defaultFlashTestConfig() *config.MockConfig {
	return &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Sysroot":          {"/sysroot"},
			"Ostree.FullBranchSuffix": {"full"},
			"Ostree.RepoDir":          {"/ostree/repo"},
			"Ostree.Remote":           {"origin"},
			"matrixOS.OsName":         {"matrixos"},
			"matrixOS.Arch":           {"x86_64"},
			"Imager.BootRoot":         {"/boot"},
			"Imager.EfiRoot":          {"/efi"},
			"Imager.Compressor":       {"xz"},
		},
	}
}

func withRootEuid(t *testing.T) {
	t.Helper()
	origEuid := getEuid
	getEuid = func() int { return 0 }
	t.Cleanup(func() { getEuid = origEuid })
}

func withNoSleep(t *testing.T) {
	t.Helper()
	origSleep := sleepFn
	sleepFn = func(time.Duration) {}
	t.Cleanup(func() { sleepFn = origSleep })
}

// --- Name / Constructor tests ---

func TestFlashName(t *testing.T) {
	cmd := NewFlashCommand()
	if name := cmd.Name(); name != "flash" {
		t.Errorf("Expected name 'flash', got %q", name)
	}
}

func TestNewFlashCommand(t *testing.T) {
	cmd := NewFlashCommand()
	if cmd == nil {
		t.Fatal("NewFlashCommand returned nil")
	}
}

// --- parseArgs tests ---

func TestFlashParseArgsMissingRoot(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 1000 }
	defer func() { getEuid = origEuid }()

	cmd := NewFlashCommand()
	err := cmd.parseArgs([]string{})
	if err == nil {
		t.Fatal("Expected error for non-root, got nil")
	}
	if !strings.Contains(err.Error(), "must be run as root") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestFlashParseArgsDefaults(t *testing.T) {
	withRootEuid(t)

	cmd := NewFlashCommand()
	if err := cmd.parseArgs([]string{}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	if cmd.batch {
		t.Error("Expected batch false by default")
	}
	if cmd.dryRun {
		t.Error("Expected dryRun false by default")
	}
	if cmd.ref != "" {
		t.Errorf("Expected empty ref, got %q", cmd.ref)
	}
}

func TestFlashParseArgsAllFlags(t *testing.T) {
	withRootEuid(t)

	cmd := NewFlashCommand()
	if err := cmd.parseArgs([]string{
		"--batch",
		"--dry-run",
		"--ref", "matrixos/x86_64/dev/gnome",
		"--ostree-repo", "/tmp/repo",
		"--install-device", "/dev/sda",
	}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"batch", cmd.batch, true},
		{"dryRun", cmd.dryRun, true},
		{"ref", cmd.ref, "matrixos/x86_64/dev/gnome"},
		{"repoDir", cmd.repoDir, "/tmp/repo"},
		{"wholeDevice", cmd.wholeDevice, "/dev/sda"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestFlashParseArgsBatchShortFlag(t *testing.T) {
	withRootEuid(t)

	cmd := NewFlashCommand()
	if err := cmd.parseArgs([]string{"-b"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}
	if !cmd.batch {
		t.Error("Expected -b to set batch mode")
	}
}

// --- resolveRef tests ---

func TestFlashResolveRefMissingPair(t *testing.T) {
	withRootEuid(t)

	ot := &ostree.MockOstree{}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--ref", "matrixos/x86_64/dev/gnome"})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}

	_, err = cmd.resolveRef()
	if err == nil {
		t.Fatal("Expected error when --ref without --ostree-repo")
	}
	if !strings.Contains(err.Error(), "specify both") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestFlashResolveBootedRefEmpty(t *testing.T) {
	withRootEuid(t)

	ot := &ostree.MockOstree{}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg, []string{})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}

	// MockOstree.BootedRef returns "" by default.
	_, err = cmd.resolveRef()
	if err == nil {
		t.Fatal("Expected error for empty booted ref")
	}
	if !strings.Contains(err.Error(), "unable to find booted ostree ref") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestFlashResolveExplicitRefNotFound(t *testing.T) {
	withRootEuid(t)

	repoDir := t.TempDir()
	ot := &ostree.MockOstree{
		LocalRefs_: []string{"matrixos/x86_64/dev/cosmic"},
	}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--ref", "matrixos/x86_64/dev/gnome", "--ostree-repo", repoDir})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}

	_, err = cmd.resolveRef()
	if err == nil {
		t.Fatal("Expected error for ref not in local repo")
	}
	if !strings.Contains(err.Error(), "not found in local repo") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestFlashResolveExplicitRefFound(t *testing.T) {
	withRootEuid(t)

	repoDir := t.TempDir()
	ot := &ostree.MockOstree{
		LocalRefs_: []string{"matrixos/x86_64/dev/gnome"},
	}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--ref", "matrixos/x86_64/dev/gnome", "--ostree-repo", repoDir})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}
	cmd.SetupPrinters("flash")

	ref, err := cmd.resolveRef()
	if err != nil {
		t.Fatalf("resolveRef failed: %v", err)
	}
	if ref != "matrixos/x86_64/dev/gnome" {
		t.Errorf("Expected ref 'matrixos/x86_64/dev/gnome', got %q", ref)
	}
}

// --- validateDeviceExistence tests ---

func TestValidateDeviceExistenceOK(t *testing.T) {
	opts := &imager.BuildOptions{}
	if err := validateDeviceExistence(opts); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestValidateDeviceExistenceNonExistent(t *testing.T) {
	opts := &imager.BuildOptions{
		EfiDevice: "/dev/nonexistent-xyz-999",
	}
	err := validateDeviceExistence(opts)
	if err == nil {
		t.Fatal("Expected error for non-existent device")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- validateDeviceCombination tests ---

func TestValidateDeviceCombinationPartialError(t *testing.T) {
	withRootEuid(t)

	cmd := NewFlashCommand()
	cmd.StartUI()
	opts := &imager.BuildOptions{
		EfiDevice: "/dev/sda1",
	}
	err := cmd.validateDeviceCombination(opts)
	if err == nil {
		t.Fatal("Expected error for partial partitions")
	}
	if !strings.Contains(err.Error(), "please specify all") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateDeviceCombinationConflict(t *testing.T) {
	withRootEuid(t)

	cmd := NewFlashCommand()
	cmd.StartUI()
	opts := &imager.BuildOptions{
		WholeDevice: "/dev/sda",
		EfiDevice:   "/dev/sda1",
		BootDevice:  "/dev/sda2",
		RootDevice:  "/dev/sda3",
	}
	err := cmd.validateDeviceCombination(opts)
	if err == nil {
		t.Fatal("Expected error for conflicting whole + partitions")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateDeviceCombinationBatchNoDevices(t *testing.T) {
	withRootEuid(t)

	cmd := NewFlashCommand()
	cmd.batch = true
	cmd.StartUI()
	opts := &imager.BuildOptions{}
	err := cmd.validateDeviceCombination(opts)
	if err == nil {
		t.Fatal("Expected error for batch mode without devices")
	}
	if !strings.Contains(err.Error(), "batch mode requires") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateDeviceCombinationAllPartitionsOK(t *testing.T) {
	withRootEuid(t)

	cmd := NewFlashCommand()
	cmd.StartUI()
	opts := &imager.BuildOptions{
		EfiDevice:  "/dev/sda1",
		BootDevice: "/dev/sda2",
		RootDevice: "/dev/sda3",
	}
	if err := cmd.validateDeviceCombination(opts); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

func TestValidateDeviceCombinationWholeDeviceOK(t *testing.T) {
	withRootEuid(t)

	cmd := NewFlashCommand()
	cmd.StartUI()
	opts := &imager.BuildOptions{
		WholeDevice: "/dev/sda",
	}
	if err := cmd.validateDeviceCombination(opts); err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
}

// --- resolveDevicesBatch tests ---

func TestResolveDevicesBatchSummary(t *testing.T) {
	withRootEuid(t)
	withNoSleep(t)

	ot := &ostree.MockOstree{}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--batch", "--dry-run", "--install-device", "/dev/null"})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}
	cmd.SetupPrinters("flash")

	opts := &imager.BuildOptions{WholeDevice: "/dev/null"}
	result, err := cmd.resolveDevicesBatch(opts)
	if err != nil {
		t.Fatalf("resolveDevicesBatch failed: %v", err)
	}
	if result.WholeDevice != "/dev/null" {
		t.Errorf("Expected whole device /dev/null, got %q", result.WholeDevice)
	}
}

// --- showSummary tests ---

func TestShowSummaryWholeDevice(t *testing.T) {
	withRootEuid(t)

	ot := &ostree.MockOstree{}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg, []string{})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}
	cmd.SetupPrinters("flash")

	opts := &imager.BuildOptions{WholeDevice: "/dev/sda"}
	// Just ensure no panic.
	cmd.showSummary(opts)
}

func TestShowSummaryPartitions(t *testing.T) {
	withRootEuid(t)

	ot := &ostree.MockOstree{}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg, []string{})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}
	cmd.SetupPrinters("flash")

	opts := &imager.BuildOptions{
		EfiDevice:  "/dev/sda1",
		BootDevice: "/dev/sda2",
		RootDevice: "/dev/sda3",
	}
	// Just ensure no panic.
	cmd.showSummary(opts)
}

// --- resolveDevices with real temp files ---

func TestResolveDevicesBatchWholeDevice(t *testing.T) {
	withRootEuid(t)
	withNoSleep(t)

	tmpFile, err := os.CreateTemp("", "flash-dev")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	ot := &ostree.MockOstree{}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--batch", "--install-device", tmpFile.Name()})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}
	cmd.SetupPrinters("flash")

	opts, err := cmd.resolveDevices()
	if err != nil {
		t.Fatalf("resolveDevices failed: %v", err)
	}
	if opts.WholeDevice != tmpFile.Name() {
		t.Errorf("Expected %q, got %q", tmpFile.Name(), opts.WholeDevice)
	}
}

func TestResolveDevicesBatchAllPartitions(t *testing.T) {
	withRootEuid(t)
	withNoSleep(t)

	efiFile, _ := os.CreateTemp("", "efi-dev")
	bootFile, _ := os.CreateTemp("", "boot-dev")
	rootFile, _ := os.CreateTemp("", "root-dev")
	defer os.Remove(efiFile.Name())
	defer os.Remove(bootFile.Name())
	defer os.Remove(rootFile.Name())
	efiFile.Close()
	bootFile.Close()
	rootFile.Close()

	ot := &ostree.MockOstree{}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{
			"--batch",
			"--efi-device-path", efiFile.Name(),
			"--boot-device-path", bootFile.Name(),
			"--root-device-path", rootFile.Name(),
		})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}
	cmd.SetupPrinters("flash")

	opts, err := cmd.resolveDevices()
	if err != nil {
		t.Fatalf("resolveDevices failed: %v", err)
	}
	if opts.EfiDevice != efiFile.Name() || opts.BootDevice != bootFile.Name() || opts.RootDevice != rootFile.Name() {
		t.Error("Device paths not returned correctly")
	}
}

func TestResolveDevicesNonExistent(t *testing.T) {
	withRootEuid(t)

	ot := &ostree.MockOstree{}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--batch", "--install-device", "/dev/nonexistent-flash-test-xyz"})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}
	cmd.SetupPrinters("flash")

	_, err = cmd.resolveDevices()
	if err == nil {
		t.Fatal("Expected error for non-existent device")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- resolveExplicitRef with shortname expansion ---

func TestFlashResolveExplicitRefShortname(t *testing.T) {
	withRootEuid(t)

	repoDir := t.TempDir()
	ot := &ostree.MockOstree{
		LocalRefs_: []string{"matrixos/x86_64/dev/gnome"},
		OsName_:    "matrixos",
		Arch_:      "x86_64",
	}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--ref", "gnome", "--ostree-repo", repoDir})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}
	cmd.SetupPrinters("flash")

	ref, err := cmd.resolveRef()
	if err != nil {
		t.Fatalf("resolveRef failed: %v", err)
	}
	if ref != "matrixos/x86_64/dev/gnome" {
		t.Errorf("Expected expanded ref, got %q", ref)
	}
}

// --- resolveDevicesInteractive with cancelled user ---

func TestResolveDevicesInteractiveCancelled(t *testing.T) {
	withRootEuid(t)
	withNoSleep(t)

	ot := &ostree.MockOstree{}
	cfg := defaultFlashTestConfig()

	cmd, err := newTestFlashCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{})
	if err != nil {
		t.Fatalf("newTestFlashCommand failed: %v", err)
	}
	cmd.SetupPrinters("flash")

	// Provide an input that triggers "no" for whole device, then EOF which
	// causes AskInput to return the default "" for the partition prompt,
	// which fails because devPathPattern doesn't match "".
	input := strings.NewReader("no\n")
	cmd.prompter = NewPrompter(input, os.Stdout, os.Stderr, &cmd.UI)

	opts := &imager.BuildOptions{}
	_, err = cmd.resolveDevicesInteractive(opts)
	// This will fail because we can't provide valid partitions via EOF.
	if err == nil {
		t.Fatal("Expected error from interactive with insufficient input")
	}
}
