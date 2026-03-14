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
//	unshare --pid --fork --kill-child --mount --uts --ipc --wd=<Dir> \
//	    --mount-proc=<chrootDir>/proc chroot <chrootDir> <chrootExec> [args...]
func chrootArgs(c *ChrootCmd) ([]string, error) {
	if c.ChrootDir == "" {
		return nil, fmt.Errorf("missing chrootDir parameter")
	}
	if c.Name == "" {
		return nil, fmt.Errorf("missing chrootExec parameter")
	}

	var dirArgs []string
	if c.Dir != "" {
		dirArgs = []string{"--wd", c.Dir}
	}

	unshareArgs := []string{
		"--pid",
		"--fork",
		"--kill-child",
		"--mount",
		"--uts",
		"--ipc",
		fmt.Sprintf("--mount-proc=%s/proc", c.ChrootDir),
	}
	unshareArgs = append(unshareArgs, dirArgs...)

	execArgs := []string{
		"chroot",
		c.ChrootDir,
		c.Name,
	}

	var cmdArgs []string
	cmdArgs = append(cmdArgs, unshareArgs...)
	cmdArgs = append(cmdArgs, execArgs...)
	cmdArgs = append(cmdArgs, c.Args...)
	return cmdArgs, nil
}

// DirRunFunc is a function type that executes an external command with the
// working directory set to dir. It mirrors Func but adds a dir parameter
// that maps to exec.Cmd.Dir.
type DirRunFunc func(dir string, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error

// DirRun is the default DirRunFunc implementation. It executes the named
// program with the given arguments, setting the working directory to dir.
var DirRun DirRunFunc = func(dir string, stdin io.Reader, stdout, stderr io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}

// ChrootRunFunc is a function type that executes a command inside a chroot
// via unshare, wiring stdin/stdout/stderr to the supplied writers.
type ChrootRunFunc func(cmd *ChrootCmd) error

// ChrootOutputFunc is a function type that executes a command inside a chroot
// via unshare and returns its standard output.
type ChrootOutputFunc func(cmd *ChrootCmd) ([]byte, error)

// ChrootRun is the default ChrootRunFunc implementation.
var ChrootRun ChrootRunFunc = func(c *ChrootCmd) error {
	uArgs, err := chrootArgs(c)
	if err != nil {
		return err
	}
	return Run(&Cmd{
		Name:   "unshare",
		Args:   uArgs,
		Env:    c.Env,
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
	})
}

// ChrootOutput is the default ChrootOutputFunc implementation.
var ChrootOutput ChrootOutputFunc = func(c *ChrootCmd) ([]byte, error) {
	uArgs, err := chrootArgs(c)
	if err != nil {
		return nil, err
	}
	return Output(&Cmd{
		Name:  "unshare",
		Args:  uArgs,
		Env:   c.Env,
		Stdin: c.Stdin,
	})
}
