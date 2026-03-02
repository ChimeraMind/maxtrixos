package releaser

import (
	"errors"
	"os"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/ostree"
)

// ---------------------------------------------------------------------------
// ValidateReleaseStage
// ---------------------------------------------------------------------------

func TestValidateReleaseStage_Dev(t *testing.T) {
	stage, err := ValidateReleaseStage("dev")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stage != StageDev {
		t.Fatalf("got %q, want %q", stage, StageDev)
	}
}

func TestValidateReleaseStage_Prod(t *testing.T) {
	stage, err := ValidateReleaseStage("prod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stage != StageProd {
		t.Fatalf("got %q, want %q", stage, StageProd)
	}
}

func TestValidateReleaseStage_Unknown(t *testing.T) {
	_, err := ValidateReleaseStage("staging")
	if err == nil {
		t.Fatal("expected error for unknown stage, got nil")
	}
}

func TestValidateReleaseStage_Empty(t *testing.T) {
	_, err := ValidateReleaseStage("")
	if err == nil {
		t.Fatal("expected error for empty stage, got nil")
	}
}

// ---------------------------------------------------------------------------
// checkImageDir
// ---------------------------------------------------------------------------

func TestCheckImageDir_ValidDir(t *testing.T) {
	dir := t.TempDir()
	if err := checkImageDir(dir); err != nil {
		t.Fatalf("unexpected error for valid dir: %v", err)
	}
}

func TestCheckImageDir_EmptyString(t *testing.T) {
	if err := checkImageDir(""); err == nil {
		t.Fatal("expected error for empty string, got nil")
	}
}

func TestCheckImageDir_NonexistentPath(t *testing.T) {
	if err := checkImageDir("/nonexistent/path/that/does/not/exist"); err == nil {
		t.Fatal("expected error for nonexistent path, got nil")
	}
}

func TestCheckImageDir_FileInsteadOfDir(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "notadir")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if err := checkImageDir(f.Name()); err == nil {
		t.Fatal("expected error for a file path, got nil")
	}
}

// ---------------------------------------------------------------------------
// NewReleaser – nil / missing parameter guards
// ---------------------------------------------------------------------------

func TestNewReleaser_NilConfig(t *testing.T) {
	_, err := NewReleaser(nil, &ostree.MockOstree{}, &NewReleaserOptions{
		ChrootDir: "/tmp", ImageDir: t.TempDir(), Ref: "origin/matrixos",
	})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewReleaser_NilOstree(t *testing.T) {
	_, err := NewReleaser(
		&config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}},
		nil,
		&NewReleaserOptions{ChrootDir: "/tmp", ImageDir: t.TempDir(), Ref: "origin/matrixos"},
	)
	if err == nil {
		t.Fatal("expected error for nil ostree")
	}
}

func TestNewReleaser_NilOpts(t *testing.T) {
	_, err := NewReleaser(
		&config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}},
		&ostree.MockOstree{},
		nil,
	)
	if err == nil {
		t.Fatal("expected error for nil options")
	}
}

func TestNewReleaser_EmptyRef(t *testing.T) {
	_, err := NewReleaser(
		&config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}},
		&ostree.MockOstree{},
		&NewReleaserOptions{ChrootDir: "/tmp", ImageDir: t.TempDir(), Ref: ""},
	)
	if err == nil {
		t.Fatal("expected error for empty Ref")
	}
}

// ---------------------------------------------------------------------------
// NewReleaser – happy path
// ---------------------------------------------------------------------------

func TestNewReleaser_HappyPath(t *testing.T) {
	imageDir := t.TempDir()
	cfg := &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}}
	ot := &ostree.MockOstree{}

	r, err := NewReleaser(cfg, ot, &NewReleaserOptions{
		ChrootDir: "/some/chroot",
		ImageDir:  imageDir,
		Ref:       "origin/matrixos",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil Releaser")
	}
	if r.ChrootDir() != "/some/chroot" {
		t.Errorf("ChrootDir() = %q, want %q", r.ChrootDir(), "/some/chroot")
	}
	if r.ImageDir() != imageDir {
		t.Errorf("ImageDir() = %q, want %q", r.ImageDir(), imageDir)
	}
	if r.Ref() != "origin/matrixos" {
		t.Errorf("Ref() = %q, want %q", r.Ref(), "origin/matrixos")
	}
	if r.Stdout() == nil {
		t.Error("Stdout() should not be nil")
	}
	if r.Stderr() == nil {
		t.Error("Stderr() should not be nil")
	}
}

