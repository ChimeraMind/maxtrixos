package seeder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/runner"
)

// newTestSeederWithConfig returns a Seeder with mock dependencies and
// the provided config, suitable for worker unit tests.
func newTestSeederWithConfig(cfg *config.MockConfig) *Seeder {
	mr := runner.NewMockRunner()
	return &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}
}

func workerTestConfig() *config.MockConfig {
	return &config.MockConfig{
		Items: map[string][]string{},
		Bools: map[string]bool{},
	}
}

func TestSeederDoneFlagFile(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeederDoneFlagFileNamePrefix"] = []string{
		"seeder.complete",
	}
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{
		"/build/.seeders_phases",
	}

	sd := newTestSeederWithConfig(cfg)
	got, err := sd.SeederDoneFlagFile("00-bedrock", "/chroot/test")
	if err != nil {
		t.Fatalf("SeederDoneFlagFile: %v", err)
	}

	want := "/chroot/test/build/.seeders_phases/seeder.complete_00-bedrock"
	if got != want {
		t.Errorf("SeederDoneFlagFile:\n got  %q\n want %q", got, want)
	}
}

func TestSeederDoneFlagFileTrimsSlash(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeederDoneFlagFileNamePrefix"] = []string{
		"seeder.complete",
	}
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{
		"/build/.seeders_phases",
	}

	sd := newTestSeederWithConfig(cfg)
	got, err := sd.SeederDoneFlagFile(
		"10-server", "/chroot/test/",
	)
	if err != nil {
		t.Fatalf("SeederDoneFlagFile: %v", err)
	}

	want := "/chroot/test/build/.seeders_phases/seeder.complete_10-server"
	if got != want {
		t.Errorf("SeederDoneFlagFile:\n got  %q\n want %q", got, want)
	}
}

func TestIsSeederDone(t *testing.T) {
	tmp := t.TempDir()
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeederDoneFlagFileNamePrefix"] = []string{
		"seeder.complete",
	}
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{
		"/build/.seeders_phases",
	}

	sd := newTestSeederWithConfig(cfg)

	// Not done yet.
	done, err := sd.IsSeederDone("my-seeder", tmp)
	if err != nil {
		t.Fatalf("IsSeederDone: %v", err)
	}
	if done {
		t.Error("Expected seeder to not be done")
	}
}

func TestMarkSeederDone(t *testing.T) {
	tmp := t.TempDir()
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeederDoneFlagFileNamePrefix"] = []string{
		"seeder.complete",
	}
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{
		"/build/.seeders_phases",
	}

	sd := newTestSeederWithConfig(cfg)

	// Mark as done.
	if err := sd.MarkSeederDone("my-seeder", tmp); err != nil {
		t.Fatalf("MarkSeederDone: %v", err)
	}

	// Verify flag file was created.
	flagFile, _ := sd.SeederDoneFlagFile("my-seeder", tmp)
	if _, err := os.Stat(flagFile); os.IsNotExist(err) {
		t.Errorf("Expected flag file to exist: %s", flagFile)
	}

	// Now IsSeederDone should return true.
	done, err := sd.IsSeederDone("my-seeder", tmp)
	if err != nil {
		t.Fatalf("IsSeederDone: %v", err)
	}
	if !done {
		t.Error("Expected seeder to be done")
	}
}

func TestNewConfigAccessors(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.DownloadsDir"] = []string{"/dl"}
	cfg.Items["Seeder.DistfilesDir"] = []string{"/distfiles"}
	cfg.Items["Seeder.BinpkgsDir"] = []string{"/binpkgs"}
	cfg.Items["Seeder.GpgKeysDir"] = []string{"/gpg"}
	cfg.Items["matrixOS.Root"] = []string{"/root"}
	cfg.Items["matrixOS.DefaultRoot"] = []string{"/matrixos"}
	cfg.Items["matrixOS.GitRepo"] = []string{
		"https://example.com/repo.git",
	}
	cfg.Items["matrixOS.DefaultPrivateGitRepoPath"] = []string{
		"/matrixos/private",
	}

	sd := newTestSeederWithConfig(cfg)

	tests := []struct {
		name string
		fn   func() (string, error)
		want string
	}{
		{"DownloadsDir", sd.DownloadsDir, "/dl"},
		{"DistfilesDir", sd.DistfilesDir, "/distfiles"},
		{"BinpkgsDir", sd.BinpkgsDir, "/binpkgs"},
		{"GpgKeysDir", sd.GpgKeysDir, "/gpg"},
		{"DevDir", sd.DevDir, "/root"},
		{"DefaultDevDir", sd.DefaultDevDir, "/matrixos"},
		{
			"GitRepo", sd.GitRepo,
			"https://example.com/repo.git",
		},
		{
			"DefaultPrivateGitRepoPath",
			sd.DefaultPrivateGitRepoPath,
			"/matrixos/private",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.fn()
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			if got != tc.want {
				t.Errorf("%s: got %q, want %q",
					tc.name, got, tc.want)
			}
		})
	}
}

// writeParamsScript creates a params.sh in dir that exports the three
// seeder variables and defines the <seedName>_params.find_latest_chroot_dir
// function. Returns the full path to the script.
func writeParamsScript(t *testing.T, dir, seedName, chrootName, chrootsDir, preferredDir, latestDir string, allDirs []string) string {
	t.Helper()
	script := fmt.Sprintf(
		"#!/bin/bash\n"+
			"SEEDER_CHROOT_NAME=%q\n"+
			"SEEDER_CHROOTS_DIR=%q\n"+
			"PREFERRED_SEEDER_CHROOT_DIR=%q\n"+
			"%s_params.find_latest_chroot_dir() { echo %q; }\n"+
			"%s_params.find_all_chroot_dirs() { echo %q | xargs -n 1; }",
		chrootName, chrootsDir, preferredDir,
		seedName, latestDir, seedName, strings.Join(allDirs, " "),
	)
	p := filepath.Join(dir, "params.sh")
	if err := os.WriteFile(p, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile params.sh: %v", err)
	}
	return p
}

// newRealSeeder creates a Seeder that uses the real runner.Run so
// bash actually executes. DevDir is set to devDir in the config.
func newRealSeeder(devDir string) *Seeder {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.Root": {devDir},
		},
		Bools: map[string]bool{},
	}
	return &Seeder{
		cfg:          cfg,
		runner:       runner.Run,
		chrootRunner: runner.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}
}

func requireBash(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/bin/bash"); os.IsNotExist(err) {
		t.Skip("skipping: /bin/bash not found")
	}
}

func TestParseSeederParams_Success(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	latestDir := filepath.Join(tmp, "chroots", "bedrock-20260228")
	os.MkdirAll(latestDir, 0755)

	paramsFile := writeParamsScript(t, tmp,
		"bedrock",
		"bedrock-20260228",
		"/mnt/chroots",
		"/mnt/chroots/bedrock-20260228",
		latestDir,
		[]string{
			"/mnt/chroots/bedrock-20260228",
			"/mnt/chroots/bedrock-20260104",
		},
	)

	sd := newRealSeeder(tmp)
	params, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err != nil {
		t.Fatalf("ParseSeederParams: %v", err)
	}

	if params.ChrootName != "bedrock-20260228" {
		t.Errorf("ChrootName: got %q, want %q",
			params.ChrootName, "bedrock-20260228")
	}
	if params.ChrootsDir != "/mnt/chroots" {
		t.Errorf("ChrootsDir: got %q, want %q",
			params.ChrootsDir, "/mnt/chroots")
	}
	if params.PreferredChrootDir != "/mnt/chroots/bedrock-20260228" {
		t.Errorf("PreferredChrootDir: got %q, want %q",
			params.PreferredChrootDir,
			"/mnt/chroots/bedrock-20260228")
	}
	if params.LatestAvailableChrootDir != latestDir {
		t.Errorf("LatestAvailableChrootDir: got %q, want %q",
			params.LatestAvailableChrootDir, latestDir)
	}

	if len(params.AllChrootDirs) != 2 {
		t.Errorf("len(AllChrootDirs): got %d, want %d",
			len(params.AllChrootDirs), 2)
	}

	if params.AllChrootDirs[0] != "/mnt/chroots/bedrock-20260228" {
		t.Errorf("AllChrootDirs[0]: got %q, want %q",
			params.AllChrootDirs[0], "/mnt/chroots/bedrock-20260228")
	}
	if params.AllChrootDirs[1] != "/mnt/chroots/bedrock-20260104" {
		t.Errorf("AllChrootDirs[1]: got %q, want %q",
			params.AllChrootDirs[1], "/mnt/chroots/bedrock-20260104")
	}
}

