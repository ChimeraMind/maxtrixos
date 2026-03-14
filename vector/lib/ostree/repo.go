package ostree

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
)

type AddRemoteOptions struct {
	Remote    string
	RemoteURL string
	GpgArgs   []string
	RepoDir   string
	Sysroot   string
	Verbose   bool
}

// InitRepo initialises the local ostree repository in archive mode.
// The collection ID is read from the Ostree.CollectionId config key;
// if set, it is passed as --collection-id.
func (o *Ostree) InitRepo(verbose bool) error {
	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}

	args := []string{"--repo=" + repoDir, "init", "--mode=archive"}

	collectionID, _ := o.cfg.GetItem("Ostree.CollectionId")
	if collectionID != "" {
		collArgs, err := CollectionIDArgs(collectionID)
		if err != nil {
			return err
		}
		args = append(args, collArgs...)
	}

	return o.ostreeRun(verbose, args...)
}

// CollectionIDArgs returns the ostree --collection-id argument if a collection ID is provided.
func CollectionIDArgs(collectionID string) ([]string, error) {
	if collectionID == "" {
		return nil, errors.New("missing collectionID parameter")
	}

	var args []string
	if collectionID != "" {
		args = append(args, "--collection-id="+collectionID)
	}
	return args, nil
}

// ListRemotes lists the remotes in an ostree repository.
func ListRemotes(repoDir string, verbose bool) ([]string, error) {
	if repoDir == "" {
		return nil, errors.New("invalid repoDir parameter")
	}
	stdout, err := RunWithStdoutCapture(
		verbose,
		"--repo="+repoDir,
		"remote",
		"list",
	)
	if err != nil {
		return nil, err
	}
	return readerToList(stdout)
}

// ListLocalRefs lists the local refs in an ostree repo.
func ListLocalRefs(repoDir string, verbose bool) ([]string, error) {
	if repoDir == "" {
		return nil, errors.New("invalid repoDir parameter")
	}
	stdout, err := RunWithStdoutCapture(
		verbose,
		"--repo="+repoDir,
		"refs",
	)
	if err != nil {
		return nil, err
	}
	refs, err := readerToList(stdout)
	if err != nil {
		return nil, err
	}

	refDeleter := func(ref string) bool {
		// Remove ostree-metadata from list.
		if ref == "ostree-metadata" {
			return true
		}
		return false
	}

	return slices.DeleteFunc(refs, refDeleter), nil
}

func AddRemoteWithOptions(opts AddRemoteOptions, verbose bool) error {
	if opts.Remote == "" {
		return errors.New("invalid Remote parameter")
	}
	if opts.RemoteURL == "" {
		return errors.New("invalid RemoteURL parameter")
	}
	if opts.RepoDir != "" && !directoryExists(opts.RepoDir) {
		return fmt.Errorf("repoDir %s does not exist", opts.RepoDir)
	}
	if opts.Sysroot != "" && !directoryExists(opts.Sysroot) {
		return fmt.Errorf("sysroot %s does not exist", opts.Sysroot)
	}
	args := []string{
		"remote",
		"add",
	}
	if opts.Sysroot != "" {
		args = append(args, "--sysroot="+opts.Sysroot)
	}
	if opts.RepoDir != "" {
		args = append(args, "--repo="+opts.RepoDir)
	}

	args = append(args, "--force")
	args = append(args, opts.GpgArgs...)
	args = append(args, opts.Remote, opts.RemoteURL)
	return Run(verbose, args...)
}

// ListRemoteRefs lists the remote refs present in the given remote.
func ListRemoteRefs(repoDir, remote string, verbose bool) ([]string, error) {
	if repoDir == "" {
		return nil, errors.New("invalid repoDir parameter")
	}
	if remote == "" {
		return nil, errors.New("invalid remote parameter")
	}
	stdout, err := RunWithStdoutCapture(
		verbose,
		"--repo="+repoDir,
		"remote",
		"refs",
		remote,
	)
	if err != nil {
		return nil, err
	}
	return readerToList(stdout)
}

