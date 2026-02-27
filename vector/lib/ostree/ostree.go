package ostree

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/runner"
	"os"
	"strings"
)

// IOstree defines the interface for ostree operations.
// It mirrors all public methods of Ostree for testability.
type IOstree interface {
	// Styled output
	SetStdout(io.Writer)
	SetStderr(io.Writer)
	Print(format string, a ...interface{})
	PrintError(format string, a ...interface{})

	// Config accessors
	FullBranchSuffix() (string, error)
	IsBranchFullSuffixed(ref string) (bool, error)
	BranchShortnameToFull(shortName, relStage, osName, arch string) (string, error)
	BranchToFull(ref string) (string, error)
	RemoveFullFromBranch(ref string) (string, error)
	GpgEnabled() (bool, error)
	GpgPrivateKeyPath() (string, error)
	GpgPublicKeyPath() (string, error)
	GpgOfficialPubKeyPath() (string, error)
	OsName() (string, error)
	Arch() (string, error)
	RepoDir() (string, error)
	Sysroot() (string, error)
	Root() (string, error)
	Remote() (string, error)
	RemoteURL() (string, error)
	AvailableGpgPubKeyPaths() ([]string, error)
	GpgBestPubKeyPath() (string, error)
	ClientSideGpgArgs() ([]string, error)
	GpgHomeDir() (string, error)
	GpgKeyID() (string, error)
	GpgArgs() ([]string, error)
	SetGpg(enabled bool) error

	// Filesystem operations
	SetupEtc(rootfs string) error
	PrepareFilesystemHierarchy(rootfs string) error
	ValidateFilesystemHierarchy(rootfs string) error

	// Verbosity
	SetVerbose(bool)

	// Repo operations
	InitRepo() error
	BootCommit(sysroot string) (string, error)
	ListRemotes() ([]string, error)
	LastCommit(ref string) (string, error)
	ImportGpgKey(keyPath string) error
	GpgSignFile(file string) error
	GpgKeys() ([]string, error)
	InitializeSigningGpg() error
	InitializeRemoteSigningGpg(remote, repoDir string) error
	MaybeInitializeGpg() error
	MaybeInitializeGpgForRepo(remote, repoDir string) error
	MaybeInitializeRemote() error
	Pull(ref string) error
	PullWithRemote(remote, ref string) error
	Prune(ref string) error
	GenerateStaticDelta(ref string) error
	UpdateSummary() error
	AddRemote() error
	AddRemoteToRootfs(rootfs string) error
	LocalRefs() ([]string, error)
	RemoteRefs() ([]string, error)
	ListDeployments() ([]Deployment, error)
	DeployedRootfs(ref string) (string, error)
	BootedRef() (string, error)
	BootedHash() (string, error)
	Switch(ref string) error
	Deploy(ref, sysroot string, bootArgs []string) error
	Upgrade(args []string) error
	ListPackages(commit string) ([]string, error)
	ListContents(commit, path string) (*[]filesystems.PathInfo, error)
	ListEtcChanges(oldSHA, newSHA string) ([]EtcChange, error)
}

// runCommand runs a generic binary with args and stdout/stderr handling.
var runCommand runner.Func = runner.Run

