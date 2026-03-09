package commands

import (
	"fmt"
	"matrixos/vector/lib/ostree"
	"strings"
	"testing"
)

// newTestReadWriteCommand creates a ReadWriteCommand with injected mock
// dependencies, bypassing initConfig/initOstree which require real config
// files and an ostree binary.
func newTestReadWriteCommand(ot ostree.IOstree, args []string) (*ReadWriteCommand, error) {
	cmd := &ReadWriteCommand{}
	cmd.ot = ot
	cmd.StartUI()
	if err := cmd.parseArgs(args); err != nil {
		return nil, err
	}
	return cmd, nil
}

func TestReadWriteName(t *testing.T) {
	cmd := NewReadWriteCommand()
	if cmd.Name() != "readwrite" {
		t.Fatalf("expected name 'readwrite', got %q", cmd.Name())
	}
}

func TestReadWriteDefaultTransient(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	mock := &ostree.MockOstree{}
	cmd, err := newTestReadWriteCommand(mock, nil)
	if err != nil {
		t.Fatalf("unexpected error creating command: %v", err)
	}

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if !mock.ReadwriteCalled {
		t.Fatal("expected Readwrite to be called")
	}
	if mock.ReadwritePermanent {
		t.Fatal("expected permanent=false (transient) by default")
	}
}

func TestReadWritePermanentFlag(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	mock := &ostree.MockOstree{}
	cmd, err := newTestReadWriteCommand(mock, []string{"--permanent"})
	if err != nil {
		t.Fatalf("unexpected error creating command: %v", err)
	}

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if !mock.ReadwriteCalled {
		t.Fatal("expected Readwrite to be called")
	}
	if !mock.ReadwritePermanent {
		t.Fatal("expected permanent=true when --permanent is passed")
	}
}

func TestReadWriteRequiresRoot(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 1000 }
	defer func() { getEuid = origEuid }()

	mock := &ostree.MockOstree{}
	cmd, err := newTestReadWriteCommand(mock, nil)
	if err != nil {
		t.Fatalf("unexpected error creating command: %v", err)
	}

	err = cmd.Run()
	if err == nil {
		t.Fatal("expected error when not running as root")
	}
	if !strings.Contains(err.Error(), "must be run as root") {
		t.Fatalf("unexpected error message: %v", err)
	}
	if mock.ReadwriteCalled {
		t.Fatal("Readwrite should not be called when not root")
	}
}

func TestReadWriteOstreeError(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	mock := &ostree.MockOstree{
		ReadwriteErr: fmt.Errorf("unlock failed: filesystem busy"),
	}
	cmd, err := newTestReadWriteCommand(mock, nil)
	if err != nil {
		t.Fatalf("unexpected error creating command: %v", err)
	}

	err = cmd.Run()
	if err == nil {
		t.Fatal("expected error from Readwrite failure")
	}
	if !strings.Contains(err.Error(), "unlock failed") {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestReadWritePermanentExplicitFalse(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	mock := &ostree.MockOstree{}
	cmd, err := newTestReadWriteCommand(mock, []string{"--permanent=false"})
	if err != nil {
		t.Fatalf("unexpected error creating command: %v", err)
	}

	err = cmd.Run()
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	if !mock.ReadwriteCalled {
		t.Fatal("expected Readwrite to be called")
	}
	if mock.ReadwritePermanent {
		t.Fatal("expected permanent=false when --permanent=false is passed")
	}
}

func TestReadWriteParseArgsUnknownFlag(t *testing.T) {
	mock := &ostree.MockOstree{}
	_, err := newTestReadWriteCommand(mock, []string{"--bogus"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
}