func TestParseSeederParams_SpaceSeparatedAllChrootDirs(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	latestDir := filepath.Join(tmp, "chroots", "bedrock-20260228")
	os.MkdirAll(latestDir, 0755)

	// Create a params.sh where find_all_chroot_dirs echoes
	// all directories on a SINGLE line, space-separated (no xargs).
	allDirs := []string{
		"/mnt/chroots/bedrock-20260228",
		"/mnt/chroots/bedrock-20260104",
		"/mnt/chroots/bedrock-20251015",
	}
	script := fmt.Sprintf(
		"#!/bin/bash\n"+
			"SEEDER_CHROOT_NAME=bedrock-20260228\n"+
			"SEEDER_CHROOTS_DIR=/mnt/chroots\n"+
			"PREFERRED_SEEDER_CHROOT_DIR=/mnt/chroots/bedrock-20260228\n"+
			"bedrock_params.find_latest_chroot_dir() { echo %q; }\n"+
			"bedrock_params.find_all_chroot_dirs() { echo %q; }",
		latestDir, strings.Join(allDirs, " "),
	)
	paramsFile := filepath.Join(tmp, "params.sh")
	os.WriteFile(paramsFile, []byte(script), 0755)

	sd := newRealSeeder(tmp)
	params, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err != nil {
		t.Fatalf("ParseSeederParams: %v", err)
	}

	if len(params.AllChrootDirs) != 3 {
		t.Fatalf("len(AllChrootDirs): got %d, want 3 (entries: %v)",
			len(params.AllChrootDirs), params.AllChrootDirs)
	}

	for i, want := range allDirs {
		if params.AllChrootDirs[i] != want {
			t.Errorf("AllChrootDirs[%d]: got %q, want %q",
				i, params.AllChrootDirs[i], want)
		}
	}
}

func TestParseSeederParams_UsesDevDir(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	latestDir := filepath.Join(tmp, "latest-chroot")
	os.MkdirAll(latestDir, 0755)

	// params.sh echoes MATRIXOS_DEV_DIR back as the chroot name
	// so we can verify it was set correctly.
	script := fmt.Sprintf("#!/bin/bash\n"+
		"SEEDER_CHROOT_NAME=\"${MATRIXOS_DEV_DIR}\"\n"+
		"SEEDER_CHROOTS_DIR=/chroots\n"+
		"PREFERRED_SEEDER_CHROOT_DIR=/chroots/test\n"+
		"bedrock_params.find_latest_chroot_dir() { echo %q; }\n"+
		"bedrock_params.find_all_chroot_dirs() { echo %q; }\n",
		latestDir, latestDir)
	paramsFile := filepath.Join(tmp, "params.sh")
	os.WriteFile(paramsFile, []byte(script), 0755)

	sd := newRealSeeder("/my/custom/devdir")
	params, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err != nil {
		t.Fatalf("ParseSeederParams: %v", err)
	}
	if params.ChrootName != "/my/custom/devdir" {
		t.Errorf("MATRIXOS_DEV_DIR not passed: got %q",
			params.ChrootName)
	}
}

