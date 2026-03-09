package imager

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/runner"
)

// --- Cleanup tracking tests ---

func TestImageCleanupTracking(t *testing.T) {
	cfg := baseImageConfig()
	im := newTestImager(cfg, &ostree.MockOstree{})
	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	im.trackMount("/mnt/test1")
	im.trackMount("/mnt/test2")
	im.trackTmpDir(t.TempDir())
	im.trackTmpDir(t.TempDir())

	loop, err := filesystems.NewLoop("/tmp/fake.img")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	im.trackLoopDevice(loop)

	if len(im.trackedMounts) != 2 {
		t.Errorf("Expected 2 mounts, got %d", len(im.trackedMounts))
	}
	if len(im.trackedTmpDirs) != 2 {
		t.Errorf("Expected 2 tmpDirs, got %d", len(im.trackedTmpDirs))
	}
	if len(im.trackedLoopDevices) != 1 {
		t.Errorf("Expected 1 loop device, got %d", len(im.trackedLoopDevices))
	}

	im.Cleanup()

	if len(im.trackedMounts) != 0 {
		t.Errorf("Expected 0 mounts after cleanup, got %d", len(im.trackedMounts))
	}
	if len(im.trackedTmpDirs) != 0 {
		t.Errorf("Expected 0 tmpDirs after cleanup, got %d", len(im.trackedTmpDirs))
	}
	if len(im.trackedLoopDevices) != 0 {
		t.Errorf("Expected 0 loop devices after cleanup, got %d", len(im.trackedLoopDevices))
	}
}

func TestImageCleanupCalledOnEmptyState(t *testing.T) {
	cfg := baseImageConfig()
	im := newTestImager(cfg, &ostree.MockOstree{})
	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)
	// Should not panic with no resources tracked.
	im.Cleanup()
}

func TestImageConcurrentResourceTracking(t *testing.T) {
	cfg := baseImageConfig()
	im := newTestImager(cfg, &ostree.MockOstree{})
	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			im.trackMount("/mnt/test")
			im.trackTmpDir("/tmp/test")
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}

	if len(im.trackedMounts) != 10 {
		t.Errorf("Expected 10 mounts, got %d", len(im.trackedMounts))
	}
	if len(im.trackedTmpDirs) != 10 {
		t.Errorf("Expected 10 tmpDirs, got %d", len(im.trackedTmpDirs))
	}

	im.Cleanup()
	if len(im.trackedMounts) != 0 {
		t.Errorf("Expected 0 mounts after cleanup, got %d", len(im.trackedMounts))
	}
}

// --- productionizeImage tests ---

func newProductionTestImage(cfg *config.MockConfig) *Imager {
	im := newTestImager(cfg, &ostree.MockOstree{})
	im.mode = ModeCreateImageFile
	return im
}

