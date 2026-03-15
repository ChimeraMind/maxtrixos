package imager

import (
	"errors"
	"fmt"
	"path/filepath"
)

// --- Config accessors ---
// Each method retrieves a single configuration value and validates it.

// configItem retrieves a non-empty config string or returns an error.
func (im *Imager) configItem(key string) (string, error) {
	v, err := im.cfg.GetItem(key)
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", fmt.Errorf("invalid %s", key)
	}
	return v, nil
}

func (im *Imager) ImagesDir() (string, error) {
	return im.configItem("Imager.ImagesDir")
}

func (im *Imager) MountDir() (string, error) {
	return im.configItem("Imager.MountDir")
}

func (im *Imager) ImageSize() (string, error) {
	return im.configItem("Imager.ImageSize")
}

func (im *Imager) EfiPartitionSize() (string, error) {
	return im.configItem("Imager.EfiPartitionSize")
}

func (im *Imager) BootPartitionSize() (string, error) {
	return im.configItem("Imager.BootPartitionSize")
}

func (im *Imager) Compressor() (string, error) {
	return im.configItem("Imager.Compressor")
}

func (im *Imager) EspPartitionType() (string, error) {
	return im.configItem("Imager.EspPartitionType")
}

func (im *Imager) BootPartitionType() (string, error) {
	return im.configItem("Imager.BootPartitionType")
}

func (im *Imager) RootPartitionType() (string, error) {
	return im.configItem("Imager.RootPartitionType")
}

func (im *Imager) OsName() (string, error) {
	return im.configItem("matrixOS.OsName")
}

func (im *Imager) BootRoot() (string, error) {
	return im.configItem("Imager.BootRoot")
}

func (im *Imager) EfiRoot() (string, error) {
	return im.configItem("Imager.EfiRoot")
}

func (im *Imager) RelativeEfiBootPath() (string, error) {
	return im.configItem("Imager.RelativeEfiBootPath")
}

func (im *Imager) EfiExecutable() (string, error) {
	return im.configItem("Imager.EfiExecutable")
}

func (im *Imager) EfiCertificateFileName() (string, error) {
	return im.configItem("Imager.EfiCertificateFileName")
}

func (im *Imager) EfiCertificateFileNameDer() (string, error) {
	return im.configItem("Imager.EfiCertificateFileNameDer")
}

func (im *Imager) EfiCertificateFileNameKek() (string, error) {
	return im.configItem("Imager.EfiCertificateFileNameKek")
}

func (im *Imager) EfiCertificateFileNameKekDer() (string, error) {
	return im.configItem("Imager.EfiCertificateFileNameKekDer")
}

func (im *Imager) ReadOnlyVdb() (string, error) {
	return im.configItem("Releaser.ReadOnlyVdb")
}

func (im *Imager) DevDir() (string, error) {
	return im.configItem("matrixOS.Root")
}

func (im *Imager) HooksDir() (string, error) {
	return im.configItem("Imager.HooksDir")
}

func (im *Imager) LockDir() (string, error) {
	return im.configItem("Imager.LocksDir")
}

func (im *Imager) LockWaitSeconds() (string, error) {
	return im.configItem("Imager.LockWaitSeconds")
}

func (im *Imager) BuildMetadataFile() (string, error) {
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

func (im *Imager) CreateQcow2() (bool, error) {
	return im.cfg.GetBool("Imager.CreateQcow2")
}

func (im *Imager) Productionize() (bool, error) {
	return im.cfg.GetBool("Imager.Productionize")
}

func (im *Imager) ImageTests() (bool, error) {
	return im.cfg.GetBool("Imager.ImageTests")
}
