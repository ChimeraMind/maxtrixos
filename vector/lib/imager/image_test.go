package imager

import (
	"bytes"
	"errors"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/runner"
)

// baseImageConfig returns a mock config with all keys needed by Image.
func baseImageConfig() *config.MockConfig {
	return &config.MockConfig{
		Items: map[string][]string{
			"Imager.ImagesDir":                      {"/tmp/images"},
			"Imager.MountDir":                       {"/tmp/mnt"},
			"Imager.ImageSize":                      {"32G"},
			"Imager.EfiPartitionSize":               {"200M"},
			"Imager.BootPartitionSize":              {"1G"},
			"Imager.Compressor":                     {"xz -f -0 -T0"},
			"Imager.EspPartitionType":               {"C12A7328-F81F-11D2-BA4B-00A0C93EC93B"},
			"Imager.BootPartitionType":              {"BC13C2FF-59E6-4262-A352-B275FD6F7172"},
			"Imager.RootPartitionType":              {"4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709"},
			"matrixOS.OsName":                       {"matrixos"},
			"Imager.BootRoot":                       {"/boot"},
			"Imager.EfiRoot":                        {"/efi"},
			"Imager.RelativeEfiBootPath":            {"EFI/BOOT"},
			"Imager.EfiExecutable":                  {"BOOTX64.EFI"},
			"Imager.EfiCertificateFileName":         {"secureboot.pem"},
			"Imager.EfiCertificateFileNameDer":      {"secureboot.der"},
			"Imager.EfiCertificateFileNameKek":      {"secureboot-kek.pem"},
			"Imager.EfiCertificateFileNameKekDer":   {"secureboot-kek.der"},
			"Releaser.ReadOnlyVdb":                  {"/usr/var-db-pkg"},
			"matrixOS.Root":                         {"/opt/matrixos"},
			"Imager.LocksDir":                       {"/tmp/locks"},
			"Imager.LockWaitSeconds":                {"300"},
			"Seeder.ChrootMetadataDir":              {"/etc/matrixos"},
			"Seeder.ChrootMetadataDirBuildFileName": {"build.txt"},
			"matrixOS.LogsDir":                      {"/tmp/logs"},
			"Imager.HooksDir":                       {"/tmp/image/hooks"},
		},
	}
}

func newTestImage(cfg *config.MockConfig, ostree *cds.MockOstree) *Image {
	im, _ := NewImage(cfg, ostree, filesystems.DefaultMockFsenc(), nil)
	return im
}

func newTestImageWithRunner(cfg *config.MockConfig, ostree *ostree.MockOstree, runner *runner.MockRunner) *Image {
	im := newTestImage(cfg, ostree)
	im.runner = runner.Run
	return im
}

// --- Interface compliance ---

func TestImageImplementsIImage(t *testing.T) {
	var _ IImage = (*Image)(nil)
}

// --- NewImage Tests ---

func TestNewImage(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		im, err := NewImage(baseImageConfig(), &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		if err != nil {
			t.Fatalf("NewImage() error: %v", err)
		}
		if im == nil {
			t.Fatal("NewImage() returned nil")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		_, err := NewImage(nil, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		if err == nil {
			t.Fatal("expected error for nil config")
		}
	})

	t.Run("NilOstree", func(t *testing.T) {
		_, err := NewImage(baseImageConfig(), nil, filesystems.DefaultMockFsenc(), nil)
		if err == nil {
			t.Fatal("expected error for nil ostree")
		}
	})

	t.Run("NilFsenc", func(t *testing.T) {
		_, err := NewImage(baseImageConfig(), &cds.MockOstree{}, nil, nil)
		if err == nil {
			t.Fatal("expected error for nil fsenc")
		}
	})
}

// --- Config Accessor Tests ---

func TestConfigAccessors(t *testing.T) {
	cfg := baseImageConfig()
	im := newTestImage(cfg, &cds.MockOstree{})

	tests := []struct {
		name     string
		fn       func() (string, error)
		expected string
	}{
		{"ImagesDir", im.ImagesDir, "/tmp/images"},
		{"MountDir", im.MountDir, "/tmp/mnt"},
		{"ImageSize", im.ImageSize, "32G"},
		{"EfiPartitionSize", im.EfiPartitionSize, "200M"},
		{"BootPartitionSize", im.BootPartitionSize, "1G"},
		{"Compressor", im.Compressor, "xz -f -0 -T0"},
		{"EspPartitionType", im.EspPartitionType, "C12A7328-F81F-11D2-BA4B-00A0C93EC93B"},
		{"BootPartitionType", im.BootPartitionType, "BC13C2FF-59E6-4262-A352-B275FD6F7172"},
		{"RootPartitionType", im.RootPartitionType, "4F68BCE3-E8CD-4DB1-96E7-FBCAF984B709"},
		{"OsName", im.OsName, "matrixos"},
		{"BootRoot", im.BootRoot, "/boot"},
		{"EfiRoot", im.EfiRoot, "/efi"},
		{"RelativeEfiBootPath", im.RelativeEfiBootPath, "EFI/BOOT"},
		{"EfiExecutable", im.EfiExecutable, "BOOTX64.EFI"},
		{"EfiCertificateFileName", im.EfiCertificateFileName, "secureboot.pem"},
		{"EfiCertificateFileNameDer", im.EfiCertificateFileNameDer, "secureboot.der"},
		{"EfiCertificateFileNameKek", im.EfiCertificateFileNameKek, "secureboot-kek.pem"},
		{"EfiCertificateFileNameKekDer", im.EfiCertificateFileNameKekDer, "secureboot-kek.der"},
		{"ReadOnlyVdb", im.ReadOnlyVdb, "/usr/var-db-pkg"},
		{"DevDir", im.DevDir, "/opt/matrixos"},
		{"LockDir", im.LockDir, "/tmp/locks"},
		{"LockWaitSeconds", im.LockWaitSeconds, "300"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			val, err := tt.fn()
			if err != nil {
				t.Fatalf("%s() error: %v", tt.name, err)
			}
			if val != tt.expected {
				t.Errorf("%s() = %q, want %q", tt.name, val, tt.expected)
			}
		})
	}
}

