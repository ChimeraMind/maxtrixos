package imager

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/runner"
)

// --- refToSuffix Tests ---

func TestRefToSuffix(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"matrixos/amd64/gnome", "matrixos_amd64_gnome"},
		{"simple", "simple"},
		{"a/b/c/d", "a_b_c_d"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := refToSuffix(tt.input)
			if got != tt.expected {
				t.Errorf("refToSuffix(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// --- BuildImagePath Tests ---

func TestBuildImagePath(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.ref = "matrixos/amd64/gnome"
		result, err := im.BuildImagePath()
		if err != nil {
			t.Fatalf("BuildImagePath() error: %v", err)
		}
		expected := "/tmp/images/matrixos_amd64_gnome.img"
		if result != expected {
			t.Errorf("BuildImagePath() = %q, want %q", result, expected)
		}
	})

	t.Run("StripsRemote", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.ref = "origin:matrixos/amd64/gnome"
		result, err := im.BuildImagePath()
		if err != nil {
			t.Fatalf("BuildImagePath() error: %v", err)
		}
		expected := "/tmp/images/matrixos_amd64_gnome.img"
		if result != expected {
			t.Errorf("BuildImagePath() = %q, want %q", result, expected)
		}
	})

	t.Run("EmptyRef", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		_, err := im.BuildImagePath()
		if err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.ref = "someref"
		_, err := im.BuildImagePath()
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

// --- BuildImagePathWithReleaseVersion Tests ---

func TestBuildImagePathWithReleaseVersion(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.ref = "matrixos/amd64/gnome"
		result, err := im.BuildImagePathWithReleaseVersion("20260221")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		expected := "/tmp/images/matrixos_amd64_gnome-20260221.img"
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("EmptyRef", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		_, err := im.BuildImagePathWithReleaseVersion("20260221")
		if err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("EmptyReleaseVersion", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.ref = "ref"
		_, err := im.BuildImagePathWithReleaseVersion("")
		if err == nil {
			t.Error("should error for empty releaseVersion")
		}
	})
}

// --- CompressedImagePath Tests ---

func TestCompressedImagePath(t *testing.T) {
	t.Run("XZ", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile
		result, err := im.CompressedImagePath()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		// Default compressor is "xz -f -0 -T0", so extension should be .xz
		if result != "/tmp/test.img.xz" {
			t.Errorf("got %q, want /tmp/test.img.xz", result)
		}
	})

	t.Run("Zstd", func(t *testing.T) {
		cfg := baseImageConfig()
		cfg.Items["Imager.Compressor"] = []string{"zstd -3"}
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile
		result, err := im.CompressedImagePath()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "/tmp/test.img.zstd" {
			t.Errorf("got %q, want /tmp/test.img.zstd", result)
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.mode = ModeCreateImageFile
		_, err := im.CompressedImagePath()
		if err == nil {
			t.Error("should error for empty imagePath")
		}
	})

	t.Run("EmptyCompressor", func(t *testing.T) {
		cfg := baseImageConfig()
		cfg.Items["Imager.Compressor"] = []string{""}
		im := newTestImage(cfg, &ostree.MockOstree{})
		im.imagePath = "/tmp/x.img"
		im.mode = ModeCreateImageFile
		_, err := im.CompressedImagePath()
		if err == nil {
			t.Error("should error for empty compressor")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.imagePath = "/tmp/x.img"
		im.mode = ModeCreateImageFile
		_, err := im.CompressedImagePath()
		if err == nil {
			t.Error("should error when config fails")
		}
	})
}

// --- CompressImage Tests ---

func TestCompressImage(t *testing.T) {
	t.Run("EmptyPath", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &ostree.MockOstree{}, runner)
		im.mode = ModeCreateImageFile
		err := im.CompressImage()
		if err == nil {
			t.Error("should error for empty imagePath")
		}
	})

	t.Run("EmptyCompressor", func(t *testing.T) {
		cfg := baseImageConfig()
		cfg.Items["Imager.Compressor"] = []string{""}
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(cfg, &ostree.MockOstree{}, runner)
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile
		err := im.CompressImage()
		if err == nil {
			t.Error("should error for empty compressor")
		}
	})

	t.Run("CommandArgs", func(t *testing.T) {
		tmpDir := t.TempDir()
		imgPath := filepath.Join(tmpDir, "test.img")
		// Create the expected output file so the existence check passes.
		os.WriteFile(imgPath+".xz", []byte("compressed"), 0644)

		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &ostree.MockOstree{}, runner)
		im.imagePath = imgPath
		im.mode = ModeCreateImageFile

		err := im.CompressImage()
		if err != nil {
			t.Fatalf("CompressImage() error: %v", err)
		}
		if len(runner.Calls) < 1 {
			t.Fatal("expected at least 1 runner call")
		}
		if runner.Calls[0].Name != "xz" {
			t.Errorf("expected xz command, got %q", runner.Calls[0].Name)
		}
		args := runner.Calls[0].Args
		// Args should be [-f -0 -T0 <imgPath>].
		if len(args) != 4 {
			t.Fatalf("expected 4 args, got %d: %v", len(args), args)
		}
		if args[len(args)-1] != imgPath {
			t.Errorf("last arg should be image path, got %q", args[len(args)-1])
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile
		err := im.CompressImage()
		if err == nil {
			t.Error("should error when config fails")
		}
	})
}

// --- ExtractReleaseVersion Tests ---

func TestExtractReleaseVersion(t *testing.T) {
	t.Run("FallbackToDate", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.SetRootfs(tmpDir)
		result, err := im.ExtractReleaseVersion()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		expected := time.Now().Format("20060102")
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("FromMetadata", func(t *testing.T) {
		tmpDir := t.TempDir()
		metadataDir := filepath.Join(tmpDir, "etc", "matrixos")
		os.MkdirAll(metadataDir, 0755)
		os.WriteFile(filepath.Join(metadataDir, "build.txt"),
			[]byte("SEED_NAME=matrixos-gnome-20260215\nBUILD_DATE=2026-02-15\n"), 0644)

		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.SetRootfs(tmpDir)
		result, err := im.ExtractReleaseVersion()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "20260215" {
			t.Errorf("got %q, want 20260215", result)
		}
	})

	t.Run("EmptyRootfs", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		_, err := im.ExtractReleaseVersion()
		if err == nil {
			t.Error("should error for empty rootfs")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.SetRootfs("/tmp/rootfs")
		_, err := im.ExtractReleaseVersion()
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

// --- Qcow2ImagePath Tests ---

func TestQcow2ImagePath(t *testing.T) {
	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})

	t.Run("Success", func(t *testing.T) {
		im.imagePath = "/tmp/images/test.img"
		im.mode = ModeCreateImageFile
		result, err := im.Qcow2ImagePath()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "/tmp/images/test.img.qcow2" {
			t.Errorf("got %q, want /tmp/images/test.img.qcow2", result)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		im.imagePath = ""
		im.mode = ModeCreateImageFile
		_, err := im.Qcow2ImagePath()
		if err == nil {
			t.Error("should error for empty imagePath")
		}
	})
}

// --- CreateQcow2Image Tests ---

func TestCreateQcow2Image(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &ostree.MockOstree{}, runner)
		im.imagePath = "/tmp/images/test.img"
		im.mode = ModeCreateImageFile

		err := im.CreateQcow2Image()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(runner.Calls) != 1 || runner.Calls[0].Name != "qemu-img" {
			t.Errorf("expected qemu-img call, got %v", runner.Calls)
		}
		// Verify output path ends with .qcow2.
		args := runner.Calls[0].Args
		if args[len(args)-1] != "/tmp/images/test.img.qcow2" {
			t.Errorf("last arg should be qcow2 path, got %q", args[len(args)-1])
		}
	})

	t.Run("Empty", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.mode = ModeCreateImageFile
		err := im.CreateQcow2Image()
		if err == nil {
			t.Error("should error for empty imagePath")
		}
	})
}

