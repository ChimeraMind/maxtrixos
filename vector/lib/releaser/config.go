package releaser

import "fmt"
// configItem retrieves a non-empty config string or returns an error.
func (r *Releaser) configItem(key string) (string, error) {
	v, err := r.cfg.GetItem(key)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", fmt.Errorf("invalid %s", key)
	}
	return v, nil
}

// Hostname returns the configured release hostname.
func (r *Releaser) Hostname() (string, error) {
	return r.configItem("Releaser.Hostname")
}

// HooksDir returns the directory where per-branch release hooks live.
func (r *Releaser) HooksDir() (string, error) {
	return r.configItem("Releaser.HooksDir")
}

// UseCpReflink returns whether cp --reflink=auto should be used instead of rsync.
func (r *Releaser) UseCpReflink() (bool, error) {
	return r.cfg.GetBool("Releaser.UseCpReflinkModeInsteadOfRsync")
}

// ReadOnlyVdb returns the path used for the read-only Portage vardb.
func (r *Releaser) ReadOnlyVdb() (string, error) {
	return r.configItem("Releaser.ReadOnlyVdb")
}

// LockDir returns the directory where releaser file locks are stored.
func (r *Releaser) LockDir() (string, error) {
	return r.configItem("Releaser.LocksDir")
}

// LockWaitSeconds returns the configured lock acquisition timeout.
func (r *Releaser) LockWaitSeconds() (string, error) {
	return r.configItem("Releaser.LockWaitSeconds")
}

// GenerateStaticDeltas returns whether ostree static deltas should be generated.
func (r *Releaser) GenerateStaticDeltas() (bool, error) {
	return r.cfg.GetBool("Releaser.GenerateStaticDeltas")
}

// SecureBootCertPath returns the path to the SecureBoot db certificate.
func (r *Releaser) SecureBootCertPath() (string, error) {
	return r.configItem("Seeder.SecureBootPublicKey")
}

// SecureBootKekPath returns the path to the SecureBoot KEK certificate.
func (r *Releaser) SecureBootKekPath() (string, error) {
	return r.configItem("Seeder.SecureBootKekPublicKey")
}

// PrivateGitRepoPath returns the private git repo path.
func (r *Releaser) PrivateGitRepoPath() (string, error) {
	return r.configItem("matrixOS.PrivateGitRepoPath")
}

// DefaultPrivateGitRepoPath returns the default private git repo path (used inside chroots).
func (r *Releaser) DefaultPrivateGitRepoPath() (string, error) {
	return r.configItem("matrixOS.DefaultPrivateGitRepoPath")
}

// BuildMetadataFile returns the seeder build metadata file path.
func (r *Releaser) BuildMetadataFile() (string, error) {
	return r.configItem("Seeder.ChrootMetadataDir")
}

// ServicesDir returns the directory where per-branch systemd service configs live.
func (r *Releaser) ServicesDir() (string, error) {
	return r.configItem("Releaser.HooksDir")
}
