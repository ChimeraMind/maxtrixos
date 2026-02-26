package imager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"matrixos/vector/lib/filesystems"
)

// CreateImage creates a sparse image file at imagePath with the given size.
func (im *Image) CreateImage(imageSize string) (retErr error) {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}

	if imageSize == "" {
		return errors.New("missing imageSize parameter")
	}

	imagesDir := filepath.Dir(im.imagePath)
	im.Print(
		"Creating images directory: %s (if it does not exist)\n",
		imagesDir,
	)
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		return fmt.Errorf("failed to create images directory %s: %w", imagesDir, err)
	}

	// Don't skip removing or sgdisk gets confused due to truncate.
	if err := im.RemoveImageFile(); err != nil {
		return err
	}

	sizeBytes, err := filesystems.ParseHumanSize(imageSize)
	if err != nil {
		return fmt.Errorf("failed to parse image size %q: %w", imageSize, err)
	}

	im.Print("Creating block device image file: %s\n", im.imagePath)
	f, err := os.Create(im.imagePath)
	if err != nil {
		return fmt.Errorf("failed to create image file %s: %w", im.imagePath, err)
	}
	defer func() {
		if cerr := f.Close(); cerr != nil && retErr == nil {
			retErr = cerr
		}
	}()

	if err := f.Truncate(sizeBytes); err != nil {
		return fmt.Errorf("failed to truncate image file %s to %d bytes: %w", im.imagePath, sizeBytes, err)
	}
	return nil
}

// ClearPartitionTable clears the partition table on a device using sgdisk.
func (im *Image) ClearPartitionTable() error {
	if im.devicePath == "" {
		return errors.New("missing devicePath, not set in NewImageOptions")
	}

	im.Print("Clearing partition table on %s ...\n", im.devicePath)
	if err := im.runner(nil, im.stdout, im.stderr, "sgdisk", "-g", "-o", im.devicePath); err != nil {
		return fmt.Errorf("sgdisk -g -o failed on %s: %w", im.devicePath, err)
	}
	return im.runner(nil, im.stdout, im.stderr, "sgdisk", "-Z", im.devicePath)
}

// DatedFsLabel returns a filesystem label based on the current date (YYYYMMDD).
func (im *Image) DatedFsLabel() string {
	return time.Now().Format("20060102")
}

// PartitionDevices creates the EFI, boot, and root partitions on a device.
func (im *Image) PartitionDevices(efiSize, bootSize, imageSize string) error {
	if efiSize == "" {
		return errors.New("missing efiSize parameter")
	}
	if bootSize == "" {
		return errors.New("missing bootSize parameter")
	}
	if imageSize == "" {
		return errors.New("missing imageSize parameter")
	}
	if im.devicePath == "" {
		return errors.New("missing devicePath, not set in NewImageOptions")
	}

	espPartType, err := im.EspPartitionType()
	if err != nil {
		return err
	}
	bootPartType, err := im.BootPartitionType()
	if err != nil {
		return err
	}
	rootPartType, err := im.RootPartitionType()
	if err != nil {
		return err
	}

	im.Print("Partitioning %s:\n", im.devicePath)
	im.Print(" --> p1 (EFI: %s)\n", efiSize)
	im.Print(" --> p2 (BOOT: %s)\n", bootSize)
	im.Print(" --> p3 (ROOT: Remainder of %s, plus autogrow)\n", imageSize)

	// Create EFI partition.
	epArgs := []string{
		"sgdisk",
		"-n", fmt.Sprintf("1:0:+%s", efiSize),
		"-t", fmt.Sprintf("1:%s", espPartType),
		im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, epArgs[0], epArgs[1:]...); err != nil {
		return fmt.Errorf("sgdisk EFI partition failed: %w", err)
	}

	// Create boot partition.
	bpArgs := []string{
		"sgdisk",
		"-n", fmt.Sprintf("2:0:+%s", bootSize),
		"-t", fmt.Sprintf("2:%s", bootPartType),
		im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, bpArgs[0], bpArgs[1:]...); err != nil {
		return fmt.Errorf("sgdisk boot partition failed: %w", err)
	}

	// Create root partition with -10M padding for systemd-repart.
	rpArgs := []string{
		"sgdisk",
		"-n", "3:0:-10M",
		"-t", fmt.Sprintf("3:%s", rootPartType),
		im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, rpArgs[0], rpArgs[1:]...); err != nil {
		return fmt.Errorf("sgdisk root partition failed: %w", err)
	}

	// Set the auto-grow flag (bit 59) on partition 3.
	agArgs := []string{
		"sgdisk", "-A", "3:set:59", im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, agArgs[0], agArgs[1:]...); err != nil {
		return fmt.Errorf("sgdisk set auto-grow flag failed: %w", err)
	}

	im.Print("Refreshing partition table ...\n")
	args := []string{
		"partprobe", "-s", im.devicePath,
	}
	if err := im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...); err != nil {
		return fmt.Errorf("partprobe failed: %w", err)
	}

	filesystems.DevicesSettle()
	return nil
}