// --- RemoveImageFile Tests ---

func TestRemoveImageFile(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		imgPath := filepath.Join(tmpDir, "test.img")
		os.WriteFile(imgPath, []byte("data"), 0644)
		os.WriteFile(imgPath+".sha256", []byte("hash"), 0644)
		os.WriteFile(imgPath+".asc", []byte("sig"), 0644)

		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.imagePath = imgPath
		im.mode = ModeCreateImageFile
		err := im.RemoveImageFile()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		for _, p := range []string{imgPath, imgPath + ".sha256", imgPath + ".asc"} {
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Errorf("%s should have been removed", p)
			}
		}
	})

	t.Run("NonexistentFile", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.imagePath = "/tmp/nonexistent.img"
		im.mode = ModeCreateImageFile
		err := im.RemoveImageFile()
		if err != nil {
			t.Error("should not error when file doesn't exist")
		}
	})

	t.Run("Empty", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.mode = ModeCreateImageFile
		err := im.RemoveImageFile()
		if err == nil {
			t.Error("should error for empty path")
		}
	})
}

// --- ShowFinalFilesystemInfo Tests ---

func TestShowFinalFilesystemInfo(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// Create temp directories so PrintDirectoryTree can walk them.
		bootDir := t.TempDir()
		efiDir := t.TempDir()
		// Create a few files to verify the walk.
		os.MkdirAll(filepath.Join(bootDir, "grub"), 0755)
		os.WriteFile(filepath.Join(bootDir, "grub", "grub.cfg"), []byte("test"), 0644)
		os.MkdirAll(filepath.Join(efiDir, "EFI", "BOOT"), 0755)

		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &ostree.MockOstree{}, runner)
		im.devicePath = "/dev/loop0"
		im.bootfsMount = bootDir
		im.efifsMount = efiDir

		err := im.ShowFinalFilesystemInfo()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		// No runner calls – directory listing and block device info
		// are now pure Go.
		if len(runner.Calls) != 0 {
			t.Fatalf("expected 0 runner calls, got %d", len(runner.Calls))
		}
	})

	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		if err := im.ShowFinalFilesystemInfo(); err == nil {
			t.Error("should error for empty devicePath")
		}
		im.devicePath = "/dev/x"
		if err := im.ShowFinalFilesystemInfo(); err == nil {
			t.Error("should error for empty bootfsMount")
		}
		im.bootfsMount = "/a"
		if err := im.ShowFinalFilesystemInfo(); err == nil {
			t.Error("should error for empty efifsMount")
		}
	})
}

