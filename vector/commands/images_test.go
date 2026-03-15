package commands

import (
	"os"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/imager"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/validation"
)

// --- Test helpers ---

func defaultImagesTestConfig() *config.MockConfig {
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

// newTestImagesCommand creates an ImagesCommand with injected mocks,
// bypassing Init() which requires real config, ostree binary, etc.
func newTestImagesCommand(
	ot ostree.IOstree,
	cfg *config.MockConfig,
	args []string,
) (*ImagesCommand, error) {
	cmd := NewImagesCommand()
	cmd.ot = ot
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

// --- Tests ---

func TestImagesName(t *testing.T) {
	cmd := NewImagesCommand()
	if name := cmd.Name(); name != "images" {
		t.Errorf("Expected name 'images', got %q", name)
	}
}

func TestNewImagesCommand(t *testing.T) {
	cmd := NewImagesCommand()
	if cmd == nil {
		t.Fatal("NewImagesCommand returned nil")
	}
	if cmd.Name() != "images" {
		t.Errorf("Expected name 'images', got %q", cmd.Name())
	}
}

func TestImagesParseArgsDefaults(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewImagesCommand()
	cmd.StartUI()
	if err := cmd.parseArgs([]string{}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	if cmd.localOstree {
		t.Error("Expected localOstree false")
	}
	if cmd.includeFullBranches {
		t.Error("Expected includeFullBranches false")
	}
	if cmd.verbose {
		t.Error("Expected verbose false")
	}
	if len(cmd.skipReleases) != 0 {
		t.Errorf("Expected empty skipReleases, got %v", cmd.skipReleases)
	}
	if len(cmd.onlyReleases) != 0 {
		t.Errorf("Expected empty onlyReleases, got %v", cmd.onlyReleases)
	}
}

func TestImagesParseArgsFlags(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewImagesCommand()
	cmd.StartUI()
	if err := cmd.parseArgs([]string{
		"--local-ostree",
		"--include-full-branches",
		"--skip-releases", "matrixos/amd64/server,matrixos/amd64/dev/gnome",
		"--only-releases", "matrixos/amd64/bedrock",
		"--verbose",
	}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	checks := []struct {
		name string
		got  interface{}
		want interface{}
	}{
		{"localOstree", cmd.localOstree, true},
		{"includeFullBranches", cmd.includeFullBranches, true},
		{"verbose", cmd.verbose, true},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, c.got, c.want)
		}
	}

	wantSkip := []string{"matrixos/amd64/server", "matrixos/amd64/dev/gnome"}
	if len(cmd.skipReleases) != len(wantSkip) {
		t.Fatalf("skipReleases: got %v, want %v", cmd.skipReleases, wantSkip)
	}
	for i, v := range wantSkip {
		if cmd.skipReleases[i] != v {
			t.Errorf("skipReleases[%d]: got %q, want %q", i, cmd.skipReleases[i], v)
		}
	}

	wantOnly := []string{"matrixos/amd64/bedrock"}
	if len(cmd.onlyReleases) != len(wantOnly) {
		t.Fatalf("onlyReleases: got %v, want %v", cmd.onlyReleases, wantOnly)
	}
	for i, v := range wantOnly {
		if cmd.onlyReleases[i] != v {
			t.Errorf("onlyReleases[%d]: got %q, want %q", i, cmd.onlyReleases[i], v)
		}
	}
}

func TestImagesParseArgsNotRoot(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 1000 }
	defer func() { getEuid = origEuid }()

	cmd := NewImagesCommand()
	cmd.StartUI()
	err := cmd.parseArgs([]string{})
	if err == nil {
		t.Fatal("Expected error for non-root, got nil")
	}
	if !strings.Contains(err.Error(), "must be run as root") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- splitCSV tests ---

func TestSplitCSVEmpty(t *testing.T) {
	if got := SplitCSV(""); got != nil {
		t.Errorf("splitCSV(\"\") = %v, want nil", got)
	}
}

func TestSplitCSVSingle(t *testing.T) {
	got := SplitCSV("abc")
	if len(got) != 1 || got[0] != "abc" {
		t.Errorf("SplitCSV(\"abc\") = %v, want [abc]", got)
	}
}

func TestSplitCSVMultiple(t *testing.T) {
	got := SplitCSV("a, b ,c")
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV length: got %d, want %d", len(got), len(want))
	}
	for i, v := range want {
		if got[i] != v {
			t.Errorf("splitCSV[%d]: got %q, want %q", i, got[i], v)
		}
	}
}

func TestSplitCSVTrailingComma(t *testing.T) {
	got := SplitCSV("a,b,")
	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("SplitCSV length: got %d, want %d", len(got), len(want))
	}
}

