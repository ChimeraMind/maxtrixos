package commands

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
)

// newTestCfgCommand creates a CfgCommand with an injected mock config,
// bypassing initBaseConfig which requires real config files.
func newTestCfgCommand(cfg *config.MockConfig, args []string) (*CfgCommand, error) {
	cmd := &CfgCommand{}
	cmd.cfg = cfg
	if err := cmd.parseArgs(args); err != nil {
		return nil, err
	}
	return cmd, nil
}

func TestCfgName(t *testing.T) {
	cmd := &CfgCommand{}
	if cmd.Name() != "cfg" {
		t.Fatalf("expected name 'cfg', got %q", cmd.Name())
	}
}

func TestCfgNoSubcommand(t *testing.T) {
	cfg := &config.MockConfig{Items: map[string][]string{}}
	_, err := newTestCfgCommand(cfg, nil)
	if err == nil || !strings.Contains(err.Error(), "no subcommand provided") {
		t.Fatalf("expected 'no subcommand' error, got: %v", err)
	}
}

func TestCfgUnknownSubcommand(t *testing.T) {
	cfg := &config.MockConfig{Items: map[string][]string{}}
	cmd, err := newTestCfgCommand(cfg, []string{"bogus"})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	err = cmd.Run()
	if err == nil || !strings.Contains(err.Error(), "unknown subcommand") {
		t.Fatalf("expected 'unknown subcommand' error, got: %v", err)
	}
}

func TestCfgGetNoKeys(t *testing.T) {
	cfg := &config.MockConfig{Items: map[string][]string{}}
	cmd, err := newTestCfgCommand(cfg, []string{"get"})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	err = cmd.Run()
	if err == nil || !strings.Contains(err.Error(), "at least one config key") {
		t.Fatalf("expected 'at least one config key' error, got: %v", err)
	}
}

func TestCfgGetStringValues(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.Root":  {"/opt/matrixos"},
			"Ostree.Remote":  {"origin"},
			"Ostree.RepoDir": {"/var/repo"},
		},
	}
	cmd, err := newTestCfgCommand(cfg, []string{"get", "matrixOS.Root", "Ostree.Remote", "Ostree.RepoDir"})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = cmd.Run()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %q", len(lines), output)
	}
	want := []string{
		"matrixOS.Root=/opt/matrixos",
		"Ostree.Remote=origin",
		"Ostree.RepoDir=/var/repo",
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

func TestCfgGetBoolValues(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Feature.Enabled":  {"true"},
			"Feature.Disabled": {"false"},
		},
		Bools: map[string]bool{
			"Feature.Enabled":  true,
			"Feature.Disabled": false,
		},
	}
	cmd, err := newTestCfgCommand(cfg, []string{"get", "Feature.Enabled", "Feature.Disabled"})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = cmd.Run()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	want := []string{
		"Feature.Enabled=true",
		"Feature.Disabled=false",
	}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d: %q", len(want), len(lines), output)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

func TestCfgGetMixedValues(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.Root":   {"/opt/matrixos"},
			"Feature.Enabled": {"true"},
		},
		Bools: map[string]bool{
			"Feature.Enabled": true,
		},
	}
	cmd, err := newTestCfgCommand(cfg, []string{"get", "matrixOS.Root", "Feature.Enabled"})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = cmd.Run()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	want := []string{
		"matrixOS.Root=/opt/matrixos",
		"Feature.Enabled=true",
	}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d: %q", len(want), len(lines), output)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

func TestCfgGetMissingKeyStderr(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.Root": {"/opt/matrixos"},
		},
	}
	// "Missing.Key" is not in the mock — MockConfig.GetItem returns ("", nil)
	// for missing keys, so it will output Missing.Key= with an empty value.
	cmd, err := newTestCfgCommand(cfg, []string{"get", "matrixOS.Root", "Missing.Key"})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = cmd.Run()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 output lines, got %d: %q", len(lines), output)
	}
	if lines[0] != "matrixOS.Root=/opt/matrixos" {
		t.Errorf("line[0] = %q, want %q", lines[0], "matrixOS.Root=/opt/matrixos")
	}
	if lines[1] != "Missing.Key=" {
		t.Errorf("line[1] = %q, want %q", lines[1], "Missing.Key=")
	}
}

func TestCfgGetPreservesOrdering(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"C.Key": {"ccc"},
			"A.Key": {"aaa"},
			"B.Key": {"bbb"},
		},
	}
	// Request in C, A, B order — output must match.
	cmd, err := newTestCfgCommand(cfg, []string{"get", "C.Key", "A.Key", "B.Key"})
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err = cmd.Run()

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("unexpected run error: %v", err)
	}

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	lines := strings.Split(strings.TrimSpace(output), "\n")
	want := []string{"C.Key=ccc", "A.Key=aaa", "B.Key=bbb"}
	if len(lines) != len(want) {
		t.Fatalf("expected %d lines, got %d: %q", len(want), len(lines), output)
	}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("line[%d] = %q, want %q", i, lines[i], w)
		}
	}
}
