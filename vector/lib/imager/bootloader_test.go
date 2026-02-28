package imager

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
)

// --- InstallBootloader Tests ---

func TestInstallBootloader(t *testing.T) {
	t.Run("EmptyOstreeDeployRootfs", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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
		im := newTestImage(cfg, &ostree.MockOstree{})
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
		im2 := newTestImage(cfg2, &ostree.MockOstree{})
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
		im3 := newTestImage(cfg3, &ostree.MockOstree{})
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
		im4 := newTestImage(cfg4, &ostree.MockOstree{})
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

// --- SetupBootloaderConfig Tests ---

func TestSetupBootloaderConfig(t *testing.T) {
	t.Run("EmptyRef", func(t *testing.T) {
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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
		mo := &ostree.MockOstree{RemoveFullErr: errors.New("ostree error")}
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
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		err := im.SetupVmtestConfig()
		if err == nil {
			t.Error("should error for empty bootfsMount")
		}
	})

	t.Run("NoLoaderConf", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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

		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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
		im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
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
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
		if err := im.InstallMemtest(); err == nil {
			t.Error("should error for empty rootfs")
		}
	})

	t.Run("NoMemtest", func(t *testing.T) {
		tmpDir := t.TempDir()
		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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

		im := newTestImage(baseImageConfig(), &ostree.MockOstree{})
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