// --- Filter tests ---

func TestSkipFilter(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewImagesCommand()
	cmd.StartUI()
	_ = cmd.parseArgs([]string{"--skip-releases", "ref1,ref3"})

	skip := cmd.skipFilter()
	if skip == nil {
		t.Fatal("Expected non-nil skip filter")
	}
	if !skip("ref1") {
		t.Error("Expected ref1 to be skipped")
	}
	if skip("ref2") {
		t.Error("Expected ref2 to NOT be skipped")
	}
	if !skip("ref3") {
		t.Error("Expected ref3 to be skipped")
	}
}

func TestSkipFilterNil(t *testing.T) {
	cmd := &ImagesCommand{}
	if cmd.skipFilter() != nil {
		t.Error("Expected nil skip filter when no skip releases set")
	}
}

func TestOnlyFilter(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewImagesCommand()
	cmd.StartUI()
	_ = cmd.parseArgs([]string{"--only-releases", "ref2"})

	only := cmd.onlyFilter()
	if only == nil {
		t.Fatal("Expected non-nil only filter")
	}
	if only("ref1") {
		t.Error("Expected ref1 to NOT pass only filter")
	}
	if !only("ref2") {
		t.Error("Expected ref2 to pass only filter")
	}
}

func TestOnlyFilterNil(t *testing.T) {
	cmd := &ImagesCommand{}
	if cmd.onlyFilter() != nil {
		t.Error("Expected nil only filter when no only releases set")
	}
}

// --- imageWorker integration (mock) ---

func TestImageWorkerBuildSuccess(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	mock := &ostree.MockOstree{
		Refs:       []string{"matrixos/amd64/dev/gnome"},
		LocalRefs_: []string{"matrixos/amd64/dev/gnome"},
		Remote_:    "origin",
	}
	cfg := defaultImagesTestConfig()
	im := imager.DefaultMockImager()
	_ = im // We can't inject mock imager into imageWorker easily since it creates its own.
	// This test verifies the plumbing without real builds.
	cmd, err := newTestImagesCommand(mock, cfg, []string{"--local-ostree"})
	if err != nil {
		t.Fatalf("newTestImagesCommand failed: %v", err)
	}

	// Verify the command builds without panics when calling the sub functions.
	// The actual build would fail because we don't have a real fsenc/imager,
	// so we just verify the ref detection and filtering pipeline works end-to-end.
	var buf strings.Builder
	refs, err := cmd.detectReleases(&buf)
	if err != nil {
		t.Fatalf("detectReleases failed: %v", err)
	}
	if len(refs) != 1 || refs[0] != "matrixos/amd64/dev/gnome" {
		t.Errorf("Expected [matrixos/amd64/dev/gnome], got %v", refs)
	}
}

// --- Temp file helpers ---

func createTempFile(t *testing.T) (string, error) {
	t.Helper()
	return createTempFileE()
}

func createTempFileE() (string, error) {
	f, err := os.CreateTemp("", "images-test")
	if err != nil {
		return "", err
	}
	name := f.Name()
	f.Close()
	return name, nil
}

func removeTempFile(path string) {
	os.Remove(path)
}