// LastCommit returns the commit hash of the latest commit in the given ref.
func LastCommit(repoDir, ref string, verbose bool) (string, error) {
	if repoDir == "" {
		return "", errors.New("invalid repoDir parameter")
	}
	if ref == "" {
		return "", errors.New("invalid ref parameter")
	}

	stdout, err := RunWithStdoutCapture(
		verbose,
		"rev-parse",
		"--repo="+repoDir,
		ref,
	)
	if err != nil {
		return "", err
	}
	lines, err := readerToList(stdout)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("no commit found for ref %s", ref)
	}
	return lines[0], nil
}

// PullWithRemote runs `ostree pull` assuming that the provided ref is
// clean from the remote prefix.
func PullWithRemote(repoDir, remote, ref string, verbose bool) error {
	if repoDir == "" {
		return errors.New("invalid repoDir parameter")
	}
	if remote == "" {
		return errors.New("invalid remote parameter")
	}
	if ref == "" {
		return errors.New("invalid ref parameter")
	}

	fmt.Printf("Pulling ostree from %s %s:%s ...\n", repoDir, remote, ref)
	return Run(verbose, "--repo="+repoDir, "pull", remote, ref)
}

// Pull pulls an ostree ref from a remote using `ostree pull`.
func Pull(repoDir, ref string, verbose bool) error {
	if repoDir == "" {
		return errors.New("invalid repoDir parameter")
	}
	if ref == "" {
		return errors.New("invalid ref parameter")
	}

	remote := ExtractRemoteFromRef(ref)
	if remote == "" {
		return fmt.Errorf("%v does not contain the remote: prefix (e.g. origin:)", ref)
	}
	ref = CleanRemoteFromRef(ref)
	return PullWithRemote(repoDir, remote, ref, verbose)
}

// Prune runs `ostree prune` for the given ref in the given repo.
func Prune(repoDir, ref, keepObjectsYoungerThan string, verbose bool) error {
	if repoDir == "" {
		return errors.New("invalid repoDir parameter")
	}
	if ref == "" {
		return errors.New("invalid ref parameter")
	}
	if keepObjectsYoungerThan == "" {
		return errors.New("invalid keepObjectsYoungerThan parameter")
	}

	fmt.Printf("Pruning ostree repo for %s ...\n", repoDir)
	return Run(verbose,
		"--repo="+repoDir,
		"prune",
		"--depth=5",
		"--refs-only",
		"--keep-younger-than="+keepObjectsYoungerThan,
		"--only-branch="+ref,
	)
}

// listRemotesFromRepo lists remotes using the instance runner.
func (o *Ostree) listRemotesFromRepo(repoDir string, verbose bool) ([]string, error) {
	if repoDir == "" {
		return nil, errors.New("invalid repoDir parameter")
	}
	stdout, err := o.ostreeRunCapture(verbose, "--repo="+repoDir, "remote", "list")
	if err != nil {
		return nil, err
	}
	return readerToList(stdout)
}

// lastCommitFromRepo returns the last commit for a ref using the instance runner.
func (o *Ostree) lastCommitFromRepo(repoDir, ref string, verbose bool) (string, error) {
	if repoDir == "" {
		return "", errors.New("invalid repoDir parameter")
	}
	if ref == "" {
		return "", errors.New("invalid ref parameter")
	}
	stdout, err := o.ostreeRunCapture(verbose, "rev-parse", "--repo="+repoDir, ref)
	if err != nil {
		return "", err
	}
	lines, err := readerToList(stdout)
	if err != nil {
		return "", err
	}
	if len(lines) == 0 {
		return "", fmt.Errorf("no commit found for ref %s", ref)
	}
	return lines[0], nil
}

