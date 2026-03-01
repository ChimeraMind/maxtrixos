package seeder

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
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

func TestCleanTemporaryArtifact(t *testing.T) {
	tmp := t.TempDir()
	artDir := filepath.Join(tmp, "artifact")
	os.MkdirAll(artDir, 0755)
	os.WriteFile(filepath.Join(artDir, "file"), []byte("x"), 0644)

	cfg := workerTestConfig()
	sd := newTestSeederWithConfig(cfg)

	if err := sd.CleanTemporaryArtifact(artDir); err != nil {
		t.Fatalf("CleanTemporaryArtifact: %v", err)
	}

	if _, err := os.Stat(artDir); !os.IsNotExist(err) {
		t.Error("Expected artifact dir to be removed")
	}
}

func TestCleanTemporaryArtifactEmptyDir(t *testing.T) {
	cfg := workerTestConfig()
	sd := newTestSeederWithConfig(cfg)

	err := sd.CleanTemporaryArtifact("")
	if err == nil {
		t.Error("Expected error for empty dir, got nil")
	}
}

func TestCleanTemporaryArtifactNonexistent(t *testing.T) {
	cfg := workerTestConfig()
	sd := newTestSeederWithConfig(cfg)

	err := sd.CleanTemporaryArtifact("/nonexistent/dir/xxx")
	if err == nil {
		t.Error("Expected error for nonexistent dir, got nil")
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
// seeder variables. Returns the full path to the script.
func writeParamsScript(t *testing.T, dir, chrootName, chrootsDir, preferredDir string) string {
	t.Helper()
	script := fmt.Sprintf(
		"#!/bin/bash\n"+
			"SEEDER_CHROOT_NAME=%q\n"+
			"SEEDER_CHROOTS_DIR=%q\n"+
			"PREFERRED_SEEDER_CHROOT_DIR=%q\n",
		chrootName, chrootsDir, preferredDir,
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
	paramsFile := writeParamsScript(t, tmp,
		"bedrock-20260228",
		"/mnt/chroots",
		"/mnt/chroots/bedrock-20260228",
	)

	sd := newRealSeeder(tmp)
	params, err := sd.ParseSeederParams(paramsFile)
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
}

func TestParseSeederParams_UsesDevDir(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()

	// params.sh echoes MATRIXOS_DEV_DIR back as the chroot name
	// so we can verify it was set correctly.
	script := "#!/bin/bash\n" +
		"SEEDER_CHROOT_NAME=\"${MATRIXOS_DEV_DIR}\"\n" +
		"SEEDER_CHROOTS_DIR=/chroots\n" +
		"PREFERRED_SEEDER_CHROOT_DIR=/chroots/test\n"
	paramsFile := filepath.Join(tmp, "params.sh")
	os.WriteFile(paramsFile, []byte(script), 0755)

	sd := newRealSeeder("/my/custom/devdir")
	params, err := sd.ParseSeederParams(paramsFile)
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
	paramsFile := writeParamsScript(t, tmp,
		"", "/mnt/chroots", "/mnt/chroots/bedrock",
	)

	sd := newRealSeeder(tmp)
	_, err := sd.ParseSeederParams(paramsFile)
	if err == nil {
		t.Fatal("Expected error for empty ChrootName, got nil")
	}
	// Empty first echo line gets collapsed by TrimSpace before
	// Split, so the parser sees fewer than 3 lines.
	if !strings.Contains(err.Error(), "expected 3 lines") {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestParseSeederParams_EmptyChrootsDir(t *testing.T) {
	requireBash(t)
	tmp := t.TempDir()
	paramsFile := writeParamsScript(t, tmp,
		"bedrock-20260228", "", "/mnt/chroots/bedrock",
	)

	sd := newRealSeeder(tmp)
	_, err := sd.ParseSeederParams(paramsFile)
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
	paramsFile := writeParamsScript(t, tmp,
		"bedrock-20260228", "/mnt/chroots", "",
	)

	sd := newRealSeeder(tmp)
	_, err := sd.ParseSeederParams(paramsFile)
	if err == nil {
		t.Fatal("Expected error for empty PreferredChrootDir, got nil")
	}
	// Empty last echo line gets collapsed by TrimSpace before
	// Split, so the parser sees fewer than 3 lines.
	if !strings.Contains(err.Error(), "expected 3 lines") {
		t.Errorf("Unexpected error: %v", err)
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
	_, err := sd.ParseSeederParams(paramsFile)
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
	_, err := sd.ParseSeederParams(paramsFile)
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

	_, err := sd.ParseSeederParams("/fake/params.sh")
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
			"matrixOS.Root":            {devDir},
			"Seeder.DownloadsDir":      {downloadsDir},
			"Seeder.Stage3DownloadUrl": {stage3URL},
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
		"MATRIXOS_DEV_DIR":            "/my/dev",
		"SEEDER_CHROOT_NAME":          "bedrock-20260228",
		"SEEDER_CHROOTS_DIR":          "/mnt/chroots",
		"PREFERRED_SEEDER_CHROOT_DIR": "/mnt/chroots/bedrock-20260228",
		"CHROOT_DIR":                  "/mnt/chroots/bedrock-20260228",
		"DOWNLOAD_DIR":                "/my/downloads",
		"CHROOT_RESUME":               "",
		"STAGE3_FILE":                 "stage3-amd64-latest.tar.xz",
		"STAGE3_URL":                  "https://example.com/stage3.tar.xz",
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

	sd := &Seeder{
		cfg:          cfg,
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}

	info := SeederInfo{
		Name:       "00-bedrock",
		ChrootExec: "/build/seeders/00-bedrock/chroot.sh",
	}
	if err := sd.Seed("/mnt/chroot", info); err != nil {
		t.Fatalf("Seed: unexpected error: %v", err)
	}

	if len(mr.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(mr.Calls))
	}
	call := mr.Calls[0]
	if call.Name != "chroot:/build/seeders/00-bedrock/chroot.sh" {
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

// --- CleanTemporaryArtifact additional tests ---

func TestCleanTemporaryArtifact_SuccessEmptyDir(t *testing.T) {
	// Test cleaning an empty directory (no submounts).
	tmp := t.TempDir()
	artDir := filepath.Join(tmp, "empty-art")
	os.MkdirAll(artDir, 0755)

	sd := newTestSeederWithConfig(workerTestConfig())

	if err := sd.CleanTemporaryArtifact(artDir); err != nil {
		t.Fatalf("CleanTemporaryArtifact: %v", err)
	}

	if _, err := os.Stat(artDir); !os.IsNotExist(err) {
		t.Error("directory should have been removed")
	}
}

func TestCleanTemporaryArtifact_NestedFiles(t *testing.T) {
	tmp := t.TempDir()
	artDir := filepath.Join(tmp, "nested")
	os.MkdirAll(filepath.Join(artDir, "sub", "deep"), 0755)
	os.WriteFile(filepath.Join(artDir, "sub", "deep", "f.txt"), []byte("data"), 0644)

	sd := newTestSeederWithConfig(workerTestConfig())

	if err := sd.CleanTemporaryArtifact(artDir); err != nil {
		t.Fatalf("CleanTemporaryArtifact: %v", err)
	}

	if _, err := os.Stat(artDir); !os.IsNotExist(err) {
		t.Error("nested directory should have been removed")
	}
}
