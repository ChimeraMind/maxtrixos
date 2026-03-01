package commands

import (
	"testing"
)

func TestBuildCommandName(t *testing.T) {
	cmd := NewBuildCommand()
	if got := cmd.Name(); got != "build" {
		t.Errorf("Name() = %q, want %q", got, "build")
	}
}

func TestBuildCommandInitNoSubcommand(t *testing.T) {
	cmd := NewBuildCommand()
	err := cmd.Init([]string{})
	if err == nil {
		t.Fatal("expected error when no subcommand provided")
	}
	if got := err.Error(); got != "no subcommand provided" {
		t.Errorf("error = %q, want %q", got, "no subcommand provided")
	}
}

func TestBuildCommandInitValidSubcommand(t *testing.T) {
	for _, sub := range []string{"image", "images", "release", "seeds"} {
		t.Run(sub, func(t *testing.T) {
			cmd := NewBuildCommand()
			// Pass "--help" would cause exit, so just check Init parses the subcommand name.
			// We don't call Run because subcommands need real config.
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

func TestBuildCommandInitExtraArgs(t *testing.T) {
	cmd := NewBuildCommand()
	err := cmd.Init([]string{"image", "--foo", "bar"})
	if err != nil {
		t.Fatalf("Init returned error: %v", err)
	}
	if cmd.sub != "image" {
		t.Errorf("sub = %q, want %q", cmd.sub, "image")
	}
	wantArgs := []string{"--foo", "bar"}
	if len(cmd.args) != len(wantArgs) {
		t.Fatalf("args len = %d, want %d", len(cmd.args), len(wantArgs))
	}
	for i, a := range wantArgs {
		if cmd.args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, cmd.args[i], a)
		}
	}
}

func TestBuildCommandRunUnknownSubcommand(t *testing.T) {
	cmd := NewBuildCommand()
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

func TestBuildCommandSubcommandsRegistered(t *testing.T) {
	cmd := NewBuildCommand()
	expected := map[string]bool{
		"image":   true,
		"images":  true,
		"release": true,
		"seeds":   true,
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

func TestBuildCommandSubcommandFactories(t *testing.T) {
	cmd := NewBuildCommand()
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