// FormatEfifs creates a FAT32 filesystem on the EFI partition.
func (im *Image) FormatEfifs() error {
	if im.efiDevice == "" {
		return errors.New("missing efiDevice, not set in NewImageOptions")
	}

	im.Print("Creating EFI partition on %s\n", im.efiDevice)
	label := "ME" + im.DatedFsLabel()
	args := []string{
		"mkfs.vfat",
		"-F", "32",
		"-n", label,
		im.efiDevice,
	}
	return im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...)
}

// MountEfifs mounts the EFI partition.
func (im *Image) MountEfifs(mountEfifs string) error {
	if im.efiDevice == "" {
		return errors.New("missing efiDevice, not set in NewImageOptions")
	}
	if mountEfifs == "" {
		return errors.New("missing mountEfifs parameter")
	}

	if !filesystems.DirectoryExists(mountEfifs) {
		im.Print("Creating %s ...\n", mountEfifs)
		if err := os.MkdirAll(mountEfifs, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountEfifs, err)
		}
	}

	im.Print("Mounting %s to %s\n", im.efiDevice, mountEfifs)
	im.trackMount(mountEfifs)
	if err := im.runner(nil, im.stdout, im.stderr, "mount", "-t", "vfat", im.efiDevice, mountEfifs); err != nil {
		return err
	}
	im.efifsMount = mountEfifs
	return nil
}

// EfiBootDir returns the full path to the EFI boot directory on the mounted
// EFI filesystem.
func (im *Image) EfiBootDir() (string, error) {
	if im.efifsMount == "" {
		return "", errors.New("EFI filesystem not mounted")
	}
	relEfiBootPath, err := im.RelativeEfiBootPath()
	if err != nil {
		return "", err
	}
	efibootDir := filepath.Join(im.efifsMount, relEfiBootPath)
	return efibootDir, nil
}

// FormatBootfs creates a btrfs filesystem on the boot partition.
func (im *Image) FormatBootfs() error {
	if im.bootDevice == "" {
		return errors.New("missing bootDevice, not set in NewImageOptions")
	}

	label := "MB" + im.DatedFsLabel()
	im.Print("Creating btrfs on %s (boot)\n", im.bootDevice)
	args := []string{
		"mkfs.btrfs",
		"-f",
		"-L", label,
		im.bootDevice,
	}
	return im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...)
}

