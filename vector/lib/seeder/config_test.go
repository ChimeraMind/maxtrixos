package seeder

import (
	"errors"
	"testing"

	"matrixos/vector/lib/config"
)

// configAccessorTestCase describes a single accessor under test.
type configAccessorTestCase struct {
	name    string
	key     string                          // expected config key
	call    func(s *Seeder) (string, error) // the accessor to invoke
	wantVal string                          // expected string value
}

// stringAccessors returns test cases for every string config accessor.
func stringAccessors() []configAccessorTestCase {
	return []configAccessorTestCase{
		{
			name:    "ChrootSeedersDir",
			key:     "Seeder.ChrootSeedersDir",
			call:    func(s *Seeder) (string, error) { return s.ChrootSeedersDir() },
			wantVal: "/build/seeders",
		},
		{
			name:    "SeedersDir",
			key:     "Seeder.SeedersDir",
			call:    func(s *Seeder) (string, error) { return s.SeedersDir() },
			wantVal: "/build/seeders",
		},
		{
			name:    "DisabledSeederFile",
			key:     "Seeder.SeederDisabledFileName",
			call:    func(s *Seeder) (string, error) { return s.DisabledSeederFile() },
			wantVal: "__disabled__",
		},
		{
			name:    "ChrootExecName",
			key:     "Seeder.ChrootExecutableName",
			call:    func(s *Seeder) (string, error) { return s.ChrootExecName() },
			wantVal: "chroot.sh",
		},
		{
			name:    "PrepperExecName",
			key:     "Seeder.PrepperExecutableName",
			call:    func(s *Seeder) (string, error) { return s.PrepperExecName() },
			wantVal: "prepper.sh",
		},
		{
			name:    "PhasesStateDir",
			key:     "Seeder.ChrootSeedersPhasesStateDir",
			call:    func(s *Seeder) (string, error) { return s.PhasesStateDir() },
			wantVal: "/build/.seeders_phases",
		},
		{
			name:    "PreppersPhasesStateDir",
			key:     "Seeder.ChrootPreppersPhasesStateDir",
			call:    func(s *Seeder) (string, error) { return s.PreppersPhasesStateDir() },
			wantVal: "/build/preppers/.preppers_phases",
		},
		{
			name:    "SeederDoneFlagFilePrefix",
			key:     "Seeder.ChrootSeederDoneFlagFileNamePrefix",
			call:    func(s *Seeder) (string, error) { return s.SeederDoneFlagFilePrefix() },
			wantVal: "seeder.complete",
		},
		{
			name:    "PrivateExampleGitRepo",
			key:     "matrixOS.PrivateExampleGitRepo",
			call:    func(s *Seeder) (string, error) { return s.PrivateExampleGitRepo() },
			wantVal: "https://example.com/private.git",
		},
		{
			name:    "PrivateGitRepoPath",
			key:     "matrixOS.PrivateGitRepoPath",
			call:    func(s *Seeder) (string, error) { return s.PrivateGitRepoPath() },
			wantVal: "/priv",
		},
		{
			name:    "GitCloneArgs",
			key:     "Seeder.GitCloneArgs",
			call:    func(s *Seeder) (string, error) { return s.GitCloneArgs() },
			wantVal: "--depth 1",
		},
		{
			name:    "LockDir",
			key:     "Seeder.LocksDir",
			call:    func(s *Seeder) (string, error) { return s.LockDir() },
			wantVal: "/locks/seeder",
		},
		{
			name:    "LockWaitSeconds",
			key:     "Seeder.LockWaitSeconds",
			call:    func(s *Seeder) (string, error) { return s.LockWaitSeconds() },
			wantVal: "86400",
		},
		{
			name:    "Stage3DownloadUrl",
			key:     "Seeder.Stage3DownloadUrl",
			call:    func(s *Seeder) (string, error) { return s.Stage3DownloadUrl() },
			wantVal: "https://distfiles.gentoo.org/releases/amd64/autobuilds/current-stage3-amd64-systemd/latest-stage3-amd64-systemd.txt",
		},
		{
			name:    "ChrootMetadataDir",
			key:     "Seeder.ChrootMetadataDir",
			call:    func(s *Seeder) (string, error) { return s.ChrootMetadataDir() },
			wantVal: "/build/.metadata",
		},
		{
			name:    "ChrootMetadataDirBuildFileName",
			key:     "Seeder.ChrootMetadataDirBuildFileName",
			call:    func(s *Seeder) (string, error) { return s.ChrootMetadataDirBuildFileName() },
			wantVal: "build.json",
		},
	}
}

func TestConfigAccessors_ReturnsValue(t *testing.T) {
	for _, tc := range stringAccessors() {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestSeeder()
			s.cfg.(*config.MockConfig).Items[tc.key] = []string{tc.wantVal}

			got, err := tc.call(s)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.wantVal {
				t.Errorf("got %q, want %q", got, tc.wantVal)
			}
		})
	}
}

