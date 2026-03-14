package seeder

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/runner"
)

// SeederParams holds the key variables exported by a seeder's params.sh.
type SeederParams struct {
	ChrootName         string // SEEDER_CHROOT_NAME
	ChrootsDir         string // SEEDER_CHROOTS_DIR
	PreferredChrootDir string // PREFERRED_SEEDER_CHROOT_DIR
}

// PrepperOptions configures how the prepper script is executed.
type PrepperOptions struct {
	ChrootDir  string
	Resume     bool
	Stage3File string
}

// --- Done flag management ---

// SeederDoneFlagFile computes the done-flag file path for the given
// seeder name and chroot directory.
// Format: <chrootDir><stateDir>/<prefix>_<name>
func (s *Seeder) SeederDoneFlagFile(name, chrootDir string) (string, error) {
	stateDir, err := s.PhasesStateDir()
	if err != nil {
		return "", err
	}
	prefix, err := s.SeederDoneFlagFilePrefix()
	if err != nil {
		return "", err
	}
	base := filepath.Join(chrootDir, stateDir)
	return filepath.Join(base, prefix+"_"+name), nil
}

// IsSeederDone checks whether the done-flag file exists for the seeder.
func (s *Seeder) IsSeederDone(name, chrootDir string) (bool, error) {
	flagFile, err := s.SeederDoneFlagFile(name, chrootDir)
	if err != nil {
		return false, err
	}
	return filesystems.FileExists(flagFile), nil
}

// MarkSeederDone creates the done-flag file for the given seeder.
func (s *Seeder) MarkSeederDone(name, chrootDir string) error {
	flagFile, err := s.SeederDoneFlagFile(name, chrootDir)
	if err != nil {
		return err
	}
	flagDir := filepath.Dir(flagFile)
	if err := os.MkdirAll(flagDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", flagDir, err)
	}
	f, err := os.Create(flagFile)
	if err != nil {
		return fmt.Errorf("failed to touch %s: %w", flagFile, err)
	}
	return f.Close()
}

// --- Params parsing ---

// ParseSeederParams executes the given params.sh in a bash subshell
// and captures the three key variables it must set.
func (s *Seeder) ParseSeederParams(paramsPath string) (*SeederParams, error) {
	devDir, err := s.DevDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get dev dir: %w", err)
	}

	// Source params.sh and echo the required variables.
	script := fmt.Sprintf(
		`set -eu; export MATRIXOS_DEV_DIR=%q; `+
			`source %q; `+
			`echo "${SEEDER_CHROOT_NAME}"; `+
			`echo "${SEEDER_CHROOTS_DIR}"; `+
			`echo "${PREFERRED_SEEDER_CHROOT_DIR}"`,
		devDir, paramsPath,
	)

	var stdout bytes.Buffer
	cmd := &runner.Cmd{
		Name:   "bash",
		Args:   []string{"-c", script},
		Stdout: &stdout,
		Stderr: s.stderr,
	}
	if err := s.runner(cmd); err != nil {
		return nil, fmt.Errorf(
			"failed to source params %s: %w", paramsPath, err,
		)
	}

	lines := strings.Split(
		strings.TrimSpace(stdout.String()), "\n",
	)
	if len(lines) < 3 {
		return nil, fmt.Errorf(
			"expected 3 lines from params %s, got %d",
			paramsPath, len(lines),
		)
	}

	params := &SeederParams{
		ChrootName:         strings.TrimSpace(lines[0]),
		ChrootsDir:         strings.TrimSpace(lines[1]),
		PreferredChrootDir: strings.TrimSpace(lines[2]),
	}

	if params.ChrootName == "" {
		return nil, fmt.Errorf(
			"SEEDER_CHROOT_NAME is empty in %s", paramsPath,
		)
	}
	if params.ChrootsDir == "" {
		return nil, fmt.Errorf(
			"SEEDER_CHROOTS_DIR is empty in %s", paramsPath,
		)
	}
	if params.PreferredChrootDir == "" {
		return nil, fmt.Errorf(
			"PREFERRED_SEEDER_CHROOT_DIR is empty in %s", paramsPath,
		)
	}
	return params, nil
}

// --- GPG key management ---