func TestParseSeederParams_EmptyChrootName(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	latestDir := filepath.Join(tmp, "latest-chroot")
	os.MkdirAll(latestDir, 0755)

	paramsFile := writeParamsScript(t, tmp,
		"bedrock", "", "/mnt/chroots", "/mnt/chroots/bedrock", latestDir,
		[]string{"/mnt/chroots/bedrock"},
	)

	sd := newRealSeeder(tmp)
	_, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err == nil {
		t.Fatal("Expected error for empty ChrootName, got nil")
	}
	if !strings.Contains(err.Error(), "SEEDER_CHROOT_NAME is empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseSeederParams_EmptyChrootsDir(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	latestDir := filepath.Join(tmp, "latest-chroot")
	os.MkdirAll(latestDir, 0755)

	paramsFile := writeParamsScript(t, tmp,
		"bedrock", "bedrock-20260228", "", "/mnt/chroots/bedrock", latestDir,
		[]string{},
	)

	sd := newRealSeeder(tmp)
	_, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err == nil {
		t.Fatal("Expected error for empty ChrootsDir, got nil")
	}
	// Empty middle line survives the split, caught by validation.
	if !strings.Contains(err.Error(), "SEEDER_CHROOTS_DIR is empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseSeederParams_EmptyPreferredChrootDir(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	latestDir := filepath.Join(tmp, "latest-chroot")
	os.MkdirAll(latestDir, 0755)

	paramsFile := writeParamsScript(t, tmp,
		"bedrock", "bedrock-20260228", "/mnt/chroots", "", latestDir,
		[]string{},
	)

	sd := newRealSeeder(tmp)
	_, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err == nil {
		t.Fatal("Expected error for empty PreferredChrootDir, got nil")
	}
	// Empty third echo line survives the split, caught by validation.
	if !strings.Contains(err.Error(), "PREFERRED_SEEDER_CHROOT_DIR is empty") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseSeederParams_EmptyLatestChrootDir(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()

	// The find_latest_chroot_dir function returns empty string.
	paramsFile := writeParamsScript(t, tmp,
		"bedrock", "bedrock-20260228", "/mnt/chroots",
		"/mnt/chroots/bedrock-20260228", "",
		[]string{"/mnt/chroots/bedrock-20260228"},
	)

	sd := newRealSeeder(tmp)
	params, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err != nil {
		t.Fatalf("ParseSeederParams: %v", err)
	}
	// Empty LatestAvailableChrootDir is now accepted.
	if params.LatestAvailableChrootDir != "" {
		t.Errorf("LatestAvailableChrootDir: got %q, want empty",
			params.LatestAvailableChrootDir)
	}
}

func TestParseSeederParams_FunctionMissing(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()

	// params.sh does NOT define bedrock_params.find_latest_chroot_dir.
	// The || true in the template should prevent set -e from killing
	// the script, yielding an empty 4th and 5th line.
	script := "#!/bin/bash\n" +
		"SEEDER_CHROOT_NAME=bedrock-20260228\n" +
		"SEEDER_CHROOTS_DIR=/mnt/chroots\n" +
		"PREFERRED_SEEDER_CHROOT_DIR=/mnt/chroots/bedrock-20260228\n"
	paramsFile := filepath.Join(tmp, "params.sh")
	os.WriteFile(paramsFile, []byte(script), 0755)

	sd := newRealSeeder(tmp)
	params, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err != nil {
		t.Fatalf("ParseSeederParams: %v", err)
	}
	// Missing function yields empty LatestAvailableChrootDir, now accepted.
	if params.LatestAvailableChrootDir != "" {
		t.Errorf("LatestAvailableChrootDir: got %q, want empty",
			params.LatestAvailableChrootDir)
	}
}

func TestParseSeederParams_LatestChrootDirNotExist(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()

	// Point to a directory that does not exist.
	missingDir := filepath.Join(tmp, "does-not-exist")
	paramsFile := writeParamsScript(t, tmp,
		"bedrock", "bedrock-20260228", "/mnt/chroots",
		"/mnt/chroots/bedrock-20260228", missingDir,
		[]string{""},
	)

	sd := newRealSeeder(tmp)
	params, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err != nil {
		t.Fatalf("ParseSeederParams: %v", err)
	}
	// Non-existent directory is now accepted without validation.
	if params.LatestAvailableChrootDir != missingDir {
		t.Errorf("LatestAvailableChrootDir: got %q, want %q",
			params.LatestAvailableChrootDir, missingDir)
	}

	if len(params.AllChrootDirs) != 0 {
		t.Errorf("len(AllChrootDirs): got %d, want %d",
			len(params.AllChrootDirs), 0)
	}
}

func TestParseParamsVariables_LatestAvailableChrootDir(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	latestDir := filepath.Join(tmp, "chroots", "bedrock-latest")
	os.MkdirAll(latestDir, 0755)

	paramsFile := writeParamsScript(t, tmp,
		"bedrock", "bedrock-20260228", "/mnt/chroots",
		"/mnt/chroots/bedrock-20260228", latestDir,
		[]string{latestDir},
	)

	sd := newRealSeeder(tmp)
	// Call parseParamsVariables directly to test the raw output
	// without ParseSeederParams' directory-existence check.
	params, err := sd.parseParamsVariables("00-bedrock", paramsFile)
	if err != nil {
		t.Fatalf("parseParamsVariables: %v", err)
	}

	if params.LatestAvailableChrootDir != latestDir {
		t.Errorf("LatestAvailableChrootDir: got %q, want %q",
			params.LatestAvailableChrootDir, latestDir)
	}
	if params.ChrootName != "bedrock-20260228" {
		t.Errorf("ChrootName: got %q, want %q",
			params.ChrootName, "bedrock-20260228")
	}
}

func TestParseParamsVariables_DifferentSeedNames(t *testing.T) {
	requireBash(t)

	tests := []struct {
		seederName string // e.g. "10-server"
		seedName   string // e.g. "server"
	}{
		{"00-bedrock", "bedrock"},
		{"10-server", "server"},
		{"20-gnome", "gnome"},
		{"21-cosmic", "cosmic"},
	}

	for _, tc := range tests {
		t.Run(tc.seederName, func(t *testing.T) {
			tmp := t.TempDir()
			latestDir := filepath.Join(tmp, "chroots", "latest")
			os.MkdirAll(latestDir, 0755)

			paramsFile := writeParamsScript(t, tmp,
				tc.seedName, "chroot-name", "/chroots",
				"/chroots/preferred", latestDir,
				[]string{latestDir},
			)

			sd := newRealSeeder(tmp)
			params, err := sd.parseParamsVariables(tc.seederName, paramsFile)
			if err != nil {
				t.Fatalf("parseParamsVariables(%q): %v", tc.seederName, err)
			}

			if params.LatestAvailableChrootDir != latestDir {
				t.Errorf("LatestAvailableChrootDir: got %q, want %q",
					params.LatestAvailableChrootDir, latestDir)
			}
		})
	}
}

func TestParseSeederParams_MissingVariable(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()

	// Script only sets one of the three required variables.
	script := "#!/bin/bash\nSEEDER_CHROOT_NAME=bedrock\n"
	paramsFile := filepath.Join(tmp, "params.sh")
	os.WriteFile(paramsFile, []byte(script), 0755)

	sd := newRealSeeder(tmp)
	_, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err == nil {
		t.Fatal("Expected error for missing variables, got nil")
	}
}

func TestParseSeederParams_BadScript(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()

	// Write a script with a syntax error.
	paramsFile := filepath.Join(tmp, "params.sh")
	os.WriteFile(paramsFile, []byte("#!/bin/bash\n(((\n"), 0755)

	sd := newRealSeeder(tmp)
	_, err := sd.ParseSeederParams("00-bedrock", paramsFile)
	if err == nil {
		t.Fatal("Expected error for bad script, got nil")
	}
	if !strings.Contains(err.Error(), "failed to source params") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseSeederParams_NonexistentFile(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()

	sd := newRealSeeder(tmp)
	_, err := sd.ParseSeederParams(
		"00-bedrock",
		filepath.Join(tmp, "does-not-exist.sh"),
	)
	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}
	if !strings.Contains(err.Error(), "failed to source params") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseSeederParams_DevDirError(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.Root": {""},
		},
		Bools: map[string]bool{},
	}
	sd := &Seeder{
		cfg:          cfg,
		runner:       runner.Run,
		chrootRunner: runner.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	_, err := sd.ParseSeederParams("00-bedrock", "/fake/params.sh")
	if err == nil {
		t.Fatal("Expected error for empty DevDir, got nil")
	}
	if !strings.Contains(err.Error(), "dev dir") {
		t.Errorf("Unexpected error: %v", err)
	}
}

// --- ExecutePrepper tests ---

// writePrepperScript creates a prepper script in dir that writes all
// the expected env vars to an output file. Returns the script path.
func writePrepperScript(t *testing.T, dir, outputFile string) string {
	t.Helper()
	script := fmt.Sprintf(`#!/bin/bash
set -eu
cat > %q <<EOF
MATRIXOS_DEV_DIR=${MATRIXOS_DEV_DIR}
SEEDER_BUILD_METADATA_FILE=${SEEDER_BUILD_METADATA_FILE}
SEEDER_LOCK_DIR=${SEEDER_LOCK_DIR}
SEEDER_LOCK_WAIT_SECS=${SEEDER_LOCK_WAIT_SECS}
PREPPERS_PHASES_STATE_DIR=${PREPPERS_PHASES_STATE_DIR}
USE_CP_REFLINK_MODE_INSTEAD_OF_RSYNC=${USE_CP_REFLINK_MODE_INSTEAD_OF_RSYNC}
SEEDER_CHROOT_NAME=${SEEDER_CHROOT_NAME}
SEEDER_CHROOTS_DIR=${SEEDER_CHROOTS_DIR}
PREFERRED_SEEDER_CHROOT_DIR=${PREFERRED_SEEDER_CHROOT_DIR}
CHROOT_DIR=${CHROOT_DIR}
DOWNLOAD_DIR=${DOWNLOAD_DIR}
CHROOT_RESUME=${CHROOT_RESUME}
STAGE3_FILE=${STAGE3_FILE}
STAGE3_URL=${STAGE3_URL}
EOF
`, outputFile)
	p := filepath.Join(dir, "prepper.sh")
	if err := os.WriteFile(p, []byte(script), 0755); err != nil {
		t.Fatalf("WriteFile prepper.sh: %v", err)
	}
	return p
}

func newPrepperSeeder(devDir, downloadsDir, stage3URL string) *Seeder {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.Root":                         {devDir},
			"Seeder.DownloadsDir":                   {downloadsDir},
			"Seeder.Stage3DownloadUrl":              {stage3URL},
			"Seeder.ChrootMetadataDir":              {"/build/.metadata"},
			"Seeder.ChrootMetadataDirBuildFileName": {"build.json"},
			"Seeder.LocksDir":                       {"/locks/seeder"},
			"Seeder.LockWaitSeconds":                {"86400"},
			"Seeder.ChrootPreppersPhasesStateDir":   {"/build/preppers/.preppers_phases"},
		},
		Bools: map[string]bool{},
	}
	return &Seeder{
		cfg:          cfg,
		runner:       runner.Run,
		chrootRunner: runner.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}
}

// readEnvFile parses a KEY=VALUE file into a map.
func readEnvFile(t *testing.T, path string) map[string]string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile %s: %v", path, err)
	}
	m := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			m[parts[0]] = parts[1]
		}
	}
	return m
}

func TestExecutePrepper_Success(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "env-output.txt")
	prepperScript := writePrepperScript(t, tmp, outputFile)

	sd := newPrepperSeeder(
		"/my/dev",
		"/my/downloads",
		"https://example.com/stage3.tar.xz",
	)

	info := SeederInfo{
		Name:        "00-bedrock",
		PrepperExec: prepperScript,
	}
	params := &SeederParams{
		ChrootName:         "bedrock-20260228",
		ChrootsDir:         "/mnt/chroots",
		PreferredChrootDir: "/mnt/chroots/bedrock-20260228",
	}
	opts := &PrepperOptions{
		ChrootDir:  "/mnt/chroots/bedrock-20260228",
		Resume:     false,
		Stage3File: "stage3-amd64-latest.tar.xz",
	}

	if err := sd.ExecutePrepper(info, params, opts); err != nil {
		t.Fatalf("ExecutePrepper: %v", err)
	}

	env := readEnvFile(t, outputFile)

	checks := map[string]string{
		"MATRIXOS_DEV_DIR":                     "/my/dev",
		"SEEDER_BUILD_METADATA_FILE":           "/build/.metadata/build.json",
		"SEEDER_LOCK_DIR":                      "/locks/seeder",
		"SEEDER_LOCK_WAIT_SECS":                "86400",
		"PREPPERS_PHASES_STATE_DIR":            "/build/preppers/.preppers_phases",
		"USE_CP_REFLINK_MODE_INSTEAD_OF_RSYNC": "",
		"SEEDER_CHROOT_NAME":                   "bedrock-20260228",
		"SEEDER_CHROOTS_DIR":                   "/mnt/chroots",
		"PREFERRED_SEEDER_CHROOT_DIR":          "/mnt/chroots/bedrock-20260228",
		"CHROOT_DIR":                           "/mnt/chroots/bedrock-20260228",
		"DOWNLOAD_DIR":                         "/my/downloads",
		"CHROOT_RESUME":                        "",
		"STAGE3_FILE":                          "stage3-amd64-latest.tar.xz",
		"STAGE3_URL":                           "https://example.com/stage3.tar.xz",
	}
	for k, want := range checks {
		got := env[k]
		if got != want {
			t.Errorf("%s: got %q, want %q", k, got, want)
		}
	}
}

