package commands

import (
	"errors"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
)

// mockEnvironmentChecker is a test double for the environmentChecker interface.
type mockEnvironmentChecker struct {
	seederErr   error
	releaserErr error
	imagerErr   error
}

func (m *mockEnvironmentChecker) VerifySeederEnvironmentSetup(_ string) error {
	return m.seederErr
}

func (m *mockEnvironmentChecker) VerifyReleaserEnvironmentSetup(_ string) error {
	return m.releaserErr
}

func (m *mockEnvironmentChecker) VerifyImagerEnvironmentSetup(_ string) error {
	return m.imagerErr
}

// newTestCheckCommand creates a CheckCommand with injected mocks,
// bypassing Init which requires a real config file on disk.
func newTestCheckCommand(
	cfg *config.MockConfig,
	checker environmentChecker,
) *CheckCommand {
	cmd := NewCheckCommand()
	cmd.cfg = cfg
	cmd.qa = checker
	cmd.StartUI()
	cmd.SetupPrinters("check")
	return cmd
}

func checkTestConfig() *config.MockConfig {
	return &config.MockConfig{
		Items: map[string][]string{},
	}
}

func TestCheckName(t *testing.T) {
	cmd := NewCheckCommand()
	if cmd.Name() != "check" {
		t.Fatalf("expected name 'check', got %q", cmd.Name())
	}
}

func TestNewCheckCommand(t *testing.T) {
	cmd := NewCheckCommand()
	if cmd.fs == nil {
		t.Fatal("expected flag set to be initialised")
	}
	if cmd.qa != nil {
		t.Fatal("expected qa to be nil before Init")
	}
}

func TestCheckRunAllPass(t *testing.T) {
	checker := &mockEnvironmentChecker{}
	cmd := newTestCheckCommand(checkTestConfig(), checker)

	err := cmd.Run()
	if err != nil {
		t.Fatalf("expected no error when all checks pass, got: %v", err)
	}
}

func TestCheckRunSeederFails(t *testing.T) {
	checker := &mockEnvironmentChecker{
		seederErr: errors.New("chroot not found"),
	}
	cmd := newTestCheckCommand(checkTestConfig(), checker)

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error when seeder check fails")
	}
	if !strings.Contains(err.Error(), "one or more environment checks failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCheckRunReleaserFails(t *testing.T) {
	checker := &mockEnvironmentChecker{
		releaserErr: errors.New("ostree not found"),
	}
	cmd := newTestCheckCommand(checkTestConfig(), checker)

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error when releaser check fails")
	}
	if !strings.Contains(err.Error(), "one or more environment checks failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCheckRunImagerFails(t *testing.T) {
	checker := &mockEnvironmentChecker{
		imagerErr: errors.New("qemu-img not found"),
	}
	cmd := newTestCheckCommand(checkTestConfig(), checker)

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error when imager check fails")
	}
	if !strings.Contains(err.Error(), "one or more environment checks failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCheckRunAllFail(t *testing.T) {
	checker := &mockEnvironmentChecker{
		seederErr:   errors.New("seeder broken"),
		releaserErr: errors.New("releaser broken"),
		imagerErr:   errors.New("imager broken"),
	}
	cmd := newTestCheckCommand(checkTestConfig(), checker)

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error when all checks fail")
	}
	if !strings.Contains(err.Error(), "one or more environment checks failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestCheckRunPartialFailure(t *testing.T) {
	// Only imager fails, others pass.
	checker := &mockEnvironmentChecker{
		imagerErr: errors.New("sgdisk not found"),
	}
	cmd := newTestCheckCommand(checkTestConfig(), checker)

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error when at least one check fails")
	}
}

func TestCheckRunSeederAndReleaserFail(t *testing.T) {
	checker := &mockEnvironmentChecker{
		seederErr:   errors.New("wget not found"),
		releaserErr: errors.New("gpg not found"),
	}
	cmd := newTestCheckCommand(checkTestConfig(), checker)

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error when multiple checks fail")
	}
}
