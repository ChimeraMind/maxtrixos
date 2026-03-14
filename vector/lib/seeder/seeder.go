// Package seeder provides seed detection, locking, and execution utilities
// for managing matrixOS seeders from outside their root filesystem.
// It is the Go port of build/seeders/lib/seeders_lib.sh.
package seeder

import (
	"errors"
	"io"
	"os"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/runner"
)

// Compile-time interface check.
var _ ISeeder = (*Seeder)(nil)

// SeederFilterFunc decides whether a seeder should be skipped.
// Returns true to skip, false to include.
type SeederFilterFunc func(name string) bool

// SeederInfo holds the resolved paths for a single discovered seeder.
type SeederInfo struct {
	Name        string // e.g. "00-bedrock"
	Dir         string // absolute directory path
	ChrootExec  string // path to the chroot executable
	PrepperExec string // path to the prepper executable
}

// NewSeederOptions contains options for creating a new Seeder.
type NewSeederOptions struct {
	Verbose bool // show detailed output
}

// ISeeder defines the interface for seeder operations.
// It mirrors all public methods of Seeder for testability.
type ISeeder interface {
	// SetStdout replaces the writer used for informational output.
	SetStdout(w io.Writer)
	// SetStderr replaces the writer used for warnings and errors.
	SetStderr(w io.Writer)
	// Stdout returns the current informational output writer.
	Stdout() io.Writer
	// Stderr returns the current warning/error output writer.
	Stderr() io.Writer

	// Print writes a formatted informational message to stdout.
	Print(format string, args ...any)
	// PrintWarning writes a formatted warning message to stderr.
	PrintWarning(format string, args ...any)
	// PrintError writes a formatted error/diagnostic message to stderr.
	PrintError(format string, args ...any)

	// ChrootSeedersDir returns the base seeders directory path.
	ChrootSeedersDir() (string, error)
	// ChrootBuildArtifactsDir returns the directory path for build artifacts inside the chroot.
	ChrootBuildArtifactsDir() (string, error)
	// DisabledSeederFile returns the sentinel file name that disables a seeder directory.
	DisabledSeederFile() (string, error)
	// UseLocalGitRepoInsideChroot returns whether to use a local git repository inside the chroot.
	UseLocalGitRepoInsideChroot() (bool, error)
	// DeleteDotGitFromGitRepo returns whether to delete the .git directory from the git repository
	// when copying it into the chroot.
	DeleteDotGitFromGitRepo() (bool, error)
	// ChrootExecName returns the name of the chroot executable inside each seeder directory.
	ChrootExecName() (string, error)
	// ParamsExecutableName returns the name of the params executable inside each seeder directory.
	ParamsExecutableName() (string, error)
	// PrepperExecName returns the name of the prepper executable inside each seeder directory.
	PrepperExecName() (string, error)
	// PhasesStateDir returns the chroot-side directory for seeder phase checkpoints.
	PhasesStateDir() (string, error)
	// SeederDoneFlagFilePrefix returns the prefix for done-flag files.
	SeederDoneFlagFilePrefix() (string, error)
	// PrivateExampleGitRepo returns the URL for the private example git repository.
	PrivateExampleGitRepo() (string, error)
	// PrivateGitRepoPath returns the local path for the private git repository.
	PrivateGitRepoPath() (string, error)
	// LockDir returns the directory where seeder file locks are stored.
	LockDir() (string, error)
	// LockWaitSeconds returns the configured lock acquisition timeout in seconds.
	LockWaitSeconds() (string, error)

	// RetryableCmd executes the command up to tries times, sleeping 5 seconds
	// between attempts. Returns nil on the first successful invocation, or the
	// last error after all retries are exhausted.
	RetryableCmd(tries int, name string, args ...string) error
	// MaybeInitializePrivateRepo ensures the private example repository exists
	// and is built. It clones the repo from PrivateExampleGitRepo if missing,
	// or runs ./make.sh if the .built marker is absent.
	MaybeInitializePrivateRepo() error

	// SeederLockDir returns the seeder lock directory, creating it if necessary.
	SeederLockDir() (string, error)
	// SeederLockPath returns the lock file path for the given seeder name.
	SeederLockPath(name string) (string, error)
	// ExecuteWithSeederLock acquires an exclusive file lock for the given seeder name,
	// executes fn under that lock, and releases the lock when fn returns.
	// If the lock cannot be acquired within the configured timeout, an error is returned.
	// If fn panics or the process crashes, the OS closes the file descriptor and
	// releases the lock automatically.
	ExecuteWithSeederLock(name string, fn func() error) error
}

// Seeder provides seed detection and manipulation operations.
type Seeder struct {
	cfg       config.IConfig
	runner    runner.Func
	dirRunner runner.DirRunFunc
	stdout    io.Writer
	stderr    io.Writer

	verbose bool
}

// NewSeeder creates a new Seeder instance.
func NewSeeder(cfg config.IConfig, opts *NewSeederOptions) (*Seeder, error) {
	if cfg == nil {
		return nil, errors.New("missing config parameter")
	}
	if opts == nil {
		opts = &NewSeederOptions{}
	}
	return &Seeder{
		cfg:       cfg,
		runner:    runner.Run,
		dirRunner: runner.DirRun,
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		verbose:   opts.Verbose,
	}, nil
}
