package imager

import (
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