func TestExecutePrepper_Resume(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "env-output.txt")
	prepperScript := writePrepperScript(t, tmp, outputFile)

	sd := newPrepperSeeder(
		"/dev/dir",
		"/downloads",
		"https://example.com/stage3.tar.xz",
	)

	info := SeederInfo{PrepperExec: prepperScript}
	params := &SeederParams{
		ChrootName:         "bedrock",
		ChrootsDir:         "/chroots",
		PreferredChrootDir: "/chroots/bedrock",
	}
	opts := &PrepperOptions{
		ChrootDir: "/chroots/bedrock",
		Resume:    true,
	}

	if err := sd.ExecutePrepper(info, params, opts); err != nil {
		t.Fatalf("ExecutePrepper: %v", err)
	}

	env := readEnvFile(t, outputFile)
	if env["CHROOT_RESUME"] != "1" {
		t.Errorf("CHROOT_RESUME: got %q, want %q",
			env["CHROOT_RESUME"], "1")
	}
}

func TestExecutePrepper_UseCpReflink(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	outputFile := filepath.Join(tmp, "env-output.txt")
	prepperScript := writePrepperScript(t, tmp, outputFile)

	sd := newPrepperSeeder(
		"/dev/dir",
		"/downloads",
		"https://example.com/stage3.tar.xz",
	)
	// Enable the reflink flag.
	sd.cfg.(*config.MockConfig).Bools["Seeder.UseCpReflinkModeInsteadOfRsync"] = true

	info := SeederInfo{PrepperExec: prepperScript}
	params := &SeederParams{
		ChrootName:         "bedrock",
		ChrootsDir:         "/chroots",
		PreferredChrootDir: "/chroots/bedrock",
	}
	opts := &PrepperOptions{
		ChrootDir: "/chroots/bedrock",
	}

	if err := sd.ExecutePrepper(info, params, opts); err != nil {
		t.Fatalf("ExecutePrepper: %v", err)
	}

	env := readEnvFile(t, outputFile)
	if env["USE_CP_REFLINK_MODE_INSTEAD_OF_RSYNC"] != "1" {
		t.Errorf("USE_CP_REFLINK_MODE_INSTEAD_OF_RSYNC: got %q, want %q",
			env["USE_CP_REFLINK_MODE_INSTEAD_OF_RSYNC"], "1")
	}
}

func TestExecutePrepper_ScriptFailure(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()

	// Write a script that exits with an error.
	failScript := filepath.Join(tmp, "fail-prepper.sh")
	os.WriteFile(failScript, []byte("#!/bin/bash\nexit 1\n"), 0755)

	sd := newPrepperSeeder(
		"/dev/dir", "/downloads",
		"https://example.com/stage3.tar.xz",
	)

	info := SeederInfo{PrepperExec: failScript}
	params := &SeederParams{
		ChrootName:         "bedrock",
		ChrootsDir:         "/chroots",
		PreferredChrootDir: "/chroots/bedrock",
	}
	opts := &PrepperOptions{ChrootDir: "/chroots/bedrock"}

	err := sd.ExecutePrepper(info, params, opts)
	if err == nil {
		t.Fatal("Expected error from failing script, got nil")
	}
}

func TestExecutePrepper_DevDirError(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.Root":            {""},
			"Seeder.DownloadsDir":      {"/dl"},
			"Seeder.Stage3DownloadUrl": {"http://x"},
		},
		Bools: map[string]bool{},
	}
	sd := &Seeder{
		cfg:          cfg,
		runner:       runner.Run,
		chrootRunner: runner.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	err := sd.ExecutePrepper(
		SeederInfo{PrepperExec: "/fake"},
		&SeederParams{ChrootName: "x", ChrootsDir: "/x", PreferredChrootDir: "/x"},
		&PrepperOptions{ChrootDir: "/x"},
	)
	if err == nil {
		t.Fatal("Expected error for empty DevDir, got nil")
	}
}

func TestExecutePrepper_DownloadsDirError(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.Root":            {"/dev"},
			"Seeder.DownloadsDir":      {""},
			"Seeder.Stage3DownloadUrl": {"http://x"},
		},
		Bools: map[string]bool{},
	}
	sd := &Seeder{
		cfg:          cfg,
		runner:       runner.Run,
		chrootRunner: runner.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	err := sd.ExecutePrepper(
		SeederInfo{PrepperExec: "/fake"},
		&SeederParams{ChrootName: "x", ChrootsDir: "/x", PreferredChrootDir: "/x"},
		&PrepperOptions{ChrootDir: "/x"},
	)
	if err == nil {
		t.Fatal("Expected error for empty DownloadsDir, got nil")
	}
}

func TestExecutePrepper_Stage3URLError(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.Root":            {"/dev"},
			"Seeder.DownloadsDir":      {"/dl"},
			"Seeder.Stage3DownloadUrl": {""},
		},
		Bools: map[string]bool{},
	}
	sd := &Seeder{
		cfg:          cfg,
		runner:       runner.Run,
		chrootRunner: runner.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	err := sd.ExecutePrepper(
		SeederInfo{PrepperExec: "/fake"},
		&SeederParams{ChrootName: "x", ChrootsDir: "/x", PreferredChrootDir: "/x"},
		&PrepperOptions{ChrootDir: "/x"},
	)
	if err == nil {
		t.Fatal("Expected error for empty Stage3URL, got nil")
	}
}

// --- Seed tests ---

func TestSeed_Success(t *testing.T) {
	mr := runner.NewMockRunner()
	cfg := workerTestConfig()
	cfg.Items["matrixOS.DefaultRoot"] = []string{"/matrixos"}
	cfg.Items["matrixOS.Root"] = []string{"/sr/build/daily"}

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	info := SeederInfo{
		Name:             "00-bedrock",
		ChrootExec:       "/srv/build/daily/build/seeders/00-bedrock/chroot.sh",
		ChrootChrootExec: "/matrixos/build/seeders/00-bedrock/chroot.sh",
	}
	if err := sd.Seed("/mnt/chroot", info); err != nil {
		t.Fatalf("Seed: unexpected error: %v", err)
	}

	if len(mr.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mr.Calls))
	}
	call := mr.Calls[0]
	if call.Name != "chroot:/matrixos/build/seeders/00-bedrock/chroot.sh" {
		t.Errorf("Name = %q, want chroot exec name", call.Name)
	}
	if call.ChrootDir != "/mnt/chroot" {
		t.Errorf("ChrootDir = %q, want %q", call.ChrootDir, "/mnt/chroot")
	}

	// Verify MATRIXOS_DEV_DIR is in the env.
	found := false
	for _, e := range call.Env {
		if e == "MATRIXOS_DEV_DIR=/matrixos" {
			found = true
			break
		}
	}
	if !found {
		t.Error("MATRIXOS_DEV_DIR=/matrixos not found in env")
	}
}

func TestSeed_DefaultDevDirError(t *testing.T) {
	mr := runner.NewMockRunner()
	cfg := workerTestConfig()
	cfg.Items["matrixOS.DefaultRoot"] = []string{""} // empty → error

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	err := sd.Seed("/mnt/chroot", SeederInfo{ChrootExec: "/x"})
	if err == nil {
		t.Fatal("expected error for empty DefaultDevDir")
	}
}

