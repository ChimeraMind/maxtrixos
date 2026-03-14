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
	// I/O writers - override to customise output rendering.
	SetStdout(w io.Writer)
	SetStderr(w io.Writer)
	Stdout() io.Writer
	Stderr() io.Writer

	// Structured output helpers.
	Print(format string, args ...any)
	PrintWarning(format string, args ...any)
	PrintError(format string, args ...any)

	// Directory accessors
	SetChrootDir(dir string)
	ChrootDir() string
	SetImageDir(dir string) error
	ImageDir() string
	SetRef(ref string)
	Ref() string

	// Config accessors
	Hostname() (string, error)
	HooksDir() (string, error)
	UseCpReflink() (bool, error)
	ReadOnlyVdb() (string, error)
	LockDir() (string, error)
	LockWaitSeconds() (string, error)
	GenerateStaticDeltas() (bool, error)
	SecureBootCertPath() (string, error)
	SecureBootKekPath() (string, error)
	PrivateGitRepoPath() (string, error)
	DefaultPrivateGitRepoPath() (string, error)
	BuildMetadataFile() (string, error)
	ServicesDir() (string, error)

	// Pre-release operations
	CheckMatrixOS() error
	SyncFilesystem() error
	PreCleanQAChecks() error
	CleanRootfs() error
	SetupHostname() error
	SetupServices() error
	ReleaseHook() error
	PostCleanShrink() error

	// OSTree FHS manipulation
	OstreePrepare() error
	MaybeOstreeInit() error
	SymlinkEtc() error
	UnlinkEtc() error
	AddExtraDotDotToUsrEtcPortage() error
	RemoveExtraDotDotFromUsrEtcPortage() error

	// Core commit operation
	Release(opts CommitOptions) error

	// Detection
	DetectLocalReleases(skip, only RefFilterFunc) ([]string, error)
	DetectRemoteReleases(skip, only RefFilterFunc) ([]string, error)

	// Locking
	ReleaseLockDir() (string, error)
	ReleaseLockPath(name string) (string, error)
	ExecuteWithReleaseLock(name string, fn func() error) error

	// Lifecycle
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
