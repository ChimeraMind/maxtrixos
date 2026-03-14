package imager

import (
	"bytes"
	"errors"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
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

func newTestImage(cfg *config.MockConfig, ostree *ostree.MockOstree) *Image {
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
		im, err := NewImage(baseImageConfig(), &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		if err != nil {
			t.Fatalf("NewImage() error: %v", err)
		}
		if im == nil {
			t.Fatal("NewImage() returned nil")
		}
	})

	t.Run("NilConfig", func(t *testing.T) {
		_, err := NewImage(nil, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
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
		_, err := NewImage(baseImageConfig(), &ostree.MockOstree{}, nil, nil)
		if err == nil {
			t.Fatal("expected error for nil fsenc")
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

	t.Run("FlashToDeviceNoDevice", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		// devicePath is empty.
		err := im.SetImageMode(ModeFlashToDevice)
		if err == nil {
			t.Error("should error for empty devicePath")
		}
	})

	t.Run("CreateImageFileSuccess", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		im.imagePath = "/tmp/test.img"
		err := im.SetImageMode(ModeCreateImageFile)
		if err != nil {
			t.Fatalf("SetImageMode() error: %v", err)
		}
		if im.ImageMode() != ModeCreateImageFile {
			t.Error("mode should be ModeCreateImageFile")
		}
	})

	t.Run("CreateImageFileNoPath", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		// imagePath is empty.
		err := im.SetImageMode(ModeCreateImageFile)
		if err == nil {
			t.Error("should error for empty imagePath")
		}
	})

	t.Run("InvalidMode", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		err := im.SetImageMode(ImageMode(99))
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

	t.Run("WithEncryption", func(t *testing.T) {
		fsenc := &filesystems.MockFsenc{EncryptionEnabled_: true}
		opts := &NewImageOptions{Mode: ModeCreateImageFile}
		im, err := NewImage(baseImageConfig(), &ostree.MockOstree{}, fsenc, opts)
		if err != nil {
			t.Fatalf("NewImage() error: %v", err)
		}
		if !im.encrypted {
			t.Error("expected encrypted = true")
		}
	})

	t.Run("EncryptionCheckError", func(t *testing.T) {
		fsenc := &filesystems.MockFsenc{EncryptionEnabledErr: errImageTest}
		opts := &NewImageOptions{} // opts must be non-nil to trigger encryption check assignment
		_, err := NewImage(baseImageConfig(), &ostree.MockOstree{}, fsenc, opts)
		if err == nil {
			t.Error("expected error from EncryptionEnabled")
		}
	})
}

// --- Device Setter/Getter Tests ---

func TestImageDeviceSettersGetters(t *testing.T) {
	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})

	im.SetEfiDevice("/dev/sda1")
	if im.EfiDevice() != "/dev/sda1" {
		t.Errorf("EfiDevice() = %q", im.EfiDevice())
	}

	im.SetBootDevice("/dev/sda2")
	if im.BootDevice() != "/dev/sda2" {
		t.Errorf("BootDevice() = %q", im.BootDevice())
	}

	im.SetRootDevice("/dev/sda3")
	if im.RootDevice() != "/dev/sda3" {
		t.Errorf("RootDevice() = %q", im.RootDevice())
	}

	im.SetDevicePath("/dev/sda")
	if im.DevicePath() != "/dev/sda" {
		t.Errorf("DevicePath() = %q", im.DevicePath())
	}

	im.SetImagePath("/tmp/test.img")
	if im.ImagePath() != "/tmp/test.img" {
		t.Errorf("ImagePath() = %q", im.ImagePath())
	}

	im.SetRootfs("/rootfs")
	if im.Rootfs() != "/rootfs" {
		t.Errorf("Rootfs() = %q", im.Rootfs())
	}

	im.SetRef("matrixos/amd64/gnome")
	if im.Ref() != "matrixos/amd64/gnome" {
		t.Errorf("Ref() = %q", im.Ref())
	}
}

// --- Mount Point Getter Tests ---