// listLocalRefsFromRepo lists local refs using the instance runner.
func (o *Ostree) listLocalRefsFromRepo(repoDir string, verbose bool) ([]string, error) {
	if repoDir == "" {
		return nil, errors.New("invalid repoDir parameter")
	}
	stdout, err := o.ostreeRunCapture(verbose, "--repo="+repoDir, "refs")
	if err != nil {
		return nil, err
	}
	return readerToList(stdout)
}

// listRemoteRefsFromRepo lists remote refs using the instance runner.
func (o *Ostree) listRemoteRefsFromRepo(repoDir, remote string, verbose bool) ([]string, error) {
	if repoDir == "" {
		return nil, errors.New("invalid repoDir parameter")
	}
	if remote == "" {
		return nil, errors.New("invalid remote parameter")
	}
	stdout, err := o.ostreeRunCapture(verbose, "--repo="+repoDir, "remote", "refs", remote)
	if err != nil {
		return nil, err
	}
	return readerToList(stdout)
}

// addRemote adds a remote using the instance runner.
func (o *Ostree) addRemote(opts AddRemoteOptions, verbose bool) error {
	if opts.Remote == "" {
		return errors.New("invalid Remote parameter")
	}
	if opts.RemoteURL == "" {
		return errors.New("invalid RemoteURL parameter")
	}
	if opts.RepoDir != "" && !directoryExists(opts.RepoDir) {
		return fmt.Errorf("repoDir %s does not exist", opts.RepoDir)
	}
	if opts.Sysroot != "" && !directoryExists(opts.Sysroot) {
		return fmt.Errorf("sysroot %s does not exist", opts.Sysroot)
	}
	args := []string{"remote", "add"}
	if opts.Sysroot != "" {
		args = append(args, "--sysroot="+opts.Sysroot)
	}
	if opts.RepoDir != "" {
		args = append(args, "--repo="+opts.RepoDir)
	}
	args = append(args, "--force")
	args = append(args, opts.GpgArgs...)
	args = append(args, opts.Remote, opts.RemoteURL)
	return o.ostreeRun(verbose, args...)
}

// pullFromRepo pulls an ostree ref using the instance runner.
func (o *Ostree) pullFromRepo(repoDir, remote, ref string, verbose bool) error {
	if repoDir == "" {
		return errors.New("invalid repoDir parameter")
	}
	if remote == "" {
		return errors.New("invalid remote parameter")
	}
	if ref == "" {
		return errors.New("invalid ref parameter")
	}
	o.Print("Pulling ostree from %s %s:%s ...\n", repoDir, remote, ref)
	return o.ostreeRun(verbose, "--repo="+repoDir, "pull", remote, ref)
}

// pruneFromRepo prunes an ostree repo using the instance runner.
func (o *Ostree) pruneFromRepo(repoDir, ref, keepObjectsYoungerThan string, verbose bool) error {
	if repoDir == "" {
		return errors.New("invalid repoDir parameter")
	}
	if ref == "" {
		return errors.New("invalid ref parameter")
	}
	if keepObjectsYoungerThan == "" {
		return errors.New("invalid keepObjectsYoungerThan parameter")
	}
	o.Print("Pruning ostree repo for %s ...\n", repoDir)
	return o.ostreeRun(verbose,
		"--repo="+repoDir, "prune",
		"--depth=5",
		"--refs-only",
		"--keep-younger-than="+keepObjectsYoungerThan,
		"--only-branch="+ref,
	)
}

// ListRemotes lists all the remote refs in the configuration's ostree repository.
func (o *Ostree) ListRemotes(verbose bool) ([]string, error) {
	repoDir, err := o.RepoDir()
	if err != nil {
		return nil, err
	}
	return o.listRemotesFromRepo(repoDir, verbose)
}