// MountBootfs mounts the boot partition.
func (im *Image) MountBootfs(mountBootfs string) error {
	if im.bootDevice == "" {
		return errors.New("missing bootDevice, not set in NewImageOptions")
	}
	if mountBootfs == "" {
		return errors.New("missing mountBootfs parameter")
	}

	if !filesystems.DirectoryExists(mountBootfs) {
		im.Print("Creating %s ...\n", mountBootfs)
		if err := os.MkdirAll(mountBootfs, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountBootfs, err)
		}
	}

	im.Print("Mounting %s to %s\n", im.bootDevice, mountBootfs)
	im.trackMount(mountBootfs)
	if err := im.runner(nil, im.stdout, im.stderr, "mount", im.bootDevice, mountBootfs); err != nil {
		return err
	}
	im.bootfsMount = mountBootfs
	return nil
}

// MaybeEncryptRootfs encrypts the root partition with LUKS if encryption is
// enabled in the configuration.
func (im *Image) MaybeEncryptRootfs() error {
	if !im.encrypted {
		return nil
	}

	// Get the current root device.
	rootDevice := im.RootDevice()
	im.realRootDevice = rootDevice

	encRootfsName, err := im.fsenc.EncryptedRootFsName()
	if err != nil {
		return err
	}
	luksDevice, err := filesystems.GetLuksRootfsDevicePath(encRootfsName)
	if err != nil {
		return err
	}
	if err := im.fsenc.LuksEncrypt(im.rootDevice, luksDevice); err != nil {
		return fmt.Errorf("LUKS encryption failed: %w", err)
	}
	im.SetRootDevice(luksDevice)
	im.Print("New encrypted rootfs partition: %s\n", luksDevice)
	return nil
}

// FormatRootfs creates a btrfs filesystem on the root partition.
func (im *Image) FormatRootfs() error {
	if im.rootDevice == "" {
		return errors.New("missing rootDevice, not set in NewImageOptions")
	}

	label := "MR" + im.DatedFsLabel()
	im.Print("Creating btrfs on %s (root)\n", im.rootDevice)
	args := []string{
		"mkfs.btrfs",
		"-f",
		"-L", label,
		im.rootDevice,
	}
	return im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...)
}

// RootfsKernelArgs returns the default kernel arguments for the root filesystem.
func (im *Image) RootfsKernelArgs() []string {
	return []string{"rootflags=discard=async"}
}

// MountRootfs mounts the root partition with btrfs compression options.
func (im *Image) MountRootfs(mountRootfs string) error {
	if im.rootDevice == "" {
		return errors.New("missing rootDevice, not set in NewImageOptions")
	}
	if mountRootfs == "" {
		return errors.New("missing mountRootfs parameter")
	}

	if !filesystems.DirectoryExists(mountRootfs) {
		im.Print("Creating %s ...\n", mountRootfs)
		if err := os.MkdirAll(mountRootfs, 0755); err != nil {
			return fmt.Errorf("failed to create mount point %s: %w", mountRootfs, err)
		}
	}

	compression := "zstd:6"
	btrfsOpts := fmt.Sprintf("compress-force=%s,space_cache=v2,commit=120", compression)
	im.Print("Mounting %s to %s\n", im.rootDevice, mountRootfs)

	im.trackMount(mountRootfs)
	args := []string{
		"mount",
		"-o", btrfsOpts,
		im.rootDevice,
		mountRootfs,
	}
	if err := im.runner(nil, im.stdout, im.stderr, args[0], args[1:]...); err != nil {
		return err
	}
	im.rootfsMount = mountRootfs

	return nil
}

// FinalizeFilesystems runs fstrim on the root and boot filesystems to improve
// compression ratios for sparse image files.
func (im *Image) FinalizeFilesystems() error {
	if im.rootfsMount == "" {
		return errors.New("missing rootfsMount, call MountRootfs first")
	}
	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}
	if im.efifsMount == "" {
		return errors.New("missing efifsMount, call MountEfifs first")
	}

	// fstrim may fail on USB sticks, so errors are intentionally ignored.
	filesystems.FstrimAll(
		im.runner, im.stdout, im.stderr,
		im.rootfsMount, im.bootfsMount,
	)

	return nil
}
