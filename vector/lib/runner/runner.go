// Package runner provides a shared command execution abstraction for running
// external programs, plus test helpers (MockRunner) for unit testing.
package runner

import (
	"fmt"
	"io"
	"os/exec"
)

// Cmd describes an external command to execute. Zero-value fields are
// treated as "inherit from parent" or "not set":
//   - Dir=""      → inherit working directory
//   - Env=nil     → inherit environment
//   - Stdin=nil   → no standard input
//   - Stdout=nil  → output discarded
//   - Stderr=nil  → output discarded
type Cmd struct {
	Name   string
	Args   []string
	Dir    string   // working directory; empty inherits parent
	Env    []string // environment; nil inherits parent
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// Func is the canonical function type for executing an external command.
// Consumers store a value of this type and call it to run shell commands;
// tests replace it with MockRunner.Run (or a custom closure) to avoid
// real process execution.
type Func func(cmd *Cmd) error

// OutputFunc is a function type that executes an external command and
// returns its standard output. It mirrors the (*exec.Cmd).Output() pattern.
// Tests can replace the default with a mock to avoid real process execution.
type OutputFunc func(cmd *Cmd) ([]byte, error)

// CombinedOutputFunc is a function type that executes an external command
// and returns its combined standard output and standard error. It mirrors
// the (*exec.Cmd).CombinedOutput() pattern.
// Tests can replace the default with a mock to avoid real process execution.
type CombinedOutputFunc func(cmd *Cmd) ([]byte, error)

// Run is the default Func implementation. It executes the named program
// with the given arguments, wiring all Cmd fields to the underlying
// exec.Cmd.
var Run Func = func(c *Cmd) error {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.Stdin = c.Stdin
	cmd.Stdout = c.Stdout
	cmd.Stderr = c.Stderr
	return cmd.Run()
}

// Output is the default OutputFunc implementation. It executes the named
// program and returns its standard output, mirroring (*exec.Cmd).Output().
var Output OutputFunc = func(c *Cmd) ([]byte, error) {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.Stdin = c.Stdin
	return cmd.Output()
}

// CombinedOutput is the default CombinedOutputFunc implementation. It
// executes the named program and returns its combined stdout and stderr,
// mirroring (*exec.Cmd).CombinedOutput().
var CombinedOutput CombinedOutputFunc = func(c *Cmd) ([]byte, error) {
	cmd := exec.Command(c.Name, c.Args...)
	cmd.Dir = c.Dir
	cmd.Env = c.Env
	cmd.Stdin = c.Stdin
	return cmd.CombinedOutput()
}

// chrootArgs builds the unshare argument list for running a command inside
// a chroot. It preserves the exact invocation pattern:
//
//	unshare --pid --fork --kill-child --mount --uts --ipc \
//	    --mount-proc=<chrootDir>/proc chroot <chrootDir> <chrootExec> [args...]
func chrootArgs(chrootDir, chrootExec string, args ...string) ([]string, error) {
	if chrootDir == "" {
		return nil, fmt.Errorf("missing chrootDir parameter")
	}
	if chrootExec == "" {
		return nil, fmt.Errorf("missing chrootExec parameter")
	}

	cmdArgs := []string{
		"--pid",
		"--fork",
		"--kill-child",
		"--mount",
		"--uts",
		"--ipc",
		fmt.Sprintf("--mount-proc=%s/proc", chrootDir),
		"chroot",
		chrootDir,
		chrootExec,
	}
	cmdArgs = append(cmdArgs, args...)
	return cmdArgs, nil
}

// ChrootCmd describes a command to execute inside a chroot via unshare.
// It embeds Cmd (where Name is the executable to run inside the chroot)
// and adds the chroot-specific ChrootDir field. The unshare argument
// list is built automatically by the default ChrootRun / ChrootOutput
// implementations.
type ChrootCmd struct {
	Cmd
	ChrootDir string // root directory for the chroot
}

// ChrootRunFunc is a function type that executes a command inside a chroot
// via unshare, wiring stdin/stdout/stderr to the supplied writers.
type ChrootRunFunc func(cmd *ChrootCmd) error

// ChrootOutputFunc is a function type that executes a command inside a chroot
// via unshare and returns its standard output.
type ChrootOutputFunc func(cmd *ChrootCmd) ([]byte, error)

// ChrootRun is the default ChrootRunFunc implementation.
var ChrootRun ChrootRunFunc = func(c *ChrootCmd) error {
	uArgs, err := chrootArgs(c.ChrootDir, c.Name, c.Args...)
	if err != nil {
		return err
	}
	return Run(&Cmd{Name: "unshare", Args: uArgs, Stdin: c.Stdin, Stdout: c.Stdout, Stderr: c.Stderr})
}

// ChrootOutput is the default ChrootOutputFunc implementation.
var ChrootOutput ChrootOutputFunc = func(c *ChrootCmd) ([]byte, error) {
	uArgs, err := chrootArgs(c.ChrootDir, c.Name, c.Args...)
	if err != nil {
		return nil, err
	}
	return Output(&Cmd{Name: "unshare", Args: uArgs})
}
