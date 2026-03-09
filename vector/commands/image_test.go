package commands

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/imager"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/validation"
)

// --- Test helpers ---

// defaultMockOstree returns a MockOstree with sensible defaults for image tests.
func defaultMockOstree() *ostree.MockOstree {
	return &ostree.MockOstree{}
}

// newTestImageCommand creates an ImageCommand with injected mocks,
// bypassing Init() which requires real config, ostree binary, etc.
func newTestImageCommand(
	ot ostree.IOstree,
	im *imager.MockImager,
	fsenc *filesystems.MockFsenc,
	cfg *config.MockConfig,
	args []string,
) (*ImageCommand, error) {
	cmd := NewImageCommand()
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

func defaultImageTestConfig() *config.MockConfig {
	return &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Sysroot":          {"/sysroot"},
			"Ostree.FullBranchSuffix": {"full"},
			"matrixOS.OsName":         {"matrixos"},
			"Imager.BootRoot":         {"/boot"},
			"Imager.EfiRoot":          {"/efi"},
			"Imager.Compressor":       {"xz"},
		},
	}
}

// --- Tests ---

func TestImageName(t *testing.T) {
	cmd := NewImageCommand()
	if name := cmd.Name(); name != "image" {
		t.Errorf("Expected name 'image', got %q", name)
	}
}

func TestNewImageCommand(t *testing.T) {
	cmd := NewImageCommand()
	if cmd == nil {
		t.Fatal("NewImageCommand returned nil")
	}
	if cmd.Name() != "image" {
		t.Errorf("Expected name 'image', got %q", cmd.Name())
	}
}

func TestImageParseArgsMissingRef(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewImageCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{})
	if err == nil {
		t.Fatal("Expected error for missing --ref, got nil")
	}
	if !strings.Contains(err.Error(), "--ref is required") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestImageParseArgs(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewImageCommand()
	cmd.StartUI()
	if err := cmd.parseArgs([]string{
		"--ref", "myref",
		"--local-ostree",
		"--install-device", "/dev/sda",
		"--verbose",
	}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"ref", cmd.ref, "myref"},
		{"localOstree", cmd.localOstree, true},
		{"wholeDevice", cmd.wholeDevice, "/dev/sda"},
		{"verbose", cmd.verbose, true},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}
}

func TestImageParseArgsDefaults(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewImageCommand()
	cmd.StartUI()
	if err := cmd.parseArgs([]string{"--ref=foo"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	if cmd.ref != "foo" {
		t.Errorf("Expected ref 'foo', got %q", cmd.ref)
	}
	if cmd.localOstree {
		t.Error("Expected localOstree false")
	}
	if cmd.verbose {
		t.Error("Expected verbose false")
	}
}

func TestImageParseArgsNotRoot(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 1000 }
	defer func() { getEuid = origEuid }()

	cmd := NewImageCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{"--ref", "mybranch"})
	if err == nil {
		t.Fatal("Expected error for non-root, got nil")
	}
	if !strings.Contains(err.Error(), "must be run as root") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// requireImagerTools skips the test if the host does not have
// the executables that VerifyImagerEnvironmentSetup checks for.
func requireImagerTools(t *testing.T) {
	t.Helper()
	for _, tool := range []string{
		"blockdev", "btrfs", "chroot", "cryptsetup", "efibootmgr",
		"findmnt", "fstrim", "gpg", "grub-install", "losetup",
		"mkfs.vfat", "mkfs.btrfs", "openssl", "ostree", "partprobe",
		"qemu-img", "qemu-system-x86_64", "sgdisk", "unshare", "udevadm", "xz",
	} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("skipping: required tool %q not found in PATH", tool)
		}
	}
}

func TestImageRunLuksValidationFail(t *testing.T) {
	requireImagerTools(t)

	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := defaultMockOstree()
	im := imager.DefaultMockImager()
	fsenc := filesystems.DefaultMockFsenc()
	// Enable encryption in config but omit the encryption key so that
	// the real Fsenc created by runImage fails ValidateLuksVariables.
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Sysroot":          {"/sysroot"},
			"Ostree.FullBranchSuffix": {"full"},
			"matrixOS.OsName":         {"matrixos"},
			"Imager.BootRoot":         {"/boot"},
			"Imager.EfiRoot":          {"/efi"},
		},
		Bools: map[string]bool{
			"Imager.Encryption": true,
		},
	}

	// Use a full branch name (with slashes) to pass IsBranchShortName check.
	cmd, err := newTestImageCommand(ot, im, fsenc, cfg, []string{"--ref", "matrixos/x86_64/dev/mybranch"})
	if err != nil {
		t.Fatalf("newTestImageCommand failed: %v", err)
	}

	err = cmd.Run()
	if err == nil {
		t.Fatal("Expected LUKS validation error, got nil")
	}
	if !strings.Contains(err.Error(), "LUKS validation failed") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- validateDevicePaths tests ---

