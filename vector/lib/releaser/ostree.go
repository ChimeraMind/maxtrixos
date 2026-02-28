package releaser

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
)

func (r *Releaser) SymlinkEtc() error {
	r.Print("Symlinking /etc to prevent emerge packages recreating it ...\n")
	return os.Symlink("usr/etc", filepath.Join(r.imageDir, "etc"))
}

func (r *Releaser) UnlinkEtc() error {
	r.Print("Removing /etc symlink before ostree commit ...\n")
	return os.Remove(filepath.Join(r.imageDir, "etc"))
}

func (r *Releaser) AddExtraDotDotToUsrEtcPortage() error {
	r.Print("Fixing /usr/etc/portage symlink after move of /etc to /usr/etc ...\n")
	etcPortageDir := filepath.Join(r.imageDir, "usr/etc/portage")

	target, err := os.Readlink(etcPortageDir)
	if err != nil {
		return fmt.Errorf("failed to read symlink %s: %w", etcPortageDir, err)
	}

	if err := os.Remove(etcPortageDir); err != nil {
		return err
	}
	newTarget := "../" + target
	if err := os.Symlink(newTarget, etcPortageDir); err != nil {
		return err
	}

	r.Print("New /usr/etc/portage symlink: %s -> %s\n", etcPortageDir, newTarget)

	// Verify the symlink is not broken.
	if _, err := os.Stat(etcPortageDir); err != nil {
		return fmt.Errorf("symlink is broken: %s", etcPortageDir)
	}
	return nil
}

func (r *Releaser) RemoveExtraDotDotFromUsrEtcPortage() error {
	r.Print("Removing extra ../ from /usr/etc/portage so that it works after client deployment.\n")
	etcPortageDir := filepath.Join(r.imageDir, "usr/etc/portage")

	target, err := os.Readlink(etcPortageDir)
	if err != nil {
		return fmt.Errorf("failed to read symlink %s: %w", etcPortageDir, err)
	}

	if err := os.Remove(etcPortageDir); err != nil {
		return err
	}

	newTarget := strings.TrimPrefix(target, "../")
	if err := os.Symlink(newTarget, etcPortageDir); err != nil {
		return err
	}

	r.Print(
		"New /usr/etc/portage symlink status (might be broken): %s -> %s\n",
		etcPortageDir,
		newTarget,
	)
	return nil
}

func (r *Releaser) OstreePrepare() error {
	if err := r.ostree.PrepareFilesystemHierarchy(r.imageDir); err != nil {
		return err
	}
	return r.ostree.ValidateFilesystemHierarchy(r.imageDir)
}

func (r *Releaser) MaybeOstreeInit() error {
	repoDir, err := r.ostree.RepoDir()
	if err != nil {
		return err
	}

	objectsDir := filepath.Join(repoDir, "objects")
	if filesystems.DirectoryExists(objectsDir) {
		r.Print("ostree repository %s already present.\n", repoDir)
		return nil
	}

	r.Print("Creating ostree repository %s ...\n", repoDir)
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		return err
	}

	if err := r.ostree.InitRepo(); err != nil {
		return err
	}

	gpgEnabled, err := r.ostree.GpgEnabled()
	if err != nil {
		return err
	}

	return r.ostree.SetGpg(gpgEnabled)
}

func (r *Releaser) Release(opts CommitOptions) error {
	imageDir := r.imageDir
	repoDir, err := r.ostree.RepoDir()
	if err != nil {
		return err
	}
	if opts.Branch == "" {
		return errors.New("missing branch in CommitOptions")
	}

	// Verify /etc does not exist (it should have been moved to /usr/etc).
	etcDir := filepath.Join(imageDir, "etc")
	if filesystems.PathExists(etcDir) {
		return fmt.Errorf("%s/etc exists; this is illegal and breaks clients", imageDir)
	}

	// Resolve parent commit if a parent branch is specified.
	var parentRev string
	if opts.ParentBranch != "" {
		r.ostree.SetRef(opts.ParentBranch)
		rev, err := r.ostree.LastCommit()
		if err != nil {
			return fmt.Errorf("unable to run ostree rev-parse for parent branch: %w", err)
		}
		r.Print(
			"Setting ostree branch parent of %s to be %s ...\n",
			opts.Branch,
			opts.ParentBranch,
		)
		parentRev = rev
	}

	// Read build metadata.
	metadataFile, err := r.BuildMetadataFile()
	if err != nil {
		return err
	}
	metadata := "not available"
	metadataPath := filepath.Join(imageDir, metadataFile, "build")
	if filesystems.FileExists(metadataPath) {
		r.Print("Reading metadata file %s for release commit subject ...\n", metadataPath)
		data, err := os.ReadFile(metadataPath)
		if err == nil {
			metadata = string(data)
		}
	}

	osName, err := r.ostree.OsName()
	if err != nil {
		return err
	}

	subject := fmt.Sprintf("Automated release of %s for %s at %s",
		osName, opts.Branch, time.Now().Format("2006-01-02"))

	parentBranch := opts.ParentBranch
	if parentBranch == "" {
		parentBranch = "none"
	}

	fancyOsName, err := r.ostree.FancyOsName()
	if err != nil {
		return err
	}

	body := fmt.Sprintf("%s %s (parent: %s) at %s\n\nBuild metadata:\n%s\n",
		fancyOsName,
		opts.Branch,
		parentBranch,
		time.Now().Format("2006-01-02"),
		metadata,
	)

	// Normalise timestamps before commit.
	r.Print("Normalizing files at %s before ostree commit to have same timestamp ...\n", imageDir)
	if err := filesystems.NormalizeTimestamps(imageDir, time.Unix(1, 0)); err != nil {
		return err
	}

	// Build and run the ostree commit.
	gpgArgs, err := r.ostree.GpgArgs()
	if err != nil {
		return err
	}
	commitOpts := ostree.CommitOptions{
		RepoDir:  repoDir,
		Branch:   opts.Branch,
		Subject:  subject,
		Body:     body,
		Parent:   parentRev,
		GpgArgs:  gpgArgs,
		Consume:  opts.Consume,
		ImageDir: imageDir,
	}
	if err := r.ostree.Commit(commitOpts); err != nil {
		return fmt.Errorf("ostree commit failed: %w", err)
	}

	if err := r.ostree.Prune(); err != nil {
		return fmt.Errorf("ostree prune failed: %w", err)
	}

	genDeltas, err := r.GenerateStaticDeltas()
	if err != nil {
		return err
	}
	if genDeltas {
		if err := r.ostree.GenerateStaticDelta(); err != nil {
			return fmt.Errorf("ostree static delta generation failed: %w", err)
		}
	} else {
		r.Print("Skipping static delta generation as requested by flags.\n")
	}

	if err := r.ostree.UpdateSummary(); err != nil {
		return fmt.Errorf("ostree update summary failed: %w", err)
	}

	return nil
}
