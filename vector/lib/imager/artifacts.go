package imager

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/filesystems"
)

// --- Helpers ---

// buildImagePath builds the full image file path from a suffix.
func (im *Image) buildImagePath(suffix string) (string, error) {
	outDir, err := im.ImagesDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(outDir, suffix), nil
}

// cleanAndStripRef cleans a remote prefix and removes the -full suffix from the stored ref.
func (im *Image) cleanAndStripRef() (string, error) {
	if im.ref == "" {
		return "", errors.New("missing ref, set Ref in NewImageOptions")
	}
	ref := ostree.CleanRemoteFromRef(im.ref)
	stripped, err := im.ostree.RemoveFullFromBranch(ref)
	if err != nil {
		return "", err
	}
	if stripped == "" {
		return "", errors.New("invalid ref parameter after cleaning")
	}
	return stripped, nil
}

// refToSuffix converts slashes in a ref to underscores for use in file names.
func refToSuffix(ref string) string {
	return strings.ReplaceAll(ref, "/", "_")
}

// validateImageModeForCreation checks that the image mode is set to
// ModeCreateImageFile and that the imagePath is not empty.
func (im *Image) validateImageModeForCreation() error {
	if im.mode != ModeCreateImageFile {
		return errors.New("invalid image creation mode")
	}
	if im.imagePath == "" {
		return errors.New("missing imagePath parameter")
	}
	return nil
}

// --- Operations ---

func (im *Image) extractSeedName(data []byte) (string, error) {
	// Extract version from SEED_NAME= line.
	var releaseVersion string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "SEED_NAME=") {
			continue
		}

		seedName := strings.TrimPrefix(line, "SEED_NAME=")
		// Version is the part after the last '-'.
		if idx := strings.LastIndex(seedName, "-"); idx >= 0 {
			releaseVersion = seedName[idx+1:]
			im.Print("Extracted release version: %s\n", releaseVersion)
		} else {
			im.PrintWarning("WARNING: SEED_NAME= value has no '-' separator\n")
		}
		break

	}
	if scanner.Err() != nil {
		return releaseVersion, fmt.Errorf("failed to scan build metadata file: %w", scanner.Err())
	}
	return releaseVersion, nil
}

// ExtractReleaseVersion extracts or generates a release version string for an image.
// It attempts to read a build metadata file from the rootfs for the version;
// if unavailable, falls back to the current date (YYYYMMDD).
func (im *Image) ExtractReleaseVersion() (string, error) {
	if im.rootfs == "" {
		return "", errors.New("rootfs not set, call SetRootfs first")
	}

	releaseVersion := time.Now().Format("20060102")
	metadataRelPath, err := im.BuildMetadataFile()
	if err != nil {
		return "", fmt.Errorf(
			"failed to determine build metadata file path: %w", err)
	}
	metadataFile := filepath.Join(im.rootfs, metadataRelPath)

	if filesystems.FileExists(metadataFile) {
		im.Print("Build metadata:\n")
		data, err := os.ReadFile(metadataFile)
		if err != nil {
			return "", fmt.Errorf(
				"failed to read build metadata file %s: %w", metadataFile, err)
		}
		im.Print("%s", string(data))

		releaseVersion, err = im.extractSeedName(data)
		if err != nil {
			return "", fmt.Errorf(
				"failed to extract release version from build metadata: %w", err)
		}
	} else {
		im.PrintWarning(
			"WARNING! Build metadata file not found: %s\n", metadataFile,
		)
	}

	return releaseVersion, nil
}

// BuildImagePath returns the image file path for the stored ostree ref.
func (im *Image) BuildImagePath() (string, error) {
	if im.ref == "" {
		return "", errors.New("missing ref, set Ref in NewImageOptions")
	}
	ref := ostree.CleanRemoteFromRef(im.ref)
	suffix := refToSuffix(ref) + ".img"
	return im.buildImagePath(suffix)
}

// BuildImagePathWithReleaseVersion returns the image file path with an embedded release version.
func (im *Image) BuildImagePathWithReleaseVersion(releaseVersion string) (string, error) {
	if im.ref == "" {
		return "", errors.New("missing ref, set Ref in NewImageOptions")
	}
	if releaseVersion == "" {
		return "", errors.New("missing releaseVersion parameter")
	}
	ref := ostree.CleanRemoteFromRef(im.ref)
	suffix := refToSuffix(ref) + "-" + releaseVersion + ".img"
	return im.buildImagePath(suffix)
}

