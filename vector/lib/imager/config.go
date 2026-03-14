package imager

import (
	"errors"
	"path/filepath"
)

// --- Config accessors ---
// Each method retrieves a single configuration value and validates it.

func (im *Image) ImagesDir() (string, error) {
	v, err := im.cfg.GetItem("Imager.ImagesDir")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.ImagesDir")
	}
	return v, nil
}

func (im *Image) MountDir() (string, error) {
	v, err := im.cfg.GetItem("Imager.MountDir")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.MountDir")
	}
	return v, nil
}

func (im *Image) ImageSize() (string, error) {
	v, err := im.cfg.GetItem("Imager.ImageSize")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.ImageSize")
	}
	return v, nil
}

func (im *Image) EfiPartitionSize() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiPartitionSize")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiPartitionSize")
	}
	return v, nil
}

func (im *Image) BootPartitionSize() (string, error) {
	v, err := im.cfg.GetItem("Imager.BootPartitionSize")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.BootPartitionSize")
	}
	return v, nil
}

func (im *Image) Compressor() (string, error) {
	v, err := im.cfg.GetItem("Imager.Compressor")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.Compressor")
	}
	return v, nil
}

func (im *Image) EspPartitionType() (string, error) {
	v, err := im.cfg.GetItem("Imager.EspPartitionType")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EspPartitionType")
	}
	return v, nil
}

func (im *Image) BootPartitionType() (string, error) {
	v, err := im.cfg.GetItem("Imager.BootPartitionType")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.BootPartitionType")
	}
	return v, nil
}

func (im *Image) RootPartitionType() (string, error) {
	v, err := im.cfg.GetItem("Imager.RootPartitionType")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.RootPartitionType")
	}
	return v, nil
}

func (im *Image) OsName() (string, error) {
	v, err := im.cfg.GetItem("matrixOS.OsName")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid matrixOS.OsName")
	}
	return v, nil
}

func (im *Image) BootRoot() (string, error) {
	v, err := im.cfg.GetItem("Imager.BootRoot")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.BootRoot")
	}
	return v, nil
}

func (im *Image) EfiRoot() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiRoot")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiRoot")
	}
	return v, nil
}

func (im *Image) RelativeEfiBootPath() (string, error) {
	v, err := im.cfg.GetItem("Imager.RelativeEfiBootPath")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.RelativeEfiBootPath")
	}
	return v, nil
}

func (im *Image) EfiExecutable() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiExecutable")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiExecutable")
	}
	return v, nil
}

func (im *Image) EfiCertificateFileName() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiCertificateFileName")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiCertificateFileName")
	}
	return v, nil
}

func (im *Image) EfiCertificateFileNameDer() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiCertificateFileNameDer")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiCertificateFileNameDer")
	}
	return v, nil
}

func (im *Image) EfiCertificateFileNameKek() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiCertificateFileNameKek")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiCertificateFileNameKek")
	}
	return v, nil
}

func (im *Image) EfiCertificateFileNameKekDer() (string, error) {
	v, err := im.cfg.GetItem("Imager.EfiCertificateFileNameKekDer")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.EfiCertificateFileNameKekDer")
	}
	return v, nil
}

func (im *Image) ReadOnlyVdb() (string, error) {
	v, err := im.cfg.GetItem("Releaser.ReadOnlyVdb")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Releaser.ReadOnlyVdb")
	}
	return v, nil
}

func (im *Image) DevDir() (string, error) {
	v, err := im.cfg.GetItem("matrixOS.Root")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid matrixOS.Root")
	}
	return v, nil
}

func (im *Image) LockDir() (string, error) {
	v, err := im.cfg.GetItem("Imager.LocksDir")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.LocksDir")
	}
	return v, nil
}

func (im *Image) LockWaitSeconds() (string, error) {
	v, err := im.cfg.GetItem("Imager.LockWaitSeconds")
	if err != nil {
		return "", err
	}
	if v == "" {
		return "", errors.New("invalid Imager.LockWaitSeconds")
	}
	return v, nil
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
	v, err := im.cfg.GetBool("Imager.CreateQcow2")
	if err != nil {
		return false, err
	}
	return v, nil
}

func (im *Image) Productionize() (bool, error) {
	v, err := im.cfg.GetBool("Imager.Productionize")
	if err != nil {
		return false, err
	}
	return v, nil
}

func (im *Image) ImageTests() (bool, error) {
	v, err := im.cfg.GetBool("Imager.ImageTests")
	if err != nil {
		return false, err
	}
	return v, nil
}