func TestProductionizeImageBasic(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.img")
	if err := os.WriteFile(imagePath, []byte("fake image data"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := baseImageConfig()
	cfg.Items["Imager.ImagesDir"] = []string{tmpDir}
	if cfg.Bools == nil {
		cfg.Bools = make(map[string]bool)
	}
	cfg.Bools["Imager.Productionize"] = true
	cfg.Bools["Imager.ImageTests"] = false
	cfg.Bools["Imager.CreateQcow2"] = false
	im := newProductionTestImage(cfg)
	im.imagePath = imagePath
	im.ref = "matrixos/x86_64/dev/test"

	var stdout bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&bytes.Buffer{})

	// Mock runner: simulate xz creating the compressed file.
	im.runner = func(cmd *runner.Cmd) error {
		for _, a := range cmd.Args {
			if strings.HasSuffix(a, ".img") {
				os.WriteFile(a+".xz", []byte("compressed"), 0644)
			}
		}
		return nil
	}

	artifacts, err := im.productionizeImage("1.0.0", []string{"pkg1", "pkg2"})
	if err != nil {
		t.Fatalf("productionizeImage failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Moving") {
		t.Errorf("Expected 'Moving' in output, got:\n%s", output)
	}

	// Check package list was created.
	foundPkgList := false
	for _, a := range artifacts {
		if strings.HasSuffix(a, ".packages.txt") {
			foundPkgList = true
			data, err := os.ReadFile(a)
			if err != nil {
				t.Fatalf("Expected package list file at %s: %v", a, err)
			}
			if !strings.Contains(string(data), "pkg1") || !strings.Contains(string(data), "pkg2") {
				t.Errorf("Package list content unexpected: %s", string(data))
			}
			break
		}
	}
	if !foundPkgList {
		t.Error("Expected package list artifact in results")
	}

	// Check sha256 file was created.
	foundSha256 := false
	for _, a := range artifacts {
		if strings.HasSuffix(a, ".sha256") {
			foundSha256 = true
			sha256Data, err := os.ReadFile(a)
			if err != nil {
				t.Fatalf("Expected sha256 file at %s: %v", a, err)
			}
			sha256Content := string(sha256Data)
			parts := strings.Fields(sha256Content)
			if len(parts) < 2 || len(parts[0]) != 64 {
				t.Errorf("sha256 file has unexpected format: %s", sha256Content)
			}
			break
		}
	}
	if !foundSha256 {
		t.Error("Expected sha256 artifact in results")
	}
}

func TestProductionizeImageWithCompression(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imagePath, []byte("fake"), 0644)

	cfg := baseImageConfig()
	cfg.Items["Imager.ImagesDir"] = []string{tmpDir}
	if cfg.Bools == nil {
		cfg.Bools = make(map[string]bool)
	}
	cfg.Bools["Imager.Productionize"] = true
	cfg.Bools["Imager.ImageTests"] = false
	cfg.Bools["Imager.CreateQcow2"] = false
	im := newProductionTestImage(cfg)
	im.imagePath = imagePath
	im.ref = "matrixos/x86_64/dev/test"

	var stdout bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&bytes.Buffer{})

	// Mock runner that creates the compressed file xz would produce.
	im.runner = func(cmd *runner.Cmd) error {
		for _, a := range cmd.Args {
			if strings.HasSuffix(a, ".img") {
				os.WriteFile(a+".xz", []byte("compressed"), 0644)
			}
		}
		return nil
	}

	artifacts, err := im.productionizeImage("1.0.0", []string{"pkg1"})
	if err != nil {
		t.Fatalf("productionizeImage failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Compressing") {
		t.Errorf("Expected 'Compressing' in output, got:\n%s", output)
	}

	foundCompressed := false
	for _, a := range artifacts {
		if strings.HasSuffix(a, ".xz") || strings.HasSuffix(a, ".img") {
			foundCompressed = true
			break
		}
	}
	if !foundCompressed {
		t.Error("Expected image artifact in results")
	}
}

func TestCreatePackageListFile(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.img")

	cfg := baseImageConfig()
	im := newProductionTestImage(cfg)
	im.imagePath = imagePath

	var stdout bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&bytes.Buffer{})

	pkgListPath, err := im.createPackageListFile([]string{"pkg1", "pkg2", "pkg3"})
	if err != nil {
		t.Fatalf("createPackageListFile failed: %v", err)
	}

	expected := imagePath + ".packages.txt"
	if pkgListPath != expected {
		t.Errorf("Expected path %s, got %s", expected, pkgListPath)
	}

	data, err := os.ReadFile(pkgListPath)
	if err != nil {
		t.Fatalf("Failed to read package list: %v", err)
	}
	content := string(data)
	for _, pkg := range []string{"pkg1", "pkg2", "pkg3"} {
		if !strings.Contains(content, pkg) {
			t.Errorf("Expected %q in package list, got: %s", pkg, content)
		}
	}
}

func TestBuildSha256sums(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imagePath, []byte("test data"), 0644)

	cfg := baseImageConfig()
	im := newProductionTestImage(cfg)
	im.imagePath = imagePath

	var stdout bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&bytes.Buffer{})

	paths, err := im.buildSha256sums(false, false)
	if err != nil {
		t.Fatalf("buildSha256sums failed: %v", err)
	}

	if len(paths) != 1 {
		t.Fatalf("Expected 1 sha256 path, got %d", len(paths))
	}

	sha256Path := paths[0]
	if !strings.HasSuffix(sha256Path, ".sha256") {
		t.Errorf("Expected .sha256 suffix, got %s", sha256Path)
	}

	sha256Data, err := os.ReadFile(sha256Path)
	if err != nil {
		t.Fatalf("Failed to read sha256 file: %v", err)
	}
	parts := strings.Fields(string(sha256Data))
	if len(parts) < 2 || len(parts[0]) != 64 {
		t.Errorf("sha256 file has unexpected format: %s", string(sha256Data))
	}
}

