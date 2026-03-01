package releaser

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/runner"
	"matrixos/vector/lib/validation"
)

// Compile-time interface check.
var _ IRelease = (*Releaser)(nil)

// ReleaseStage represents a release stage (dev or prod).
type ReleaseStage string

const (
	StageDev  ReleaseStage = "dev"
	StageProd ReleaseStage = "prod"
)

// ValidateReleaseStage validates a release stage string.
func ValidateReleaseStage(stage string) (ReleaseStage, error) {
	switch ReleaseStage(stage) {
	case StageDev, StageProd:
		return ReleaseStage(stage), nil
	default:
		return "", fmt.Errorf("unknown release stage: %s", stage)
	}
}

// CommitOptions controls how an ostree commit is performed.
type CommitOptions struct {
	Branch       string // ostree branch to commit to
	ParentBranch string // parent branch (empty for root branches)
	Consume      bool   // --consume flag for ostree commit
}

// RefFilterFunc decides whether a ref should be skipped.
// Returns true to skip, false to include.
type RefFilterFunc func(ref string) bool

// NewReleaserOptions contains options for creating a new Releaser.
type NewReleaserOptions struct {
	ChrootDir string // source chroot directory
	ImageDir  string // destination image directory
	Ref       string // ostree ref (branch) to operate on
	Verbose   bool   // show detailed output
}

// IRelease defines the interface for release operations.
// It mirrors all public methods of Releaser for testability.
type IRelease interface {
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

	// SetChrootDir sets the source chroot directory.
	SetChrootDir(dir string)
	// ChrootDir returns the source chroot directory.
	ChrootDir() string
	// SetImageDir sets the destination image directory.
	// It validates that dir is a non-empty existing directory.
	SetImageDir(dir string) error
	// ImageDir returns the destination image directory.
	ImageDir() string
	// SetRef sets the ostree ref (branch).
	SetRef(ref string)
	// Ref returns the ostree ref (branch).
	Ref() string

	// Hostname returns the configured release hostname.
	Hostname() (string, error)
	// HooksDir returns the directory where per-branch release hooks live.
	HooksDir() (string, error)
	// DevDir returns the matrixOS dev directory (Root).
	DevDir() (string, error)
	// UseCpReflink returns whether cp --reflink=auto should be used instead of rsync.
	UseCpReflink() (bool, error)
	// ReadOnlyVdb returns the path used for the read-only Portage vardb.
	ReadOnlyVdb() (string, error)
	// LockDir returns the directory where releaser file locks are stored.
	LockDir() (string, error)
	// LockWaitSeconds returns the configured lock acquisition timeout.
	LockWaitSeconds() (string, error)
	// GenerateStaticDeltas returns whether ostree static deltas should be generated.
	GenerateStaticDeltas() (bool, error)
	// SecureBootCertPath returns the path to the SecureBoot db certificate.
	SecureBootCertPath() (string, error)
	// SecureBootKekPath returns the path to the SecureBoot KEK certificate.
	SecureBootKekPath() (string, error)
	// PrivateGitRepoPath returns the private git repo path.
	PrivateGitRepoPath() (string, error)
	// DefaultPrivateGitRepoPath returns the default private git repo path (used inside chroots).
	DefaultPrivateGitRepoPath() (string, error)
	// BuildMetadataFile returns the seeder build metadata file path.
	BuildMetadataFile() (string, error)
	// ServicesDir returns the directory where per-branch systemd service configs live.
	ServicesDir() (string, error)

	// CheckMatrixOS validates the matrixOS development environment.
	CheckMatrixOS() error
	// SyncFilesystem synchronises the chroot directory into the image directory
	// using either cp --reflink=auto or rsync.
	SyncFilesystem() error
	// PreCleanQAChecks runs pre-clean quality assurance checks on the image directory.
	PreCleanQAChecks() error
	// CleanRootfs cleans the image directory rootfs for release.
	CleanRootfs() error
	// SetupHostname configures the hostname inside the image directory.
	SetupHostname() error
	// SetupServices configures systemd services inside the image directory
	// based on the per-ref services configuration file.
	SetupServices() error
	// ReleaseHook runs the per-ref release hook script, if one exists.
	ReleaseHook() error
	// PostCleanShrink removes unnecessary development artifacts to save space.
	PostCleanShrink() error

	// OstreePrepare prepares and validates the filesystem hierarchy for OSTree.
	OstreePrepare() error
	// MaybeOstreeInit initialises the local ostree repository if it does not already exist.
	MaybeOstreeInit() error
	// SymlinkEtc creates a /etc -> usr/etc symlink in the image directory
	// to prevent emerge from recreating /etc during post-clean.
	SymlinkEtc() error
	// UnlinkEtc removes the /etc symlink before an ostree commit.
	UnlinkEtc() error
	// AddExtraDotDotToUsrEtcPortage adjusts the /usr/etc/portage symlink
	// after /etc has been moved to /usr/etc, adding an extra "../" prefix.
	AddExtraDotDotToUsrEtcPortage() error
	// RemoveExtraDotDotFromUsrEtcPortage removes the extra "../" prefix from the
	// /usr/etc/portage symlink so it works after client-side deployment.
	RemoveExtraDotDotFromUsrEtcPortage() error

	// Release commits the image directory to the ostree repository.
	Release(opts CommitOptions) error

	// ReleaseLockDir returns the lock directory, creating it if necessary.
	ReleaseLockDir() (string, error)
	// ReleaseLockPath returns the lock file path for the given release name.
	ReleaseLockPath(name string) (string, error)
	// ExecuteWithReleaseLock acquires an exclusive file lock for the given release name,
	// executes fn under that lock, and releases the lock when fn returns.
	ExecuteWithReleaseLock(name string, fn func() error) error

	// Cleanup unmounts all mount points tracked by this Releaser instance
	// in reverse order. It is safe to call multiple times.
	Cleanup()
}

