package imager

import (
	"errors"
	"path/filepath"
)

// --- Config accessors ---
// Each method retrieves a single configuration value and validates it.

// ImagesDir returns the directory where generated images are stored.
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

// MountDir returns the directory where image partitions are mounted.
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

// ImageSize returns the configured image size (e.g. "32G").
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

// EfiPartitionSize returns the configured EFI partition size (e.g. "200M").
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

// BootPartitionSize returns the configured boot partition size (e.g. "1G").
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

// Compressor returns the configured compressor command string (e.g. "xz -f -0 -T0").
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

// EspPartitionType returns the ESP partition type GUID.
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

// BootPartitionType returns the boot partition type GUID.
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

// RootPartitionType returns the root partition type GUID.
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

// OsName returns the OS name.
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

// BootRoot returns the boot filesystem mount point (e.g. "/boot").
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

// EfiRoot returns the EFI filesystem mount point (e.g. "/efi").
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

// RelativeEfiBootPath returns the path relative to EfiRoot where the standard ESP
// boot directory is (e.g. "efi/BOOT").
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

// EfiExecutable returns the EFI executable name (e.g. "BOOTX64.EFI").
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

// EfiCertificateFileName returns the SecureBoot PEM certificate file name.
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

// EfiCertificateFileNameDer returns the SecureBoot DER certificate file name.
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

// EfiCertificateFileNameKek returns the SecureBoot KEK PEM certificate file name.
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

// EfiCertificateFileNameKekDer returns the SecureBoot KEK DER certificate file name.
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

// ReadOnlyVdb returns the read-only VDB path (e.g. "/usr/var-db-pkg").
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

// DevDir returns the matrixOS dev directory (Root).
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

// LockDir returns the configured image lock directory.
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

// LockWaitSeconds returns the configured lock wait timeout in seconds.
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

// BuildMetadataFile returns the build metadata file path (combining
// ChrootMetadataDir and ChrootMetadataDirBuildFileName).
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

// CreateQcow2 returns whether a QCOW2 image should be created in addition to the raw .img file.
func (im *Image) CreateQcow2() (bool, error) {
	v, err := im.cfg.GetBool("Imager.CreateQcow2")
	if err != nil {
		return false, err
	}
	return v, nil
}

// Productionize returns whether productionization steps should be executed after image creation.
func (im *Image) Productionize() (bool, error) {
	v, err := im.cfg.GetBool("Imager.Productionize")
	if err != nil {
		return false, err
	}
	return v, nil
}

// ImageTests returns whether integration tests should be run after image creation.
func (im *Image) ImageTests() (bool, error) {
	v, err := im.cfg.GetBool("Imager.ImageTests")
	if err != nil {
		return false, err
	}
	return v, nil
}