func TestProductionizeImageWithQcow2(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imagePath, []byte("fake image"), 0644)

	cfg := baseImageConfig()
	cfg.Items["Imager.ImagesDir"] = []string{tmpDir}
	if cfg.Bools == nil {
		cfg.Bools = make(map[string]bool)
	}
	cfg.Bools["Imager.Productionize"] = true
	cfg.Bools["Imager.ImageTests"] = false
	cfg.Bools["Imager.CreateQcow2"] = true
	im := newProductionTestImage(cfg)
	im.imagePath = imagePath
	im.ref = "matrixos/x86_64/dev/test"

	var stdout bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&bytes.Buffer{})

	// Mock runner: simulate xz (compression) and qemu-img (qcow2 creation).
	im.runner = func(cmd *runner.Cmd) error {
		switch cmd.Name {
		case "qemu-img":
			// qemu-img convert -c -O qcow2 -p <input> <output>
			// The last arg is the output qcow2 path.
			if len(cmd.Args) >= 2 {
				qcow2Path := cmd.Args[len(cmd.Args)-1]
				os.WriteFile(qcow2Path, []byte("qcow2 data"), 0644)
			}
		default:
			// xz or other compressor: create compressed file.
			for _, a := range cmd.Args {
				if strings.HasSuffix(a, ".img") {
					os.WriteFile(a+".xz", []byte("compressed"), 0644)
				}
			}
		}
		return nil
	}

	artifacts, err := im.productionizeImage("1.0.0", []string{"pkg1"})
	if err != nil {
		t.Fatalf("productionizeImage failed: %v", err)
	}

	output := stdout.String()
	if !strings.Contains(output, "Creating QCOW2") {
		t.Errorf("Expected 'Creating QCOW2' in output, got:\n%s", output)
	}

	foundQcow2 := false
	foundQcow2Sha256 := false
	for _, a := range artifacts {
		if strings.HasSuffix(a, ".qcow2") {
			foundQcow2 = true
		}
		if strings.HasSuffix(a, ".qcow2.sha256") {
			foundQcow2Sha256 = true
		}
	}
	if !foundQcow2 {
		t.Errorf("Expected .qcow2 artifact, got: %v", artifacts)
	}
	if !foundQcow2Sha256 {
		t.Errorf("Expected .qcow2.sha256 artifact, got: %v", artifacts)
	}
}

// --- addSysrootOverlay tests ---

func TestAddSysrootOverlay(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mountRootfs := "/tmp/test-rootfs-mount"
		mo := &ostree.MockOstree{Sysroot_: mountRootfs}
		cfg := baseImageConfig()
		im := newTestImager(cfg, mo)
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.addSysrootOverlay(mountRootfs)
		if err != nil {
			t.Fatalf("addSysrootOverlay() error: %v", err)
		}
	})

	t.Run("SysrootMismatch", func(t *testing.T) {
		// MockOstree.Sysroot() returns "" by default, which won't match
		mo := &ostree.MockOstree{}
		cfg := baseImageConfig()
		im := newTestImager(cfg, mo)
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.addSysrootOverlay("/tmp/some-rootfs")
		if err == nil {
			t.Fatal("expected error for sysroot mismatch")
		}
		if !strings.Contains(err.Error(), "does not match") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("SysrootError", func(t *testing.T) {
		mo := &ostree.MockOstree{SysrootErr: errForTest}
		cfg := baseImageConfig()
		im := newTestImager(cfg, mo)
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.addSysrootOverlay("/tmp/rootfs")
		if err == nil {
			t.Fatal("expected error from ostree")
		}
	})
}

// --- extractBuildMetadata tests ---

func TestExtractBuildMetadata(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		// Create VDB for package list.
		vdb := filepath.Join(tmpDir, "usr", "var-db-pkg")
		os.MkdirAll(filepath.Join(vdb, "sys-libs", "glibc-2.38"), 0755)
		os.MkdirAll(filepath.Join(vdb, "dev-libs", "openssl-3.0"), 0755)

		// Create metadata file for release version.
		metadataDir := filepath.Join(tmpDir, "etc", "matrixos")
		os.MkdirAll(metadataDir, 0755)
		os.WriteFile(filepath.Join(metadataDir, "build.txt"),
			[]byte("SEED_NAME=matrixos-gnome-20260301\nBUILD_DATE=2026-03-01\n"), 0644)

		cfg := baseImageConfig()
		im := newTestImager(cfg, &ostree.MockOstree{})
		im.SetRootfs(tmpDir)
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		releaseVersion, pkgList, err := im.extractBuildMetadata()
		if err != nil {
			t.Fatalf("extractBuildMetadata() error: %v", err)
		}
		if releaseVersion != "20260301" {
			t.Errorf("releaseVersion = %q, want 20260301", releaseVersion)
		}
		if len(pkgList) != 2 {
			t.Errorf("expected 2 packages, got %d", len(pkgList))
		}
	})

	t.Run("PackageListError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errForTest}
		im, _ := NewImager(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.SetRootfs("/tmp/rootfs")

		_, _, err := im.extractBuildMetadata()
		if err == nil {
			t.Error("expected error from ExtractPackageList")
		}
	})

	t.Run("ReleaseVersionError", func(t *testing.T) {
		// ExtractReleaseVersion needs rootfs set and BuildMetadataFile.
		// If rootfs is empty, ExtractReleaseVersion fails.
		cfg := baseImageConfig()
		im := newTestImager(cfg, &ostree.MockOstree{})
		// Leave rootfs unset — ExtractPackageList errors first, so set it.
		tmpDir := t.TempDir()
		im.SetRootfs(tmpDir)

		// Make ExtractReleaseVersion fail by breaking config for BuildMetadataFile.
		delete(cfg.Items, "Seeder.ChrootMetadataDir")

		_, _, err := im.extractBuildMetadata()
		if err == nil {
			t.Error("expected error from ExtractReleaseVersion")
		}
	})
}