func TestConfigAccessorsEmptyValue(t *testing.T) {
	accessors := []struct {
		key  string
		name string
		fn   func(*Image) (string, error)
	}{
		{"Imager.ImagesDir", "ImagesDir", func(im *Image) (string, error) { return im.ImagesDir() }},
		{"Imager.MountDir", "MountDir", func(im *Image) (string, error) { return im.MountDir() }},
		{"Imager.ImageSize", "ImageSize", func(im *Image) (string, error) { return im.ImageSize() }},
		{"matrixOS.OsName", "OsName", func(im *Image) (string, error) { return im.OsName() }},
		{"Imager.LocksDir", "LockDir", func(im *Image) (string, error) { return im.LockDir() }},
	}

	for _, tt := range accessors {
		t.Run(tt.name+"_Empty", func(t *testing.T) {
			cfg := baseImageConfig()
			cfg.Items[tt.key] = []string{""}
			im := newTestImage(cfg, &cds.MockOstree{})
			_, err := tt.fn(im)
			if err == nil {
				t.Errorf("%s() should return error for empty value", tt.name)
			}
		})
	}
}

func TestConfigAccessorsConfigError(t *testing.T) {
	ec := &config.ErrConfig{Err: errors.New("cfg error")}
	im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
	im.runner = runner.NewMockRunner().Run

	accessors := []struct {
		name string
		fn   func() (string, error)
	}{
		{"ImagesDir", im.ImagesDir},
		{"MountDir", im.MountDir},
		{"ImageSize", im.ImageSize},
		{"OsName", im.OsName},
		{"BootRoot", im.BootRoot},
		{"EfiRoot", im.EfiRoot},
		{"LockDir", im.LockDir},
	}

	for _, tt := range accessors {
		t.Run(tt.name+"_ConfigError", func(t *testing.T) {
			_, err := tt.fn()
			if err == nil {
				t.Errorf("%s() should return error from broken config", tt.name)
			}
		})
	}
}

// --- BuildMetadataFile Tests ---

func TestBuildMetadataFile(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		cfg := baseImageConfig()
		im := newTestImage(cfg, &cds.MockOstree{})
		result, err := im.BuildMetadataFile()
		if err != nil {
			t.Fatalf("BuildMetadataFile() error: %v", err)
		}
	})
}

// --- Stdout/Stderr Getter Tests ---

func TestImageOutputGetters(t *testing.T) {
	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})

	// Default writers are os.Stdout and os.Stderr.
	if im.Stdout() == nil {
		t.Error("Stdout() should not be nil")
	}
	if im.Stderr() == nil {
		t.Error("Stderr() should not be nil")
	}

	// After setting custom writers.
	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	if im.Stdout() != &stdout {
		t.Error("Stdout() should return the custom writer")
	}
	if im.Stderr() != &stderr {
		t.Error("Stderr() should return the custom writer")
	}
}

// --- Print Method Tests ---

func TestImagePrintMethods(t *testing.T) {
	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	im.Print("hello %s\n", "world")
	if stdout.String() != "hello world\n" {
		t.Errorf("Print() output: %q", stdout.String())
	}

	im.PrintWarning("warn %d\n", 42)
	if stderr.String() != "warn 42\n" {
		t.Errorf("PrintWarning() output: %q", stderr.String())
	}

	stderr.Reset()
	im.PrintError("err %s\n", "test")
	if stderr.String() != "err test\n" {
		t.Errorf("PrintError() output: %q", stderr.String())
	}
}

// --- trackMounts Tests ---

func TestImageTrackMounts(t *testing.T) {
	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
	var stdout, stderr bytes.Buffer
	im.SetStdout(&stdout)
	im.SetStderr(&stderr)

	im.trackMounts([]string{"/mnt/a", "/mnt/b", "/mnt/c"})
	if len(im.trackedMounts) != 3 {
		t.Errorf("expected 3 mounts, got %d", len(im.trackedMounts))
	}

	im.trackMount("/mnt/d")
	if len(im.trackedMounts) != 4 {
		t.Errorf("expected 4 mounts, got %d", len(im.trackedMounts))
	}
}

// --- SetImageMode Tests ---

