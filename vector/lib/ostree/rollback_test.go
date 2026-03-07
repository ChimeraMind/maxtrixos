package ostree

import (
	"fmt"
	"matrixos/vector/lib/config"
	"matrixos/vector/lib/runner"
	"strings"
	"testing"
)

func TestPin(t *testing.T) {
	root := t.TempDir()

	fakeJSON := `{
		"deployments": [
			{
				"checksum": "abc123",
				"stateroot": "matrixos",
				"refspec": "origin:matrixos/amd64/gnome",
				"booted": true,
				"pending": false,
				"rollback": false,
				"staged": false,
				"index": 0,
				"serial": 1
			},
			{
				"checksum": "def456",
				"stateroot": "matrixos",
				"refspec": "origin:matrixos/amd64/gnome",
				"booted": false,
				"pending": false,
				"rollback": true,
				"staged": false,
				"index": 1,
				"serial": 0
			}
		]
	}`

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Root": {root},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	var capturedArgs []string
	o.runner = func(cmd *runner.Cmd) error {
		args := append([]string{cmd.Name}, cmd.Args...)
		cmdStr := strings.Join(args, " ")
		// First call is ListDeployments (admin status --json)
		if strings.Contains(cmdStr, "admin status") {
			cmd.Stdout.Write([]byte(fakeJSON))
			return nil
		}
		// Second call is admin pin
		capturedArgs = args
		return nil
	}

	err = o.Pin(0)
	if err != nil {
		t.Fatalf("Pin failed: %v", err)
	}

	cmdStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(cmdStr, "admin pin") {
		t.Errorf("expected 'admin pin' in command, got: %s", cmdStr)
	}
	if strings.Contains(cmdStr, "--unpin") {
		t.Errorf("Pin should not include --unpin flag, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "--sysroot="+root) {
		t.Errorf("expected --sysroot=%s in command, got: %s", root, cmdStr)
	}
	if !strings.HasSuffix(cmdStr, " 0") {
		t.Errorf("expected target index '0' at end of command, got: %s", cmdStr)
	}
}

func TestPin_SecondDeployment(t *testing.T) {
	root := t.TempDir()

	fakeJSON := `{
		"deployments": [
			{
				"checksum": "abc123",
				"stateroot": "matrixos",
				"booted": true,
				"index": 0,
				"serial": 1
			},
			{
				"checksum": "def456",
				"stateroot": "matrixos",
				"booted": false,
				"rollback": true,
				"index": 1,
				"serial": 0
			}
		]
	}`

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Root": {root},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	var capturedArgs []string
	o.runner = func(cmd *runner.Cmd) error {
		args := append([]string{cmd.Name}, cmd.Args...)
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "admin status") {
			cmd.Stdout.Write([]byte(fakeJSON))
			return nil
		}
		capturedArgs = args
		return nil
	}

	err = o.Pin(1)
	if err != nil {
		t.Fatalf("Pin(1) failed: %v", err)
	}

	cmdStr := strings.Join(capturedArgs, " ")
	if !strings.HasSuffix(cmdStr, " 1") {
		t.Errorf("expected target index '1' at end of command, got: %s", cmdStr)
	}
}