func TestSeed_ChrootRunnerError(t *testing.T) {
	mr := runner.NewMockRunnerFailOnCall(0, fmt.Errorf("chroot boom"))
	cfg := workerTestConfig()
	cfg.Items["matrixOS.DefaultRoot"] = []string{"/matrixos"}

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	err := sd.Seed("/mnt/chroot", SeederInfo{ChrootExec: "/x"})
	if err == nil {
		t.Fatal("expected error from chrootRunner")
	}
	if !strings.Contains(err.Error(), "chroot boom") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- ImportGentooGpgKeys tests ---

func TestImportGentooGpgKeys_Success(t *testing.T) {
	tmp := t.TempDir()
	gpgDir := filepath.Join(tmp, "gpg")

	mr := runner.NewMockRunner()
	cfg := workerTestConfig()
	cfg.Items["Seeder.GpgKeysDir"] = []string{gpgDir}

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	if err := sd.ImportGentooGpgKeys(); err != nil {
		t.Fatalf("ImportGentooGpgKeys: %v", err)
	}

	// gpgDir should have been created with 0700 permissions.
	info, err := os.Stat(gpgDir)
	if err != nil {
		t.Fatalf("Stat gpgDir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("gpgDir permissions = %o, want 0700", perm)
	}

	// The mock runner should have been called with gpg.
	if len(mr.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mr.Calls))
	}
	if mr.Calls[0].Name != "gpg" {
		t.Errorf("Name = %q, want %q", mr.Calls[0].Name, "gpg")
	}
}

func TestImportGentooGpgKeys_FixesFilePermissions(t *testing.T) {
	tmp := t.TempDir()
	gpgDir := filepath.Join(tmp, "gpg")
	os.MkdirAll(gpgDir, 0755)

	// Create a file with overly broad permissions.
	testFile := filepath.Join(gpgDir, "pubring.kbx")
	os.WriteFile(testFile, []byte("key"), 0644)

	mr := runner.NewMockRunner()
	cfg := workerTestConfig()
	cfg.Items["Seeder.GpgKeysDir"] = []string{gpgDir}

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	if err := sd.ImportGentooGpgKeys(); err != nil {
		t.Fatalf("ImportGentooGpgKeys: %v", err)
	}

	info, err := os.Stat(testFile)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("file permissions = %o, want 0600", perm)
	}
}

func TestImportGentooGpgKeys_GpgKeysDirError(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.GpgKeysDir"] = []string{""} // empty → error

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	if err := sd.ImportGentooGpgKeys(); err == nil {
		t.Fatal("expected error for empty GpgKeysDir")
	}
}

