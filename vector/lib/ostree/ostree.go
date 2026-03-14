package ostree

import (
	"bytes"
	"bufio"
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

	// Filesystem operations
	SetupEtc(imageDir string) error
	PrepareFilesystemHierarchy(imageDir string) error
	ValidateFilesystemHierarchy(imageDir string) error

	// Repo operations
	BootCommit(sysroot string) (string, error)
	ListRemotes(verbose bool) ([]string, error)
	LastCommit(ref string, verbose bool) (string, error)
	ImportGpgKey(keyPath string) error
	GpgSignFile(file string) error
	GpgKeys() ([]string, error)
	InitializeSigningGpg(verbose bool) error
	InitializeRemoteSigningGpg(remote, repoDir string, verbose bool) error
	MaybeInitializeGpg(verbose bool) error
	MaybeInitializeGpgForRepo(remote, repoDir string, verbose bool) error
	MaybeInitializeRemote(verbose bool) error
	Pull(ref string, verbose bool) error
	PullWithRemote(remote, ref string, verbose bool) error
	Prune(ref string, verbose bool) error
	GenerateStaticDelta(ref string, verbose bool) error
	UpdateSummary(verbose bool) error
	AddRemote(verbose bool) error
	AddRemoteToRootfs(rootfs string, verbose bool) error
	LocalRefs(verbose bool) ([]string, error)
	RemoteRefs(verbose bool) ([]string, error)
	ListDeployments(verbose bool) ([]Deployment, error)
	DeployedRootfs(ref string, verbose bool) (string, error)
	BootedRef(verbose bool) (string, error)
	BootedHash(verbose bool) (string, error)
	Switch(ref string, verbose bool) error
	Deploy(ref, sysroot string, bootArgs []string, verbose bool) error
	Upgrade(args []string, verbose bool) error
	ListPackages(commit string, verbose bool) ([]string, error)
	ListContents(commit, path string, verbose bool) (*[]filesystems.PathInfo, error)
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
	cfg    config.IConfig
	stdout io.Writer
	stderr io.Writer
	runner runner.Func
}

// NewOstreeWithRunner creates a new Ostree instance with a custom command runner (for testing).
type NewOstreeOptions struct {
	Config config.IConfig
	Stdout io.Writer
	Stderr io.Writer
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
		cfg:    opts.Config,
		stdout: stdout,
		stderr: stderr,
		runner: runCommand,
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

// runCmd runs a command via the instance's command runner, adding --verbose
// and the "ostree" binary name automatically.
func (o *Ostree) runCmd(stdout, stderr io.Writer, verbose bool, args ...string) error {
	var finalArgs []string
	if verbose {
		finalArgs = append(finalArgs, "--verbose")
		o.PrintError(">> Executing: ostree --verbose %s\n", strings.Join(args, " "))
	}
	finalArgs = append(finalArgs, args...)
	return o.runner(nil, stdout, stderr, "ostree", finalArgs...)
}

// ostreeRun runs an ostree command with stdout/stderr directed to the instance's stdout/stderr.
func (o *Ostree) ostreeRun(verbose bool, args ...string) error {
	return o.runCmd(o.stdout, o.stderr, verbose, args...)
}

// ostreeRunCapture runs an ostree command and captures its stdout.
func (o *Ostree) ostreeRunCapture(verbose bool, args ...string) (io.Reader, error) {
	if verbose {
		o.PrintError(">> Executing: ostree (stdout capture) %s\n", strings.Join(args, " "))
	}
	stdo := new(bytes.Buffer)
	err := o.runCmd(stdo, o.stderr, false, args...)
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
