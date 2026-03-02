package releaser

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/validation"
)

// ---------------------------------------------------------------------------
// parseServicesFile
// ---------------------------------------------------------------------------

func TestParseServicesFile_HappyPath(t *testing.T) {
	f := filepath.Join(t.TempDir(), "svc.conf")
	content := `# comment line
enable sshd.service NetworkManager.service
disable bluetooth.service
mask cups.service
preset-enable pipewire.service
preset-disable tracker.service
preset-mask avahi.service
set-default graphical.target

`
	os.WriteFile(f, []byte(content), 0o644)

	actions, err := parseServicesFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := []serviceAction{
		{action: "enable", services: []string{"sshd.service", "NetworkManager.service"}},
		{action: "disable", services: []string{"bluetooth.service"}},
		{action: "mask", services: []string{"cups.service"}},
		{action: "preset-enable", services: []string{"pipewire.service"}},
		{action: "preset-disable", services: []string{"tracker.service"}},
		{action: "preset-mask", services: []string{"avahi.service"}},
		{action: "set-default", services: []string{"graphical.target"}},
	}

	if len(actions) != len(want) {
		t.Fatalf("got %d actions, want %d", len(actions), len(want))
	}
	for i, w := range want {
		a := actions[i]
		if a.action != w.action {
			t.Errorf("actions[%d].action = %q, want %q", i, a.action, w.action)
		}
		if len(a.services) != len(w.services) {
			t.Errorf("actions[%d].services len = %d, want %d", i, len(a.services), len(w.services))
			continue
		}
		for j := range w.services {
			if a.services[j] != w.services[j] {
				t.Errorf("actions[%d].services[%d] = %q, want %q", i, j, a.services[j], w.services[j])
			}
		}
	}
}

func TestParseServicesFile_SkipsBlanksAndComments(t *testing.T) {
	f := filepath.Join(t.TempDir(), "svc.conf")
	content := `
# full-line comment
   # indented comment

enable foo.service
`
	os.WriteFile(f, []byte(content), 0o644)

	actions, err := parseServicesFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(actions))
	}
	if actions[0].action != "enable" {
		t.Errorf("action = %q, want %q", actions[0].action, "enable")
	}
}

func TestParseServicesFile_SkipsSingleFieldLines(t *testing.T) {
	f := filepath.Join(t.TempDir(), "svc.conf")
	// A line with only an action keyword and no services should be skipped.
	content := "enable\n"
	os.WriteFile(f, []byte(content), 0o644)

	actions, err := parseServicesFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Fatalf("got %d actions, want 0 (single-field line should be skipped)", len(actions))
	}
}

func TestParseServicesFile_NonexistentFile(t *testing.T) {
	_, err := parseServicesFile("/nonexistent/file.conf")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestParseServicesFile_EmptyFile(t *testing.T) {
	f := filepath.Join(t.TempDir(), "empty.conf")
	os.WriteFile(f, []byte(""), 0o644)

	actions, err := parseServicesFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 0 {
		t.Errorf("got %d actions, want 0", len(actions))
	}
}

func TestParseServicesFile_MultipleServicesPerLine(t *testing.T) {
	f := filepath.Join(t.TempDir(), "svc.conf")
	content := "enable a.service b.service c.service\n"
	os.WriteFile(f, []byte(content), 0o644)

	actions, err := parseServicesFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(actions))
	}
	if len(actions[0].services) != 3 {
		t.Errorf("got %d services, want 3", len(actions[0].services))
	}
}

func TestParseServicesFile_WhitespaceVariations(t *testing.T) {
	f := filepath.Join(t.TempDir(), "svc.conf")
	content := "  enable   foo.service   bar.service  \n"
	os.WriteFile(f, []byte(content), 0o644)

	actions, err := parseServicesFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("got %d actions, want 1", len(actions))
	}
	if actions[0].action != "enable" {
		t.Errorf("action = %q, want %q", actions[0].action, "enable")
	}
	if len(actions[0].services) != 2 {
		t.Errorf("got %d services, want 2", len(actions[0].services))
	}
}