func TestSetImageMode(t *testing.T) {
	t.Run("FlashToDeviceSuccess", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.devicePath = "/dev/sda"
		err := im.SetImageMode(ModeFlashToDevice)
		if err != nil {
			t.Fatalf("SetImageMode() error: %v", err)
		}
		if im.ImageMode() != ModeFlashToDevice {
			t.Error("mode should be ModeFlashToDevice")
		}
	})

	t.Run("EmptyDir", func(t *testing.T) {
		cfg := baseImageConfig()
		cfg.Items["Seeder.ChrootMetadataDir"] = []string{""}
		im := newTestImage(cfg, &cds.MockOstree{})
		_, err := im.BuildMetadataFile()
		if err == nil {
			t.Error("should error for empty metadata dir")
		}
	})

	t.Run("EmptyFileName", func(t *testing.T) {
		cfg := baseImageConfig()
		cfg.Items["Seeder.ChrootMetadataDirBuildFileName"] = []string{""}
		im := newTestImage(cfg, &cds.MockOstree{})
		_, err := im.BuildMetadataFile()
		if err == nil {
			t.Error("should error for empty build file name")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		_, err := im.BuildMetadataFile()
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		_, err := im.BuildImagePath()
		if err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		_, err := im.BuildImagePathWithReleaseVersion("20260221")
		if err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("EmptyReleaseVersion", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.ref = "ref"
		_, err := im.BuildImagePathWithReleaseVersion("")
		if err == nil {
			t.Error("should error for empty releaseVersion")
		}
	})
}

// --- BuildImagePathWithCompressorExtension Tests ---

func TestBuildImagePathWithCompressorExtension(t *testing.T) {
	t.Run("XZ", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile
		result, err := im.BuildImagePathWithCompressorExtension()
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
		im := newTestImage(cfg, &cds.MockOstree{})
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile
		result, err := im.BuildImagePathWithCompressorExtension()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "/tmp/test.img.zstd" {
			t.Errorf("got %q, want /tmp/test.img.zstd", result)
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.mode = ModeCreateImageFile
		_, err := im.BuildImagePathWithCompressorExtension()
		if err == nil {
			t.Error("should error for empty imagePath")
		}
	})

	t.Run("EmptyCompressor", func(t *testing.T) {
		cfg := baseImageConfig()
		cfg.Items["Imager.Compressor"] = []string{""}
		im := newTestImage(cfg, &cds.MockOstree{})
		im.imagePath = "/tmp/x.img"
		im.mode = ModeCreateImageFile
		_, err := im.BuildImagePathWithCompressorExtension()
		if err == nil {
			t.Error("should error for empty compressor")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.imagePath = "/tmp/x.img"
		im.mode = ModeCreateImageFile
		_, err := im.BuildImagePathWithCompressorExtension()
		if err == nil {
			t.Error("should error when config fails")
		}
	})
}

// --- CreateImage Tests ---

func TestCreateImage(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		imagePath := filepath.Join(tmpDir, "subdir", "test.img")
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.imagePath = imagePath
		im.mode = ModeCreateImageFile

		err := im.CreateImage("1M")
		if err != nil {
			t.Fatalf("CreateImage() error: %v", err)
		}
		// Verify sparse file was created with the right size.
		info, err := os.Stat(imagePath)
		if err != nil {
			t.Fatalf("image file not created: %v", err)
		}
		expectedSize := int64(1024 * 1024)
		if info.Size() != expectedSize {
			t.Errorf("expected size %d, got %d", expectedSize, info.Size())
		}
	})

	t.Run("SuccessWithGigabytes", func(t *testing.T) {
		tmpDir := t.TempDir()
		imagePath := filepath.Join(tmpDir, "test.img")
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.imagePath = imagePath
		im.mode = ModeCreateImageFile

		err := im.CreateImage("1G")
		if err != nil {
			t.Fatalf("CreateImage() error: %v", err)
		}
		info, err := os.Stat(imagePath)
		if err != nil {
			t.Fatalf("image file not created: %v", err)
		}
		expectedSize := int64(1024 * 1024 * 1024)
		if info.Size() != expectedSize {
			t.Errorf("expected size %d, got %d", expectedSize, info.Size())
		}
	})

	t.Run("CreatesParentDirectories", func(t *testing.T) {
		tmpDir := t.TempDir()
		imagePath := filepath.Join(tmpDir, "a", "b", "c", "test.img")
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.imagePath = imagePath
		im.mode = ModeCreateImageFile

		err := im.CreateImage("1K")
		if err != nil {
			t.Fatalf("CreateImage() error: %v", err)
		}
		info, err := os.Stat(imagePath)
		if err != nil {
			t.Fatalf("image file not created: %v", err)
		}
		if info.Size() != 1024 {
			t.Errorf("expected size 1024, got %d", info.Size())
		}
	})

	t.Run("EmptyPath", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.mode = ModeCreateImageFile
		err := im.CreateImage("32G")
		if err == nil {
			t.Error("should error for empty imagePath")
		}
	})

	t.Run("EmptySize", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile
		err := im.CreateImage("")
		if err == nil {
			t.Error("should error for empty imageSize")
		}
	})

	t.Run("InvalidSize", func(t *testing.T) {
		tmpDir := t.TempDir()
		imagePath := filepath.Join(tmpDir, "test.img")
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.imagePath = imagePath
		im.mode = ModeCreateImageFile

		err := im.CreateImage("notanumber")
		if err == nil {
			t.Error("should error for invalid size")
		}
	})
}

// --- CompressImage Tests ---

func TestCompressImage(t *testing.T) {
	t.Run("EmptyPath", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
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
		im := newTestImageWithRunner(cfg, &cds.MockOstree{}, runner)
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
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
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
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile
		err := im.CompressImage()
		if err == nil {
			t.Error("should error when config fails")
		}
	})
}

// --- ClearPartitionTable Tests ---

