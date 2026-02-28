package runner

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Run / Output / CombinedOutput – real execution with a trivial command
// ---------------------------------------------------------------------------

func TestRun_Echo(t *testing.T) {
	var stdout, stderr bytes.Buffer
	err := Run(&Cmd{Name: "echo", Args: []string{"hello"}, Stdout: &stdout, Stderr: &stderr})
	if err != nil {
		t.Fatalf("Run(echo hello): unexpected error: %v", err)
	}
	if got := strings.TrimSpace(stdout.String()); got != "hello" {
		t.Errorf("stdout = %q, want %q", got, "hello")
	}
}

func TestRun_Failure(t *testing.T) {
	err := Run(&Cmd{Name: "false", Stdout: io.Discard, Stderr: io.Discard})
	if err == nil {
		t.Fatal("Run(false): expected error, got nil")
	}
}

func TestOutput_Echo(t *testing.T) {
	out, err := Output(&Cmd{Name: "echo", Args: []string{"world"}})
	if err != nil {
		t.Fatalf("Output(echo world): unexpected error: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "world" {
		t.Errorf("output = %q, want %q", got, "world")
	}
}

func TestCombinedOutput_Echo(t *testing.T) {
	out, err := CombinedOutput(
		&Cmd{Name: "echo", Args: []string{"combined"}},
	)
	if err != nil {
		t.Fatalf("CombinedOutput(echo combined): unexpected error: %v", err)
	}
	if got := strings.TrimSpace(string(out)); got != "combined" {
		t.Errorf("output = %q, want %q", got, "combined")
	}
}

func TestRun_WithDir(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &Cmd{Name: "pwd", Stdout: &stdout, Stderr: io.Discard}
	cmd.Dir = "/tmp"
	if err := Run(cmd); err != nil {
		t.Fatalf("Run(pwd): unexpected error: %v", err)
	}
	got := strings.TrimSpace(stdout.String())
	// /tmp may be a symlink (e.g. /private/tmp on macOS, /sysroot/tmp on
	// ostree-based systems), so resolve it before comparing.
	want, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		want = "/tmp"
	}
	if got != want {
		t.Errorf("stdout = %q, want %q", got, want)
	}
}

func TestRun_WithEnv(t *testing.T) {
	var stdout bytes.Buffer
	cmd := &Cmd{Name: "env", Stdout: &stdout, Stderr: io.Discard}
	cmd.Env = []string{"MY_TEST_VAR=hello_runner"}
	if err := Run(cmd); err != nil {
		t.Fatalf("Run(env): unexpected error: %v", err)
	}
	if !strings.Contains(stdout.String(), "MY_TEST_VAR=hello_runner") {
		t.Errorf("env output should contain MY_TEST_VAR, got: %s",
			stdout.String())
	}
}

// ---------------------------------------------------------------------------
// chrootArgs
// ---------------------------------------------------------------------------

func TestChrootArgs_Valid(t *testing.T) {
	args, err := chrootArgs("/mnt/root", "/bin/bash", "-c", "ls")
	if err != nil {
		t.Fatalf("chrootArgs: unexpected error: %v", err)
	}

	expected := []string{
		"--pid", "--fork", "--kill-child",
		"--mount", "--uts", "--ipc",
		"--mount-proc=/mnt/root/proc",
		"chroot", "/mnt/root", "/bin/bash",
		"-c", "ls",
	}
	if len(args) != len(expected) {
		t.Fatalf("len(args) = %d, want %d", len(args), len(expected))
	}
	for i := range expected {
		if args[i] != expected[i] {
			t.Errorf("args[%d] = %q, want %q", i, args[i], expected[i])
		}
	}
}

func TestChrootArgs_NoExtraArgs(t *testing.T) {
	args, err := chrootArgs("/chroot", "/bin/sh")
	if err != nil {
		t.Fatalf("chrootArgs: unexpected error: %v", err)
	}
	if args[len(args)-1] != "/bin/sh" {
		t.Errorf("last arg = %q, want %q", args[len(args)-1], "/bin/sh")
	}
}

func TestChrootArgs_EmptyDir(t *testing.T) {
	_, err := chrootArgs("", "/bin/sh")
	if err == nil {
		t.Fatal("expected error for empty chrootDir")
	}
}

func TestChrootArgs_EmptyExec(t *testing.T) {
	_, err := chrootArgs("/mnt", "")
	if err == nil {
		t.Fatal("expected error for empty chrootExec")
	}
}

// ---------------------------------------------------------------------------
// ChrootRun / ChrootOutput – with mocked Run/Output
// ---------------------------------------------------------------------------