// ---------------------------------------------------------------------------
// SetupHostname
// ---------------------------------------------------------------------------

func TestSetupHostname_HappyPath(t *testing.T) {
	imageDir := t.TempDir()
	os.MkdirAll(filepath.Join(imageDir, "etc"), 0o755)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.Hostname": {"myhost"},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: imageDir,
	}

	if err := r.SetupHostname(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(imageDir, "etc/hostname"))
	if err != nil {
		t.Fatalf("failed to read hostname file: %v", err)
	}
	if string(data) != "myhost\n" {
		t.Errorf("hostname = %q, want %q", data, "myhost\n")
	}

	out := r.stdout.(*bytes.Buffer).String()
	if out != "Setting hostname to: myhost\n" {
		t.Errorf("stdout = %q, want %q", out, "Setting hostname to: myhost\n")
	}
}

func TestSetupHostname_HostnameConfigError(t *testing.T) {
	// Missing Releaser.Hostname key → error.
	cfg := &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
	}

	if err := r.SetupHostname(); err == nil {
		t.Fatal("expected error for missing Hostname config, got nil")
	}
}

func TestSetupHostname_ErrConfigPropagates(t *testing.T) {
	wantErr := errors.New("cfg broken")
	cfg := &config.ErrConfig{Err: wantErr}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
	}

	err := r.SetupHostname()
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want %v", err, wantErr)
	}
}

func TestSetupHostname_WriteFailsNoEtcDir(t *testing.T) {
	// imageDir has no etc/ → WriteFile fails.
	imageDir := t.TempDir()
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.Hostname": {"host"},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: imageDir,
	}

	if err := r.SetupHostname(); err == nil {
		t.Fatal("expected error when etc/ dir doesn't exist, got nil")
	}
}

func TestSetupHostname_OverwritesExistingFile(t *testing.T) {
	imageDir := t.TempDir()
	os.MkdirAll(filepath.Join(imageDir, "etc"), 0o755)
	os.WriteFile(filepath.Join(imageDir, "etc/hostname"), []byte("old\n"), 0o644)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.Hostname": {"newhost"},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: imageDir,
	}

	if err := r.SetupHostname(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(imageDir, "etc/hostname"))
	if string(data) != "newhost\n" {
		t.Errorf("hostname = %q, want %q", data, "newhost\n")
	}
}

// ---------------------------------------------------------------------------
// SetupServices
// ---------------------------------------------------------------------------

func TestSetupServices_HooksDirError(t *testing.T) {
	// Missing Releaser.HooksDir → error.
	cfg := &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "origin/matrixos"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "origin/matrixos",
	}

	if err := r.SetupServices(); err == nil {
		t.Fatal("expected error for missing HooksDir config, got nil")
	}
}

func TestSetupServices_NoServicesFile_SkipsGracefully(t *testing.T) {
	hooksDir := t.TempDir()
	// No .conf file exists → should print warning and return nil.
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.HooksDir": {hooksDir},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "myref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "myref",
	}

	if err := r.SetupServices(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	warn := r.stderr.(*bytes.Buffer).String()
	if warn == "" {
		t.Error("expected a warning about missing services file")
	}
}

func TestSetupServices_FallbackToServicesSubdir(t *testing.T) {
	mockMountSyscalls(t)

	// Primary path doesn't exist, but the fallback (parent/services/ref.conf) does.
	baseDir := t.TempDir()
	hooksDir := filepath.Join(baseDir, "hooks")
	os.MkdirAll(hooksDir, 0o755)

	servicesDir := filepath.Join(baseDir, "services")
	os.MkdirAll(servicesDir, 0o755)
	// Create a minimal (empty) services file at the fallback location.
	svcFile := filepath.Join(servicesDir, "myref.conf")
	os.WriteFile(svcFile, []byte("# empty\n"), 0o644)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.HooksDir": {hooksDir},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "myref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "myref",
	}

	// SetupServices will find the fallback file and parse it, then
	// set up (mocked) chroot mounts and run (mocked) systemctl.
	err := r.SetupServices()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify no "does not exist" warning was emitted.
	warn := r.stderr.(*bytes.Buffer).String()
	if bytes.Contains([]byte(warn), []byte("does not exist")) {
		t.Error("fallback path was not tried; got 'does not exist' warning")
	}
}

