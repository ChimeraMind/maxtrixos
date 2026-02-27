package imager

import (
	"errors"
	"path/filepath"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/runner"
)

// --- Config Accessor Tests ---

func TestConfigAccessors(t *testing.T) {
	cfg := baseImageConfig()
	im := newTestImage(cfg, &ostree.MockOstree{})

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
			im := newTestImage(cfg, &ostree.MockOstree{})
			_, err := tt.fn(im)
			if err == nil {
				t.Errorf("%s() should return error for empty value", tt.name)
			}
		})
	}
}

func TestConfigAccessorsConfigError(t *testing.T) {
	ec := &config.ErrConfig{Err: errors.New("cfg error")}
	im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
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
		im := newTestImage(cfg, &ostree.MockOstree{})
		result, err := im.BuildMetadataFile()
		if err != nil {
			t.Fatalf("BuildMetadataFile() error: %v", err)
		}
		expected := filepath.Join("/etc/matrixos", "build.txt")
		if result != expected {
			t.Errorf("BuildMetadataFile() = %q, want %q", result, expected)
		}
	})

	t.Run("EmptyDir", func(t *testing.T) {
		cfg := baseImageConfig()
		cfg.Items["Seeder.ChrootMetadataDir"] = []string{""}
		im := newTestImage(cfg, &ostree.MockOstree{})
		_, err := im.BuildMetadataFile()
		if err == nil {
			t.Error("should error for empty metadata dir")
		}
	})

	t.Run("EmptyFileName", func(t *testing.T) {
		cfg := baseImageConfig()
		cfg.Items["Seeder.ChrootMetadataDirBuildFileName"] = []string{""}
		im := newTestImage(cfg, &ostree.MockOstree{})
		_, err := im.BuildMetadataFile()
		if err == nil {
			t.Error("should error for empty build file name")
		}
	})

	t.Run("ConfigError", func(t *testing.T) {
		ec := &config.ErrConfig{Err: errors.New("cfg error")}
		im, _ := NewImage(ec, &ostree.MockOstree{}, filesystems.DefaultMockFsenc(), nil)
		_, err := im.BuildMetadataFile()
		if err == nil {
			t.Error("should error from broken config")
		}
	})
}
