package imager

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/runner"
)

// --- GetKernelPath Tests ---

func TestGetKernelPath(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulesDir := filepath.Join(tmpDir, "usr", "lib", "modules")
		os.MkdirAll(filepath.Join(modulesDir, "6.1.0-matrixos"), 0755)
		os.MkdirAll(filepath.Join(modulesDir, "6.2.0-matrixos"), 0755)

		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.SetRootfs(tmpDir)
		result, err := im.GetKernelPath()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		// Should return the first sorted (6.1.0).
		if result != "6.1.0-matrixos" {
			t.Errorf("got %q, want 6.1.0-matrixos", result)
		}
	})

	t.Run("NoModulesDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.SetRootfs(tmpDir)
		_, err := im.GetKernelPath()
		if err == nil {
			t.Error("should error when modules dir doesn't exist")
		}
	})

	t.Run("EmptyModulesDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, "usr", "lib", "modules"), 0755)
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.SetRootfs(tmpDir)
		_, err := im.GetKernelPath()
		if err == nil {
			t.Error("should error for empty modules dir")
		}
	})

	t.Run("EmptyParam", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		_, err := im.GetKernelPath()
		if err == nil {
			t.Error("should error for empty param")
		}
	})
}

// --- SetupPasswords Tests ---

func TestSetupPasswords(t *testing.T) {
	t.Run("EmptyParam", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		err := im.SetupPasswords()
		if err == nil {
			t.Error("should error for empty param")
		}
	})
}

// --- ExtractPackageList Tests ---