func TestConfigAccessors_ErrorOnEmpty(t *testing.T) {
	for _, tc := range stringAccessors() {
		t.Run(tc.name, func(t *testing.T) {
			s := newTestSeeder()
			// Key absent => GetItem returns "" => configItem returns error.

			_, err := tc.call(s)
			if err == nil {
				t.Fatal("expected error for empty config value, got nil")
			}
		})
	}
}

func TestBuildMetadataFile_ReturnsJoinedPath(t *testing.T) {
	s := newTestSeeder()
	s.cfg.(*config.MockConfig).Items["Seeder.ChrootMetadataDir"] = []string{"/build/.metadata"}
	s.cfg.(*config.MockConfig).Items["Seeder.ChrootMetadataDirBuildFileName"] = []string{"build.json"}

	got, err := s.BuildMetadataFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "/build/.metadata/build.json"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildMetadataFile_ErrorOnMissingDir(t *testing.T) {
	s := newTestSeeder()
	// ChrootMetadataDir key absent => error
	s.cfg.(*config.MockConfig).Items["Seeder.ChrootMetadataDirBuildFileName"] = []string{"build.json"}

	_, err := s.BuildMetadataFile()
	if err == nil {
		t.Fatal("expected error for missing ChrootMetadataDir, got nil")
	}
}

func TestBuildMetadataFile_ErrorOnMissingFileName(t *testing.T) {
	s := newTestSeeder()
	s.cfg.(*config.MockConfig).Items["Seeder.ChrootMetadataDir"] = []string{"/build/.metadata"}
	// ChrootMetadataDirBuildFileName key absent => error

	_, err := s.BuildMetadataFile()
	if err == nil {
		t.Fatal("expected error for missing ChrootMetadataDirBuildFileName, got nil")
	}
}

func TestConfigAccessors_PropagatesGetItemError(t *testing.T) {
	for _, tc := range stringAccessors() {
		t.Run(tc.name, func(t *testing.T) {
			wantErr := errors.New("cfg broken")
			s := newTestSeeder()
			s.cfg = &config.ErrConfig{Err: wantErr}

			_, err := tc.call(s)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !errors.Is(err, wantErr) {
				t.Errorf("got error %v, want %v", err, wantErr)
			}
		})
	}
}

func TestSeedsVersioningCadence_ValidValues(t *testing.T) {
	for _, val := range []string{"daily", "weekly", "monthly"} {
		t.Run(val, func(t *testing.T) {
			s := newTestSeeder()
			s.cfg.(*config.MockConfig).Items["Seeder.SeedsVersioningCadence"] = []string{val}

			got, err := s.SeedsVersioningCadence()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != val {
				t.Errorf("got %q, want %q", got, val)
			}
		})
	}
}

func TestSeedsVersioningCadence_DefaultsToWeekly(t *testing.T) {
	s := newTestSeeder()
	// Key absent => GetItem returns "" => defaults to "weekly".

	got, err := s.SeedsVersioningCadence()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "weekly" {
		t.Errorf("got %q, want %q", got, "weekly")
	}
}

func TestSeedsVersioningCadence_InvalidValue(t *testing.T) {
	s := newTestSeeder()
	s.cfg.(*config.MockConfig).Items["Seeder.SeedsVersioningCadence"] = []string{"biweekly"}

	_, err := s.SeedsVersioningCadence()
	if err == nil {
		t.Fatal("expected error for invalid cadence value, got nil")
	}
}

func TestSeedsVersioningCadence_PropagatesError(t *testing.T) {
	wantErr := errors.New("cfg broken")
	s := newTestSeeder()
	s.cfg = &config.ErrConfig{Err: wantErr}

	_, err := s.SeedsVersioningCadence()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("got error %v, want %v", err, wantErr)
	}
}

func TestUseCpReflinkModeInsteadOfRsync_ReturnsTrue(t *testing.T) {
	s := newTestSeeder()
	s.cfg.(*config.MockConfig).Bools["Seeder.UseCpReflinkModeInsteadOfRsync"] = true

	got, err := s.UseCpReflinkModeInsteadOfRsync()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("expected true, got false")
	}
}

func TestUseCpReflinkModeInsteadOfRsync_DefaultFalse(t *testing.T) {
	s := newTestSeeder()

	got, err := s.UseCpReflinkModeInsteadOfRsync()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got {
		t.Error("expected false, got true")
	}
}

func TestUseCpReflinkModeInsteadOfRsync_PropagatesError(t *testing.T) {
	wantErr := errors.New("cfg broken")
	s := newTestSeeder()
	s.cfg = &config.ErrConfig{Err: wantErr}

	_, err := s.UseCpReflinkModeInsteadOfRsync()
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("got error %v, want %v", err, wantErr)
	}
}
