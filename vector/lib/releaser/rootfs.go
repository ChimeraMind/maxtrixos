package releaser

import (
	"fmt"
	"os"
	"path/filepath"

	"matrixos/vector/lib/filesystems"
)

const (
	imagerPortageSecureBootPem = "etc/portage/secureboot.pem"
	imagerPortageSecureBootKek = "etc/portage/secureboot-kek.pem"
)

func (r *Releaser) CleanRootfs() error {
	imageDir := r.imageDir

	if imageDir == "" {
		return fmt.Errorf("imageDir is not set")
	}
	if err := filesystems.CheckDirIsRoot(imageDir); err != nil {
		return err
	}

	// Copy SecureBoot certs from private repo.
	certSrc, err := r.SecureBootCertPath()
	if err != nil {
		return err
	}
	certDst := filepath.Join(imageDir, imagerPortageSecureBootPem)
	if err := filesystems.CopyFile(certSrc, certDst); err != nil {
		return fmt.Errorf("failed to copy secureboot cert: %w", err)
	}

	kekSrc, err := r.SecureBootKekPath()
	if err != nil {
		return err
	}
	kekDst := filepath.Join(imageDir, imagerPortageSecureBootKek)
	if err := filesystems.CopyFile(kekSrc, kekDst); err != nil {
		return fmt.Errorf("failed to copy secureboot KEK cert: %w", err)
	}

	defaultPrivPath, err := r.DefaultPrivateGitRepoPath()
	if err != nil {
		return err
	}

	removeDirs := []string{
		"/root/.bash_history",
		"/root/.ssh",
		"/root/.gnupg",
		"/root/.cache",
		"/root/.local",
		"/var/lib/gdm/.cache",
		"/var/lib/gdm/.local",
		"/var/lib/gdm/.config",
		defaultPrivPath,
		"/var/lib/sbctl/keys",
		"/var/tmp/ostree-gpg-private",
	}

	emptyDirs := []string{
		"/tmp",
		"/dev",
		"/boot",
		"/root",
		"/var/lib/systemd/coredump",
		"/var/tmp/portage",
	}

	removeFiles := []string{
		"/etc/resolv.conf",
		"/etc/portage/secureboot.x509",
		"/root/.bash_history",
		"/root/.lesshst",
		"/root/.bashrc",
		"/root/.xauth*",
		"/var/lib/sbctl/keys",
	}

	for _, d := range removeDirs {
		_ = filesystems.RemoveDir(filepath.Join(imageDir, d))
	}
	for _, d := range emptyDirs {
		_ = filesystems.EmptyDir(filepath.Join(imageDir, d))
	}
	for _, p := range removeFiles {
		_ = filesystems.RemoveFileWithGlob(filepath.Join(imageDir, p))
	}

	// Prepare Portage directory.
	os.MkdirAll(filepath.Join(imageDir, "var/db/repos/gentoo"), 0755)

	return nil
}

func (r *Releaser) PostCleanShrink() error {
	imageDir := r.imageDir

	if imageDir == "" {
		return fmt.Errorf("imageDir is not set")
	}
	if err := filesystems.CheckDirIsRoot(imageDir); err != nil {
		return err
	}

	r.Print("Shrinking the rootfs to save space ...\n")

	// Set up chroot mounts for emerge.
	mounts, err := filesystems.NewCommonRootfsMounts(
		filesystems.CommonRootfsMountsOptions{
			MountPoint: imageDir,
			Mounting: func(mnt string) {
				r.Print("Mounting: %s ...\n", mnt)
				r.trackMount(mnt)
			},
			Mounted: func(mnt string) {
				r.Print("Mounted: %s\n", mnt)
			},
		},
	)
	if err != nil {
		return err
	}
	defer mounts.Cleanup()

	if err := mounts.Setup(); err != nil {
		return fmt.Errorf("failed to set up chroot mounts: %w", err)
	}

	err = filesystems.ChrootRun(imageDir,
		"emerge",
		"--depclean",
		"--with-bdeps=n",
		"--complete-graph",
	)
	if err != nil {
		return fmt.Errorf("emerge --depclean failed: %w", err)
	}

	removeDirs := []string{"/usr/include"}
	emptyDirs := []string{
		"/usr/lib/pkgconfig",
		"/var/db/repos",
		"/usr/src",
	}

	for _, d := range removeDirs {
		_ = filesystems.RemoveDir(filepath.Join(imageDir, d))
	}
	for _, d := range emptyDirs {
		_ = filesystems.EmptyDir(filepath.Join(imageDir, d))
	}

	r.Print("Removing all {.a,.la} files\n")

	walker := func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			ext := filepath.Ext(path)
			if ext == ".a" || ext == ".la" {
				os.Remove(path)
			}
		}
		return nil
	}

	_ = filepath.WalkDir(filepath.Join(imageDir, "usr"), walker)

	r.Print("Shrink completed.\n")
	return nil
}