func TestValidateDevicePathsNone(t *testing.T) {
	cmd := NewImageCommand()
	opts, err := cmd.validateDevicePaths()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if opts.EfiDevice != "" || opts.BootDevice != "" || opts.RootDevice != "" || opts.WholeDevice != "" {
		t.Errorf("Expected all empty, got efi=%q boot=%q root=%q whole=%q",
			opts.EfiDevice, opts.BootDevice, opts.RootDevice, opts.WholeDevice)
	}
}

func TestValidateDevicePathsPartialError(t *testing.T) {
	// Create a temp file to simulate an existing device.
	tmpFile, err := os.CreateTemp("", "dev-path")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	cmd := NewImageCommand()
	cmd.efiDevicePath = tmpFile.Name()
	cmd.bootDevicePath = ""
	cmd.rootDevicePath = ""

	_, err = cmd.validateDevicePaths()
	if err == nil {
		t.Fatal("Expected error for partial device paths, got nil")
	}
	if !strings.Contains(err.Error(), "please specify all") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateDevicePathsAllThree(t *testing.T) {
	// Create temp files to simulate existing devices.
	efiFile, _ := os.CreateTemp("", "efi-dev")
	bootFile, _ := os.CreateTemp("", "boot-dev")
	rootFile, _ := os.CreateTemp("", "root-dev")
	defer os.Remove(efiFile.Name())
	defer os.Remove(bootFile.Name())
	defer os.Remove(rootFile.Name())
	efiFile.Close()
	bootFile.Close()
	rootFile.Close()

	cmd := NewImageCommand()
	cmd.efiDevicePath = efiFile.Name()
	cmd.bootDevicePath = bootFile.Name()
	cmd.rootDevicePath = rootFile.Name()

	opts, err := cmd.validateDevicePaths()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if opts.EfiDevice != efiFile.Name() || opts.BootDevice != bootFile.Name() || opts.RootDevice != rootFile.Name() {
		t.Error("Device paths not returned correctly")
	}
	if opts.WholeDevice != "" {
		t.Errorf("Expected empty whole device, got %q", opts.WholeDevice)
	}
}

func TestValidateDevicePathsWholeAndPartitionsConflict(t *testing.T) {
	efiFile, _ := os.CreateTemp("", "efi-dev")
	bootFile, _ := os.CreateTemp("", "boot-dev")
	rootFile, _ := os.CreateTemp("", "root-dev")
	wholeFile, _ := os.CreateTemp("", "whole-dev")
	defer os.Remove(efiFile.Name())
	defer os.Remove(bootFile.Name())
	defer os.Remove(rootFile.Name())
	defer os.Remove(wholeFile.Name())
	efiFile.Close()
	bootFile.Close()
	rootFile.Close()
	wholeFile.Close()

	cmd := NewImageCommand()
	cmd.efiDevicePath = efiFile.Name()
	cmd.bootDevicePath = bootFile.Name()
	cmd.rootDevicePath = rootFile.Name()
	cmd.wholeDevice = wholeFile.Name()

	_, err := cmd.validateDevicePaths()
	if err == nil {
		t.Fatal("Expected error for conflicting device flags, got nil")
	}
	if !strings.Contains(err.Error(), "not both") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestValidateDevicePathsWholeDeviceOnly(t *testing.T) {
	wholeFile, _ := os.CreateTemp("", "whole-dev")
	defer os.Remove(wholeFile.Name())
	wholeFile.Close()

	cmd := NewImageCommand()
	cmd.wholeDevice = wholeFile.Name()

	paths, err := cmd.validateDevicePaths()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if paths.WholeDevice != wholeFile.Name() {
		t.Errorf("Expected whole=%q, got %q", wholeFile.Name(), paths.WholeDevice)
	}
	if paths.EfiDevice != "" || paths.BootDevice != "" || paths.RootDevice != "" {
		t.Error("Expected partition paths empty for whole-device mode")
	}
}

func TestValidateDevicePathsNonExistentDevice(t *testing.T) {
	cmd := NewImageCommand()
	cmd.efiDevicePath = "/dev/nonexistent-test-xyz-999"

	_, err := cmd.validateDevicePaths()
	if err == nil {
		t.Fatal("Expected error for non-existent device, got nil")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- initializeOstree tests ---

func TestInitializeOstreeRemote(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	mock := &ostree.MockOstree{
		Refs:    []string{"remote:branch1"},
		Remote_: "remote",
	}
	mock.SetRef("mybranch")
	cfg := defaultImageTestConfig()

	cmd, err := newTestImageCommand(
		mock, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--ref", "matrixos/x86_64/dev/someref"},
	)
	if err != nil {
		t.Fatalf("newTestImageCommand failed: %v", err)
	}

	output, err := runCaptureStdout(func() error {
		return cmd.initializeRemoteOstree()
	})
	if err != nil {
		t.Fatalf("initializeRemoteOstree failed: %v", err)
	}

	plain := stripAnsi(output)
	if !strings.Contains(plain, "Remote refs:") {
		t.Errorf("Expected 'Remote refs:' in output, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Pulling ostree ref remote:mybranch") {
		t.Errorf("Expected pull message in output, got:\n%s", plain)
	}
}

// --- Run() tests ---

func TestImageRunShortNameRefRejected(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := defaultMockOstree()
	cfg := defaultImageTestConfig()

	// Use a branch shortname (no slashes — IsBranchShortName returns true).
	cmd, err := newTestImageCommand(
		ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--ref", "mybranch"},
	)
	if err != nil {
		t.Fatalf("newTestImageCommand failed: %v", err)
	}

	// Short branch names are now rejected with a hard error.
	err = cmd.Run()
	if err == nil {
		t.Fatal("Expected error for branch shortname, got nil")
	}
	if !strings.Contains(err.Error(), "specify a complete branch name") {
		t.Errorf("Expected 'specify a complete branch name' error, got: %v", err)
	}
}

// --- Ref with remote extraction test ---

func TestExtractRemoteFromRefIntegration(t *testing.T) {
	// Verify that ostree.ExtractRemoteFromRef works as expected.
	remote := ostree.ExtractRemoteFromRef("origin:matrixos/x86_64/dev/mybranch")
	if remote != "origin" {
		t.Errorf("Expected 'origin', got %q", remote)
	}

	remote = ostree.ExtractRemoteFromRef("matrixos/x86_64/dev/mybranch")
	if remote != "" {
		t.Errorf("Expected empty, got %q", remote)
	}
}

func TestCleanRemoteFromRefIntegration(t *testing.T) {
	cleaned := ostree.CleanRemoteFromRef("origin:matrixos/x86_64/dev/mybranch")
	if cleaned != "matrixos/x86_64/dev/mybranch" {
		t.Errorf("Expected ref without remote, got %q", cleaned)
	}

	cleaned = ostree.CleanRemoteFromRef("matrixos/x86_64/dev/mybranch")
	if cleaned != "matrixos/x86_64/dev/mybranch" {
		t.Errorf("Expected same ref, got %q", cleaned)
	}
}

// --- detectRemotedAndPlainRefs call-site tests ---

func TestImageDetectRemotedAndPlainRefs_Clean(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := &ostree.MockOstree{
		LocalRefs_: []string{
			"matrixos/x86_64/dev/gnome",
			"matrixos/x86_64/dev/cosmic",
		},
	}
	cfg := defaultImageTestConfig()
	cmd, err := newTestImageCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--ref", "matrixos/x86_64/dev/gnome"})
	if err != nil {
		t.Fatalf("newTestImageCommand failed: %v", err)
	}

	// No duplicates — should pass.
	if err := cmd.detectRemotedAndPlainRefs(cmd.im.PrintError); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestImageDetectRemotedAndPlainRefs_Ambiguous(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := &ostree.MockOstree{
		LocalRefs_: []string{
			"origin:matrixos/x86_64/dev/gnome",
			"matrixos/x86_64/dev/gnome",
		},
	}
	cfg := defaultImageTestConfig()
	cmd, err := newTestImageCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--ref", "matrixos/x86_64/dev/gnome"})
	if err != nil {
		t.Fatalf("newTestImageCommand failed: %v", err)
	}

	err = cmd.detectRemotedAndPlainRefs(cmd.im.PrintError)
	if err == nil {
		t.Fatal("expected error for ambiguous refs, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous refs detected") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestImageDetectRemotedAndPlainRefs_LocalRefsError(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	ot := &ostree.MockOstree{
		LocalRefsErr: fmt.Errorf("localrefs failed"),
	}
	cfg := defaultImageTestConfig()
	cmd, err := newTestImageCommand(ot, imager.DefaultMockImager(), filesystems.DefaultMockFsenc(), cfg,
		[]string{"--ref", "matrixos/x86_64/dev/gnome"})
	if err != nil {
		t.Fatalf("newTestImageCommand failed: %v", err)
	}

	err = cmd.detectRemotedAndPlainRefs(cmd.im.PrintError)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list local refs") {
		t.Errorf("unexpected error: %v", err)
	}
}
