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
		cfg:    cfg,
		runner: mr.Run,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
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
		cfg:    cfg,
		runner: runner.Run,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
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
		cfg:    cfg,
		runner: runner.Run,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
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
		cfg:    cfg,
		runner: runner.Run,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
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
		cfg:    cfg,
		runner: runner.Run,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
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
		cfg:    cfg,
		runner: runner.Run,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
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
		cfg:    cfg,
		runner: runner.Run,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
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