// ImportGentooGpgKeys patches the GPG homedir permissions and imports
// the Gentoo release engineering GPG keys.
func (s *Seeder) ImportGentooGpgKeys() error {
	homedir, err := s.GpgKeysDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(homedir, 0700); err != nil {
		return fmt.Errorf("mkdir GPG homedir %s: %w", homedir, err)
	}
	if err := os.Chmod(homedir, 0700); err != nil {
		return fmt.Errorf("chmod GPG homedir: %w", err)
	}

	// Fix permissions on existing files inside the GPG homedir.
	_ = filepath.WalkDir(homedir,
		func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			return os.Chmod(path, 0600)
		},
	)

	s.Print("Importing Gentoo GPG keys (--homedir=%s) ...\n", homedir)
	return s.runner(&runner.Cmd{
		Name: "gpg",
		Args: []string{
			"--homedir=" + homedir,
			"--batch", "--yes",
			"--auto-key-locate=clear,nodefault,wkd",
			"--locate-key",
			"releng@gentoo.org",
		},
		Stdout: s.stdout,
		Stderr: s.stderr,
	})
}

// --- Prepper execution ---

// ExecutePrepper runs the prepper script with the required env vars.
func (s *Seeder) ExecutePrepper(info SeederInfo, params *SeederParams, opts *PrepperOptions) error {
	devDir, err := s.DevDir()
	if err != nil {
		return err
	}
	downloadsDir, err := s.DownloadsDir()
	if err != nil {
		return err
	}
	stage3URL, err := s.Stage3DownloadUrl()
	if err != nil {
		return err
	}

	resume := ""
	if opts.Resume {
		resume = "1"
	}

	env := os.Environ()
	env = append(env,
		"MATRIXOS_DEV_DIR="+devDir,
		"SEEDER_CHROOT_NAME="+params.ChrootName,
		"SEEDER_CHROOTS_DIR="+params.ChrootsDir,
		"PREFERRED_SEEDER_CHROOT_DIR="+params.PreferredChrootDir,
		"CHROOT_DIR="+opts.ChrootDir,
		"DOWNLOAD_DIR="+downloadsDir,
		"CHROOT_RESUME="+resume,
		"STAGE3_FILE="+opts.Stage3File,
		"STAGE3_URL="+stage3URL,
	)

	cmd := &runner.Cmd{
		Name:   info.PrepperExec,
		Stdout: s.stdout,
		Stderr: s.stderr,
	}
	cmd.Env = env
	return s.runner(cmd)
}

// --- Mount management ---

// mountPrivateGitRepo sets up a read-only bind mount for the private git
// repo inside the chroot.
func (s *Seeder) mountPrivateGitRepo(chrootDir string) error {
	privatePath, err := s.PrivateGitRepoPath()
	if err != nil {
		return fmt.Errorf("error getting private repo path: %w", err)
	}
	defaultPrivatePath, err := s.DefaultPrivateGitRepoPath()
	if err != nil {
		return fmt.Errorf("error getting default private git repo path: %w", err)
	}
	privateDst := filepath.Join(chrootDir, defaultPrivatePath)
	privateBind, err := filesystems.NewBindMount(
		filesystems.BindMountOptions{
			Src:      privatePath,
			Dst:      privateDst,
			ReadOnly: true,
			MkdirAll: true,
			Stdout:   s.stdout,
			Stderr:   s.stderr,
		},
	)
	if err != nil {
		return fmt.Errorf("error creating RO mount for private repo: %w", err)
	}
	s.trackMount(privateBind.Dst())
	if err := privateBind.Mount(); err != nil {
		return fmt.Errorf("error mounting RO private repo: %w", err)
	}
	return nil
}

// mountDistDir sets up a bind mount for the distfiles directory inside the chroot.
func (s *Seeder) mountDistDir(chrootDir string) error {
	distDir, err := s.DistfilesDir()
	if err != nil {
		return fmt.Errorf("error getting distfiles dir: %w", err)
	}

	if err := os.MkdirAll(distDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", distDir, err)
	}

	distMount, err := filesystems.BindMountDistdir(
		filesystems.BindMountDistdirOptions{
			DistfilesDir: distDir,
			Rootfs:       chrootDir,
			Stdout:       s.stdout,
			Stderr:       s.stderr,
		},
	)
	if err != nil {
		return fmt.Errorf("error creating distfiles mount: %w", err)
	}
	s.trackMount(distMount.Dst())
	if err := distMount.Mount(); err != nil {
		return fmt.Errorf("error mounting distfiles: %w", err)
	}
	return nil
}

