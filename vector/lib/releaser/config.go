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

func (r *Releaser) Hostname() (string, error) {
	return r.configItem("Releaser.Hostname")
}

func (r *Releaser) HooksDir() (string, error) {
	return r.configItem("Releaser.HooksDir")
}

func (r *Releaser) DevDir() (string, error) {
	return r.configItem("matrixOS.Root")
}

func (r *Releaser) UseCpReflink() (bool, error) {
	return r.cfg.GetBool("Releaser.UseCpReflinkModeInsteadOfRsync")
}

func (r *Releaser) ReadOnlyVdb() (string, error) {
	return r.configItem("Releaser.ReadOnlyVdb")
}

func (r *Releaser) LockDir() (string, error) {
	return r.configItem("Releaser.LocksDir")
}

func (r *Releaser) LockWaitSeconds() (string, error) {
	return r.configItem("Releaser.LockWaitSeconds")
}

func (r *Releaser) GenerateStaticDeltas() (bool, error) {
	return r.cfg.GetBool("Releaser.GenerateStaticDeltas")
}

func (r *Releaser) SecureBootCertPath() (string, error) {
	return r.configItem("Seeder.SecureBootPublicKey")
}

func (r *Releaser) SecureBootKekPath() (string, error) {
	return r.configItem("Seeder.SecureBootKekPublicKey")
}

func (r *Releaser) PrivateGitRepoPath() (string, error) {
	return r.configItem("matrixOS.PrivateGitRepoPath")
}

func (r *Releaser) DefaultPrivateGitRepoPath() (string, error) {
	return r.configItem("matrixOS.DefaultPrivateGitRepoPath")
}

func (r *Releaser) BuildMetadataFile() (string, error) {
	return r.configItem("Seeder.ChrootMetadataDir")
}

func (r *Releaser) ServicesDir() (string, error) {
	return r.configItem("Releaser.HooksDir")
}