func TestUnpin(t *testing.T) {
	root := t.TempDir()

	fakeJSON := `{
		"deployments": [
			{
				"checksum": "abc123",
				"stateroot": "matrixos",
				"booted": true,
				"index": 0,
				"serial": 1
			},
			{
				"checksum": "def456",
				"stateroot": "matrixos",
				"booted": false,
				"rollback": true,
				"pinned": true,
				"index": 1,
				"serial": 0
			}
		]
	}`

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Root": {root},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	var capturedArgs []string
	o.runner = func(cmd *runner.Cmd) error {
		args := append([]string{cmd.Name}, cmd.Args...)
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "admin status") {
			cmd.Stdout.Write([]byte(fakeJSON))
			return nil
		}
		capturedArgs = args
		return nil
	}

	err = o.Unpin(1)
	if err != nil {
		t.Fatalf("Unpin failed: %v", err)
	}

	cmdStr := strings.Join(capturedArgs, " ")
	if !strings.Contains(cmdStr, "admin pin") {
		t.Errorf("expected 'admin pin' in command, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "--unpin") {
		t.Errorf("Unpin should include --unpin flag, got: %s", cmdStr)
	}
	if !strings.Contains(cmdStr, "--sysroot="+root) {
		t.Errorf("expected --sysroot=%s in command, got: %s", root, cmdStr)
	}
	if !strings.HasSuffix(cmdStr, " 1") {
		t.Errorf("expected target index '1' at end of command, got: %s", cmdStr)
	}
}

func TestPin_DeploymentNotFound(t *testing.T) {
	root := t.TempDir()

	fakeJSON := `{
		"deployments": [
			{
				"checksum": "abc123",
				"stateroot": "matrixos",
				"booted": true,
				"index": 0,
				"serial": 1
			}
		]
	}`

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
		args := append([]string{cmd.Name}, cmd.Args...)
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "admin status") {
			cmd.Stdout.Write([]byte(fakeJSON))
			return nil
		}
		return nil
	}

	err = o.Pin(5)
	if err == nil {
		t.Fatal("expected error for non-existent deployment index, got nil")
	}
	if !strings.Contains(err.Error(), "deployment with index 5 not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestUnpin_DeploymentNotFound(t *testing.T) {
	root := t.TempDir()

	fakeJSON := `{
		"deployments": [
			{
				"checksum": "abc123",
				"stateroot": "matrixos",
				"booted": true,
				"index": 0,
				"serial": 1
			}
		]
	}`

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
		args := append([]string{cmd.Name}, cmd.Args...)
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "admin status") {
			cmd.Stdout.Write([]byte(fakeJSON))
			return nil
		}
		return nil
	}

	err = o.Unpin(99)
	if err == nil {
		t.Fatal("expected error for non-existent deployment index, got nil")
	}
	if !strings.Contains(err.Error(), "deployment with index 99 not found") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPin_ListDeploymentsError(t *testing.T) {
	root := t.TempDir()

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
		return fmt.Errorf("ostree command failed")
	}

	err = o.Pin(0)
	if err == nil {
		t.Fatal("expected error when ListDeployments fails, got nil")
	}
}

func TestUnpin_ListDeploymentsError(t *testing.T) {
	root := t.TempDir()

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
		return fmt.Errorf("ostree command failed")
	}

	err = o.Unpin(0)
	if err == nil {
		t.Fatal("expected error when ListDeployments fails, got nil")
	}
}

func TestPin_RootError(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	err = o.Pin(0)
	if err == nil {
		t.Fatal("expected error when Root() fails, got nil")
	}
}

func TestUnpin_RootError(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	err = o.Unpin(0)
	if err == nil {
		t.Fatal("expected error when Root() fails, got nil")
	}
}