// CompressedImagePath appends the compressor's file extension to the image path.
// The extension is derived from the first word of the compressor command string.
func (im *Image) CompressedImagePath() (string, error) {
	if err := im.validateImageModeForCreation(); err != nil {
		return "", err
	}
	compressor, err := im.Compressor()
	if err != nil {
		return "", fmt.Errorf("failed to get compressor: %w", err)
	}
	if compressor == "" {
		return "", errors.New("missing compressor parameter")
	}
	parts := strings.Fields(compressor)
	if len(parts) == 0 {
		return "", errors.New("invalid compressor parameters: empty command")
	}
	return im.imagePath + "." + parts[0], nil
}

// CompressImage compresses an image file using the configured compressor.
func (im *Image) CompressImage() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}
	compressor, err := im.Compressor()
	if err != nil {
		return fmt.Errorf("failed to get compressor: %w", err)
	}
	if compressor == "" {
		return errors.New("missing compressor parameter")
	}

	imagePathWithExt, err := im.CompressedImagePath()
	if err != nil {
		return err
	}

	parts := strings.Fields(compressor)
	if len(parts) == 0 {
		return errors.New("invalid compressor parameters: empty command")
	}
	args := append(parts[1:], im.imagePath)
	if err := im.runner(nil, im.stdout, im.stderr, parts[0], args...); err != nil {
		return fmt.Errorf("compression failed: %w", err)
	}

	if !filesystems.FileExists(imagePathWithExt) {
		return fmt.Errorf("compressed image was not created at the expected path: %s", imagePathWithExt)
	}
	return nil
}

// Qcow2ImagePath returns the qcow2 image path for a given .img path.
func (im *Image) Qcow2ImagePath() (string, error) {
	if err := im.validateImageModeForCreation(); err != nil {
		return "", err
	}
	return im.imagePath + ".qcow2", nil
}

// CreateQcow2Image creates a compressed qcow2 image from a raw image.
func (im *Image) CreateQcow2Image() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}
	qcow2Path, _ := im.Qcow2ImagePath()
	return im.runner(nil, im.stdout, im.stderr,
		"qemu-img", "convert", "-c", "-O", "qcow2", "-p", im.imagePath, qcow2Path)
}

// RemoveImageFile removes an image file and its associated .sha256 and .asc files.
func (im *Image) RemoveImageFile() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}

	im.Print("Removing %s ...\n", im.imagePath)
	for _, path := range []string{im.imagePath, im.imagePath + ".sha256", im.imagePath + ".asc"} {
		os.Remove(path) // Ignore errors (file may not exist).
	}
	return nil
}

// ShowFinalFilesystemInfo displays information about the final filesystem layout.
func (im *Image) ShowFinalFilesystemInfo() error {
	if im.devicePath == "" {
		return errors.New("missing devicePath, not set in NewImageOptions")
	}
	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}
	if im.efifsMount == "" {
		return errors.New("missing efifsMount, call MountEfifs first")
	}

	im.Print("Final boot partition directory tree:\n")
	if err := filesystems.PrintDirectoryTree(im.stdout, im.bootfsMount); err != nil {
		im.PrintWarning("WARNING: failed to list boot directory tree: %v\n", err)
	}

	im.Print("Final EFI partition directory tree:\n")
	if err := filesystems.PrintDirectoryTree(im.stdout, im.efifsMount); err != nil {
		im.PrintWarning("WARNING: failed to list EFI directory tree: %v\n", err)
	}

	im.Print("Block devices on %s:\n", im.devicePath)
	if err := filesystems.PrintBlockDeviceInfo(im.stdout, im.devicePath); err != nil {
		im.PrintWarning("WARNING: failed to get block device info: %v\n", err)
	}

	im.Print("Filesystem setup complete!\n")
	return nil
}

// ShowImageTestInfo prints information about generated artifacts and how to test them.
func (im *Image) ShowImageTestInfo(artifacts []string) error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}

	if len(artifacts) != 0 {
		im.Print("Generated artifacts:\n")
		for _, a := range artifacts {
			im.Print(">> %s\n", a)
		}
	}

	im.Print("How to test:\n")
	im.Print("    # vector dev vm -image %s -memory 8G -interactive\n", im.imagePath)
	im.Print("To move to a USB stick:\n")
	im.Print("    xz -dc %s | dd of=/dev/sdX bs=4M conv=sparse,sync status=progress\n", im.imagePath)
	return nil
}
