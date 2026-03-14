package imager

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/cds"
	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
)

// --- Cleanup tracking tests ---

func TestImageCleanupTracking(t *testing.T) {
	cfg := baseImageConfig()
	im := newTestImage(cfg, &cds.MockOstree{})
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
	im := newTestImage(cfg, &cds.MockOstree{})
	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)
	// Should not panic with no resources tracked.
	im.Cleanup()
}

func TestImageConcurrentResourceTracking(t *testing.T) {
	cfg := baseImageConfig()
	im := newTestImage(cfg, &cds.MockOstree{})
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

func newProductionTestImage(cfg *config.MockConfig) *Image {
	im := newTestImage(cfg, &cds.MockOstree{})
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
	im.runner = func(_ io.Reader, _, _ io.Writer, name string, args ...string) error {
		for _, a := range args {
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
	im.runner = func(_ io.Reader, _, _ io.Writer, name string, args ...string) error {
		for _, a := range args {
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
	im.runner = func(_ io.Reader, _, _ io.Writer, name string, args ...string) error {
		switch name {
		case "qemu-img":
			// qemu-img convert -c -O qcow2 -p <input> <output>
			// The last arg is the output qcow2 path.
			if len(args) >= 2 {
				qcow2Path := args[len(args)-1]
				os.WriteFile(qcow2Path, []byte("qcow2 data"), 0644)
			}
		default:
			// xz or other compressor: create compressed file.
			for _, a := range args {
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