// mountBinpkgsDir sets up a bind mount for the binpkgs directory inside the chroot.
func (s *Seeder) mountBinpkgsDir(chrootDir string) error {
	binDir, err := s.BinpkgsDir()
	if err != nil {
		return fmt.Errorf("error getting binpkgs dir: %w", err)
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", binDir, err)
	}

	binMount, err := filesystems.BindMountBinpkgs(
		filesystems.BindMountBinpkgsOptions{
			BinpkgsDir: binDir,
			Rootfs:     chrootDir,
			Stdout:     s.stdout,
			Stderr:     s.stderr,
		},
	)
	if err != nil {
		return fmt.Errorf("error creating binpkgs mount: %w", err)
	}
	s.trackMount(binMount.Dst())
	if err := binMount.Mount(); err != nil {
		return fmt.Errorf("error mounting binpkgs: %w", err)
	}
	return nil
}

// SetupChrootMounts sets up all bind mounts needed for a seeder chroot.
// All mount points are tracked and can be cleaned up by calling Cleanup().
func (s *Seeder) SetupChrootMounts(chrootDir string) error {
	// 1. Common rootfs mounts (dev, sys, proc, shm, run/lock).
	common, err := filesystems.NewCommonRootfsMounts(
		filesystems.CommonRootfsMountsOptions{
			MountPoint: chrootDir,
			Mounting: func(mnt string) {
				s.trackMount(mnt)
				s.Print("Mounting: %s ...\n", mnt)
			},
			Mounted: func(mnt string) {
				s.Print("Mounted: %s ...\n", mnt)
			},
			Stdout: s.stdout,
			Stderr: s.stderr,
		},
	)
	if err != nil {
		return fmt.Errorf("error creating common mounts init: %w", err)
	}
	if err := common.Setup(); err != nil {
		return fmt.Errorf("error setting up common mounts setup: %w", err)
	}

	if err := s.mountPrivateGitRepo(chrootDir); err != nil {
		return fmt.Errorf("error mounting private git repo: %w", err)
	}

	if err := s.mountDistDir(chrootDir); err != nil {
		return fmt.Errorf("error mounting distfiles: %w", err)
	}

	if err := s.mountBinpkgsDir(chrootDir); err != nil {
		return fmt.Errorf("error mounting binpkgs: %w", err)
	}

	return nil
}

// --- Chroot DNS ---

// SetupChrootDNS copies /etc/resolv.conf into the chroot.
func (s *Seeder) SetupChrootDNS(chrootDir string) error {
	src := "/etc/resolv.conf"
	dst := filepath.Join(chrootDir, "etc", "resolv.conf")

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(dst), err)
	}

	data, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", src, err)
	}

	s.Print("Copying %s to %s\n", src, dst)
	return os.WriteFile(dst, data, 0644)
}

// --- Chroot directory setup ---

// setupDevDir sets up the dev toolkit into the chroot.
func (s *Seeder) setupDevDir(chrootDevDir string) error {
	localClone, err := s.UseLocalGitRepoInsideChroot()
	if err != nil {
		return err
	}
	cloneArgs, err := s.gitCloneArgs()
	if err != nil {
		return fmt.Errorf("error getting git clone args: %w", err)
	}

	if localClone {
		devDir, err := s.DevDir()
		if err != nil {
			return fmt.Errorf("error getting dev dir: %w", err)
		}
		s.Print(
			"Cloning %s (local) into %s ...\n",
			devDir, chrootDevDir,
		)
		args := append([]string{"clone"}, cloneArgs...)
		args = append(args, devDir, chrootDevDir)
		if err := s.runner(&runner.Cmd{
			Name:   "git",
			Args:   args,
			Stdout: s.stdout,
			Stderr: s.stderr,
		}); err != nil {
			return fmt.Errorf("error cloning git repo (local): %w", err)
		}
	} else {
		gitRepo, err := s.GitRepo()
		if err != nil {
			return fmt.Errorf("error getting git repo: %w", err)
		}
		s.Print(
			"Cloning %s (remote) into %s ...\n",
			gitRepo, chrootDevDir,
		)
		args := append([]string{"clone"}, cloneArgs...)
		args = append(args, gitRepo, chrootDevDir)
		if err := s.RetryableCmd(
			6, "git", args...,
		); err != nil {
			return fmt.Errorf("error cloning git repo (remote): %w", err)
		}
	}

	return nil
}