func TestImportGentooGpgKeys_RunnerError(t *testing.T) {
	tmp := t.TempDir()
	gpgDir := filepath.Join(tmp, "gpg")

	mr := runner.NewMockRunnerFailOnCall(0, fmt.Errorf("gpg failed"))
	cfg := workerTestConfig()
	cfg.Items["Seeder.GpgKeysDir"] = []string{gpgDir}

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	err := sd.ImportGentooGpgKeys()
	if err == nil {
		t.Fatal("expected error from runner")
	}
	if !strings.Contains(err.Error(), "gpg failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- SetupChrootDNS tests ---

func TestSetupChrootDNS_Success(t *testing.T) {
	// This test requires /etc/resolv.conf to exist on the host.
	if _, err := os.Stat("/etc/resolv.conf"); os.IsNotExist(err) {
		t.Skip("skipping: /etc/resolv.conf not found")
	}

	tmp := t.TempDir()
	cfg := workerTestConfig()
	sd := newTestSeederWithConfig(cfg)

	if err := sd.SetupChrootDNS(tmp); err != nil {
		t.Fatalf("SetupChrootDNS: %v", err)
	}

	dst := filepath.Join(tmp, "etc", "resolv.conf")
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Fatal("resolv.conf not copied into chroot")
	}

	// Content should match host.
	hostData, _ := os.ReadFile("/etc/resolv.conf")
	chrootData, _ := os.ReadFile(dst)
	if string(chrootData) != string(hostData) {
		t.Error("chroot resolv.conf does not match host")
	}
}

func TestSetupChrootDNS_CreatesEtcDir(t *testing.T) {
	if _, err := os.Stat("/etc/resolv.conf"); os.IsNotExist(err) {
		t.Skip("skipping: /etc/resolv.conf not found")
	}

	tmp := t.TempDir()
	cfg := workerTestConfig()
	sd := newTestSeederWithConfig(cfg)

	// etc dir does not exist yet — SetupChrootDNS should create it.
	if err := sd.SetupChrootDNS(tmp); err != nil {
		t.Fatalf("SetupChrootDNS: %v", err)
	}

	etcDir := filepath.Join(tmp, "etc")
	info, err := os.Stat(etcDir)
	if err != nil {
		t.Fatalf("Stat etc: %v", err)
	}
	if !info.IsDir() {
		t.Error("etc is not a directory")
	}
}

// --- SetupChrootDirs tests ---

func TestSetupChrootDirs_CreatesPhasesDirAndCallsSetupDevDir(t *testing.T) {
	tmp := t.TempDir()

	mr := runner.NewMockRunner()
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{"/build/.seeders_phases"}
	cfg.Items["matrixOS.DefaultRoot"] = []string{"/matrixos"}
	cfg.Items["Seeder.UseLocalGitRepoInsideChroot"] = []string{"true"}
	cfg.Bools["Seeder.UseLocalGitRepoInsideChroot"] = true
	cfg.Items["Seeder.GitCloneArgs"] = []string{"--depth 1"}
	cfg.Items["matrixOS.Root"] = []string{"/dev/toolkit"}
	cfg.Items["Seeder.DeleteDotGitFromGitRepo"] = []string{"false"}
	cfg.Bools["Seeder.DeleteDotGitFromGitRepo"] = false

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	if err := sd.SetupChrootDirs(tmp); err != nil {
		t.Fatalf("SetupChrootDirs: %v", err)
	}

	// Phases dir should exist.
	phasesDir := filepath.Join(tmp, "build", ".seeders_phases")
	if _, err := os.Stat(phasesDir); os.IsNotExist(err) {
		t.Error("phases dir not created")
	}

	// git clone should have been called (local clone).
	if len(mr.Calls) < 1 {
		t.Fatal("expected at least 1 runner call (git clone)")
	}
	call := mr.Calls[0]
	if call.Name != "git" {
		t.Errorf("expected git call, got %q", call.Name)
	}
	if call.Args[0] != "clone" {
		t.Errorf("expected clone subcommand, got %q", call.Args[0])
	}
}

func TestSetupChrootDirs_SkipsCloneIfDevDirExists(t *testing.T) {
	tmp := t.TempDir()

	mr := runner.NewMockRunner()
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{"/build/.seeders_phases"}
	cfg.Items["matrixOS.DefaultRoot"] = []string{"/matrixos"}
	cfg.Items["Seeder.DeleteDotGitFromGitRepo"] = []string{"false"}
	cfg.Bools["Seeder.DeleteDotGitFromGitRepo"] = false

	// Pre-create the dev dir so clone is skipped.
	chrootDevDir := filepath.Join(tmp, "matrixos")
	os.MkdirAll(chrootDevDir, 0755)

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	if err := sd.SetupChrootDirs(tmp); err != nil {
		t.Fatalf("SetupChrootDirs: %v", err)
	}

	// No runner calls expected (clone skipped).
	if len(mr.Calls) != 0 {
		t.Errorf("expected 0 runner calls, got %d", len(mr.Calls))
	}
}

func TestSetupChrootDirs_RemoteClone(t *testing.T) {
	tmp := t.TempDir()

	mr := runner.NewMockRunner()
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{"/build/.seeders_phases"}
	cfg.Items["matrixOS.DefaultRoot"] = []string{"/matrixos"}
	cfg.Items["Seeder.UseLocalGitRepoInsideChroot"] = []string{"false"}
	cfg.Bools["Seeder.UseLocalGitRepoInsideChroot"] = false
	cfg.Items["Seeder.GitCloneArgs"] = []string{"--depth 1"}
	cfg.Items["matrixOS.GitRepo"] = []string{"https://example.com/repo.git"}
	cfg.Items["Seeder.DeleteDotGitFromGitRepo"] = []string{"false"}
	cfg.Bools["Seeder.DeleteDotGitFromGitRepo"] = false

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	if err := sd.SetupChrootDirs(tmp); err != nil {
		t.Fatalf("SetupChrootDirs: %v", err)
	}

	// RetryableCmd calls the runner multiple times on failure, but on
	// success the first call is git clone with the remote URL.
	if len(mr.Calls) < 1 {
		t.Fatal("expected at least 1 runner call")
	}
	call := mr.Calls[0]
	if call.Name != "git" {
		t.Errorf("expected git, got %q", call.Name)
	}
	// The URL should appear in the args.
	found := false
	for _, a := range call.Args {
		if a == "https://example.com/repo.git" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("remote URL not found in args: %v", call.Args)
	}
}

func TestSetupChrootDirs_PhasesStateDirError(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{""} // error
	cfg.Items["matrixOS.DefaultRoot"] = []string{"/matrixos"}

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	err := sd.SetupChrootDirs("/tmp/fake")
	if err == nil {
		t.Fatal("expected error for empty PhasesStateDir")
	}
}

// --- cleanDevDirGitDir tests ---

func TestCleanDevDirGitDir_DeletesWhenConfigured(t *testing.T) {
	tmp := t.TempDir()
	dotGit := filepath.Join(tmp, ".git")
	os.MkdirAll(dotGit, 0755)
	os.WriteFile(filepath.Join(dotGit, "HEAD"), []byte("ref"), 0644)

	cfg := workerTestConfig()
	cfg.Items["Seeder.DeleteDotGitFromGitRepo"] = []string{"true"}
	cfg.Bools["Seeder.DeleteDotGitFromGitRepo"] = true

	sd := newTestSeederWithConfig(cfg)

	if err := sd.cleanDevDirGitDir(tmp); err != nil {
		t.Fatalf("cleanDevDirGitDir: %v", err)
	}

	if _, err := os.Stat(dotGit); !os.IsNotExist(err) {
		t.Error(".git dir should have been deleted")
	}
}

func TestCleanDevDirGitDir_SkipsWhenNotConfigured(t *testing.T) {
	tmp := t.TempDir()
	dotGit := filepath.Join(tmp, ".git")
	os.MkdirAll(dotGit, 0755)

	cfg := workerTestConfig()
	cfg.Items["Seeder.DeleteDotGitFromGitRepo"] = []string{"false"}
	cfg.Bools["Seeder.DeleteDotGitFromGitRepo"] = false

	sd := newTestSeederWithConfig(cfg)

	if err := sd.cleanDevDirGitDir(tmp); err != nil {
		t.Fatalf("cleanDevDirGitDir: %v", err)
	}

	if _, err := os.Stat(dotGit); os.IsNotExist(err) {
		t.Error(".git dir should NOT have been deleted")
	}
}

func TestCleanDevDirGitDir_NoDotGitDir(t *testing.T) {
	tmp := t.TempDir()

	cfg := workerTestConfig()
	cfg.Items["Seeder.DeleteDotGitFromGitRepo"] = []string{"true"}
	cfg.Bools["Seeder.DeleteDotGitFromGitRepo"] = true

	sd := newTestSeederWithConfig(cfg)

	// Should succeed without error when .git doesn't exist.
	if err := sd.cleanDevDirGitDir(tmp); err != nil {
		t.Fatalf("cleanDevDirGitDir: %v", err)
	}
}

// --- Done-flag error branch tests ---

func TestSeederDoneFlagFile_PhasesStateDirError(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{""}
	cfg.Items["Seeder.ChrootSeederDoneFlagFileNamePrefix"] = []string{"done"}

	sd := newTestSeederWithConfig(cfg)
	_, err := sd.SeederDoneFlagFile("x", "/chroot")
	if err == nil {
		t.Fatal("expected error for empty PhasesStateDir")
	}
}

func TestSeederDoneFlagFile_PrefixError(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{"/build/.phases"}
	cfg.Items["Seeder.ChrootSeederDoneFlagFileNamePrefix"] = []string{""}

	sd := newTestSeederWithConfig(cfg)
	_, err := sd.SeederDoneFlagFile("x", "/chroot")
	if err == nil {
		t.Fatal("expected error for empty prefix")
	}
}

func TestIsSeederDone_ConfigError(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{""}

	sd := newTestSeederWithConfig(cfg)
	_, err := sd.IsSeederDone("x", "/chroot")
	if err == nil {
		t.Fatal("expected error propagated from SeederDoneFlagFile")
	}
}

func TestMarkSeederDone_ConfigError(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.ChrootSeedersPhasesStateDir"] = []string{""}

	sd := newTestSeederWithConfig(cfg)
	err := sd.MarkSeederDone("x", "/chroot")
	if err == nil {
		t.Fatal("expected error propagated from SeederDoneFlagFile")
	}
}

// --- Mount tests ---
//
// These tests mock the syscall-level Mount/Unmount in the filesystems
// package and use real temp directories so the os.Stat/MkdirAll calls
// inside the constructor succeed.

// mockMountSyscalls replaces filesystems.Mount and filesystems.Unmount
// with no-ops for the duration of a test.
func mockMountSyscalls(t *testing.T) {
	t.Helper()
	origMount := filesystems.Mount
	origUnmount := filesystems.Unmount
	filesystems.Mount = func(source, target, fstype string, flags uintptr, data string) error {
		return nil
	}
	filesystems.Unmount = func(target string, flags int) error {
		return nil
	}
	t.Cleanup(func() {
		filesystems.Mount = origMount
		filesystems.Unmount = origUnmount
	})
}

// mountTestConfig returns a config with all keys needed by the mount
// functions. The paths point into tmp so os.Stat succeeds.
func mountTestConfig(tmp string) *config.MockConfig {
	privateSrc := filepath.Join(tmp, "private-repo")
	distSrc := filepath.Join(tmp, "distfiles")
	binSrc := filepath.Join(tmp, "binpkgs")

	// Pre-create source directories the constructors will Stat.
	os.MkdirAll(privateSrc, 0755)
	os.MkdirAll(distSrc, 0755)
	os.MkdirAll(binSrc, 0755)

	return &config.MockConfig{
		Items: map[string][]string{
			"matrixOS.PrivateGitRepoPath":        {privateSrc},
			"matrixOS.DefaultPrivateGitRepoPath": {"/matrixos/private"},
			"Seeder.DistfilesDir":                {distSrc},
			"Seeder.BinpkgsDir":                  {binSrc},
		},
		Bools: map[string]bool{},
	}
}

func TestMountPrivateGitRepo_Success(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: false,
	}
	if err := sd.mountPrivateGitRepo(&opts); err != nil {
		t.Fatalf("mountPrivateGitRepo: %v", err)
	}

	// Verify the destination directory was created inside the chroot.
	dst := filepath.Join(chrootDir, "matrixos", "private")
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Errorf("expected dst dir %s to exist", dst)
	}

	// Verify the mount was tracked.
	if len(sd.trackedMounts) != 1 {
		t.Fatalf("expected 1 tracked mount, got %d", len(sd.trackedMounts))
	}
	if sd.trackedMounts[0] != dst {
		t.Errorf("tracked mount = %q, want %q", sd.trackedMounts[0], dst)
	}
}

func TestMountPrivateGitRepo_ConfigError(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["matrixOS.PrivateGitRepoPath"] = []string{""}
	cfg.Items["matrixOS.DefaultPrivateGitRepoPath"] = []string{"/x"}

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir:     "/tmp/fake",
		SkipIfMounted: false,
	}
	err := sd.mountPrivateGitRepo(&opts)
	if err == nil {
		t.Fatal("expected error for empty PrivateGitRepoPath")
	}
	if !strings.Contains(err.Error(), "private repo path") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMountPrivateGitRepo_SkipIfMounted(t *testing.T) {
	mockMountSyscalls(t)
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	mr := runner.NewMockRunner()
	var stdout bytes.Buffer
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &stdout,
		stderr:       &bytes.Buffer{},
	}

	// Pre-create and pretend the destination is already mounted.
	dst := filepath.Join(chrootDir, "matrixos", "private")
	os.MkdirAll(dst, 0755)
	mockMountInfo(t, []*filesystems.MountInfoEntry{
		{Mountpoint: dst},
	})

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}
	if err := sd.mountPrivateGitRepo(&opts); err != nil {
		t.Fatalf("mountPrivateGitRepo: %v", err)
	}

	// Must NOT be tracked.
	if len(sd.trackedMounts) != 0 {
		t.Errorf("expected 0 tracked mounts, got %d: %v",
			len(sd.trackedMounts), sd.trackedMounts)
	}
	if !strings.Contains(stdout.String(), "Skipping (already mounted)") {
		t.Errorf("expected skip message, got:\n%s", stdout.String())
	}
}