func TestSetupServices_ParseError(t *testing.T) {
	hooksDir := t.TempDir()
	// Write a line longer than bufio.MaxScanTokenSize (64 KiB) so the
	// scanner returns bufio.ErrTooLong. This triggers the parse-error
	// path regardless of uid (no permission tricks needed).
	svcFile := filepath.Join(hooksDir, "myref.conf")
	longLine := make([]byte, 64*1024+1)
	for i := range longLine {
		longLine[i] = 'x'
	}
	longLine[len(longLine)-1] = '\n'
	os.WriteFile(svcFile, longLine, 0o644)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.HooksDir": {hooksDir},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "myref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "myref",
	}

	err := r.SetupServices()
	if err == nil {
		t.Fatal("expected parse error from oversized line, got nil")
	}
}

func TestSetupServices_MountSetupFailure(t *testing.T) {
	// Mock Mount to fail so Setup returns an error.
	origMount := filesystems.Mount
	filesystems.Mount = func(source, target, fstype string, flags uintptr, data string) error {
		return fmt.Errorf("mock mount failure")
	}
	t.Cleanup(func() { filesystems.Mount = origMount })

	hooksDir := t.TempDir()
	svcFile := filepath.Join(hooksDir, "myref.conf")
	os.WriteFile(svcFile, []byte("enable sshd.service\n"), 0o644)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.HooksDir": {hooksDir},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "myref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "myref",
	}

	err := r.SetupServices()
	if err == nil {
		t.Fatal("expected error from mount Setup, got nil")
	}
}

func TestSetupServices_ErrConfigPropagates(t *testing.T) {
	wantErr := errors.New("cfg broken")
	cfg := &config.ErrConfig{Err: wantErr}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "ref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "ref",
	}

	err := r.SetupServices()
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want %v", err, wantErr)
	}
}

// ---------------------------------------------------------------------------
// ReleaseHook
// ---------------------------------------------------------------------------

func TestReleaseHook_HooksDirError(t *testing.T) {
	cfg := &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "ref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "ref",
	}

	if err := r.ReleaseHook(); err == nil {
		t.Fatal("expected error for missing HooksDir config, got nil")
	}
}

func TestReleaseHook_NoHookFile_SkipsGracefully(t *testing.T) {
	devDir := t.TempDir()
	hooksDir := filepath.Join(devDir, "release", "hooks")
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.HooksDir": {hooksDir},
			"matrixOS.Root":     {devDir},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "myref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "myref",
	}

	if err := r.ReleaseHook(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	warn := r.stderr.(*bytes.Buffer).String()
	if warn == "" {
		t.Error("expected a warning about missing hook file")
	}
}

func TestReleaseHook_ExecutesHookScript(t *testing.T) {
	devDir := t.TempDir()
	hooksDir := filepath.Join(devDir, "release", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}
	imageDir := t.TempDir()
	hookFile := filepath.Join(hooksDir, "myref.sh")
	markerFile := filepath.Join(t.TempDir(), "marker")

	// Write a script that creates a marker file with $CHROOT_DIR content.
	script := "#!/bin/sh\necho \"$CHROOT_DIR\" > " + markerFile + "\n"
	os.WriteFile(hookFile, []byte(script), 0o755)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.HooksDir": {hooksDir},
			"matrixOS.Root":     {devDir},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "myref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: imageDir,
		ref:      "myref",
	}

	if err := r.ReleaseHook(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(markerFile)
	if err != nil {
		t.Fatalf("marker file not created: %v", err)
	}
	got := string(bytes.TrimSpace(data))
	if got != imageDir {
		t.Errorf("CHROOT_DIR = %q, want %q", got, imageDir)
	}

	out := r.stdout.(*bytes.Buffer).String()
	want := "Running release hook " + hookFile + " ...\n"
	if out != want {
		t.Errorf("stdout = %q, want %q", out, want)
	}
}