func TestChrootRun_DelegatesToRun(t *testing.T) {
	origRun := Run
	defer func() { Run = origRun }()

	var captured struct {
		name string
		args []string
	}
	Run = func(c *Cmd) error {
		captured.name = c.Name
		captured.args = c.Args
		return nil
	}

	err := ChrootRun(&ChrootCmd{
		Cmd: Cmd{
			Name:   "/bin/bash",
			Args:   []string{"-c", "ls"},
			Stdout: io.Discard,
			Stderr: io.Discard,
		},
		ChrootDir: "/mnt",
	})
	if err != nil {
		t.Fatalf("ChrootRun: unexpected error: %v", err)
	}
	if captured.name != "unshare" {
		t.Errorf("command = %q, want %q", captured.name, "unshare")
	}
	if captured.args[len(captured.args)-1] != "ls" {
		t.Errorf("last arg = %q, want %q", captured.args[len(captured.args)-1], "ls")
	}
}

func TestChrootRun_ErrorOnEmptyDir(t *testing.T) {
	err := ChrootRun(&ChrootCmd{
		Cmd: Cmd{Name: "/bin/bash", Stdout: io.Discard, Stderr: io.Discard},
	})
	if err == nil {
		t.Fatal("expected error for empty chrootDir")
	}
}

func TestChrootOutput_DelegatesToOutput(t *testing.T) {
	origOutput := Output
	defer func() { Output = origOutput }()

	Output = func(c *Cmd) ([]byte, error) {
		return []byte("mocked"), nil
	}

	out, err := ChrootOutput(&ChrootCmd{
		Cmd:       Cmd{Name: "/bin/echo"},
		ChrootDir: "/mnt",
	})
	if err != nil {
		t.Fatalf("ChrootOutput: unexpected error: %v", err)
	}
	if string(out) != "mocked" {
		t.Errorf("output = %q, want %q", string(out), "mocked")
	}
}

func TestChrootOutput_ErrorOnEmptyName(t *testing.T) {
	_, err := ChrootOutput(&ChrootCmd{ChrootDir: "/mnt"})
	if err == nil {
		t.Fatal("expected error for empty Name")
	}
}

// ---------------------------------------------------------------------------
// MockRunner basics
// ---------------------------------------------------------------------------

func TestMockRunner_Success(t *testing.T) {
	mr := NewMockRunner()
	err := mr.Run(&Cmd{Name: "cmd", Args: []string{"a", "b"}, Stdout: io.Discard, Stderr: io.Discard})
	if err != nil {
		t.Fatalf("MockRunner.Run: unexpected error: %v", err)
	}
	if len(mr.Calls) != 1 {
		t.Fatalf("len(Calls) = %d, want 1", len(mr.Calls))
	}
	if mr.Calls[0].Name != "cmd" {
		t.Errorf("Calls[0].Name = %q, want %q", mr.Calls[0].Name, "cmd")
	}
}

func TestMockRunner_FailOnCall(t *testing.T) {
	testErr := errors.New("boom")
	mr := NewMockRunnerFailOnCall(1, testErr)

	// Call 0 succeeds
	if err := mr.Run(&Cmd{Name: "a", Stdout: io.Discard, Stderr: io.Discard}); err != nil {
		t.Fatalf("call 0: unexpected error: %v", err)
	}
	// Call 1 fails
	if err := mr.Run(&Cmd{Name: "b", Stdout: io.Discard, Stderr: io.Discard}); !errors.Is(err, testErr) {
		t.Fatalf("call 1: got %v, want %v", err, testErr)
	}
	// Call 2 succeeds
	if err := mr.Run(&Cmd{Name: "c", Stdout: io.Discard, Stderr: io.Discard}); err != nil {
		t.Fatalf("call 2: unexpected error: %v", err)
	}
}

func TestMockRunner_OutputWithData(t *testing.T) {
	mr := NewMockRunnerWithOutput(map[int][]byte{
		0: []byte("first"),
		1: []byte("second"),
	})

	out0, err := mr.Output(&Cmd{Name: "cmd0"})
	if err != nil {
		t.Fatalf("Output call 0: %v", err)
	}
	out1, err := mr.Output(&Cmd{Name: "cmd1"})
	if err != nil {
		t.Fatalf("Output call 1: %v", err)
	}
	if string(out0) != "first" {
		t.Errorf("out0 = %q, want %q", string(out0), "first")
	}
	if string(out1) != "second" {
		t.Errorf("out1 = %q, want %q", string(out1), "second")
	}
}