func TestMountPrivateGitRepo_NoSkipWhenNotMounted(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	mr := runner.NewMockRunner()
	var stdout bytes.Buffer
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &stdout,
		stderr:       &bytes.Buffer{},
	}

	// Nothing is mounted.
	mockMountInfo(t, []*filesystems.MountInfoEntry{})

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}
	if err := sd.mountPrivateGitRepo(&opts); err != nil {
		t.Fatalf("mountPrivateGitRepo: %v", err)
	}

	// Must be tracked since it wasn't already mounted.
	if len(sd.trackedMounts) != 1 {
		t.Errorf("expected 1 tracked mount, got %d: %v",
			len(sd.trackedMounts), sd.trackedMounts)
	}
	if strings.Contains(stdout.String(), "Skipping (already mounted)") {
		t.Errorf("did not expect skip message, got:\n%s", stdout.String())
	}
}

func TestMountDistDir_Success(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	distSrc := filepath.Join(tmp, "distfiles")
	os.MkdirAll(distSrc, 0755)

	cfg := workerTestConfig()
	cfg.Items["Seeder.DistfilesDir"] = []string{distSrc}

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: false,
	}
	if err := sd.mountDistDir(&opts); err != nil {
		t.Fatalf("mountDistDir: %v", err)
	}

	dst := filepath.Join(chrootDir, "var", "cache", "distfiles")
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Errorf("expected dst dir %s to exist", dst)
	}
	if len(sd.trackedMounts) != 1 {
		t.Fatalf("expected 1 tracked mount, got %d", len(sd.trackedMounts))
	}
}

func TestMountDistDir_ConfigError(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.DistfilesDir"] = []string{""}

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir:     "/tmp/fake",
		SkipIfMounted: false,
	}
	err := sd.mountDistDir(&opts)
	if err == nil {
		t.Fatal("expected error for empty DistfilesDir")
	}
}

func TestMountDistDir_SkipIfMounted(t *testing.T) {
	mockMountSyscalls(t)
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	distSrc := filepath.Join(tmp, "distfiles")
	os.MkdirAll(distSrc, 0755)

	cfg := workerTestConfig()
	cfg.Items["Seeder.DistfilesDir"] = []string{distSrc}

	mr := runner.NewMockRunner()
	var stdout bytes.Buffer
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &stdout,
		stderr:       &bytes.Buffer{},
	}

	// Pre-create and pretend the destination is already mounted.
	dst := filepath.Join(chrootDir, "var", "cache", "distfiles")
	os.MkdirAll(dst, 0755)
	mockMountInfo(t, []*filesystems.MountInfoEntry{
		{Mountpoint: dst},
	})

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}
	if err := sd.mountDistDir(&opts); err != nil {
		t.Fatalf("mountDistDir: %v", err)
	}

	if len(sd.trackedMounts) != 0 {
		t.Errorf("expected 0 tracked mounts, got %d: %v",
			len(sd.trackedMounts), sd.trackedMounts)
	}
	if !strings.Contains(stdout.String(), "Skipping (already mounted)") {
		t.Errorf("expected skip message, got:\n%s", stdout.String())
	}
}

func TestMountDistDir_NoSkipWhenNotMounted(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	distSrc := filepath.Join(tmp, "distfiles")
	os.MkdirAll(distSrc, 0755)

	cfg := workerTestConfig()
	cfg.Items["Seeder.DistfilesDir"] = []string{distSrc}

	mr := runner.NewMockRunner()
	var stdout bytes.Buffer
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &stdout,
		stderr:       &bytes.Buffer{},
	}

	mockMountInfo(t, []*filesystems.MountInfoEntry{})

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}
	if err := sd.mountDistDir(&opts); err != nil {
		t.Fatalf("mountDistDir: %v", err)
	}

	if len(sd.trackedMounts) != 1 {
		t.Errorf("expected 1 tracked mount, got %d: %v",
			len(sd.trackedMounts), sd.trackedMounts)
	}
	if strings.Contains(stdout.String(), "Skipping (already mounted)") {
		t.Errorf("did not expect skip message, got:\n%s", stdout.String())
	}
}

func TestMountBinpkgsDir_Success(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	binSrc := filepath.Join(tmp, "binpkgs")
	os.MkdirAll(binSrc, 0755)

	cfg := workerTestConfig()
	cfg.Items["Seeder.BinpkgsDir"] = []string{binSrc}

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: false,
	}
	if err := sd.mountBinpkgsDir(&opts); err != nil {
		t.Fatalf("mountBinpkgsDir: %v", err)
	}

	dst := filepath.Join(chrootDir, "var", "cache", "binpkgs")
	if _, err := os.Stat(dst); os.IsNotExist(err) {
		t.Errorf("expected dst dir %s to exist", dst)
	}
	if len(sd.trackedMounts) != 1 {
		t.Fatalf("expected 1 tracked mount, got %d", len(sd.trackedMounts))
	}
}

func TestMountBinpkgsDir_ConfigError(t *testing.T) {
	cfg := workerTestConfig()
	cfg.Items["Seeder.BinpkgsDir"] = []string{""}

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir:     "/tmp/fake",
		SkipIfMounted: false,
	}
	err := sd.mountBinpkgsDir(&opts)
	if err == nil {
		t.Fatal("expected error for empty BinpkgsDir")
	}
}

func TestMountBinpkgsDir_SkipIfMounted(t *testing.T) {
	mockMountSyscalls(t)
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	binSrc := filepath.Join(tmp, "binpkgs")
	os.MkdirAll(binSrc, 0755)

	cfg := workerTestConfig()
	cfg.Items["Seeder.BinpkgsDir"] = []string{binSrc}

	mr := runner.NewMockRunner()
	var stdout bytes.Buffer
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &stdout,
		stderr:       &bytes.Buffer{},
	}

	// Pre-create and pretend the destination is already mounted.
	dst := filepath.Join(chrootDir, "var", "cache", "binpkgs")
	os.MkdirAll(dst, 0755)
	mockMountInfo(t, []*filesystems.MountInfoEntry{
		{Mountpoint: dst},
	})

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}
	if err := sd.mountBinpkgsDir(&opts); err != nil {
		t.Fatalf("mountBinpkgsDir: %v", err)
	}

	if len(sd.trackedMounts) != 0 {
		t.Errorf("expected 0 tracked mounts, got %d: %v",
			len(sd.trackedMounts), sd.trackedMounts)
	}
	if !strings.Contains(stdout.String(), "Skipping (already mounted)") {
		t.Errorf("expected skip message, got:\n%s", stdout.String())
	}
}

func TestMountBinpkgsDir_NoSkipWhenNotMounted(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	binSrc := filepath.Join(tmp, "binpkgs")
	os.MkdirAll(binSrc, 0755)

	cfg := workerTestConfig()
	cfg.Items["Seeder.BinpkgsDir"] = []string{binSrc}

	mr := runner.NewMockRunner()
	var stdout bytes.Buffer
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &stdout,
		stderr:       &bytes.Buffer{},
	}

	mockMountInfo(t, []*filesystems.MountInfoEntry{})

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}
	if err := sd.mountBinpkgsDir(&opts); err != nil {
		t.Fatalf("mountBinpkgsDir: %v", err)
	}

	if len(sd.trackedMounts) != 1 {
		t.Errorf("expected 1 tracked mount, got %d: %v",
			len(sd.trackedMounts), sd.trackedMounts)
	}
	if strings.Contains(stdout.String(), "Skipping (already mounted)") {
		t.Errorf("did not expect skip message, got:\n%s", stdout.String())
	}
}