func TestPin_AlreadyPinned(t *testing.T) {
	root := t.TempDir()

	fakeJSON := `{
		"deployments": [
			{
				"checksum": "abc123",
				"stateroot": "matrixos",
				"booted": true,
				"pinned": true,
				"index": 0,
				"serial": 1
			}
		]
	}`

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
		args := append([]string{cmd.Name}, cmd.Args...)
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "admin status") {
			cmd.Stdout.Write([]byte(fakeJSON))
			return nil
		}
		return nil
	}

	err = o.Pin(0)
	if err == nil {
		t.Fatal("expected error when pinning already-pinned deployment, got nil")
	}
	if !strings.Contains(err.Error(), "is already pinned") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestUnpin_NotPinned(t *testing.T) {
	root := t.TempDir()

	fakeJSON := `{
		"deployments": [
			{
				"checksum": "abc123",
				"stateroot": "matrixos",
				"booted": true,
				"pinned": false,
				"index": 0,
				"serial": 1
			}
		]
	}`

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
		args := append([]string{cmd.Name}, cmd.Args...)
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "admin status") {
			cmd.Stdout.Write([]byte(fakeJSON))
			return nil
		}
		return nil
	}

	err = o.Unpin(0)
	if err == nil {
		t.Fatal("expected error when unpinning a not-pinned deployment, got nil")
	}
	if !strings.Contains(err.Error(), "is not pinned") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPin_OstreeRunError(t *testing.T) {
	root := t.TempDir()

	fakeJSON := `{
		"deployments": [
			{
				"checksum": "abc123",
				"stateroot": "matrixos",
				"booted": true,
				"index": 0,
				"serial": 1
			}
		]
	}`

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Root": {root},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	callCount := 0
	o.runner = func(cmd *runner.Cmd) error {
		callCount++
		args := append([]string{cmd.Name}, cmd.Args...)
		cmdStr := strings.Join(args, " ")
		if strings.Contains(cmdStr, "admin status") {
			cmd.Stdout.Write([]byte(fakeJSON))
			return nil
		}
		// Fail on the pin command
		return fmt.Errorf("pin command failed")
	}

	err = o.Pin(0)
	if err == nil {
		t.Fatal("expected error when ostreeRun fails, got nil")
	}
	if !strings.Contains(err.Error(), "pin command failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestPin_CommandArguments(t *testing.T) {
	root := t.TempDir()

	fakeJSON := `{
		"deployments": [
			{
				"checksum": "abc123",
				"stateroot": "matrixos",
				"booted": true,
				"index": 0,
				"serial": 1
			},
			{
				"checksum": "def456",
				"stateroot": "matrixos",
				"booted": false,
				"pinned": true,
				"index": 1,
				"serial": 0
			},
			{
				"checksum": "ghi789",
				"stateroot": "matrixos",
				"booted": false,
				"index": 2,
				"serial": 0
			}
		]
	}`

	tests := []struct {
		name        string
		targetIndex int
		unpin       bool
		wantArgs    []string
	}{
		{
			name:        "pin index 0",
			targetIndex: 0,
			unpin:       false,
			wantArgs:    []string{"admin", "pin", "--sysroot=" + root, "0"},
		},
		{
			name:        "pin index 2",
			targetIndex: 2,
			unpin:       false,
			wantArgs:    []string{"admin", "pin", "--sysroot=" + root, "2"},
		},
		{
			name:        "unpin index 1",
			targetIndex: 1,
			unpin:       true,
			wantArgs:    []string{"admin", "pin", "--unpin", "--sysroot=" + root, "1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.MockConfig{
				Items: map[string][]string{
					"Ostree.Root": {root},
				},
			}
			o, err := NewOstree(NewOstreeOptions{Config: cfg})
			if err != nil {
				t.Fatalf("NewOstree failed: %v", err)
			}

			var capturedArgs []string
			o.runner = func(cmd *runner.Cmd) error {
				args := append([]string{cmd.Name}, cmd.Args...)
				cmdStr := strings.Join(args, " ")
				if strings.Contains(cmdStr, "admin status") {
					cmd.Stdout.Write([]byte(fakeJSON))
					return nil
				}
				capturedArgs = cmd.Args
				return nil
			}

			if tt.unpin {
				err = o.Unpin(tt.targetIndex)
			} else {
				err = o.Pin(tt.targetIndex)
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(capturedArgs) != len(tt.wantArgs) {
				t.Fatalf("args length mismatch: got %v, want %v", capturedArgs, tt.wantArgs)
			}
			for i, arg := range capturedArgs {
				if arg != tt.wantArgs[i] {
					t.Errorf("arg[%d] = %q, want %q", i, arg, tt.wantArgs[i])
				}
			}
		})
	}
}
