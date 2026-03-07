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