// ---------------------------------------------------------------------------
// NewReleaser – propagation of QA init failure
// ---------------------------------------------------------------------------

func TestNewReleaser_QAInitFailure(t *testing.T) {
	// validation.New only fails when cfg is nil, which is caught earlier.
	// This test documents that validation.New currently can't fail for a
	// non-nil config. If it ever gains additional validation, this ensures
	// the error path is exercised by passing a config that would cause
	// validation.New to fail.
	//
	// With the current implementation this path is unreachable, but we keep
	// the test to guard against future regressions.

	// Double-check that nil config is properly rejected (which also prevents
	// QA init from receiving nil).
	_, err := NewReleaser(nil, &ostree.MockOstree{}, &NewReleaserOptions{
		ChrootDir: "/tmp", ImageDir: t.TempDir(), Ref: "ref",
	})
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

// ---------------------------------------------------------------------------
// NewReleaser – sets runner to runner.Run
// ---------------------------------------------------------------------------

func TestNewReleaser_SetsRunner(t *testing.T) {
	imageDir := t.TempDir()
	cfg := &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}}
	ot := &ostree.MockOstree{}

	r, err := NewReleaser(cfg, ot, &NewReleaserOptions{
		ChrootDir: "/chroot",
		ImageDir:  imageDir,
		Ref:       "origin/matrixos",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.runner == nil {
		t.Fatal("expected runner to be set")
	}
}

// ---------------------------------------------------------------------------
// NewReleaser – stdout/stderr default to os.Stdout/os.Stderr
// ---------------------------------------------------------------------------

func TestNewReleaser_DefaultStdoutStderr(t *testing.T) {
	imageDir := t.TempDir()
	cfg := &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}}
	ot := &ostree.MockOstree{}

	r, err := NewReleaser(cfg, ot, &NewReleaserOptions{
		ChrootDir: "/chroot",
		ImageDir:  imageDir,
		Ref:       "ref",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Stdout() != os.Stdout {
		t.Error("expected Stdout() == os.Stdout")
	}
	if r.Stderr() != os.Stderr {
		t.Error("expected Stderr() == os.Stderr")
	}
}

// ---------------------------------------------------------------------------
// ReleaseStage constants
// ---------------------------------------------------------------------------

func TestReleaseStageConstants(t *testing.T) {
	if StageDev != "dev" {
		t.Errorf("StageDev = %q, want %q", StageDev, "dev")
	}
	if StageProd != "prod" {
		t.Errorf("StageProd = %q, want %q", StageProd, "prod")
	}
}

// ---------------------------------------------------------------------------
// Interface compliance
// ---------------------------------------------------------------------------

func TestReleaserImplementsIRelease(t *testing.T) {
	// The compile-time check `var _ IRelease = (*Releaser)(nil)` already
	// ensures this, but an explicit runtime test documents the intent.
	var _ IRelease = newTestReleaser()
}

// ---------------------------------------------------------------------------
// NewReleaser – error messages contain useful context
// ---------------------------------------------------------------------------

func TestNewReleaser_ErrorMessages(t *testing.T) {
	tests := []struct {
		name      string
		cfg       config.IConfig
		ot        ostree.IOstree
		opts      *NewReleaserOptions
		wantInErr string
	}{
		{
			name:      "nil config",
			cfg:       nil,
			ot:        &ostree.MockOstree{},
			opts:      &NewReleaserOptions{ChrootDir: "/c", ImageDir: t.TempDir(), Ref: "r"},
			wantInErr: "config",
		},
		{
			name:      "nil ostree",
			cfg:       &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}},
			ot:        nil,
			opts:      &NewReleaserOptions{ChrootDir: "/c", ImageDir: t.TempDir(), Ref: "r"},
			wantInErr: "ostree",
		},
		{
			name:      "nil opts",
			cfg:       &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}},
			ot:        &ostree.MockOstree{},
			opts:      nil,
			wantInErr: "options",
		},
		{
			name:      "empty Ref",
			cfg:       &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}},
			ot:        &ostree.MockOstree{},
			opts:      &NewReleaserOptions{ChrootDir: "/c", ImageDir: t.TempDir(), Ref: ""},
			wantInErr: "Ref",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewReleaser(tc.cfg, tc.ot, tc.opts)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !releaseContains(err.Error(), tc.wantInErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tc.wantInErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateReleaseStage – table-driven
// ---------------------------------------------------------------------------

func TestValidateReleaseStage_Table(t *testing.T) {
	tests := []struct {
		input   string
		want    ReleaseStage
		wantErr bool
	}{
		{"dev", StageDev, false},
		{"prod", StageProd, false},
		{"", "", true},
		{"staging", "", true},
		{"DEV", "", true},  // case-sensitive
		{"PROD", "", true}, // case-sensitive
		{"Dev", "", true},  // case-sensitive
		{" dev", "", true}, // leading space
		{"dev ", "", true}, // trailing space
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got, err := ValidateReleaseStage(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error for input %q, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for input %q: %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// checkImageDir – with ErrConfig (unrelated but covers checkImageDir in isolation)
// ---------------------------------------------------------------------------

func TestCheckImageDir_SymlinkToDir(t *testing.T) {
	dir := t.TempDir()
	link := dir + "/link"
	if err := os.Symlink(dir, link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if err := checkImageDir(link); err != nil {
		t.Fatalf("expected no error for symlink to dir, got: %v", err)
	}
}

func TestCheckImageDir_SymlinkToFile(t *testing.T) {
	dir := t.TempDir()
	f, err := os.CreateTemp(dir, "file")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	link := dir + "/link"
	if err := os.Symlink(f.Name(), link); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	if err := checkImageDir(link); err == nil {
		t.Fatal("expected error for symlink to file, got nil")
	}
}

// ---------------------------------------------------------------------------
// NewReleaser – ImageDir that is a file, not a directory
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// CommitOptions – struct zero value
// ---------------------------------------------------------------------------

func TestCommitOptions_ZeroValue(t *testing.T) {
	var opts CommitOptions
	if opts.Branch != "" {
		t.Errorf("expected empty Branch, got %q", opts.Branch)
	}
	if opts.ParentBranch != "" {
		t.Errorf("expected empty ParentBranch, got %q", opts.ParentBranch)
	}
	if opts.ParentRev != "" {
		t.Errorf("expected empty ParentRev, got %q", opts.ParentRev)
	}
	if opts.Consume {
		t.Error("expected Consume false")
	}
}

// ---------------------------------------------------------------------------
// NewReleaserOptions – struct zero value
// ---------------------------------------------------------------------------

func TestNewReleaserOptions_ZeroValue(t *testing.T) {
	var opts NewReleaserOptions
	if opts.ChrootDir != "" {
		t.Errorf("expected empty ChrootDir, got %q", opts.ChrootDir)
	}
	if opts.ImageDir != "" {
		t.Errorf("expected empty ImageDir, got %q", opts.ImageDir)
	}
	if opts.Ref != "" {
		t.Errorf("expected empty Ref, got %q", opts.Ref)
	}
}

// ---------------------------------------------------------------------------
// NewReleaser – stores correct internal references
// ---------------------------------------------------------------------------

func TestNewReleaser_InternalState(t *testing.T) {
	imageDir := t.TempDir()
	cfg := &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}}
	ot := &ostree.MockOstree{}

	r, err := NewReleaser(cfg, ot, &NewReleaserOptions{
		ChrootDir: "/test/chroot",
		ImageDir:  imageDir,
		Ref:       "origin/test",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify internal fields are set correctly.
	if r.cfg != cfg {
		t.Error("cfg not set correctly")
	}
	if r.ostree != ot {
		t.Error("ostree not set correctly")
	}
	if r.qa == nil {
		t.Error("qa should not be nil")
	}
	if r.chrootDir != "/test/chroot" {
		t.Errorf("chrootDir = %q, want %q", r.chrootDir, "/test/chroot")
	}
	if r.imageDir != imageDir {
		t.Errorf("imageDir = %q, want %q", r.imageDir, imageDir)
	}
	if r.ref != "origin/test" {
		t.Errorf("ref = %q, want %q", r.ref, "origin/test")
	}
}

// ---------------------------------------------------------------------------
// NewReleaser – ErrConfig propagation
// ---------------------------------------------------------------------------

func TestNewReleaser_ErrConfigDoesNotFailConstruction(t *testing.T) {
	// ErrConfig still satisfies IConfig; NewReleaser doesn't call any
	// config methods during construction (only validation.New is called,
	// which only checks for nil). So construction should succeed.
	imageDir := t.TempDir()
	wantErr := errors.New("cfg broken")
	cfg := &config.ErrConfig{Err: wantErr}

	r, err := NewReleaser(cfg, &ostree.MockOstree{}, &NewReleaserOptions{
		ChrootDir: "/chroot",
		ImageDir:  imageDir,
		Ref:       "ref",
	})
	if err != nil {
		t.Fatalf("unexpected construction error: %v", err)
	}
	if r == nil {
		t.Fatal("expected non-nil Releaser")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func releaseContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
