package seeder

import (
	"fmt"
	"path/filepath"
)

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

func (s *Seeder) SeedersDir() (string, error) {
	return s.configItem("Seeder.SeedersDir")
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

func (s *Seeder) DelegatedChrootSystemMounts() (bool, error) {
	return s.cfg.GetBool("Seeder.DelegatedChrootSystemMounts")
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

func (s *Seeder) ChrootMetadataDir() (string, error) {
	return s.configItem("Seeder.ChrootMetadataDir")
}

func (s *Seeder) ChrootMetadataDirBuildFileName() (string, error) {
	return s.configItem("Seeder.ChrootMetadataDirBuildFileName")
}

func (s *Seeder) BuildMetadataFile() (string, error) {
	dir, err := s.ChrootMetadataDir()
	if err != nil {
		return "", err
	}
	fileName, err := s.ChrootMetadataDirBuildFileName()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, fileName), nil
}

func (s *Seeder) PhasesStateDir() (string, error) {
	return s.configItem("Seeder.ChrootSeedersPhasesStateDir")
}

func (s *Seeder) PreppersPhasesStateDir() (string, error) {
	return s.configItem("Seeder.ChrootPreppersPhasesStateDir")
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

func (s *Seeder) DownloadsDir() (string, error) {
	return s.configItem("Seeder.DownloadsDir")
}

func (s *Seeder) DistfilesDir() (string, error) {
	return s.configItem("Seeder.DistfilesDir")
}

func (s *Seeder) BinpkgsDir() (string, error) {
	return s.configItem("Seeder.BinpkgsDir")
}

func (s *Seeder) GpgKeysDir() (string, error) {
	return s.configItem("Seeder.GpgKeysDir")
}

func (s *Seeder) DevDir() (string, error) {
	return s.configItem("matrixOS.Root")
}

func (s *Seeder) DefaultDevDir() (string, error) {
	return s.configItem("matrixOS.DefaultRoot")
}

func (s *Seeder) GitRepo() (string, error) {
	return s.configItem("matrixOS.GitRepo")
}

func (s *Seeder) DefaultPrivateGitRepoPath() (string, error) {
	return s.configItem("matrixOS.DefaultPrivateGitRepoPath")
}

// SeedsVersioningCadence returns the configured seed versioning cadence.
// Valid values are "daily", "weekly", or "monthly".
// If the value is not set, it defaults to "weekly".
func (s *Seeder) SeedsVersioningCadence() (string, error) {
	v, err := s.cfg.GetItem("Seeder.SeedsVersioningCadence")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "weekly", nil
	}
	switch v {
	case "daily", "weekly", "monthly":
		return v, nil
	default:
		return "", fmt.Errorf(
			"invalid Seeder.SeedsVersioningCadence %q: must be daily, weekly, or monthly", v,
		)
	}
}