func TestClearPartitionTable(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
		im.devicePath = "/dev/sda"

		err := im.ClearPartitionTable()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(runner.Calls) != 2 {
			t.Fatalf("expected 2 sgdisk calls, got %d", len(runner.Calls))
		}
		if runner.Calls[0].Name != "sgdisk" {
			t.Errorf("call 0: expected sgdisk, got %q", runner.Calls[0].Name)
		}
		if runner.Calls[1].Name != "sgdisk" {
			t.Errorf("call 1: expected sgdisk, got %q", runner.Calls[1].Name)
		}
	})

	t.Run("EmptyDevice", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		err := im.ClearPartitionTable()
		if err == nil {
			t.Error("should error for empty devicePath")
		}
	})

	t.Run("FirstSgdiskFails", func(t *testing.T) {
		runner := runner.NewMockRunnerFailOnCall(0, errors.New("sgdisk error"))
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
		im.devicePath = "/dev/sda"

		err := im.ClearPartitionTable()
		if err == nil {
			t.Error("should propagate sgdisk error")
		}
	})
}

// --- DatedFsLabel Tests ---

func TestDatedFsLabel(t *testing.T) {
	im := newTestImage(baseImageConfig(), &cds.MockOstree{})
	label := im.DatedFsLabel()
	expected := time.Now().Format("20060102")
	if label != expected {
		t.Errorf("DatedFsLabel() = %q, want %q", label, expected)
	}
}

// --- PartitionDevices Tests ---

func TestPartitionDevices(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)

		im.devicePath = "/dev/loop0"
		err := im.PartitionDevices("200M", "1G", "32G")
		if err != nil {
			t.Fatalf("SetImageMode() error: %v", err)
		}
		if im.ImageMode() != ModeCreateImageFile {
			t.Error("mode should be ModeCreateImageFile")
		}
	})

	t.Run("EmptyParams", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)

		im.devicePath = "/dev/x"
		if err := im.PartitionDevices("", "1G", "32G"); err == nil {
			t.Error("should error for empty efiSize")
		}
		if err := im.PartitionDevices("200M", "", "32G"); err == nil {
			t.Error("should error for empty bootSize")
		}
		if err := im.PartitionDevices("200M", "1G", ""); err == nil {
			t.Error("should error for empty imageSize")
		}
		im.devicePath = ""
		if err := im.PartitionDevices("200M", "1G", "32G"); err == nil {
			t.Error("should error for empty devicePath")
		}
	})

	t.Run("SgdiskFails", func(t *testing.T) {
		runner := runner.NewMockRunnerFailOnCall(0, errors.New("sgdisk failed"))
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
		im.devicePath = "/dev/loop0"

		err := im.PartitionDevices("200M", "1G", "32G")
		if err == nil {
			t.Error("should propagate sgdisk error")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		runner := runner.NewMockRunner()
		im.runner = runner.Run
		im.devicePath = "/dev/loop0"

		err := im.PartitionDevices("200M", "1G", "32G")
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

// --- FormatEfifs Tests ---

func TestFormatEfifs(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
		im.efiDevice = "/dev/loop0p1"

		err := im.FormatEfifs()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(runner.Calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(runner.Calls))
		}
		if runner.Calls[0].Name != "mkfs.vfat" {
			t.Errorf("expected mkfs.vfat, got %q", runner.Calls[0].Name)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.FormatEfifs(); err == nil {
			t.Error("should error for empty device")
		}
	})
}

// --- MountEfifs Tests ---

func TestMountEfifs(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountPoint := filepath.Join(tmpDir, "efi")
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
		im.efiDevice = "/dev/loop0p1"

		err := im.MountEfifs(mountPoint)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(runner.Calls) != 1 || runner.Calls[0].Name != "mount" {
			t.Errorf("expected mount call, got %v", runner.Calls)
		}
	})

	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.MountEfifs("/tmp/efi"); err == nil {
			t.Error("should error for empty device")
		}
		im.efiDevice = "/dev/x"
		if err := im.MountEfifs(""); err == nil {
			t.Error("should error for empty mount point")
		}
	})
}

// --- FormatBootfs Tests ---

func TestFormatBootfs(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
		im.bootDevice = "/dev/loop0p2"

		err := im.FormatBootfs()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if runner.Calls[0].Name != "mkfs.btrfs" {
			t.Errorf("expected mkfs.btrfs, got %q", runner.Calls[0].Name)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.FormatBootfs(); err == nil {
			t.Error("should error for empty device")
		}
	})
}

// --- MountBootfs Tests ---

func TestMountBootfs(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		mountPoint := filepath.Join(tmpDir, "boot")
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
		im.bootDevice = "/dev/loop0p2"

		err := im.MountBootfs(mountPoint)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(runner.Calls) != 1 || runner.Calls[0].Name != "mount" {
			t.Errorf("expected mount call, got %v", runner.Calls)
		}
	})

	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.MountBootfs("/boot"); err == nil {
			t.Error("should error for empty device")
		}
		im.bootDevice = "/dev/x"
		if err := im.MountBootfs(""); err == nil {
			t.Error("should error for empty mount point")
		}
	})
}

// --- FormatRootfs Tests ---

func TestFormatRootfs(t *testing.T) {
	runner := runner.NewMockRunner()
	im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
	im.rootDevice = "/dev/loop0p3"

	err := im.FormatRootfs()
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if runner.Calls[0].Name != "mkfs.btrfs" {
		t.Errorf("expected mkfs.btrfs, got %q", runner.Calls[0].Name)
	}
}

// --- RootfsKernelArgs Tests ---

