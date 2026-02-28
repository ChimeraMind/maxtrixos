package seeder

import "fmt"

// configItem retrieves a non-empty config string or returns an error.
func (s *Seeder) configItem(key string) (string, error) {
	v, err := s.cfg.GetItem(key)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", fmt.Errorf("invalid %s", key)
	}
	return v, nil
}

func (s *Seeder) ChrootSeedersDir() (string, error) {
	return s.configItem("Seeder.ChrootSeedersDir")
}

func (s *Seeder) ChrootBuildArtifactsDir() (string, error) {
	return s.configItem("Seeder.ChrootBuildArtifactsDir")
}

func (s *Seeder) DisabledSeederFile() (string, error) {
	return s.configItem("Seeder.SeederDisabledFileName")
}

func (s *Seeder) UseLocalGitRepoInsideChroot() (bool, error) {
	return s.cfg.GetBool("Seeder.UseLocalGitRepoInsideChroot")
}

func (s *Seeder) DeleteDotGitFromGitRepo() (bool, error) {
	return s.cfg.GetBool("Seeder.DeleteDotGitFromGitRepo")
}

func (s *Seeder) GitCloneArgs() (string, error) {
	return s.configItem("Seeder.GitCloneArgs")
}

func (s *Seeder) ChrootExecName() (string, error) {
	return s.configItem("Seeder.ChrootExecutableName")
}

func (s *Seeder) ParamsExecutableName() (string, error) {
	return s.configItem("Seeder.ParamsExecutableName")
}

func (s *Seeder) PrepperExecName() (string, error) {
	return s.configItem("Seeder.PrepperExecutableName")
}

func (s *Seeder) PhasesStateDir() (string, error) {
	return s.configItem("Seeder.ChrootSeedersPhasesStateDir")
}

func (s *Seeder) SeederDoneFlagFilePrefix() (string, error) {
	return s.configItem("Seeder.ChrootSeederDoneFlagFileNamePrefix")
}

func (s *Seeder) PrivateExampleGitRepo() (string, error) {
	return s.configItem("matrixOS.PrivateExampleGitRepo")
}

func (s *Seeder) PrivateGitRepoPath() (string, error) {
	return s.configItem("matrixOS.PrivateGitRepoPath")
}

func (s *Seeder) LockDir() (string, error) {
	return s.configItem("Seeder.LocksDir")
}

func (s *Seeder) LockWaitSeconds() (string, error) {
	return s.configItem("Seeder.LockWaitSeconds")
}

func (s *Seeder) Stage3DownloadUrl() (string, error) {
	return s.configItem("Seeder.Stage3DownloadUrl")
}
