package imager

import (
	"errors"
	"fmt"
	"path/filepath"
)

// --- Config accessors ---
// Each method retrieves a single configuration value and validates it.

// configItem retrieves a non-empty config string or returns an error.
func (im *Image) configItem(key string) (string, error) {
	v, err := im.cfg.GetItem(key)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", fmt.Errorf("invalid %s", key)
	}
	return v, nil
}

func (im *Image) ImagesDir() (string, error) {
	return im.configItem("Imager.ImagesDir")
}

func (im *Image) MountDir() (string, error) {
	return im.configItem("Imager.MountDir")
}

func (im *Image) ImageSize() (string, error) {
	return im.configItem("Imager.ImageSize")
}

func (im *Image) EfiPartitionSize() (string, error) {
	return im.configItem("Imager.EfiPartitionSize")
}

func (im *Image) BootPartitionSize() (string, error) {
	return im.configItem("Imager.BootPartitionSize")
}

func (im *Image) Compressor() (string, error) {
	return im.configItem("Imager.Compressor")
}

func (im *Image) EspPartitionType() (string, error) {
	return im.configItem("Imager.EspPartitionType")
}

func (im *Image) BootPartitionType() (string, error) {
	return im.configItem("Imager.BootPartitionType")
}

func (im *Image) RootPartitionType() (string, error) {
	return im.configItem("Imager.RootPartitionType")
}

func (im *Image) OsName() (string, error) {
	return im.configItem("matrixOS.OsName")
}

func (im *Image) BootRoot() (string, error) {
	return im.configItem("Imager.BootRoot")
}

func (im *Image) EfiRoot() (string, error) {
	return im.configItem("Imager.EfiRoot")
}

func (im *Image) RelativeEfiBootPath() (string, error) {
	return im.configItem("Imager.RelativeEfiBootPath")
}

func (im *Image) EfiExecutable() (string, error) {
	return im.configItem("Imager.EfiExecutable")
}

func (im *Image) EfiCertificateFileName() (string, error) {
	return im.configItem("Imager.EfiCertificateFileName")
}

func (im *Image) EfiCertificateFileNameDer() (string, error) {
	return im.configItem("Imager.EfiCertificateFileNameDer")
}

func (im *Image) EfiCertificateFileNameKek() (string, error) {
	return im.configItem("Imager.EfiCertificateFileNameKek")
}

func (im *Image) EfiCertificateFileNameKekDer() (string, error) {
	return im.configItem("Imager.EfiCertificateFileNameKekDer")
}

func (im *Image) ReadOnlyVdb() (string, error) {
	return im.configItem("Releaser.ReadOnlyVdb")
}

func (im *Image) DevDir() (string, error) {
	return im.configItem("matrixOS.Root")
}

func (im *Image) HooksDir() (string, error) {
	return im.configItem("Imager.HooksDir")
}

func (im *Image) LockDir() (string, error) {
	return im.configItem("Imager.LocksDir")
}

func (im *Image) LockWaitSeconds() (string, error) {
	return im.configItem("Imager.LockWaitSeconds")
}

func (im *Image) BuildMetadataFile() (string, error) {
	metadataDir, err := im.cfg.GetItem("Seeder.ChrootMetadataDir")
	if err != nil {
		return "", err
	}
	if metadataDir == "" {
		return "", errors.New("invalid Seeder.ChrootMetadataDir")
	}
	buildFileName, err := im.cfg.GetItem("Seeder.ChrootMetadataDirBuildFileName")
	if err != nil {
		return "", err
	}
	if buildFileName == "" {
		return "", errors.New("invalid Seeder.ChrootMetadataDirBuildFileName")
	}
	return filepath.Join(metadataDir, buildFileName), nil
}

func (im *Image) CreateQcow2() (bool, error) {
	return im.cfg.GetBool("Imager.CreateQcow2")
}

func (im *Image) Productionize() (bool, error) {
	return im.cfg.GetBool("Imager.Productionize")
}

func (im *Image) ImageTests() (bool, error) {
	return im.cfg.GetBool("Imager.ImageTests")
}