func TestImageMountGetters(t *testing.T) {
	im := newTestImage(baseImageConfig(), &ostree.MockOstree{})

	// Initially empty.
	if im.EfifsMount() != "" {
		t.Errorf("EfifsMount() should be empty, got %q", im.EfifsMount())
	}
	if im.BootfsMount() != "" {
		t.Errorf("BootfsMount() should be empty, got %q", im.BootfsMount())
	}
	if im.RootfsMount() != "" {
		t.Errorf("RootfsMount() should be empty, got %q", im.RootfsMount())
	}
}

// --- ShowFinalFilesystemInfo Tests ---

func TestShowFinalFilesystemInfo(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(baseImageConfig(), &cds.MockOstree{}, runner)

		err := im.ShowFinalFilesystemInfo("/dev/loop0", "/mnt/boot", "/mnt/efi")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		// find (boot) + find (efi) + blkid = 3 calls.
		if len(runner.Calls) != 3 {
			t.Fatalf("expected 3 runner calls, got %d", len(runner.Calls))
		}
	})

	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.ShowFinalFilesystemInfo("", "/a", "/b"); err == nil {
			t.Error("should error for empty blockDevice")
		}
		if err := im.ShowFinalFilesystemInfo("/dev/x", "", "/b"); err == nil {
			t.Error("should error for empty mountBootfs")
		}
		if err := im.ShowFinalFilesystemInfo("/dev/x", "/a", ""); err == nil {
			t.Error("should error for empty mountEfifs")
		}
	})
}

// --- ShowTestInfo Tests ---

func TestShowTestInfo(t *testing.T) {
	im := newTestImage(baseImageConfig(), &cds.MockOstree{})
	// Should not panic with valid artifacts.
	im.ShowTestInfo([]string{"/tmp/test.img", "/tmp/test.img.xz"})
	// Should not panic with empty artifacts.
	im.ShowTestInfo(nil)
}

// --- PackageList Tests ---