func TestExtractPackageList(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		vdb := filepath.Join(tmpDir, "usr", "var-db-pkg")
		os.MkdirAll(filepath.Join(vdb, "sys-libs", "glibc-2.38"), 0755)
		os.MkdirAll(filepath.Join(vdb, "dev-libs", "openssl-3.0"), 0755)
		os.MkdirAll(filepath.Join(vdb, "app-misc", "screen-4.9"), 0755)

		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.SetRootfs(tmpDir)
		result, err := im.ExtractPackageList()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(result) != 3 {
			t.Fatalf("expected 3 packages, got %d: %v", len(result), result)
		}
	})

	t.Run("VdbNotExists", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.SetRootfs(tmpDir)
		result, err := im.ExtractPackageList()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil for non-existent VDB, got %v", result)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		_, err := im.ExtractPackageList()
		if err == nil {
			t.Error("should error for empty rootfs")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.SetRootfs("/tmp/rootfs")
		_, err := im.ExtractPackageList()
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

// --- SetupHooks Tests ---

func TestSetupHooks(t *testing.T) {
	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.ref = "ref"
		if err := im.SetupHooks(); err == nil {
			t.Error("should error for empty rootfs")
		}
		im.SetRootfs("/tmp/rootfs")
		im.ref = ""
		if err := im.SetupHooks(); err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("NoHooksDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := baseImageConfig()
		cfg.Items["matrixOS.Root"] = []string{tmpDir}
		im := newTestImage(cfg, &ostree.MockOstree{Ref_: "matrixos/amd64/gnome"})
		im.SetRootfs("/tmp/rootfs")
		im.ref = "matrixos/amd64/gnome"
		// Should return error when hooks dir doesn't exist.
		err := im.SetupHooks()
		if err == nil {
			t.Fatal("expected error when hooks dir does not exist")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("NoHookScript", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := baseImageConfig()
		cfg.Items["matrixOS.Root"] = []string{tmpDir}
		os.MkdirAll(filepath.Join(tmpDir, "image", "hooks"), 0755)
		im := newTestImage(cfg, &ostree.MockOstree{Ref_: "matrixos/amd64/gnome"})
		im.SetRootfs("/tmp/rootfs")
		im.ref = "matrixos/amd64/gnome"

		err := im.SetupHooks()
		if err == nil {
			t.Fatal("expected error when hook script does not exist")
		}
		if !strings.Contains(err.Error(), "does not exist") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("OstreeError", func(t *testing.T) {
		mo := &ostree.MockOstree{RemoveFullErr: errors.New("ostree error")}
		im := newTestImage(baseImageConfig(), mo)
		im.SetRootfs("/tmp/rootfs")
		im.ref = "ref"
		err := im.SetupHooks()
		if err == nil {
			t.Error("should propagate ostree error")
		}
	})
}

// --- TestImage Tests ---

func TestTestImageMethod(t *testing.T) {
	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.ref = "ref"
		im.mode = ModeCreateImageFile
		if err := im.TestImage(); err == nil {
			t.Error("should error for empty imagePath")
		}
		im.imagePath = "/tmp/x.img"
		im.ref = ""
		if err := im.TestImage(); err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("NoTestDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := baseImageConfig()
		cfg.Items["matrixOS.Root"] = []string{tmpDir}
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(cfg, &ostree.MockOstree{Ref_: "matrixos/amd64/gnome"}, runner)
		im.ref = "matrixos/amd64/gnome"
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile

		err := im.TestImage()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("OstreeError", func(t *testing.T) {
		mo := &ostree.MockOstree{RemoveFullErr: errors.New("ostree error")}
		im := newTestImage(baseImageConfig(), mo)
		im.ref = "ref"
		im.imagePath = "/tmp/x.img"
		im.mode = ModeCreateImageFile
		err := im.TestImage()
		if err == nil {
			t.Error("should propagate ostree error")
		}
	})
}

// --- SetupPasswords Additional Tests ---

func TestSetupPasswordsSuccess(t *testing.T) {
	// Check if openssl is available.
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not available, skipping SetupPasswords success test")
	}

	tmpDir := t.TempDir()
	etcDir := filepath.Join(tmpDir, "etc")
	os.MkdirAll(etcDir, 0755)

	// Create a shadow file with existing entries.
	shadowContent := "nobody:!:19000:0:99999:7:::\ndaemon:!:19000:0:99999:7:::\n"
	shadowFile := filepath.Join(etcDir, "shadow")
	os.WriteFile(shadowFile, []byte(shadowContent), 0640)

	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
	im.SetRootfs(tmpDir)

	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	err := im.SetupPasswords()
	if err != nil {
		t.Fatalf("SetupPasswords() error: %v", err)
	}

	// Read the updated shadow file.
	data, err := os.ReadFile(shadowFile)
	if err != nil {
		t.Fatalf("failed to read shadow file: %v", err)
	}
	content := string(data)

	// Should contain matrix and root entries.
	if !strings.Contains(content, "matrix:") {
		t.Error("shadow file should contain matrix user")
	}
	if !strings.Contains(content, "root:") {
		t.Error("shadow file should contain root user")
	}
	// Should still contain the non-root/non-matrix entries.
	if !strings.Contains(content, "nobody:") {
		t.Error("shadow file should still contain nobody user")
	}
	// Verify password hashes are present (SHA-512 starts with $6$).
	if !strings.Contains(content, "$6$") {
		t.Error("shadow file should contain SHA-512 password hashes")
	}
}

func TestSetupPasswordsShadowFileNotFound(t *testing.T) {
	// Check if openssl is available.
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not available")
	}

	tmpDir := t.TempDir()
	// Don't create the shadow file.
	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
	im.SetRootfs(tmpDir)

	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	err := im.SetupPasswords()
	if err == nil {
		t.Error("should error when shadow file doesn't exist")
	}
}

func TestSetupPasswordsReplacesExisting(t *testing.T) {
	// Check if openssl is available.
	if _, err := exec.LookPath("openssl"); err != nil {
		t.Skip("openssl not available")
	}

	tmpDir := t.TempDir()
	etcDir := filepath.Join(tmpDir, "etc")
	os.MkdirAll(etcDir, 0755)

	// Create a shadow file with existing matrix and root entries.
	shadowContent := "matrix:oldpass:19000:0:99999:7:::\nroot:oldpass:19000:0:99999:7:::\nnobody:!:19000:0:99999:7:::\n"
	shadowFile := filepath.Join(etcDir, "shadow")
	os.WriteFile(shadowFile, []byte(shadowContent), 0640)

	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
	im.SetRootfs(tmpDir)

	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	err := im.SetupPasswords()
	if err != nil {
		t.Fatalf("SetupPasswords() error: %v", err)
	}

	data, err := os.ReadFile(shadowFile)
	if err != nil {
		t.Fatalf("failed to read shadow file: %v", err)
	}
	content := string(data)

	// Old pass strings should be gone.
	if strings.Contains(content, "oldpass") {
		t.Error("shadow file should not contain old password hashes")
	}
	// New hashes should be present.
	if !strings.Contains(content, "$6$") {
		t.Error("shadow file should contain new SHA-512 password hashes")
	}
}

// --- SetupHooks Additional Tests ---

func TestSetupHooksNonExecutable(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := baseImageConfig()
	cfg.Items["matrixOS.Root"] = []string{tmpDir}

	ref := "matrixos/amd64/gnome"
	hookDir := filepath.Join(tmpDir, "image", "hooks")
	os.MkdirAll(hookDir, 0755)

	// Create a non-executable hook script.
	hookScript := filepath.Join(hookDir, ref+".sh")
	os.MkdirAll(filepath.Dir(hookScript), 0755)
	os.WriteFile(hookScript, []byte("#!/bin/sh\nexit 0\n"), 0644) // not executable!

	im := newTestImage(cfg, &ostree.MockOstree{Ref_: ref})
	im.SetRootfs("/tmp/rootfs")
	im.ref = ref

	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	err := im.SetupHooks()
	if err == nil {
		t.Fatal("expected error for non-executable hook")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSetupHooksSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := baseImageConfig()
	cfg.Items["matrixOS.Root"] = []string{tmpDir}

	ref := "matrixos/amd64/gnome"
	hookDir := filepath.Join(tmpDir, "image", "hooks")
	os.MkdirAll(hookDir, 0755)

	// Create an executable hook script that succeeds.
	hookScript := filepath.Join(hookDir, ref+".sh")
	os.MkdirAll(filepath.Dir(hookScript), 0755)
	os.WriteFile(hookScript, []byte("#!/bin/sh\nexit 0\n"), 0755)

	rootfs := filepath.Join(tmpDir, "rootfs")
	os.MkdirAll(rootfs, 0755)

	im := newTestImage(cfg, &ostree.MockOstree{Ref_: ref})
	im.SetRootfs(rootfs)
	im.ref = ref

	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	err := im.SetupHooks()
	if err != nil {
		t.Fatalf("SetupHooks() error: %v", err)
	}
}

func TestSetupHooksConfigError(t *testing.T) {
	ec := &config.ErrConfig{Err: errors.New("cfg error")}
	im, _ := NewImage(ec, &ostree.MockOstree{Ref_: "ref"}, filesystems.DefaultMockFsenc(), nil)
	im.SetRootfs("/tmp/rootfs")
	im.ref = "ref"

	err := im.SetupHooks()
	if err == nil {
		t.Error("should error from broken config")
	}
}

// --- TestImage Additional Tests ---

func TestTestImageWithScripts(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := baseImageConfig()
	cfg.Items["matrixOS.Root"] = []string{tmpDir}
	cfg.Items["matrixOS.LogsDir"] = []string{tmpDir}
	cfg.Items["Imager.MountDir"] = []string{tmpDir}

	ref := "matrixos/amd64/gnome"
	testDir := filepath.Join(tmpDir, "image", "tests", ref)
	os.MkdirAll(testDir, 0755)

	// Create a test script that succeeds.
	scriptContent := "#!/bin/sh\nexit 0\n"
	scriptPath := filepath.Join(testDir, "test1.sh")
	os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	// Create a non-executable file to test the skip path.
	nonExecPath := filepath.Join(testDir, "data.txt")
	os.WriteFile(nonExecPath, []byte("not a script"), 0644)

	// Create the image file.
	imagePath := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imagePath, []byte("fake image"), 0644)

	mo := &ostree.MockOstree{Ref_: ref}
	im := newTestImage(cfg, mo)
	im.ref = ref
	im.imagePath = imagePath
	im.mode = ModeCreateImageFile

	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	err := im.TestImage()
	if err != nil {
		t.Fatalf("TestImage() error: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Running test script") {
		t.Errorf("expected test script execution message, got:\n%s", output)
	}
}

func TestTestImageScriptFails(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := baseImageConfig()
	cfg.Items["matrixOS.Root"] = []string{tmpDir}
	cfg.Items["matrixOS.LogsDir"] = []string{tmpDir}
	cfg.Items["Imager.MountDir"] = []string{tmpDir}

	ref := "matrixos/amd64/gnome"
	testDir := filepath.Join(tmpDir, "image", "tests", ref)
	os.MkdirAll(testDir, 0755)

	// Create a test script that fails.
	scriptContent := "#!/bin/sh\nexit 1\n"
	scriptPath := filepath.Join(testDir, "test_fail.sh")
	os.WriteFile(scriptPath, []byte(scriptContent), 0755)

	imagePath := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imagePath, []byte("fake image"), 0644)

	mo := &ostree.MockOstree{Ref_: ref}
	im := newTestImage(cfg, mo)
	im.ref = ref
	im.imagePath = imagePath
	im.mode = ModeCreateImageFile

	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	err := im.TestImage()
	if err == nil {
		t.Error("expected error from failing test script")
	}
	if !strings.Contains(err.Error(), "test script") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTestImageModeFlashToDevice(t *testing.T) {
	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
	im.mode = ModeFlashToDevice
	im.imagePath = "/tmp/test.img"
	im.ref = "ref"

	err := im.TestImage()
	if err == nil {
		t.Error("should error for ModeFlashToDevice")
	}
}

// --- ExtractPackageList Additional Tests ---

func TestExtractPackageListEmptyVdb(t *testing.T) {
	tmpDir := t.TempDir()
	vdb := filepath.Join(tmpDir, "usr", "var-db-pkg")
	os.MkdirAll(vdb, 0755)
	// VDB exists but is empty.

	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
	im.SetRootfs(tmpDir)

	result, err := im.ExtractPackageList()
	if err != nil {
		t.Fatalf("ExtractPackageList() error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected empty package list, got %v", result)
	}
}