func TestReleaseHook_HookScriptFails(t *testing.T) {
	devDir := t.TempDir()
	hooksDir := filepath.Join(devDir, "release", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}
	hookFile := filepath.Join(hooksDir, "myref.sh")
	os.WriteFile(hookFile, []byte("#!/bin/sh\nexit 1\n"), 0o755)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.HooksDir": {hooksDir},
			"matrixOS.Root":     {devDir},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "myref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "myref",
	}

	err := r.ReleaseHook()
	if err == nil {
		t.Fatal("expected error when hook exits non-zero, got nil")
	}
}

func TestReleaseHook_HookScriptStdout(t *testing.T) {
	devDir := t.TempDir()
	hooksDir := filepath.Join(devDir, "release", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}
	hookFile := filepath.Join(hooksDir, "myref.sh")
	os.WriteFile(hookFile, []byte("#!/bin/sh\necho hello\n"), 0o755)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.HooksDir": {hooksDir},
			"matrixOS.Root":     {devDir},
		},
		Bools: map[string]bool{},
	}
	var stdout, stderr bytes.Buffer
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "myref"},
		stdout:   &stdout,
		stderr:   &stderr,
		imageDir: t.TempDir(),
		ref:      "myref",
	}

	if err := r.ReleaseHook(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := stdout.String()
	// Output should contain the initial print message AND "hello\n" from the script.
	if !bytes.Contains([]byte(out), []byte("hello\n")) {
		t.Errorf("stdout = %q, expected to contain %q", out, "hello\n")
	}
}

func TestReleaseHook_ErrConfigPropagates(t *testing.T) {
	wantErr := errors.New("cfg broken")
	cfg := &config.ErrConfig{Err: wantErr}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "ref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "ref",
	}

	err := r.ReleaseHook()
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want %v", err, wantErr)
	}
}

func TestReleaseHook_NonExecutableFile(t *testing.T) {
	devDir := t.TempDir()
	hooksDir := filepath.Join(devDir, "release", "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		t.Fatalf("failed to create hooks dir: %v", err)
	}
	hookFile := filepath.Join(hooksDir, "myref.sh")
	// Write script without execute permission.
	os.WriteFile(hookFile, []byte("#!/bin/sh\necho hi\n"), 0o644)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.HooksDir": {hooksDir},
			"matrixOS.Root":     {devDir},
		},
		Bools: map[string]bool{},
	}
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{Ref_: "myref"},
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: t.TempDir(),
		ref:      "myref",
	}

	// Even root cannot execute a file with zero execute bits (0o644).
	// CAP_DAC_OVERRIDE only grants execute when at least one x-bit is set.
	err := r.ReleaseHook()
	if err == nil {
		t.Fatal("expected permission error from exec, got nil")
	}
}

// ---------------------------------------------------------------------------
// serviceAction struct
// ---------------------------------------------------------------------------

func TestServiceAction_ZeroValue(t *testing.T) {
	var sa serviceAction
	if sa.action != "" {
		t.Errorf("expected empty action, got %q", sa.action)
	}
	if sa.services != nil {
		t.Errorf("expected nil services, got %v", sa.services)
	}
}

// ---------------------------------------------------------------------------
// SetupHostname + QA helper
// ---------------------------------------------------------------------------

func TestSetupHostname_WithQA(t *testing.T) {
	imageDir := t.TempDir()
	os.MkdirAll(filepath.Join(imageDir, "etc"), 0o755)

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Releaser.Hostname": {"qahost"},
		},
		Bools: map[string]bool{},
	}
	qa, _ := validation.New(cfg)
	r := &Releaser{
		cfg:      cfg,
		ostree:   &ostree.MockOstree{},
		qa:       qa,
		stdout:   &bytes.Buffer{},
		stderr:   &bytes.Buffer{},
		imageDir: imageDir,
	}

	if err := r.SetupHostname(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(imageDir, "etc/hostname"))
	if string(data) != "qahost\n" {
		t.Errorf("hostname = %q, want %q", data, "qahost\n")
	}
}