func TestPackageList(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		vdb := filepath.Join(tmpDir, "usr", "var-db-pkg")
		os.MkdirAll(filepath.Join(vdb, "sys-libs", "glibc-2.38"), 0755)
		os.MkdirAll(filepath.Join(vdb, "dev-libs", "openssl-3.0"), 0755)
		os.MkdirAll(filepath.Join(vdb, "app-misc", "screen-4.9"), 0755)

		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		result, err := im.PackageList(tmpDir)
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
		result, err := im.PackageList(tmpDir)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil for non-existent VDB, got %v", result)
		}
	})

	t.Run("Empty", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		_, err := im.PackageList("")
		if err == nil {
			t.Error("should error for empty rootfs")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{})
		_, err := im.PackageList("/tmp/rootfs")
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

// --- SetupHooks Tests ---

func TestSetupHooks(t *testing.T) {
	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.SetupHooks("", "ref"); err == nil {
			t.Error("should error for empty ostreeDeployRootfs")
		}
		if err := im.SetupHooks("/tmp/rootfs", ""); err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("NoHooksDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := baseImageConfig()
		cfg.Items["matrixOS.Root"] = []string{tmpDir}
		im := newTestImage(cfg, &cds.MockOstree{})
		// Should return nil when hooks dir doesn't exist.
		err := im.SetupHooks("/tmp/rootfs", "matrixos/amd64/gnome")
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

		err := im.SetupHooks("/tmp/rootfs", "matrixos/amd64/gnome")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("OstreeError", func(t *testing.T) {
		mo := &cds.MockOstree{RemoveFullErr: errors.New("ostree error")}
		im := newTestImage(baseImageConfig(), mo)
		err := im.SetupHooks("/tmp/rootfs", "ref")
		if err == nil {
			t.Error("should propagate ostree error")
		}
	})
}

// --- TestImage Tests ---

func TestTestImageMethod(t *testing.T) {
	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.TestImage("", "ref"); err == nil {
			t.Error("should error for empty imagePath")
		}
		if err := im.TestImage("/tmp/x.img", ""); err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("NoTestDir", func(t *testing.T) {
		tmpDir := t.TempDir()
		cfg := baseImageConfig()
		cfg.Items["matrixOS.Root"] = []string{tmpDir}
		runner := runner.NewMockRunner()
		im := newTestImageWithRunner(cfg, &cds.MockOstree{}, runner)

		err := im.TestImage("/tmp/test.img", "matrixos/amd64/gnome")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("OstreeError", func(t *testing.T) {
		mo := &cds.MockOstree{RemoveFullErr: errors.New("ostree error")}
		im := newTestImage(baseImageConfig(), mo)
		err := im.TestImage("/tmp/x.img", "ref")
		if err == nil {
			t.Error("should propagate ostree error")
		}
	})
}

// --- cleanAndStripRef Tests ---

func TestCleanAndStripRef(t *testing.T) {
	t.Run("WithRemoteAndFull", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		result, err := im.cleanAndStripRef("origin:matrixos/amd64/gnome-full")
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		if result != "matrixos/amd64/gnome" {
			t.Errorf("got %q, want matrixos/amd64/gnome", result)
		}
	})

	t.Run("WithoutSuffix", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		result, err := im.cleanAndStripRef("matrixos/amd64/gnome")
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
		_, err := im.cleanAndStripRef("ref")
		if err == nil {
			t.Error("should propagate ostree error")
		}
	})

	t.Run("EmptyAfterStrip", func(t *testing.T) {
		mo := &cds.MockOstree{RemoveFullResult: "", RemoveFullResultSet: true}
		im := newTestImage(baseImageConfig(), mo)
		_, err := im.cleanAndStripRef("ref")
		if err == nil {
			t.Error("should error for empty result after cleaning")
		}
	})
}

// --- SetupBootloaderConfig Tests ---

func TestSetupBootloaderConfig(t *testing.T) {
	t.Run("EmptyRef", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		err := im.SetupBootloaderConfig("", "/rootfs", "/sysroot", "/boot", "/efiboot", "uuid1", "uuid2")
		if err == nil {
			t.Error("should error for empty ref")
		}
	})

	t.Run("OstreeError", func(t *testing.T) {
		mo := &cds.MockOstree{RemoveFullErr: errors.New("ostree error")}
		im := newTestImage(baseImageConfig(), mo)
		err := im.SetupBootloaderConfig("ref", "/rootfs", "/sysroot", "/boot", "/efiboot", "uuid1", "uuid2")
		if err == nil {
			t.Error("should propagate ostree error")
		}
	})

	t.Run("EmptyOtherParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.SetupBootloaderConfig("ref", "", "/sysroot", "/boot", "/efi", "u1", "u2"); err == nil {
			t.Error("should error for empty ostreeDeployRootfs")
		}
		if err := im.SetupBootloaderConfig("ref", "/rootfs", "", "/boot", "/efi", "u1", "u2"); err == nil {
			t.Error("should error for empty sysroot")
		}
		if err := im.SetupBootloaderConfig("ref", "/rootfs", "/sys", "", "/efi", "u1", "u2"); err == nil {
			t.Error("should error for empty bootdir")
		}
		if err := im.SetupBootloaderConfig("ref", "/rootfs", "/sys", "/boot", "", "u1", "u2"); err == nil {
			t.Error("should error for empty efibootdir")
		}
		if err := im.SetupBootloaderConfig("ref", "/rootfs", "/sys", "/boot", "/efi", "", "u2"); err == nil {
			t.Error("should error for empty efiUUID")
		}
		if err := im.SetupBootloaderConfig("ref", "/rootfs", "/sys", "/boot", "/efi", "u1", ""); err == nil {
			t.Error("should error for empty bootUUID")
		}
	})
}

// --- SetupVmtestConfig Tests ---