// LastCommit returns the last commit for a given ref.
func (o *Ostree) LastCommit(ref string, verbose bool) (string, error) {
	repoDir, err := o.RepoDir()
	if err != nil {
		return "", err
	}
	return o.lastCommitFromRepo(repoDir, ref, verbose)
}

// Pull pulls an ostree ref from a remote.
func (o *Ostree) Pull(ref string, verbose bool) error {
	if ref == "" {
		return errors.New("invalid ref parameter")
	}
	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}
	remote := ExtractRemoteFromRef(ref)
	if remote == "" {
		return fmt.Errorf("%v does not contain the remote: prefix (e.g. origin:)", ref)
	}
	ref = CleanRemoteFromRef(ref)
	return o.pullFromRepo(repoDir, remote, ref, verbose)
}

// PullWithRemote runs `ostree pull` assuming that the provided ref is
// clean from the remote prefix.
func (o *Ostree) PullWithRemote(remote, ref string, verbose bool) error {
	if remote == "" {
		return errors.New("invalid remote parameter")
	}
	if ref == "" {
		return errors.New("invalid ref parameter")
	}
	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}
	return o.pullFromRepo(repoDir, remote, ref, verbose)
}

// Prune prunes the ostree repo for the given ref.
func (o *Ostree) Prune(ref string, verbose bool) error {
	if ref == "" {
		return errors.New("invalid ref parameter")
	}
	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}
	keepObjectsYoungerThan, err := o.cfg.GetItem("Ostree.KeepObjectsYoungerThan")
	if err != nil {
		return err
	}
	return o.pruneFromRepo(repoDir, ref, keepObjectsYoungerThan, verbose)
}

// GenerateStaticDelta generates a static delta for an ostree repository.
func (o *Ostree) GenerateStaticDelta(ref string, verbose bool) error {
	if ref == "" {
		return errors.New("invalid ref parameter")
	}

	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}

	o.Print("Generating static delta for %s and ref %s ...\n", repoDir, ref)

	stdout, err := o.ostreeRunCapture(
		verbose,
		"--repo="+repoDir,
		"rev-parse",
		ref,
	)
	if err != nil {
		return err
	}

	revNew, err := readerToFirstNonEmptyLine(stdout)
	if err != nil {
		return err
	}

	stdout, err = o.ostreeRunCapture(
		verbose,
		"--repo="+repoDir,
		"rev-parse",
		ref+"^",
	)
	if err != nil {
		// This is not a fatal error, the branch might not have a previous commit.
	}
	revOld, _ := readerToFirstNonEmptyLine(stdout)

	if revOld != "" {
		err := o.runCmd(
			io.Discard,
			os.Stderr,
			verbose,
			"--repo="+repoDir,
			"rev-parse",
			revOld,
		)
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"WARNING: rev-parse for old revision %s failed, Falling back to full delta ...\n",
				revOld,
			)
			revOld = ""
		}
	}
	// SAFETY CHECK: Does the parent object actually exist?
	if revOld != "" {
		err := o.runCmd(
			io.Discard,
			os.Stderr,
			verbose,
			"show",
			"--repo="+repoDir,
			revOld,
		)
		if err != nil {
			fmt.Fprintf(
				os.Stderr,
				"WARNING: Parent commit %s is referenced but missing. Falling back to full delta.\n",
				revOld,
			)
			revOld = ""
		}
	}

	args := []string{
		"--repo=" + repoDir,
		"static-delta", "generate",
		"--to=" + revNew,
		"--inline",
		"--min-fallback-size=0",
		"--disable-bsdiff",
		"--max-chunk-size=64",
	}

	if revOld == "" {
		args = append(args, "--empty")
	} else {
		args = append(args, "--from="+revOld)
	}

	return o.ostreeRun(verbose, args...)
}

// UpdateSummary updates the summary of an ostree repository.
func (o *Ostree) UpdateSummary(verbose bool) error {
	o.Print("Updating ostree summary ...")

	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}

	args := []string{
		"--repo=" + repoDir,
		"summary",
		"--update",
	}

	gpgArgs, err := o.GpgArgs()
	if err != nil {
		return err
	}
	args = append(args, gpgArgs...)

	return o.ostreeRun(verbose, args...)
}

