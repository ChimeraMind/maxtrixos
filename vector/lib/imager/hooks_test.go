package imager

import (
	"errors"
	"os"
	"path/filepath"
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
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.SetRootfs("/tmp/rootfs")
		im.ref = "matrixos/amd64/gnome"
		// Should return nil when hooks dir doesn't exist.
		err := im.SetupHooks()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("NoHookScript", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := baseImageConfig()
		cfg.Items["matrixOS.Root"] = []string{tmpDir}
		os.MkdirAll(filepath.Join(tmpDir, "image", "hooks"), 0755)
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.SetRootfs("/tmp/rootfs")
		im.ref = "matrixos/amd64/gnome"

		err := im.SetupHooks()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
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
		im := newTestImageWithRunner(cfg, &ostree.MockOstree{}, runner)
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