func readerToList(reader io.Reader) ([]string, error) {
	var elements []string
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		elements = append(elements, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return elements, nil
}

func readerToFirstNonEmptyLine(reader io.Reader) (string, error) {
	scanner := bufio.NewScanner(reader)
	var line string
	for scanner.Scan() {
		line = scanner.Text()
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		break
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return line, nil
}

// SetupEnvironment sets the LC_TIME environment variable to "C".
// This is to ensure that Cloudflare can correctly process requests
// without throwing HTTP 400 errors.
func SetupEnvironment() {
	os.Setenv("LC_TIME", "C")
}

type Ostree struct {
	cfg     config.IConfig
	stdout  io.Writer
	stderr  io.Writer
	runner  runner.Func
	verbose bool
}

// NewOstreeWithRunner creates a new Ostree instance with a custom command runner (for testing).
type NewOstreeOptions struct {
	Config  config.IConfig
	Stdout  io.Writer
	Stderr  io.Writer
	Verbose bool
}

// NewOstree creates a new Ostree instance.
func NewOstree(opts NewOstreeOptions) (*Ostree, error) {
	if opts.Config == nil {
		return nil, errors.New("missing config parameter")
	}
	stdout := opts.Stdout
	if stdout == nil {
		stdout = os.Stdout
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = os.Stderr
	}
	return &Ostree{
		cfg:     opts.Config,
		stdout:  stdout,
		stderr:  stderr,
		runner:  runCommand,
		verbose: opts.Verbose,
	}, nil
}

// SetStdout sets the stdout writer for the Ostree instance.
func (o *Ostree) SetStdout(w io.Writer) {
	o.stdout = w
}

// SetStderr sets the stderr writer for the Ostree instance.
func (o *Ostree) SetStderr(w io.Writer) {
	o.stderr = w
}

// Print prints to stdout with the given format and arguments.
func (o *Ostree) Print(format string, a ...interface{}) {
	fmt.Fprintf(o.stdout, format, a...)
}

// PrintError prints to stderr with the given format and arguments.
func (o *Ostree) PrintError(format string, a ...interface{}) {
	fmt.Fprintf(o.stderr, format, a...)
}

// SetVerbose sets the verbose flag for the Ostree instance.
func (o *Ostree) SetVerbose(v bool) {
	o.verbose = v
}

// runCmd runs a command via the instance's command runner, adding --verbose
// and the "ostree" binary name automatically.
func (o *Ostree) runCmd(stdout, stderr io.Writer, args ...string) error {
	var finalArgs []string
	if o.verbose {
		finalArgs = append(finalArgs, "--verbose")
		o.PrintError(">> Executing: ostree --verbose %s\n", strings.Join(args, " "))
	}
	finalArgs = append(finalArgs, args...)
	return o.runner(nil, stdout, stderr, "ostree", finalArgs...)
}

// ostreeRun runs an ostree command with stdout/stderr directed to the instance's stdout/stderr.
func (o *Ostree) ostreeRun(args ...string) error {
	return o.runCmd(o.stdout, o.stderr, args...)
}

// ostreeRunCapture runs an ostree command and captures its stdout.
func (o *Ostree) ostreeRunCapture(args ...string) (io.Reader, error) {
	if o.verbose {
		o.PrintError(">> Executing: ostree (stdout capture) %s\n", strings.Join(args, " "))
	}
	stdo := new(bytes.Buffer)
	verbose := o.verbose
	o.verbose = false
	err := o.runCmd(stdo, o.stderr, args...)
	o.verbose = verbose
	return stdo, err
}

func run(stdout, stderr io.Writer, verbose bool, args ...string) error {
	var finalArgs []string
	if verbose {
		finalArgs = append(finalArgs, "--verbose")
		fmt.Fprintf(stderr, ">> Executing: ostree --verbose %s\n", strings.Join(args, " "))
	}
	finalArgs = append(finalArgs, args...)
	return runCommand(nil, stdout, stderr, "ostree", finalArgs...)
}

// Run runs an ostree command with --verbose if requested.
var Run = func(verbose bool, args ...string) error {
	return run(os.Stdout, os.Stderr, verbose, args...)
}

// RunWithStdoutCapture runs an ostree command and captures its stdout,
// with --verbose if requested.
var RunWithStdoutCapture = func(verbose bool, args ...string) (io.Reader, error) {
	if verbose {
		fmt.Fprintf(os.Stderr, ">> Executing: ostree (stdout capture) %s\n", strings.Join(args, " "))
	}
	stdo := new(bytes.Buffer)
	err := run(stdo, os.Stderr, false /* do not run ostree with verbose! */, args...)
	return stdo, err
}

var pathExists = filesystems.PathExists
var fileExists = filesystems.FileExists
var directoryExists = filesystems.DirectoryExists