// cleanDevDirGitDir deletes the .git directory from the dev toolkit in the chroot
// if it exists and the seeder is configured to do so.
func (s *Seeder) cleanDevDirGitDir(chrootDevDir string) error {
	// Maybe delete .git directory.
	deleteDotGit, err := s.DeleteDotGitFromGitRepo()
	if err != nil {
		return fmt.Errorf("error checking if .git should be deleted: %w", err)
	}
	if !deleteDotGit {
		return nil
	}

	dotGitDir := filepath.Join(chrootDevDir, ".git")
	if !filesystems.DirectoryExists(dotGitDir) {
		return nil
	}

	s.Print("Deleting %s ...\n", dotGitDir)
	if err := os.RemoveAll(dotGitDir); err != nil {
		return fmt.Errorf("error deleting %s: %w", dotGitDir, err)
	}

	return nil
}

// SetupChrootDirs creates the seeder phase directories inside the chroot
// and clones the dev toolkit if it does not already exist.
func (s *Seeder) SetupChrootDirs(chrootDir string) error {
	// Create phases state dir.
	stateDir, err := s.PhasesStateDir()
	if err != nil {
		return err
	}
	phasesDir := filepath.Join(chrootDir, stateDir)
	if err := os.MkdirAll(phasesDir, 0755); err != nil {
		return fmt.Errorf("error creating phases dir %s: %w", phasesDir, err)
	}

	// Clone the dev toolkit into the chroot.
	defaultDevDir, err := s.DefaultDevDir()
	if err != nil {
		return err
	}
	chrootDevDir := filepath.Join(chrootDir, defaultDevDir)

	if !filesystems.DirectoryExists(chrootDevDir) {
		if err := s.setupDevDir(chrootDevDir); err != nil {
			return fmt.Errorf("error setting up dev dir: %w", err)
		}
	}

	// Maybe delete .git directory.
	if err := s.cleanDevDirGitDir(chrootDevDir); err != nil {
		return fmt.Errorf("error cleaning dev dir .git: %w", err)
	}

	return nil
}

// --- Chroot execution ---

// ExecuteInChroot runs the seeder's chroot script inside the chroot
// using unshare for namespace isolation.
func (s *Seeder) ExecuteInChroot(chrootDir string, info SeederInfo) error {
	defaultDevDir, err := s.DefaultDevDir()
	if err != nil {
		return err
	}

	env := os.Environ()
	env = append(env, "MATRIXOS_DEV_DIR="+defaultDevDir)

	unshareArgs := []string{
		"--pid", "--fork", "--kill-child",
		"--mount", "--uts", "--ipc",
		fmt.Sprintf("--mount-proc=%s/proc", chrootDir),
		"chroot", chrootDir, info.ChrootExec,
	}

	cmd := &runner.Cmd{
		Name:   "unshare",
		Args:   unshareArgs,
		Stdout: s.stdout,
		Stderr: s.stderr,
	}
	cmd.Env = env
	return s.runner(cmd)
}

// --- Artifact cleanup ---

// CleanTemporaryArtifact removes a temporary artifact directory after
// verifying no submounts remain.
func (s *Seeder) CleanTemporaryArtifact(dir string) error {
	if dir == "" {
		return fmt.Errorf("missing directory parameter")
	}
	if !filesystems.DirectoryExists(dir) {
		return fmt.Errorf(
			"%s is not a directory or does not exist", dir,
		)
	}

	s.Print("Cleaning artifacts for dir: %s ...\n", dir)

	submounts, err := filesystems.ListSubmounts(dir)
	if err != nil {
		return err
	}
	if len(submounts) > 0 {
		for _, mnt := range submounts {
			s.PrintError("Dangling mount: %s\n", mnt)
		}
		return fmt.Errorf(
			"cannot remove %s: %d dangling submounts",
			dir, len(submounts),
		)
	}

	return os.RemoveAll(dir)
}