// Releaser provides release creation and manipulation operations.
type Releaser struct {
	cfg    config.IConfig
	ostree ostree.IOstree
	runner runner.Func
	stdout io.Writer
	stderr io.Writer

	chrootDir string // source chroot directory
	imageDir  string // destination image directory
	ref       string // ostree ref (branch) to operate on
	verbose   bool   // show detailed output

	// QA validation instance.
	qa *validation.QA

	// trackedMounts records every mount point created by this Releaser
	// so that Cleanup can attempt to unmount them all on failure or signal.
	trackedMountsMu sync.Mutex
	trackedMounts   []string
}

// NewReleaser creates a new Releaser instance.
func NewReleaser(cfg config.IConfig, ot ostree.IOstree, opts *NewReleaserOptions) (*Releaser, error) {
	if cfg == nil {
		return nil, errors.New("missing config parameter")
	}
	if ot == nil {
		return nil, errors.New("missing ostree parameter")
	}
	if opts == nil {
		return nil, errors.New("missing options parameter")
	}
	if opts.ChrootDir == "" {
		return nil, errors.New("missing ChrootDir in options")
	}
	if opts.ImageDir == "" {
		return nil, errors.New("missing ImageDir in options")
	}
	if opts.Ref == "" {
		return nil, errors.New("missing Ref in options")
	}
	if err := checkImageDir(opts.ImageDir); err != nil {
		return nil, fmt.Errorf("invalid ImageDir: %w", err)
	}

	qa, err := validation.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize QA: %w", err)
	}

	return &Releaser{
		cfg:       cfg,
		ostree:    ot,
		runner:    runner.Run,
		qa:        qa,
		stdout:    os.Stdout,
		stderr:    os.Stderr,
		chrootDir: opts.ChrootDir,
		imageDir:  opts.ImageDir,
		ref:       opts.Ref,
		verbose:   opts.Verbose,
	}, nil
}

// checkImageDir validates that imageDir is a non-empty existing directory.
func checkImageDir(imageDir string) error {
	if imageDir == "" {
		return errors.New("imageDir is empty")
	}
	if !filesystems.DirectoryExists(imageDir) {
		return errors.New("imageDir not found")
	}
	return nil
}
