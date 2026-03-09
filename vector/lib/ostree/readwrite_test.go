package ostree

import (
	"fmt"
	"matrixos/vector/lib/config"
	"matrixos/vector/lib/runner"
	"strings"
	"testing"
)

func TestReadwrite_Hotfix(t *testing.T) {
	var commands [][]string
	root := "/test-root"

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Root": {root},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	o.runner = func(cmd *runner.Cmd) error {
		cmdArgs := append([]string{cmd.Name}, cmd.Args...)
		commands = append(commands, cmdArgs)
		return nil
	}

	err = o.Readwrite(true)
	if err != nil {
		t.Fatalf("Readwrite(true) failed: %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	expected := fmt.Sprintf("ostree admin unlock --sysroot=%s --hotfix", root)
	got := strings.Join(commands[0], " ")
	if got != expected {
		t.Errorf("command mismatch:\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestReadwrite_Transient(t *testing.T) {
	var commands [][]string
	root := "/test-root"

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Root": {root},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	o.runner = func(cmd *runner.Cmd) error {
		cmdArgs := append([]string{cmd.Name}, cmd.Args...)
		commands = append(commands, cmdArgs)
		return nil
	}

	err = o.Readwrite(false)
	if err != nil {
		t.Fatalf("Readwrite(false) failed: %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	expected := fmt.Sprintf("ostree admin unlock --sysroot=%s --transient", root)
	got := strings.Join(commands[0], " ")
	if got != expected {
		t.Errorf("command mismatch:\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestReadwrite_MissingRoot(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	err = o.Readwrite(false)
	if err == nil {
		t.Fatal("Readwrite should fail when Root is not configured")
	}
}

func TestReadwrite_CommandError(t *testing.T) {
	root := "/test-root"

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Root": {root},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	o.runner = func(cmd *runner.Cmd) error {
		return fmt.Errorf("unlock failed")
	}

	err = o.Readwrite(true)
	if err == nil {
		t.Fatal("Readwrite should propagate command error")
	}
	if !strings.Contains(err.Error(), "unlock failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestReadwrite_Verbose(t *testing.T) {
	var commands [][]string
	root := "/test-root"

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Root": {root},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg, Verbose: true})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	o.runner = func(cmd *runner.Cmd) error {
		cmdArgs := append([]string{cmd.Name}, cmd.Args...)
		commands = append(commands, cmdArgs)
		return nil
	}

	err = o.Readwrite(true)
	if err != nil {
		t.Fatalf("Readwrite(true) verbose failed: %v", err)
	}

	if len(commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(commands))
	}

	// When verbose, --verbose is prepended before admin
	expected := fmt.Sprintf("ostree --verbose admin unlock --sysroot=%s --hotfix", root)
	got := strings.Join(commands[0], " ")
	if got != expected {
		t.Errorf("command mismatch:\n  got:  %s\n  want: %s", got, expected)
	}
}