// AddRemote adds a remote to an ostree repo.
func (o *Ostree) AddRemote(verbose bool) error {
	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}

	gpgArgs, err := o.ClientSideGpgArgs()
	if err != nil {
		return err
	}

	remote, err := o.Remote()
	if err != nil {
		return err
	}
	remoteURL, err := o.RemoteURL()
	if err != nil {
		return err
	}

	opts := AddRemoteOptions{
		Remote:    remote,
		RemoteURL: remoteURL,
		GpgArgs:   gpgArgs,
		RepoDir:   repoDir,
		Verbose:   verbose,
	}
	return o.addRemote(opts, verbose)
}

// AddRemoteToRootfs adds a remote to an ostree rootfs.
func (o *Ostree) AddRemoteToRootfs(rootfs string, verbose bool) error {
	gpgArgs, err := o.ClientSideGpgArgs()
	if err != nil {
		return err
	}

	remote, err := o.Remote()
	if err != nil {
		return err
	}
	remoteURL, err := o.RemoteURL()
	if err != nil {
		return err
	}

	opts := AddRemoteOptions{
		Remote:    remote,
		RemoteURL: remoteURL,
		GpgArgs:   gpgArgs,
		Sysroot:   rootfs,
		Verbose:   verbose,
	}
	return o.addRemote(opts, verbose)
}

// LocalRefs lists the locally available ostree refs.
func (o *Ostree) LocalRefs(verbose bool) ([]string, error) {
	repoDir, err := o.RepoDir()
	if err != nil {
		return nil, err
	}
	return o.listLocalRefsFromRepo(repoDir, verbose)
}

// RemoteRefs lists the remote available ostree refs.
func (o *Ostree) RemoteRefs(verbose bool) ([]string, error) {
	repoDir, err := o.RepoDir()
	if err != nil {
		return nil, err
	}
	remote, err := o.Remote()
	if err != nil {
		return nil, err
	}
	return o.listRemoteRefsFromRepo(repoDir, remote, verbose)
}

// MaybeInitializeRemote initializes an ostree remote.
func (o *Ostree) MaybeInitializeRemote(verbose bool) error {
	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}
	if !directoryExists(repoDir) {
		if err := os.MkdirAll(repoDir, 0755); err != nil {
			return err
		}
	}

	remote, err := o.Remote()
	if err != nil {
		return err
	}
	remoteURL, err := o.RemoteURL()
	if err != nil {
		return err
	}

	objectsDir := filepath.Join(repoDir, "objects")
	if !directoryExists(objectsDir) {
		o.Print("Initializing local ostree repo at %v ...\n", repoDir)
		if err := o.InitRepo(verbose); err != nil {
			return err
		}
	} else {
		o.Print("ostree repo at %v already initialized. Reusing ...\n", repoDir)
	}

	remotes, err := o.listRemotesFromRepo(repoDir, verbose)
	if err != nil {
		return err
	}
	remoteFound := slices.Contains(remotes, remote)
	if remoteFound {
		o.Print("Remote %v already exists, reusing ...\n", remote)
	} else {
		o.Print("Initializing remote %v at %v ...\n", remote, repoDir)
		gpgArgs, err := o.ClientSideGpgArgs()
		if err != nil {
			return err
		}
		args := []string{"--repo=" + repoDir, "remote", "add"}
		args = append(args, gpgArgs...)
		args = append(args, remote, remoteURL)
		err = o.ostreeRun(verbose, args...)
		if err != nil {
			return err
		}
	}

	o.Print("Showing current ostree remotes:")
	err = o.ostreeRun(verbose, "--repo="+repoDir, "remote", "list", "-u")
	return err
}