func TestSetupChrootMounts_Success(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir: chrootDir,
	}
	if err := sd.SetupChrootMounts(opts); err != nil {
		t.Fatalf("SetupChrootMounts: %v", err)
	}

	// Common rootfs mounts create: dev, dev/pts, sys, dev/shm, proc, run/lock = 6
	// Plus: private repo, distfiles, binpkgs = 3
	// Total tracked mounts >= 8
	if len(sd.trackedMounts) < 8 {
		t.Errorf("expected at least 8 tracked mounts, got %d: %v",
			len(sd.trackedMounts), sd.trackedMounts)
	}

	// Verify key directories were created inside the chroot.
	for _, sub := range []string{"dev", "sys", "dev/shm", "run/lock"} {
		p := filepath.Join(chrootDir, sub)
		if _, err := os.Stat(p); os.IsNotExist(err) {
			t.Errorf("expected %s to exist", p)
		}
	}
}

func TestSetupChrootMounts_PrivateRepoError(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	// Break the private repo path so mountPrivateGitRepo fails.
	cfg.Items["matrixOS.PrivateGitRepoPath"] = []string{""}

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir: chrootDir,
	}
	err := sd.SetupChrootMounts(opts)
	if err == nil {
		t.Fatal("expected error from broken private repo config")
	}
	if !strings.Contains(err.Error(), "private git repo") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetupChrootMounts_DistDirError(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	cfg.Items["Seeder.DistfilesDir"] = []string{""} // break distfiles

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir: chrootDir,
	}
	err := sd.SetupChrootMounts(opts)
	if err == nil {
		t.Fatal("expected error from broken distfiles config")
	}
	if !strings.Contains(err.Error(), "distfiles") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetupChrootMounts_BinpkgsError(t *testing.T) {
	mockMountSyscalls(t)
	tmp := t.TempDir()
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	cfg.Items["Seeder.BinpkgsDir"] = []string{""} // break binpkgs

	mr := runner.NewMockRunner()
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	opts := SetupChrootMountsOptions{
		ChrootDir: chrootDir,
	}
	err := sd.SetupChrootMounts(opts)
	if err == nil {
		t.Fatal("expected error from broken binpkgs config")
	}
	if !strings.Contains(err.Error(), "binpkgs") {
		t.Errorf("unexpected error: %v", err)
	}
}

// mockMountInfo replaces filesystems.ReadMountInfo with a function
// returning the given entries for the duration of the test.
func mockMountInfo(t *testing.T, entries []*filesystems.MountInfoEntry) {
	t.Helper()
	orig := filesystems.ReadMountInfo
	filesystems.ReadMountInfo = func() ([]*filesystems.MountInfoEntry, error) {
		return entries, nil
	}
	t.Cleanup(func() { filesystems.ReadMountInfo = orig })
}

func TestSetupChrootMounts_EmptyChrootDir(t *testing.T) {
	sd := newTestSeederWithConfig(workerTestConfig())
	opts := SetupChrootMountsOptions{
		ChrootDir: "",
	}
	err := sd.SetupChrootMounts(opts)
	if err == nil {
		t.Fatal("expected error for empty ChrootDir")
	}
	if !strings.Contains(err.Error(), "missing ChrootDir") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetupChrootMounts_SkipIfMounted_SkipsPreMounted(t *testing.T) {
	mockMountSyscalls(t)
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	mr := runner.NewMockRunner()
	var stdout bytes.Buffer
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &stdout,
		stderr:       &bytes.Buffer{},
	}

	// Pre-create the /dev directory and pretend it is already mounted.
	preMountedDev := filepath.Join(chrootDir, "dev")
	os.MkdirAll(preMountedDev, 0755)

	mockMountInfo(t, []*filesystems.MountInfoEntry{
		{Mountpoint: preMountedDev},
	})

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}
	if err := sd.SetupChrootMounts(opts); err != nil {
		t.Fatalf("SetupChrootMounts: %v", err)
	}

	// /dev should have been skipped: it must NOT appear in trackedMounts.
	for _, mnt := range sd.trackedMounts {
		if mnt == preMountedDev {
			t.Errorf("%s should not be tracked (was already mounted)", preMountedDev)
		}
	}

	// The stdout should mention it was skipped.
	if !strings.Contains(stdout.String(), "Skipping (already mounted)") {
		t.Errorf("expected 'Skipping (already mounted)' in output, got:\n%s",
			stdout.String())
	}

	// Other slave mounts (/dev/pts, /sys) should still be tracked.
	devPts := filepath.Join(chrootDir, "dev", "pts")
	sysDir := filepath.Join(chrootDir, "sys")
	for _, want := range []string{devPts, sysDir} {
		found := false
		for _, mnt := range sd.trackedMounts {
			if mnt == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s to be tracked, tracked: %v", want, sd.trackedMounts)
		}
	}
}

func TestSetupChrootMounts_SkipIfMounted_AllPreMounted(t *testing.T) {
	mockMountSyscalls(t)
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	mr := runner.NewMockRunner()
	var stdout bytes.Buffer
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &stdout,
		stderr:       &bytes.Buffer{},
	}

	// Pre-create all slave mount dirs and pretend they are all mounted.
	slaveDirs := []string{"dev", "dev/pts", "sys"}
	var entries []*filesystems.MountInfoEntry
	for _, d := range slaveDirs {
		p := filepath.Join(chrootDir, d)
		os.MkdirAll(p, 0755)
		entries = append(entries, &filesystems.MountInfoEntry{Mountpoint: p})
	}
	mockMountInfo(t, entries)

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: true,
	}
	if err := sd.SetupChrootMounts(opts); err != nil {
		t.Fatalf("SetupChrootMounts: %v", err)
	}

	// None of the slave mounts should be tracked because they were all skipped.
	for _, d := range slaveDirs {
		p := filepath.Join(chrootDir, d)
		for _, mnt := range sd.trackedMounts {
			if mnt == p {
				t.Errorf("%s should not be tracked (was already mounted)", p)
			}
		}
	}

	// dev/shm and run/lock are still always mounted.
	devShm := filepath.Join(chrootDir, "dev", "shm")
	runLock := filepath.Join(chrootDir, "run", "lock")
	for _, want := range []string{devShm, runLock} {
		found := false
		for _, mnt := range sd.trackedMounts {
			if mnt == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s to be tracked, tracked: %v", want, sd.trackedMounts)
		}
	}

	output := stdout.String()
	skipCount := strings.Count(output, "Skipping (already mounted)")
	if skipCount != 3 {
		t.Errorf("expected 3 'Skipping (already mounted)' messages, got %d in:\n%s",
			skipCount, output)
	}
}

func TestSetupChrootMounts_SkipIfMountedFalse_MountsAll(t *testing.T) {
	mockMountSyscalls(t)
	tmp, _ := filepath.EvalSymlinks(t.TempDir())
	chrootDir := filepath.Join(tmp, "chroot")
	os.MkdirAll(chrootDir, 0755)

	cfg := mountTestConfig(tmp)
	mr := runner.NewMockRunner()
	var stdout bytes.Buffer
	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &stdout,
		stderr:       &bytes.Buffer{},
	}

	// Even with entries in mountinfo, SkipIfMounted=false should mount everything.
	preMountedDev := filepath.Join(chrootDir, "dev")
	os.MkdirAll(preMountedDev, 0755)
	mockMountInfo(t, []*filesystems.MountInfoEntry{
		{Mountpoint: preMountedDev},
	})

	opts := SetupChrootMountsOptions{
		ChrootDir:     chrootDir,
		SkipIfMounted: false,
	}
	if err := sd.SetupChrootMounts(opts); err != nil {
		t.Fatalf("SetupChrootMounts: %v", err)
	}

	// /dev should appear in tracked mounts since we didn't skip.
	found := false
	for _, mnt := range sd.trackedMounts {
		if mnt == preMountedDev {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %s to be tracked (SkipIfMounted=false), tracked: %v",
			preMountedDev, sd.trackedMounts)
	}

	// No "Skipping" messages should appear.
	if strings.Contains(stdout.String(), "Skipping (already mounted)") {
		t.Errorf("did not expect 'Skipping (already mounted)' with SkipIfMounted=false, got:\n%s",
			stdout.String())
	}
}