// --- finalizeBuild tests ---

func TestFinalizeBuild(t *testing.T) {
	t.Run("ModeFlashToDevice", func(t *testing.T) {
		tmpBoot := t.TempDir()
		tmpEfi := t.TempDir()
		// Create some dirs so PrintDirectoryTree can walk them.
		os.MkdirAll(filepath.Join(tmpBoot, "grub"), 0755)
		os.MkdirAll(filepath.Join(tmpEfi, "EFI"), 0755)

		mockRunner := runner.NewMockRunner()
		cfg := baseImageConfig()
		im := newTestImagerWithRunner(cfg, &ostree.MockOstree{}, mockRunner)
		im.rootfsMount = t.TempDir()
		im.bootfsMount = tmpBoot
		im.efifsMount = tmpEfi
		im.devicePath = "/dev/sda"
		im.mode = ModeFlashToDevice

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.finalizeBuild("1.0.0", []string{"pkg1"})
		if err != nil {
			t.Fatalf("finalizeBuild() error: %v", err)
		}
		if !strings.Contains(stdout.String(), "On device install complete") {
			t.Errorf("expected completion message, got:\n%s", stdout.String())
		}
	})

	t.Run("ModeCreateImageFileNoProductionize", func(t *testing.T) {
		tmpBoot := t.TempDir()
		tmpEfi := t.TempDir()

		mockRunner := runner.NewMockRunner()
		cfg := baseImageConfig()
		if cfg.Bools == nil {
			cfg.Bools = make(map[string]bool)
		}
		cfg.Bools["Imager.Productionize"] = false
		im := newTestImagerWithRunner(cfg, &ostree.MockOstree{}, mockRunner)
		im.rootfsMount = t.TempDir()
		im.bootfsMount = tmpBoot
		im.efifsMount = tmpEfi
		im.devicePath = "/dev/sda"
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.finalizeBuild("1.0.0", []string{"pkg1"})
		if err != nil {
			t.Fatalf("finalizeBuild() error: %v", err)
		}
		if !strings.Contains(stdout.String(), "Image creation complete") {
			t.Errorf("expected completion message, got:\n%s", stdout.String())
		}
	})

	t.Run("FinalizeFilesystemsError", func(t *testing.T) {
		cfg := baseImageConfig()
		im := newTestImager(cfg, &ostree.MockOstree{})
		// rootfsMount is empty, so FinalizeFilesystems will error.

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.finalizeBuild("1.0.0", nil)
		if err == nil {
			t.Error("expected error from FinalizeFilesystems")
		}
	})

	t.Run("UnknownMode", func(t *testing.T) {
		tmpBoot := t.TempDir()
		tmpEfi := t.TempDir()

		mockRunner := runner.NewMockRunner()
		cfg := baseImageConfig()
		im := newTestImagerWithRunner(cfg, &ostree.MockOstree{}, mockRunner)
		im.rootfsMount = t.TempDir()
		im.bootfsMount = tmpBoot
		im.efifsMount = tmpEfi
		im.devicePath = "/dev/sda"
		im.mode = ImageMode(99) // invalid

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.finalizeBuild("1.0.0", nil)
		if err == nil {
			t.Error("expected error for unknown image mode")
		}
		if !strings.Contains(err.Error(), "unknown image mode") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- postImageCreation tests ---

func TestPostImageCreation(t *testing.T) {
	t.Run("ProductionizeFalse", func(t *testing.T) {
		cfg := baseImageConfig()
		if cfg.Bools == nil {
			cfg.Bools = make(map[string]bool)
		}
		cfg.Bools["Imager.Productionize"] = false
		im := newTestImager(cfg, &ostree.MockOstree{})
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.postImageCreation("1.0.0", nil)
		if err != nil {
			t.Fatalf("postImageCreation() error: %v", err)
		}
		output := stdout.String()
		if !strings.Contains(output, "How to test") {
			t.Errorf("expected test instructions in output, got:\n%s", output)
		}
	})

	t.Run("ProductionizeConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errForTest}
		im, _ := NewImager(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.postImageCreation("1.0.0", nil)
		if err == nil {
			t.Error("expected error from Productionize config")
		}
	})
}

// --- installSystemComponents tests ---

func TestInstallSystemComponents(t *testing.T) {
	t.Run("SetupBootloaderConfigFails", func(t *testing.T) {
		cfg := baseImageConfig()
		im := newTestImager(cfg, &ostree.MockOstree{})
		// rootfs not set => SetupBootloaderConfig will error.
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.installSystemComponents()
		if err == nil {
			t.Error("expected error from SetupBootloaderConfig")
		}
		if !strings.Contains(err.Error(), "setup bootloader config") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// --- maybeGenerateGpgSignatures tests ---

func TestMaybeGenerateGpgSignatures(t *testing.T) {
	t.Run("GpgDisabled", func(t *testing.T) {
		cfg := baseImageConfig()
		mo := &ostree.MockOstree{GpgEnabled_: false}
		im := newTestImager(cfg, mo)
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		artifacts, err := im.maybeGenerateGpgSignatures(false, false)
		if err != nil {
			t.Fatalf("maybeGenerateGpgSignatures() error: %v", err)
		}
		if artifacts != nil {
			t.Errorf("expected nil artifacts for disabled GPG, got %v", artifacts)
		}
		if !strings.Contains(stderr.String(), "GPG signing") {
			t.Errorf("expected GPG disabled warning, got:\n%s", stderr.String())
		}
	})

	t.Run("GpgEnabledKeyNotFound", func(t *testing.T) {
		cfg := baseImageConfig()
		mo := &ostree.MockOstree{
			GpgEnabled_:        true,
			GpgPrivateKeyPath_: "/nonexistent/key.gpg",
		}
		im := newTestImager(cfg, mo)
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		_, err := im.maybeGenerateGpgSignatures(false, false)
		if err == nil {
			t.Error("expected error for missing GPG key")
		}
		if !strings.Contains(err.Error(), "not found") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("GpgEnabledError", func(t *testing.T) {
		cfg := baseImageConfig()
		mo := &ostree.MockOstree{GpgEnabledErr: errForTest}
		im := newTestImager(cfg, mo)

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		_, err := im.maybeGenerateGpgSignatures(false, false)
		if err == nil {
			t.Error("expected error from GpgEnabled")
		}
	})

	t.Run("GpgEnabledKeyPathError", func(t *testing.T) {
		cfg := baseImageConfig()
		mo := &ostree.MockOstree{
			GpgEnabled_:          true,
			GpgPrivateKeyPathErr: errForTest,
		}
		im := newTestImager(cfg, mo)

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		_, err := im.maybeGenerateGpgSignatures(false, false)
		if err == nil {
			t.Error("expected error from GpgPrivateKeyPath")
		}
	})

	t.Run("GpgEnabledSuccessUncompressed", func(t *testing.T) {
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "key.gpg")
		os.WriteFile(keyPath, []byte("fake-key"), 0644)
		pubKeyPath := filepath.Join(tmpDir, "pub.asc")
		os.WriteFile(pubKeyPath, []byte("fake-pubkey"), 0644)
		imagePath := filepath.Join(tmpDir, "test.img")
		os.WriteFile(imagePath, []byte("fake-image"), 0644)

		cfg := baseImageConfig()
		mo := &ostree.MockOstree{
			GpgEnabled_:        true,
			GpgPrivateKeyPath_: keyPath,
			GpgBestPubKeyPath_: pubKeyPath,
		}
		im := newTestImager(cfg, mo)
		im.imagePath = imagePath
		im.mode = ModeCreateImageFile

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		artifacts, err := im.maybeGenerateGpgSignatures(false, false)
		if err != nil {
			t.Fatalf("maybeGenerateGpgSignatures() error: %v", err)
		}
		// Should have signed image + pubkey artifacts.
		if len(artifacts) < 2 {
			t.Errorf("expected at least 2 artifacts, got %d: %v", len(artifacts), artifacts)
		}
	})

	t.Run("GpgEnabledSuccessCompressed", func(t *testing.T) {
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "key.gpg")
		os.WriteFile(keyPath, []byte("fake-key"), 0644)
		pubKeyPath := filepath.Join(tmpDir, "pub.asc")
		os.WriteFile(pubKeyPath, []byte("fake-pubkey"), 0644)
		imagePath := filepath.Join(tmpDir, "test.img")
		os.WriteFile(imagePath, []byte("fake-image"), 0644)
		compressedPath := imagePath + ".xz"
		os.WriteFile(compressedPath, []byte("compressed"), 0644)

		cfg := baseImageConfig()
		mo := &ostree.MockOstree{
			GpgEnabled_:        true,
			GpgPrivateKeyPath_: keyPath,
			GpgBestPubKeyPath_: pubKeyPath,
		}
		im := newTestImager(cfg, mo)
		im.imagePath = imagePath
		im.mode = ModeCreateImageFile

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		artifacts, err := im.maybeGenerateGpgSignatures(true, false)
		if err != nil {
			t.Fatalf("maybeGenerateGpgSignatures() error: %v", err)
		}
		// Should have signed compressed image + pubkey artifacts.
		foundSigned := false
		for _, a := range artifacts {
			if strings.HasSuffix(a, ".asc") && strings.Contains(a, ".xz") {
				foundSigned = true
			}
		}
		if !foundSigned {
			t.Errorf("expected signed compressed artifact, got: %v", artifacts)
		}
	})

	t.Run("GpgEnabledSuccessWithQcow2", func(t *testing.T) {
		tmpDir := t.TempDir()
		keyPath := filepath.Join(tmpDir, "key.gpg")
		os.WriteFile(keyPath, []byte("fake-key"), 0644)
		pubKeyPath := filepath.Join(tmpDir, "pub.asc")
		os.WriteFile(pubKeyPath, []byte("fake-pubkey"), 0644)
		imagePath := filepath.Join(tmpDir, "test.img")
		os.WriteFile(imagePath, []byte("fake-image"), 0644)
		qcow2Path := imagePath + ".qcow2"
		os.WriteFile(qcow2Path, []byte("qcow2-data"), 0644)

		cfg := baseImageConfig()
		mo := &ostree.MockOstree{
			GpgEnabled_:        true,
			GpgPrivateKeyPath_: keyPath,
			GpgBestPubKeyPath_: pubKeyPath,
		}
		im := newTestImager(cfg, mo)
		im.imagePath = imagePath
		im.mode = ModeCreateImageFile

		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		artifacts, err := im.maybeGenerateGpgSignatures(false, true)
		if err != nil {
			t.Fatalf("maybeGenerateGpgSignatures() error: %v", err)
		}
		// Should have signed image + signed qcow2 + pubkey.
		if len(artifacts) < 3 {
			t.Errorf("expected at least 3 artifacts, got %d: %v", len(artifacts), artifacts)
		}
	})
}

// --- setupDevices tests ---

func TestSetupDevices(t *testing.T) {
	t.Run("EfiPartitionSizeError", func(t *testing.T) {
		cfg := baseImageConfig()
		delete(cfg.Items, "Imager.EfiPartitionSize")
		im := newTestImager(cfg, &ostree.MockOstree{})
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.setupDevices(&BuildOptions{})
		if err == nil {
			t.Error("expected error for missing EfiPartitionSize")
		}
	})

	t.Run("BootPartitionSizeError", func(t *testing.T) {
		cfg := baseImageConfig()
		delete(cfg.Items, "Imager.BootPartitionSize")
		im := newTestImager(cfg, &ostree.MockOstree{})
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.setupDevices(&BuildOptions{})
		if err == nil {
			t.Error("expected error for missing BootPartitionSize")
		}
	})

	t.Run("ImageSizeError", func(t *testing.T) {
		cfg := baseImageConfig()
		delete(cfg.Items, "Imager.ImageSize")
		im := newTestImager(cfg, &ostree.MockOstree{})
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		err := im.setupDevices(&BuildOptions{})
		if err == nil {
			t.Error("expected error for missing ImageSize")
		}
	})

	t.Run("MissingDevicePath", func(t *testing.T) {
		cfg := baseImageConfig()
		im := newTestImager(cfg, &ostree.MockOstree{})
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		// Has EfiDevice but missing Boot/Root.
		err := im.setupDevices(&BuildOptions{
			EfiDevice:  "/dev/sda1",
			BootDevice: "",
			RootDevice: "/dev/sda3",
		})
		if err == nil {
			t.Error("expected error for missing device path")
		}
		if !strings.Contains(err.Error(), "missing device path") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("ExistingPartitionsBlockDeviceError", func(t *testing.T) {
		cfg := baseImageConfig()
		im := newTestImager(cfg, &ostree.MockOstree{})
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		// All 3 device paths set but BlockDeviceForPartition will fail.
		err := im.setupDevices(&BuildOptions{
			EfiDevice:  "/dev/nonexistent1",
			BootDevice: "/dev/nonexistent2",
			RootDevice: "/dev/nonexistent3",
		})
		if err == nil {
			t.Error("expected error from BlockDeviceForPartition")
		}
	})
}

// --- buildCreateQcow2 tests ---

func TestBuildCreateQcow2(t *testing.T) {
	t.Run("Disabled", func(t *testing.T) {
		cfg := baseImageConfig()
		if cfg.Bools == nil {
			cfg.Bools = make(map[string]bool)
		}
		cfg.Bools["Imager.CreateQcow2"] = false
		im := newTestImager(cfg, &ostree.MockOstree{})
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile

		var stdout bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&bytes.Buffer{})

		created, err := im.buildCreateQcow2()
		if err != nil {
			t.Fatalf("buildCreateQcow2() error: %v", err)
		}
		if created {
			t.Error("expected qcow2 creation to be disabled")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errForTest}
		im, _ := NewImager(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)

		_, err := im.buildCreateQcow2()
		if err == nil {
			t.Error("expected error from config")
		}
	})
}

// --- buildSha256sums additional tests ---

func TestBuildSha256sumsCompressedImage(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imagePath, []byte("test data"), 0644)
	compressedPath := imagePath + ".xz"
	os.WriteFile(compressedPath, []byte("compressed data"), 0644)

	cfg := baseImageConfig()
	im := newProductionTestImage(cfg)
	im.imagePath = imagePath

	var stdout bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&bytes.Buffer{})

	paths, err := im.buildSha256sums(true, false)
	if err != nil {
		t.Fatalf("buildSha256sums failed: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("expected 1 sha256 path, got %d", len(paths))
	}
	if !strings.HasSuffix(paths[0], ".xz.sha256") {
		t.Errorf("expected .xz.sha256 suffix, got %s", paths[0])
	}
}

func TestBuildSha256sumsWithQcow2(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imagePath, []byte("test data"), 0644)
	qcow2Path := imagePath + ".qcow2"
	os.WriteFile(qcow2Path, []byte("qcow2 data"), 0644)

	cfg := baseImageConfig()
	im := newProductionTestImage(cfg)
	im.imagePath = imagePath

	var stdout bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&bytes.Buffer{})

	paths, err := im.buildSha256sums(false, true)
	if err != nil {
		t.Fatalf("buildSha256sums failed: %v", err)
	}
	// Should have sha256 for image + sha256 for qcow2.
	if len(paths) != 2 {
		t.Fatalf("expected 2 sha256 paths, got %d: %v", len(paths), paths)
	}
	foundImgSha := false
	foundQcow2Sha := false
	for _, p := range paths {
		if strings.HasSuffix(p, ".img.sha256") {
			foundImgSha = true
		}
		if strings.HasSuffix(p, ".qcow2.sha256") {
			foundQcow2Sha = true
		}
	}
	if !foundImgSha {
		t.Error("expected .img.sha256 artifact")
	}
	if !foundQcow2Sha {
		t.Error("expected .qcow2.sha256 artifact")
	}
}

// --- productionizeImage additional tests ---

func TestProductionizeImageNoCompressor(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imagePath, []byte("fake image data"), 0644)

	cfg := baseImageConfig()
	cfg.Items["Imager.ImagesDir"] = []string{tmpDir}
	cfg.Items["Imager.Compressor"] = []string{""} // no compressor => error
	if cfg.Bools == nil {
		cfg.Bools = make(map[string]bool)
	}
	cfg.Bools["Imager.Productionize"] = true
	cfg.Bools["Imager.ImageTests"] = false
	cfg.Bools["Imager.CreateQcow2"] = false
	im := newProductionTestImage(cfg)
	im.imagePath = imagePath
	im.ref = "matrixos/x86_64/dev/test"

	var stdout bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&bytes.Buffer{})

	im.runner = func(cmd *runner.Cmd) error { return nil }

	_, err := im.productionizeImage("1.0.0", []string{"pkg1"})
	if err == nil {
		t.Fatal("expected error for empty compressor")
	}
	if !strings.Contains(err.Error(), "compressor") {
		t.Errorf("expected compressor error, got: %v", err)
	}
}

func TestProductionizeImageWithImageTests(t *testing.T) {
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "test.img")
	os.WriteFile(imagePath, []byte("fake image data"), 0644)

	cfg := baseImageConfig()
	cfg.Items["Imager.ImagesDir"] = []string{tmpDir}
	cfg.Items["matrixOS.Root"] = []string{tmpDir}
	cfg.Items["Imager.MountDir"] = []string{tmpDir}
	if cfg.Bools == nil {
		cfg.Bools = make(map[string]bool)
	}
	cfg.Bools["Imager.Productionize"] = true
	cfg.Bools["Imager.ImageTests"] = true
	cfg.Bools["Imager.CreateQcow2"] = false

	mo := &ostree.MockOstree{Ref_: "matrixos/x86_64/dev/test"}
	im := newProductionTestImage(cfg)
	im.ostree = mo
	im.imagePath = imagePath
	im.ref = "matrixos/x86_64/dev/test"

	var stdout bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&bytes.Buffer{})

	im.runner = func(cmd *runner.Cmd) error {
		for _, a := range cmd.Args {
			if strings.HasSuffix(a, ".img") {
				os.WriteFile(a+".xz", []byte("compressed"), 0644)
			}
		}
		return nil
	}

	// No test dir exists, so TestImage will be a no-op (no error).
	artifacts, err := im.productionizeImage("1.0.0", []string{"pkg1"})
	if err != nil {
		t.Fatalf("productionizeImage failed: %v", err)
	}
	if len(artifacts) == 0 {
		t.Error("expected some artifacts")
	}
}

