package commands

import (
	"bytes"
	"matrixos/vector/lib/ostree"
	"os"
	"testing"
)

// newTestBranchCommand creates a BranchCommand with injected mock dependencies,
// bypassing initConfig/initOstree which require real config files.
func newTestBranchCommand(ot ostree.IOstree) *BranchCommand {
	cmd := &BranchCommand{}
	cmd.ot = ot
	return cmd
}

// captureStdout runs fn while capturing os.Stdout and returns the output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestBranchShow(t *testing.T) {
	mock := &ostree.MockOstree{
		Deployments: []ostree.Deployment{
			{
				Booted:    true,
				Checksum:  "abc123",
				Stateroot: "matrixos",
				Refspec:   "origin:matrixos/amd64/gnome",
				Index:     0,
				Serial:    1,
			},
		},
	}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"show"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	output := captureStdout(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("Run failed: %v", err)
		}
	})

	expected := "Booted deployment:\n" +
		"  Name: matrixos\n" +
		"  Index: 0\n" +
		"  Branch/Refspec: origin:matrixos/amd64/gnome\n" +
		"  Checksum: abc123\n" +
		"  Serial: 1\n" +
		"  Pending: false\n" +
		"  Staged: false\n" +
		"  Rollback: false\n"
	if output != expected {
		t.Errorf("output mismatch\nwant: %q\n got: %q", expected, output)
	}
}

func TestBranchShowNoBooted(t *testing.T) {
	mock := &ostree.MockOstree{
		Deployments: []ostree.Deployment{
			{Booted: false, Stateroot: "matrixos"},
		},
	}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"show"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for no booted deployment, got nil")
	}
	if err.Error() != "no booted deployment found" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestBranchList(t *testing.T) {
	mock := &ostree.MockOstree{
		Refs: []string{"origin:branch1", "origin:branch2"},
	}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"remote"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	output := captureStdout(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("Run failed: %v", err)
		}
	})

	expected := "origin:branch1\norigin:branch2\n"
	if output != expected {
		t.Errorf("output mismatch\nwant: %q\n got: %q", expected, output)
	}
}

func TestBranchLocal(t *testing.T) {
	mock := &ostree.MockOstree{
		LocalRefs_: []string{"matrixos/amd64/gnome", "matrixos/amd64/cosmic"},
	}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"local"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	output := captureStdout(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("Run failed: %v", err)
		}
	})

	expected := "matrixos/amd64/gnome\nmatrixos/amd64/cosmic\n"
	if output != expected {
		t.Errorf("output mismatch\nwant: %q\n got: %q", expected, output)
	}
}

func TestBranchDeployment(t *testing.T) {
	mock := &ostree.MockOstree{
		Deployments: []ostree.Deployment{
			{
				Booted:    true,
				Checksum:  "abc123",
				Stateroot: "matrixos",
				Refspec:   "origin:matrixos/amd64/gnome",
				Index:     0,
				Serial:    1,
			},
			{
				Booted:    false,
				Checksum:  "def456",
				Stateroot: "matrixos",
				Refspec:   "origin:matrixos/amd64/gnome",
				Index:     1,
				Serial:    0,
				Rollback:  true,
			},
		},
	}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"deployment"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	output := captureStdout(t, func() {
		if err := cmd.Run(); err != nil {
			t.Fatalf("Run failed: %v", err)
		}
	})

	expected := "Booted deployment:\n" +
		"  Name: matrixos\n" +
		"  Index: 0\n" +
		"  Branch/Refspec: origin:matrixos/amd64/gnome\n" +
		"  Checksum: abc123\n" +
		"  Serial: 1\n" +
		"  Pending: false\n" +
		"  Staged: false\n" +
		"  Rollback: false\n" +
		"Available deployment:\n" +
		"  Name: matrixos\n" +
		"  Index: 1\n" +
		"  Branch/Refspec: origin:matrixos/amd64/gnome\n" +
		"  Checksum: def456\n" +
		"  Serial: 0\n" +
		"  Pending: false\n" +
		"  Staged: false\n" +
		"  Rollback: true\n"
	if output != expected {
		t.Errorf("output mismatch\nwant: %q\n got: %q", expected, output)
	}
}

func TestBranchPin(t *testing.T) {
	mock := &ostree.MockOstree{
		Deployments: []ostree.Deployment{
			{
				Booted:    true,
				Checksum:  "abc123",
				Stateroot: "matrixos",
				Index:     0,
			},
			{
				Booted:    false,
				Checksum:  "def456",
				Stateroot: "matrixos",
				Index:     1,
			},
		},
	}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"pin", "1"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	if err := cmd.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if mock.PinIndex != 1 {
		t.Errorf("expected PinIndex 1, got %d", mock.PinIndex)
	}
	if mock.PinChecksum != "def456" {
		t.Errorf("expected PinChecksum %q, got %q", "def456", mock.PinChecksum)
	}
}

func TestBranchPinMissingArg(t *testing.T) {
	mock := &ostree.MockOstree{}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"pin"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for missing pin arg, got nil")
	}
}

func TestBranchPinNotFound(t *testing.T) {
	mock := &ostree.MockOstree{
		Deployments: []ostree.Deployment{
			{Booted: true, Index: 0, Checksum: "abc123"},
		},
	}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"pin", "99"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for non-existent deployment index, got nil")
	}
}

func TestBranchUnpin(t *testing.T) {
	mock := &ostree.MockOstree{
		PinIndex: 1,
		Deployments: []ostree.Deployment{
			{
				Booted:   true,
				Checksum: "abc123",
				Index:    0,
			},
			{
				Booted:   false,
				Checksum: "def456",
				Index:    1,
			},
		},
	}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"unpin", "1"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	if err := cmd.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if mock.PinIndex != -1 {
		t.Errorf("expected PinIndex -1 after unpin, got %d", mock.PinIndex)
	}
}

func TestBranchUnpinMissingArg(t *testing.T) {
	mock := &ostree.MockOstree{}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"unpin"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for missing unpin arg, got nil")
	}
}

func TestBranchSwitch(t *testing.T) {
	mock := &ostree.MockOstree{}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"switch", "new/branch"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	if err := cmd.Run(); err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if mock.SwitchRef != "new/branch" {
		t.Errorf("expected switch ref %q, got %q", "new/branch", mock.SwitchRef)
	}
}

func TestBranchSwitchMissingArg(t *testing.T) {
	mock := &ostree.MockOstree{}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"switch"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for missing switch arg, got nil")
	}
}

func TestBranchUnknownSubcommand(t *testing.T) {
	mock := &ostree.MockOstree{}
	cmd := newTestBranchCommand(mock)
	if err := cmd.parseArgs([]string{"foo"}); err != nil {
		t.Fatalf("parseArgs failed: %v", err)
	}

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected error for unknown subcommand, got nil")
	}
}

func TestBranchNoSubcommand(t *testing.T) {
	mock := &ostree.MockOstree{}
	cmd := newTestBranchCommand(mock)
	err := cmd.parseArgs([]string{})
	if err == nil {
		t.Fatal("expected error for missing subcommand, got nil")
	}
}