func TestRootfsKernelArgs(t *testing.T) {
	im := newTestImage(baseImageConfig(), &cds.MockOstree{})
	args := im.RootfsKernelArgs()
	if len(args) != 1 || args[0] != "rootflags=discard=async" {
		t.Errorf("unexpected kernel args: %v", args)
	}
}

// --- MountRootfs Tests ---

func TestMountRootfs(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
		im.rootDevice = "/dev/loop0p3"

		err := im.MountRootfs("/tmp/rootfs")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if runner.Calls[0].Name != "mount" {
			t.Errorf("expected mount, got %q", runner.Calls[0].Name)
		}
		// Check btrfs options.
		found := false
		for _, arg := range runner.Calls[0].Args {
			if strings.Contains(arg, "compress-force=zstd:6") {
				found = true
				break
			}
		}
		if !found {
			t.Error("expected btrfs compression options in mount args")
		}
	})

	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.MountRootfs("/tmp/mnt"); err == nil {
			t.Error("should error for empty rootDevice")
		}
		im.rootDevice = "/dev/x"
		if err := im.MountRootfs(""); err == nil {
			t.Error("should error for empty mountRootfs")
		}
	})
}

// --- GetKernelPath Tests ---

func TestGetKernelPath(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		modulesDir := filepath.Join(tmpDir, "usr", "lib", "modules")
		os.MkdirAll(filepath.Join(modulesDir, "6.1.0-matrixos"), 0755)
		os.MkdirAll(filepath.Join(modulesDir, "6.2.0-matrixos"), 0755)

		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.SetRootfs(tmpDir)
		_, err := im.GetKernelPath()
		if err == nil {
			t.Error("should error when modules dir doesn't exist")
		}
	})

	t.Run("EmptyModulesDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		os.MkdirAll(filepath.Join(tmpDir, "usr", "lib", "modules"), 0755)
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.SetRootfs(tmpDir)
		_, err := im.GetKernelPath()
		if err == nil {
			t.Error("should error for empty modules dir")
		}
	})

	t.Run("EmptyParam", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		_, err := im.GetKernelPath()
		if err == nil {
			t.Error("should error for empty param")
		}
	})
}

// --- SetupPasswords Tests ---

func TestSetupPasswords(t *testing.T) {
	t.Run("EmptyParam", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		err := im.SetupPasswords()
		if err == nil {
			t.Error("should error for empty param")
		}
	})
}

// --- ExtractReleaseVersion Tests ---

func TestExtractReleaseVersion(t *testing.T) {
	t.Run("FallbackToDate", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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

		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		_, err := im.ExtractReleaseVersion()
		if err == nil {
			t.Error("should error for empty rootfs")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.SetRootfs("/tmp/rootfs")
		_, err := im.ExtractReleaseVersion()
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

// --- Qcow2ImagePath Tests ---

func TestQcow2ImagePath(t *testing.T) {
	im := newTestImage(baseImageConfig(), &cds.MockOstree{})

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
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.mode = ModeCreateImageFile
		err := im.CreateQcow2Image()
		if err == nil {
			t.Error("should error for empty imagePath")
		}
	})

// --- RemoveImageFile Tests ---

func TestRemoveImageFile(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		imgPath := filepath.Join(tmpDir, "test.img")
		os.WriteFile(imgPath, []byte("data"), 0644)
		os.WriteFile(imgPath+".sha256", []byte("hash"), 0644)
		os.WriteFile(imgPath+".asc", []byte("sig"), 0644)

		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.imagePath = "/tmp/nonexistent.img"
		im.mode = ModeCreateImageFile
		err := im.RemoveImageFile()
		if err != nil {
			t.Error("should not error when file doesn't exist")
		}
	})

	t.Run("Empty", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.mode = ModeCreateImageFile
		err := im.RemoveImageFile()
		if err == nil {
			t.Error("should error for invalid mode")
		}
	})
}

// --- NewImage with Options Tests ---

func TestNewImageWithOptions(t *testing.T) {
	t.Run("WithDeviceOpts", func(t *testing.T) {
		opts := &NewImageOptions{
			EfiDevice:  "/dev/sda1",
			BootDevice: "/dev/sda2",
			RootDevice: "/dev/sda3",
			DevicePath: "/dev/sda",
			Mode:       ModeFlashToDevice,
		}
		im, err := NewImage(baseImageConfig(), &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), opts)
		if err != nil {
			t.Fatalf("NewImage() error: %v", err)
		}
		if im.EfiDevice() != "/dev/sda1" {
			t.Errorf("EfiDevice() = %q", im.EfiDevice())
		}
		if im.BootDevice() != "/dev/sda2" {
			t.Errorf("BootDevice() = %q", im.BootDevice())
		}
		if im.RootDevice() != "/dev/sda3" {
			t.Errorf("RootDevice() = %q", im.RootDevice())
		}
		if im.DevicePath() != "/dev/sda" {
			t.Errorf("DevicePath() = %q", im.DevicePath())
		}
		if im.ImageMode() != ModeFlashToDevice {
			t.Errorf("ImageMode() = %d", im.ImageMode())
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		_, err := im.ImageLockDir()
		if err == nil {
			t.Error("expected error from EncryptionEnabled")
		}
	})
}

// --- Device Setter/Getter Tests ---

func TestImageLockPath(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		im := newTestImage(cfg, &cds.MockOstree{})
		im.ref = "matrixos/amd64/gnome"

		result, err := im.ImageLockPath()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		expected := filepath.Join(lockDir, "matrixos/amd64/gnome.lock")
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("EmptyRef", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		_, err := im.ImageLockPath()
		if err == nil {
			t.Error("should error for empty ref")
		}
	})
}