func TestSetupVmtestConfig(t *testing.T) {
	t.Run("EmptyParam", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		err := im.SetupVmtestConfig("")
		if err == nil {
			t.Error("should error for empty bootdir")
		}
	})

	t.Run("NoLoaderConf", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		err := im.SetupVmtestConfig(tmpDir)
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
		err := im.SetupVmtestConfig(tmpDir)
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
		if err := im.InstallSecurebootCerts("", "/efi", "/efiboot"); err == nil {
			t.Error("should error for empty ostreeDeployRootfs")
		}
		if err := im.InstallSecurebootCerts("/rootfs", "", "/efiboot"); err == nil {
			t.Error("should error for empty mountEfifs")
		}
		if err := im.InstallSecurebootCerts("/rootfs", "/efi", ""); err == nil {
			t.Error("should error for empty efibootdir")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{})
		err := im.InstallSecurebootCerts("/rootfs", "/efi", "/efiboot")
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}

// --- InstallMemtest Tests ---

func TestInstallMemtest(t *testing.T) {
	t.Run("EmptyParams", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		if err := im.InstallMemtest("", "/efiboot"); err == nil {
			t.Error("should error for empty ostreeDeployRootfs")
		}
		if err := im.InstallMemtest("/rootfs", ""); err == nil {
			t.Error("should error for empty efibootdir")
		}
	})

	t.Run("NoMemtest", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		err := im.InstallMemtest(tmpDir, filepath.Join(tmpDir, "efiboot"))
		if err != nil {
			t.Fatalf("should not error when memtest not found: %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		memtestDir := filepath.Join(tmpDir, "usr", "share", "memtest86+")
		os.MkdirAll(memtestDir, 0755)
		os.WriteFile(filepath.Join(memtestDir, "memtest.efi64"), []byte("EFI"), 0644)
		efibootdir := filepath.Join(tmpDir, "efiboot")
		os.MkdirAll(efibootdir, 0755)

		im := newTestImage(baseImageConfig(), &cds.MockOstree{})
		err := im.InstallMemtest(tmpDir, efibootdir)
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

		called := false
		err := im.ExecuteWithImageLock("test/ref", func() error {
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

		fnErr := errors.New("fn failed")
		err := im.ExecuteWithImageLock("test/ref", func() error {
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
		err := im.ExecuteWithImageLock("", func() error { return nil })
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

		err := im.ExecuteWithImageLock("test/ref", func() error { return nil })
		if err == nil {
			t.Error("should error for invalid lock wait seconds")
		}
		if !strings.Contains(err.Error(), "invalid lock wait seconds") {
			t.Errorf("unexpected error message: %v", err)
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &cds.MockOstree{})
		err := im.ExecuteWithImageLock("test/ref", func() error { return nil })
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

		// Acquire the lock in the callback and verify a second goroutine blocks.
		started := make(chan struct{})
		proceed := make(chan struct{})
		done := make(chan error, 1)

		go func() {
			done <- im.ExecuteWithImageLock("exclusive/ref", func() error {
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

		err := im2.ExecuteWithImageLock("exclusive/ref", func() error {
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

		// First call acquires and releases the lock.
		err := im.ExecuteWithImageLock("release/ref", func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("first call error: %v", err)
		}

		// Second call should succeed since the lock was released.
		err = im.ExecuteWithImageLock("release/ref", func() error {
			return nil
		})
		if err != nil {
			t.Fatalf("second call should succeed after lock release: %v", err)
		}
	})
}

// --- copyFile Tests ---

func TestCopyFile(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()
		src := filepath.Join(tmpDir, "src.txt")
		dst := filepath.Join(tmpDir, "dst.txt")
		os.WriteFile(src, []byte("hello world"), 0644)

		err := copyFile(src, dst)
		if err != nil {
			t.Fatalf("error: %v", err)
		}
		data, _ := os.ReadFile(dst)
		if string(data) != "hello world" {
			t.Errorf("got %q, want 'hello world'", string(data))
		}
	})

	t.Run("SrcNotFound", func(t *testing.T) {
		err := copyFile("/nonexistent", "/tmp/dst")
		if err == nil {
			t.Error("should error for nonexistent source")
		}
	})
}
