package commands

import (
	"matrixos/vector/lib/config"
	"strings"
	"testing"
)

func TestDevCommandName(t *testing.T) {
	cmd := NewDevCommand()
	if got := cmd.Name(); got != "dev" {
		t.Errorf("Name() = %q, want %q", got, "dev")
	}
}

func TestDevCommandInitNoSubcommand(t *testing.T) {
	cmd := NewDevCommand()
	err := cmd.Init([]string{})
	if err == nil {
		t.Fatal("expected error when no subcommand provided")
	}
	if got := err.Error(); got != "no subcommand provided" {
		t.Errorf("error = %q, want %q", got, "no subcommand provided")
	}
}

func TestDevCommandInitValidSubcommands(t *testing.T) {
	for _, sub := range []string{"check", "enter", "janitor", "vm"} {
		t.Run(sub, func(t *testing.T) {
			cmd := NewDevCommand()
			err := cmd.Init([]string{sub})
			if err != nil {
				t.Fatalf("Init(%q) returned error: %v", sub, err)
			}
			if cmd.sub != sub {
				t.Errorf("sub = %q, want %q", cmd.sub, sub)
			}
		})
	}
}

func TestDevCommandInitExtraArgs(t *testing.T) {
	cmd := NewDevCommand()
	err := cmd.Init([]string{"check", "--foo", "bar"})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if cmd.sub != "check" {
		t.Errorf("sub = %q, want %q", cmd.sub, "check")
	}
	if len(cmd.args) != 2 {
		t.Fatalf("args len = %d, want 2", len(cmd.args))
	}
	if cmd.args[0] != "--foo" || cmd.args[1] != "bar" {
		t.Errorf("args = %v, want [--foo bar]", cmd.args)
	}
}

func TestDevCommandRunUnknownSubcommand(t *testing.T) {
	cmd := NewDevCommand()
	err := cmd.Init([]string{"nonexistent"})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	err = cmd.Run()
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
	want := "unknown subcommand: nonexistent"
	if got := err.Error(); got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestDevCommandSubcommandsRegistered(t *testing.T) {
	cmd := NewDevCommand()
	expected := map[string]bool{
		"check":   true,
		"enter":   true,
		"janitor": true,
		"vm":      true,
	}
	if len(cmd.subcommands) != len(expected) {
		t.Fatalf("subcommands count = %d, want %d", len(cmd.subcommands), len(expected))
	}
	for name := range expected {
		if _, ok := cmd.subcommands[name]; !ok {
			t.Errorf("missing subcommand %q", name)
		}
	}
}

func TestDevCommandSubcommandFactories(t *testing.T) {
	cmd := NewDevCommand()
	for name, factory := range cmd.subcommands {
		t.Run(name, func(t *testing.T) {
			sub := factory()
			if sub == nil {
				t.Fatalf("factory for %q returned nil", name)
			}
			if got := sub.Name(); got != name {
				t.Errorf("subcommand Name() = %q, want %q", got, name)
			}
		})
	}
}

func TestDevCommandRunSubcommandInitFailure(t *testing.T) {
	// Run a real subcommand that will fail during Init because
	// config files don't exist in temp directory.
	t.Chdir(t.TempDir())
	config.SystemSearchPathEnabled = false
	t.Cleanup(func() { config.SystemSearchPathEnabled = true })

	cmd := NewDevCommand()
	err := cmd.Init([]string{"check"})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	// Run should fail because the check subcommand's Init tries to load config.
	err = cmd.Run()
	if err == nil {
		t.Fatal("expected error from subcommand init failure")
	}
	if !strings.Contains(err.Error(), "failed to initialize subcommand") {
		t.Errorf("error = %q, want to contain 'failed to initialize subcommand'", err.Error())
	}
}