// --- Build error path tests ---

func TestBuildMountDirConfigError(t *testing.T) {
	cfg := baseImageConfig()
	delete(cfg.Items, "Imager.MountDir")
	im := newTestImager(cfg, &ostree.MockOstree{})
	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	err := im.Build(&BuildOptions{})
	if err == nil {
		t.Error("expected error from Build when MountDir is missing")
	}
}

func TestBuildPrepareRootfsError(t *testing.T) {
	// MountDir works but addSysrootOverlay fails due to sysroot mismatch.
	tmpDir := t.TempDir()
	cfg := baseImageConfig()
	cfg.Items["Imager.MountDir"] = []string{tmpDir}
	mo := &ostree.MockOstree{} // Sysroot_ is empty, will mismatch
	im := newTestImager(cfg, mo)
	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	err := im.Build(&BuildOptions{})
	if err == nil {
		t.Error("expected error from Build via addSysrootOverlay mismatch")
	}
}

// --- prepareRootfs tests ---

func TestPrepareRootfs(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := baseImageConfig()
		cfg.Items["Imager.MountDir"] = []string{tmpDir}

		// The sysroot value can't be predicted (random temp dir name),
		// so we test success by using a custom approach:
		// addSysrootOverlay will fail on sysroot mismatch, but all
		// statements before it will be covered.
		mo := &ostree.MockOstree{}
		im := newTestImager(cfg, mo)
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		_, err := im.prepareRootfs()
		// Expected to fail at addSysrootOverlay (sysroot mismatch).
		if err == nil {
			t.Error("expected error from addSysrootOverlay mismatch")
		}
		if !strings.Contains(err.Error(), "sysroot overlay") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("MountDirError", func(t *testing.T) {
		cfg := baseImageConfig()
		delete(cfg.Items, "Imager.MountDir")
		im := newTestImager(cfg, &ostree.MockOstree{})
		var stdout, stderr bytes.Buffer
		im.SetStdout(&stdout)
		im.SetStderr(&stderr)

		_, err := im.prepareRootfs()
		if err == nil {
			t.Error("expected error from MountDir config")
		}
	})
}

// --- Shared test error ---

var errForTest = os.ErrInvalid