// --- ShowImageTestInfo Tests ---

func TestShowImageTestInfo(t *testing.T) {
	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
	im.imagePath = "/tmp/test.img"
	im.mode = ModeCreateImageFile
	// Should not error with valid artifacts.
	if err := im.ShowImageTestInfo([]string{"/tmp/test.img", "/tmp/test.img.xz"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should not error with empty artifacts.
	if err := im.ShowImageTestInfo(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- cleanAndStripRef Tests ---

func TestCleanAndStripRef(t *testing.T) {
	t.Run("WithRemoteAndFull", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{Ref_: "origin:matrixos/amd64/gnome-full"})
		im.ref = "origin:matrixos/amd64/gnome-full"
		result, err := im.cleanAndStripRef()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "matrixos/amd64/gnome" {
			t.Errorf("got %q, want matrixos/amd64/gnome", result)
		}
	})

	t.Run("WithoutSuffix", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{Ref_: "matrixos/amd64/gnome"})
		im.ref = "matrixos/amd64/gnome"
		result, err := im.cleanAndStripRef()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "matrixos/amd64/gnome" {
			t.Errorf("got %q, want matrixos/amd64/gnome", result)
		}
	})

	t.Run("OstreeError", func(t *testing.T) {
		mo := &ostree.MockOstree{RemoveFullErr: errors.New("ostree error")}
		im := newTestImage(baseImageConfig(), mo)
		im.ref = "ref"
		_, err := im.cleanAndStripRef()
		if err == nil {
			t.Error("should propagate ostree error")
		}
	})

	t.Run("EmptyAfterStrip", func(t *testing.T) {
		mo := &ostree.MockOstree{RemoveFullResult: "", RemoveFullResultSet: true}
		im := newTestImage(baseImageConfig(), mo)
		im.ref = "ref"
		_, err := im.cleanAndStripRef()
		if err == nil {
			t.Error("should error for empty result after cleaning")
		}
	})
}