// --- Mount Point Getter Tests ---

func TestFinalizeFilesystems(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
		im.rootfsMount = "/mnt/rootfs"
		im.bootfsMount = "/mnt/boot"
		im.efifsMount = "/mnt/efi"

		err := im.FinalizeFilesystems()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if len(runner.Calls) != 2 {
			t.Fatalf("expected 2 fstrim calls, got %d", len(runner.Calls))
		}
		for _, c := range runner.Calls {
			if c.Name != "fstrim" {
				t.Errorf("expected fstrim, got %q", c.Name)
			}
		}
	})

	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.FinalizeFilesystems(); err == nil {
			t.Error("should error for empty rootfsMount")
		}
		im.rootfsMount = "/mnt/rootfs"
		if err := im.FinalizeFilesystems(); err == nil {
			t.Error("should error for empty bootfsMount")
		}
		im.bootfsMount = "/mnt/boot"
		if err := im.FinalizeFilesystems(); err == nil {
			t.Error("should error for empty efifsMount")
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
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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

// --- InstallBootloader Tests ---

func TestInstallBootloader(t *testing.T) {
	t.Run("EmptyOstreeDeployRootfs", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.devicePath = "/dev/sda"
		im.efifsMount = "/mnt/efi"
		im.bootfsMount = "/mnt/boot"
		err := im.InstallBootloader()
		if err == nil {
			t.Fatal("expected error for empty ostreeDeployRootfs")
		}
		if !strings.Contains(err.Error(), "rootfs not set") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("EmptyEfifsMount", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.devicePath = "/dev/sda"
		im.SetRootfs("/rootfs")
		im.bootfsMount = "/mnt/boot"
		err := im.InstallBootloader()
		if err == nil {
			t.Fatal("expected error for empty efifsMount")
		}
		if !strings.Contains(err.Error(), "efifsMount") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("EmptyBootfsMount", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.devicePath = "/dev/sda"
		im.SetRootfs("/rootfs")
		im.efifsMount = "/mnt/efi"
		err := im.InstallBootloader()
		if err == nil {
			t.Fatal("expected error for empty bootfsMount")
		}
		if !strings.Contains(err.Error(), "bootfsMount") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("EmptyDevicePath", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.SetRootfs("/rootfs")
		im.efifsMount = "/mnt/efi"
		im.bootfsMount = "/mnt/boot"
		err := im.InstallBootloader()
		if err == nil {
			t.Fatal("expected error for empty devicePath")
		}
		if !strings.Contains(err.Error(), "devicePath") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("ConfigErrors", func(t *testing.T) {
		// Missing EfiRoot config
		cfg := baseImageConfig()
		delete(cfg.Items, "Imager.EfiRoot")
		im := newTestImage(cfg, &cds.MockOstree{})
		im.devicePath = "/dev/sda"
		im.SetRootfs("/rootfs")
		im.efifsMount = "/mnt/efi"
		im.bootfsMount = "/mnt/boot"
		err := im.InstallBootloader()
		if err == nil {
			t.Fatal("expected error for missing EfiRoot config")
		}

		// Missing BootRoot config
		cfg2 := baseImageConfig()
		delete(cfg2.Items, "Imager.BootRoot")
		im2 := newTestImage(cfg2, &cds.MockOstree{})
		im2.devicePath = "/dev/sda"
		im2.SetRootfs("/rootfs")
		im2.efifsMount = "/mnt/efi"
		im2.bootfsMount = "/mnt/boot"
		err = im2.InstallBootloader()
		if err == nil {
			t.Fatal("expected error for missing BootRoot config")
		}

		// Missing OsName config
		cfg3 := baseImageConfig()
		delete(cfg3.Items, "matrixOS.OsName")
		im3 := newTestImage(cfg3, &cds.MockOstree{})
		im3.devicePath = "/dev/sda"
		im3.SetRootfs("/rootfs")
		im3.efifsMount = "/mnt/efi"
		im3.bootfsMount = "/mnt/boot"
		err = im3.InstallBootloader()
		if err == nil {
			t.Fatal("expected error for missing OsName config")
		}

		// Missing EfiExecutable config
		cfg4 := baseImageConfig()
		delete(cfg4.Items, "Imager.EfiExecutable")
		im4 := newTestImage(cfg4, &cds.MockOstree{})
		im4.devicePath = "/dev/sda"
		im4.SetRootfs("/rootfs")
		im4.efifsMount = "/mnt/efi"
		im4.bootfsMount = "/mnt/boot"
		err = im4.InstallBootloader()
		if err == nil {
			t.Fatal("expected error for missing EfiExecutable config")
		}
	})
}

// --- ShowImageTestInfo Tests ---

func TestShowImageTestInfo(t *testing.T) {
	im := newTestImage(baseImageConfig(), &cds.MockOstree{})
	// Should not panic with valid artifacts.
	im.ShowImageTestInfo([]string{"/tmp/test.img", "/tmp/test.img.xz"})
	// Should not panic with empty artifacts.
	im.ShowImageTestInfo(nil)
}

// --- ExtractPackageList Tests ---

func TestExtractPackageList(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		vdb := filepath.Join(tmpDir, "usr", "var-db-pkg")
		os.MkdirAll(filepath.Join(vdb, "sys-libs", "glibc-2.38"), 0755)
		os.MkdirAll(filepath.Join(vdb, "dev-libs", "openssl-3.0"), 0755)
		os.MkdirAll(filepath.Join(vdb, "app-misc", "screen-4.9"), 0755)

		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		_, err := im.ExtractPackageList()
		if err == nil {
			t.Error("should error for empty rootfs")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(cfg, &cds.MockOstree{})
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
		im := newTestImage(cfg, &cds.MockOstree{})
		im.SetRootfs("/tmp/rootfs")
		im.ref = "matrixos/amd64/gnome"

		err := im.SetupHooks()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("OstreeError", func(t *testing.T) {
		mo := &cds.MockOstree{RemoveFullErr: errors.New("ostree error")}
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImageWithRunner(cfg, &cds.MockOstree{}, runner)
		im.ref = "matrixos/amd64/gnome"
		im.imagePath = "/tmp/test.img"
		im.mode = ModeCreateImageFile

		err := im.TestImage()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("OstreeError", func(t *testing.T) {
		mo := &cds.MockOstree{RemoveFullErr: errors.New("ostree error")}
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

// --- cleanAndStripRef Tests ---

func TestCleanAndStripRef(t *testing.T) {
	t.Run("WithRemoteAndFull", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
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
		mo := &cds.MockOstree{RemoveFullErr: errors.New("ostree error")}
		im := newTestImage(baseImageConfig(), mo)
		im.ref = "ref"
		_, err := im.cleanAndStripRef()
		if err == nil {
			t.Error("should propagate ostree error")
		}
	})

	t.Run("EmptyAfterStrip", func(t *testing.T) {
		mo := &cds.MockOstree{RemoveFullResult: "", RemoveFullResultSet: true}
		im := newTestImage(baseImageConfig(), mo)
		im.ref = "ref"
		_, err := im.cleanAndStripRef()
		if err == nil {
			t.Error("should error for empty result after cleaning")
		}
	})
}

// --- SetupBootloaderConfig Tests ---

func TestSetupBootloaderConfig(t *testing.T) {
	t.Run("EmptyRef", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.SetRootfs("/rootfs")
		im.rootfsMount = "/sysroot"
		im.bootfsMount = "/boot"
		im.efifsMount = "/efi"
		err := im.SetupBootloaderConfig()
		if err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("OstreeError", func(t *testing.T) {
		mo := &cds.MockOstree{RemoveFullErr: errors.New("ostree error")}
		im := newTestImage(baseImageConfig(), mo)
		im.SetRootfs("/rootfs")
		im.ref = "ref"
		im.rootfsMount = "/sysroot"
		im.bootfsMount = "/boot"
		im.efifsMount = "/efi"
		err := im.SetupBootloaderConfig()
		if err == nil {
			t.Error("should propagate ostree error")
		}
	})

	t.Run("EmptyOtherParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.ref = "ref"
		im.rootfsMount = "/sysroot"
		im.bootfsMount = "/boot"
		im.efifsMount = "/efi"
		if err := im.SetupBootloaderConfig(); err == nil {
			t.Error("should error for empty rootfs")
		}
		im.SetRootfs("/rootfs")
		im.rootfsMount = ""
		if err := im.SetupBootloaderConfig(); err == nil {
			t.Error("should error for empty rootfsMount")
		}
		im.rootfsMount = "/sys"
		im.bootfsMount = ""
		if err := im.SetupBootloaderConfig(); err == nil {
			t.Error("should error for empty bootfsMount")
		}
		im.bootfsMount = "/boot"
		im.efifsMount = ""
		if err := im.SetupBootloaderConfig(); err == nil {
			t.Error("should error for empty efifsMount")
		}
		im.efifsMount = "/efi"
		if err := im.SetupBootloaderConfig(); err == nil {
			t.Error("should error for empty efiDevice")
		}
		im.efiDevice = "/dev/sda1"
		if err := im.SetupBootloaderConfig(); err == nil {
			t.Error("should error for empty bootDevice")
		}
	})
}

// --- SetupVmtestConfig Tests ---

func TestSetupVmtestConfig(t *testing.T) {
	t.Run("EmptyParam", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		err := im.SetupVmtestConfig()
		if err == nil {
			t.Error("should error for empty bootfsMount")
		}
	})

	t.Run("NoLoaderConf", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.bootfsMount = tmpDir
		err := im.SetupVmtestConfig()
		if err == nil {
			t.Error("should error when ostree boot config doesn't exist")
		}
	})

	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		loaderDir := filepath.Join(tmpDir, "loader", "entries")
		os.MkdirAll(loaderDir, 0755)
		confContent := "title matrixos\noptions root=UUID=xxx quiet splash rw\n"
		os.WriteFile(filepath.Join(loaderDir, "ostree-1.conf"), []byte(confContent), 0644)

		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.SetImagePath(filepath.Join(tmpDir, "test.img"))
		im.SetImageMode(ModeCreateImageFile)
		im.bootfsMount = tmpDir
		err := im.SetupVmtestConfig()
		if err != nil {
			t.Fatalf("error: %v", err)
		}

		vmtestCfg := filepath.Join(tmpDir, ".imager.vmtest", "entries", "ostree-1.conf")
		data, err := os.ReadFile(vmtestCfg)
		if err != nil {
			t.Fatalf("failed to read vmtest config: %v", err)
		}
		content := string(data)
		if strings.Contains(content, "splash") {
			t.Error("vmtest config should not contain 'splash'")
		}
		if !strings.Contains(content, "console=ttyS0,115200") {
			t.Error("vmtest config should contain console params")
		}
		if !strings.Contains(content, "systemd.log_color=0") {
			t.Error("vmtest config should contain systemd params")
		}
	})
}

// --- InstallSecurebootCerts Tests ---

func TestInstallSecurebootCerts(t *testing.T) {
	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.InstallSecurebootCerts(); err == nil {
			t.Error("should error for empty rootfs")
		}
		im.SetRootfs("/rootfs")
		if err := im.InstallSecurebootCerts(); err == nil {
			t.Error("should error for empty efifsMount")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.SetRootfs("/rootfs")
		im.efifsMount = "/efi"
		err := im.InstallSecurebootCerts()
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

// --- InstallMemtest Tests ---

func TestInstallMemtest(t *testing.T) {
	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.InstallMemtest(); err == nil {
			t.Error("should error for empty rootfs")
		}
	})

	t.Run("NoMemtest", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.SetRootfs(tmpDir)
		im.efifsMount = filepath.Join(tmpDir, "efimount")
		os.MkdirAll(filepath.Join(im.efifsMount, "EFI/BOOT"), 0755)
		err := im.InstallMemtest()
		if err != nil {
			t.Fatalf("should not error when memtest not found: %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		memtestDir := filepath.Join(tmpDir, "usr", "share", "memtest86+")
		os.MkdirAll(memtestDir, 0755)
		os.WriteFile(filepath.Join(memtestDir, "memtest.efi64"), []byte("EFI"), 0644)
		efiMount := filepath.Join(tmpDir, "efimount")
		efibootdir := filepath.Join(efiMount, "EFI/BOOT")
		os.MkdirAll(efibootdir, 0755)

		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		im.SetRootfs(tmpDir)
		im.efifsMount = efiMount
		err := im.InstallMemtest()
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		copied := filepath.Join(efibootdir, "memtest86plus.efi")
		if _, err := os.Stat(copied); os.IsNotExist(err) {
			t.Error("memtest86plus.efi should have been copied")
		}
	})
}

// --- ExecuteWithImageLock Tests ---

func TestExecuteWithImageLock(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"5"}
		im := newTestImage(cfg, &cds.MockOstree{})
		im.ref = "test/ref"

		called := false
		err := im.ExecuteWithImageLock(func() error {
			called = true
			return nil
		})
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if !called {
			t.Error("fn should have been called")
		}
	})

	t.Run("FnErrorPropagated", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"5"}
		im := newTestImage(cfg, &cds.MockOstree{})
		im.ref = "test/ref"

		fnErr := errors.New("fn failed")
		err := im.ExecuteWithImageLock(func() error {
			return fnErr
		})
		if err == nil {
			t.Fatal("expected error from fn")
		}
		if !errors.Is(err, fnErr) {
			t.Errorf("got error %v, want %v", err, fnErr)
		}
	})

	t.Run("EmptyRef", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		err := im.ExecuteWithImageLock(func() error { return nil })
		if err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("InvalidLockWaitSeconds", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"notanumber"}
		im := newTestImage(cfg, &cds.MockOstree{})
		im.ref = "test/ref"

		err := im.ExecuteWithImageLock(func() error { return nil })
		if err == nil {
			t.Error("should error for invalid lock wait seconds")
		}
		if !strings.Contains(err.Error(), "invalid lock wait seconds") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		im.ref = "test/ref"
		err := im.ExecuteWithImageLock(func() error { return nil })
		if err == nil {
			t.Error("should error from broken config")
		}
	})

	t.Run("LockIsExclusive", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"5"}
		im := newTestImage(cfg, &cds.MockOstree{})
		im.ref = "exclusive/ref"

		// Acquire the lock in the callback and verify a second goroutine blocks.
		started := make(chan struct{})
		proceed := make(chan struct{})
		done := make(chan error, 1)

		go func() {
			done <- im.ExecuteWithImageLock(func() error {
				close(started) // signal we hold the lock
				<-proceed      // wait until test says to release
				return nil
			})
		}()

		<-started // first goroutine holds the lock

		// Try to acquire the same lock with a very short timeout.
		cfg2 := baseImageConfig()
		cfg2.Items["Imager.LocksDir"] = []string{lockDir}
		cfg2.Items["Imager.LockWaitSeconds"] = []string{"1"}
		im2 := newTestImage(cfg2, &cds.MockOstree{})
		im2.ref = "exclusive/ref"

		err := im2.ExecuteWithImageLock(func() error {
			return nil
		})
		if err == nil {
			t.Error("second lock acquisition should have timed out")
		}
		if !strings.Contains(err.Error(), "timed out") {
			t.Errorf("expected timeout error, got: %v", err)
		}

		close(proceed) // release the first lock
		if err := <-done; err != nil {
			t.Fatalf("first goroutine errored: %v", err)
		}
	})

	t.Run("LockReleasedAfterFn", func(t *testing.T) {
		tmpDir := t.TempDir()
		lockDir := filepath.Join(tmpDir, "locks")
		cfg := baseImageConfig()
		cfg.Items["Imager.LocksDir"] = []string{lockDir}
		cfg.Items["Imager.LockWaitSeconds"] = []string{"5"}
		im := newTestImage(cfg, &cds.MockOstree{})
		im.ref = "release/ref"

		// First call acquires and releases the lock.
		err := im.ExecuteWithImageLock(func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("first call error: %v", err)
		}

		// Second call should succeed since the lock was released.
		err = im.ExecuteWithImageLock(func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("second call should succeed after lock release: %v", err)
		}
	})
}
