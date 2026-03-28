package imager

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"matrixos/vector/lib/filesystems"
	"matrixos/vector/lib/runner"
)

func (im *Imager) SetupBootloaderConfig() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}

	if im.efiDevice == "" {
		return errors.New("missing efiDevice, not set in NewImagerOptions")
	}
	if im.bootDevice == "" {
		return errors.New("missing bootDevice, not set in NewImagerOptions")
	}

	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}
	if im.rootfsMount == "" {
		return errors.New("missing rootfsMount, call MountRootfs first")
	}

	ref, err := im.cleanAndStripRef()
	if err != nil {
		return fmt.Errorf("failed to clean ref: %w", err)
	}

	efibootDir, err := im.EfiBootDir()
	if err != nil {
		return fmt.Errorf("failed to determine EFI boot directory: %w", err)
	}

	efiDeviceUUID, err := filesystems.DeviceUUID(im.efiDevice)
	if err != nil {
		return fmt.Errorf("unable to get UUID for %s: %w", im.efiDevice, err)
	}

	bootDeviceUUID, err := filesystems.DeviceUUID(im.bootDevice)
	if err != nil {
		return fmt.Errorf("unable to get UUID for %s: %w", im.bootDevice, err)
	}

	// Verify kernel exists.
	if _, err := im.GetKernelPath(); err != nil {
		return fmt.Errorf("failed to determine kernel version: %w", err)
	}

	// Get the boot commit.
	bootCommit, err := im.ostree.BootCommit(im.rootfsMount)
	if err != nil || bootCommit == "" {
		return fmt.Errorf("cannot determine ostree boot commit: %w", err)
	}
	im.Print("Found boot commit: %s\n", bootCommit)

	devDir, err := im.DevDir()
	if err != nil {
		return err
	}

	srcGrubCfg := filepath.Join(devDir, "image", "boot", ref, "grub.cfg")
	resolved, err := filepath.EvalSymlinks(srcGrubCfg)
	if err != nil {
		return fmt.Errorf("failed to resolve grub config path %s: %w", srcGrubCfg, err)
	}
	srcGrubCfg = resolved

	if !filesystems.FileExists(srcGrubCfg) {
		return fmt.Errorf("grub config %s does not exist", srcGrubCfg)
	}
	im.Print("Using grub config from %s\n", srcGrubCfg)

	// Ensure efibootDir exists.
	if err := os.MkdirAll(efibootDir, 0755); err != nil {
		return fmt.Errorf("failed to create efibootDir %s: %w", efibootDir, err)
	}

	dstGrubCfg := filepath.Join(efibootDir, "grub.cfg")
	im.Print("Copying grub: %s -> %s\n", srcGrubCfg, dstGrubCfg)
	if err := filesystems.CopyFile(srcGrubCfg, dstGrubCfg); err != nil {
		return fmt.Errorf("failed to copy grub config: %w", err)
	}

	// Copy GRUB themes if available.
	osName, err := im.OsName()
	if err != nil {
		return err
	}
	themesDir := filepath.Join(
		im.rootfs,
		"usr", "share", "grub",
		"themes", osName+"-theme",
	)
	if filesystems.DirectoryExists(themesDir) {
		im.Print("Copying GRUB themes from %s ...\n", themesDir)
		dstThemesDir := filepath.Join(im.bootfsMount, "grub", "themes")

		if err := os.MkdirAll(dstThemesDir, 0755); err != nil {
			return fmt.Errorf("failed to create themes dir: %w", err)
		}

		dstThemeDir := filepath.Join(dstThemesDir, filepath.Base(themesDir))

		if err := filesystems.CopyDirPreserve(themesDir, dstThemeDir); err != nil {
			return fmt.Errorf("failed to copy themes: %w", err)
		}
	}

	// Write GRUB_CFG environment file.
	efiRoot, err := im.EfiRoot()
	if err != nil {
		return err
	}
	relEfiBootPath, err := im.RelativeEfiBootPath()
	if err != nil {
		return err
	}

	envDir := filepath.Join(im.rootfs, "etc", "environment.d")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		return fmt.Errorf("failed to create environment.d dir: %w", err)
	}

	grubCfgEnv := fmt.Sprintf("GRUB_CFG=%s/%s/grub.cfg\n", efiRoot, relEfiBootPath)
	grubCfgEnvPath := filepath.Join(envDir, "99-matrixos-imager-grub.conf")
	if err := os.WriteFile(grubCfgEnvPath, []byte(grubCfgEnv), 0644); err != nil {
		return fmt.Errorf("failed to write grub env config: %w", err)
	}

	// Perform template substitutions in grub.cfg.
	grubData, err := os.ReadFile(dstGrubCfg)
	if err != nil {
		return fmt.Errorf("failed to read grub config for substitution: %w", err)
	}
	grubContent := string(grubData)
	grubContent = strings.ReplaceAll(grubContent, "%BOOTUUID%", bootDeviceUUID)
	grubContent = strings.ReplaceAll(grubContent, "%EFIUUID%", efiDeviceUUID)
	grubContent = strings.ReplaceAll(grubContent, "%OSNAME%", osName)
	if err := os.WriteFile(dstGrubCfg, []byte(grubContent), 0644); err != nil {
		return fmt.Errorf("failed to write substituted grub config: %w", err)
	}

	im.Print("Current grub.cfg:\n")
	im.Print("%s\n", grubContent)

	return nil
}

func (im *Imager) SetupVmtestConfig() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}

	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}

	im.Print("Setting up vmtest grub config based on the ostree boot config in %s ...\n", im.bootfsMount)

	ostreeBootCfg := filepath.Join(im.bootfsMount, "loader", "entries", "ostree-1.conf")
	if !filesystems.FileExists(ostreeBootCfg) {
		return fmt.Errorf("%s does not exist, cannot set up vmtest config", ostreeBootCfg)
	}

	vmtestCfgDir := filepath.Join(im.bootfsMount, ".imager.vmtest", "entries")
	if err := os.MkdirAll(vmtestCfgDir, 0755); err != nil {
		return fmt.Errorf("failed to create vmtest config dir: %w", err)
	}

	vmtestBootCfg := filepath.Join(vmtestCfgDir, "ostree-1.conf")

	consoleParams := "console=ttyS0,115200"
	systemdParams := "systemd.log_color=0"
	envParams := "systemd.setenv=SYSTEMD_COLORS=0 systemd.setenv=SYSTEMD_URLIFY=0"
	bootParams := consoleParams + " " + systemdParams + " " + envParams

	if err := filesystems.CopyFile(ostreeBootCfg, vmtestBootCfg); err != nil {
		return fmt.Errorf("failed to copy vmtest config: %w", err)
	}

	data, err := os.ReadFile(vmtestBootCfg)
	if err != nil {
		return fmt.Errorf("failed to read vmtest config: %w", err)
	}

	content := string(data)
	content = strings.ReplaceAll(content, "splash", "")
	content = strings.ReplaceAll(content, "quiet", bootParams)

	if err := os.WriteFile(vmtestBootCfg, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write vmtest config: %w", err)
	}

	im.Print("Set up vmtest grub config at %s\n", vmtestBootCfg)
	im.Print("Current vmtest grub config:\n")
	im.Print("%s\n", content)

	return nil
}

func (im *Imager) InstallSecurebootCerts() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}
	if im.efifsMount == "" {
		return errors.New("missing efifsMount, call MountEfifs first")
	}
	efibootDir, err := im.EfiBootDir()
	if err != nil {
		return err
	}

	certFileName, err := im.EfiCertificateFileName()
	if err != nil {
		return err
	}
	certDerFileName, err := im.EfiCertificateFileNameDer()
	if err != nil {
		return err
	}
	kekFileName, err := im.EfiCertificateFileNameKek()
	if err != nil {
		return err
	}
	kekDerFileName, err := im.EfiCertificateFileNameKekDer()
	if err != nil {
		return err
	}

	// SecureBoot certificate (db).
	sbCert := filepath.Join(im.rootfs, "etc", "portage", "secureboot.pem")
	if filesystems.FileExists(sbCert) {
		im.Print("Copying SecureBoot cert to EFI partition ...\n")
		if err := filesystems.CopyFile(sbCert, filepath.Join(im.efifsMount, certFileName)); err != nil {
			return fmt.Errorf("failed to copy SecureBoot cert: %w", err)
		}

		im.Print("Generating SecureBoot MOK ...\n")
		cmd := &runner.Cmd{
			Name: "openssl",
			Args: []string{
				"x509",
				"-in", sbCert,
				"-outform", "DER",
				"-out", filepath.Join(im.efifsMount, certDerFileName),
			},
			Stdout: im.stdout,
			Stderr: im.stderr,
		}
		if err := im.runner(cmd); err != nil {
			return fmt.Errorf("openssl DER conversion failed: %w", err)
		}
	} else {
		im.PrintWarning("NO SECUREBOOT CERT AT: %s -- ignoring.\n", sbCert)
	}

	// SecureBoot KEK certificate.
	sbKek := filepath.Join(im.rootfs, "etc", "portage", "secureboot-kek.pem")
	if filesystems.FileExists(sbKek) {
		im.Print("Copying SecureBoot KEK cert to EFI partition ...\n")
		if err := filesystems.CopyFile(sbKek, filepath.Join(im.efifsMount, kekFileName)); err != nil {
			return fmt.Errorf("failed to copy SecureBoot KEK cert: %w", err)
		}

		im.Print("Generating SecureBoot KEK DER for convenience ...\n")
		if err := im.runner(&runner.Cmd{
			Name: "openssl",
			Args: []string{
				"x509",
				"-in", sbKek,
				"-outform", "DER",
				"-out", filepath.Join(im.efifsMount, kekDerFileName),
			},
			Stdout: im.stdout,
			Stderr: im.stderr,
		}); err != nil {
			return fmt.Errorf("openssl KEK DER conversion failed: %w", err)
		}
	} else {
		im.PrintWarning("NO SECUREBOOT CERT AT: %s -- ignoring.\n", sbKek)
	}

	// Copy the shim binaries.
	shimDir := filepath.Join(im.rootfs, "usr", "share", "shim")
	im.Print("Copying shim for Secureboot from %s to %s ...\n", shimDir, efibootDir)
	return filesystems.CopyDirPreserve(shimDir, efibootDir)
}

func (im *Imager) InstallMemtest() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}
	efibootDir, err := im.EfiBootDir()
	if err != nil {
		return err
	}

	memtestBin := filepath.Join(im.rootfs, "usr", "share", "memtest86+", "memtest.efi64")
	if !filesystems.PathExists(memtestBin) {
		im.PrintWarning("WARNING: %s not available, please install memtest86+\n", memtestBin)
		return nil
	}
	return filesystems.CopyFile(memtestBin, filepath.Join(efibootDir, "memtest86plus.efi"))
}

func (im *Imager) InstallBootloader() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}
	if im.efifsMount == "" {
		return errors.New("missing efifsMount, call MountEfifs first")
	}
	if im.bootfsMount == "" {
		return errors.New("missing bootfsMount, call MountBootfs first")
	}
	if im.devicePath == "" {
		return errors.New("missing devicePath, not set in NewImagerOptions")
	}
	efibootDir, err := im.EfiBootDir()
	if err != nil {
		return fmt.Errorf("failed to determine EFI boot directory: %w", err)
	}

	im.Print("Installing bootloader ...\n")

	efiRoot, err := im.EfiRoot()
	if err != nil {
		return fmt.Errorf("failed to determine EFI root: %w", err)
	}
	bootRoot, err := im.BootRoot()
	if err != nil {
		return fmt.Errorf("failed to determine boot root: %w", err)
	}
	osName, err := im.OsName()
	if err != nil {
		return fmt.Errorf("failed to determine OS name: %w", err)
	}
	efiExe, err := im.EfiExecutable()
	if err != nil {
		return fmt.Errorf("failed to determine EFI executable: %w", err)
	}

	env := []string{
		"IMAGER_EFI_MOUNT=" + im.efifsMount,
		"IMAGER_BOOT_MOUNT=" + im.bootfsMount,
		"IMAGER_EFI_ROOT=" + efiRoot,
		"IMAGER_BOOT_ROOT=" + bootRoot,
	}

	err = im.chroot(
		env,
		"/usr/bin/grub-install",
		[]string{
			"--target=x86_64-efi",
			"--directory=/usr/lib/grub/x86_64-efi",
			"--efi-directory=" + efiRoot,
			"--boot-directory=" + bootRoot,
			"--themes=" + osName + "-theme",
			"--removable",
			"--modules=ext2 btrfs gzio part_gpt fat part_msdos all_video",
			im.devicePath,
		},
	)
	if err != nil {
		return fmt.Errorf("grub-install failed: %w", err)
	}

	// Verify BOOTX64.EFI was created.
	bootx64efi := filepath.Join(efibootDir, efiExe)
	if !filesystems.PathExists(bootx64efi) {
		return fmt.Errorf("%s does not exist after grub-install", bootx64efi)
	}

	// Replace unsigned GRUBX64.EFI with the signed one.
	grubx64efi := filepath.Join(efibootDir, "GRUBX64.EFI")
	im.Print("Removing existing %s as it's not signed ...\n", grubx64efi)
	os.Remove(grubx64efi)

	signedGrubx64efi := filepath.Join(im.rootfs, "usr", "lib", "grub", "grub-x86_64.efi.signed")
	im.Print("Moving %s to %s\n", signedGrubx64efi, grubx64efi)
	if err := filesystems.Move(signedGrubx64efi, grubx64efi); err != nil {
		return fmt.Errorf("failed to move signed grub binary: %w", err)
	}

	return nil
}

func (im *Imager) GenerateKernelBootArgs() ([]string, error) {
	ref, err := im.cleanAndStripRef()
	if err != nil {
		return nil, fmt.Errorf("failed to clean ref: %w", err)
	}
	if im.efiDevice == "" {
		return nil, errors.New("missing efiDevice, not set in NewImagerOptions")
	}
	if im.bootDevice == "" {
		return nil, errors.New("missing bootDevice, not set in NewImagerOptions")
	}
	if im.rootDevice == "" {
		return nil, errors.New("missing rootDevice, not set in NewImagerOptions")
	}

	// if we are encrypting, use the realRootDevice
	rootDevice := im.rootDevice
	if im.encrypted {
		if im.realRootDevice == "" {
			return nil, errors.New("missing realRootDevice for encrypted image")
		}
		rootDevice = im.realRootDevice
	}

	bootArgs := im.RootfsKernelArgs()

	// Root device UUID for LUKS.
	rootDeviceUUID, err := filesystems.DeviceUUID(rootDevice)
	if err != nil {
		return nil, fmt.Errorf("unable to get device UUID for %s: %w", rootDevice, err)
	}
	if im.encrypted {
		bootArgs = append(bootArgs, fmt.Sprintf("rd.luks.uuid=%s", rootDeviceUUID))
	}

	// EFI partition mount via systemd.
	efiRoot, err := im.EfiRoot()
	if err != nil {
		return nil, err
	}
	efiPartUUID, err := filesystems.DevicePartUUID(im.efiDevice)
	if err != nil {
		return nil, fmt.Errorf("unable to get PARTUUID of EFI partition: %w", err)
	}
	bootArgs = append(bootArgs, fmt.Sprintf("systemd.mount-extra=PARTUUID=%s:%s:auto:defaults", efiPartUUID, efiRoot))

	// Boot partition mount via systemd.
	bootRoot, err := im.BootRoot()
	if err != nil {
		return nil, err
	}
	bootPartUUID, err := filesystems.DevicePartUUID(im.bootDevice)
	if err != nil {
		return nil, fmt.Errorf("unable to get PARTUUID of boot partition: %w", err)
	}
	bootArgs = append(bootArgs, fmt.Sprintf("systemd.mount-extra=PARTUUID=%s:%s:auto:defaults", bootPartUUID, bootRoot))

	// Read additional kernel cmdline params from the image boot directory.
	devDir, err := im.DevDir()
	if err != nil {
		return nil, err
	}
	cmdlineFile := filepath.Join(devDir, "image", "boot", ref, "cmdline.conf")
	if filesystems.FileExists(cmdlineFile) {
		im.Print("Reading additional kernel args from %s ...\n", cmdlineFile)
		data, err := os.ReadFile(cmdlineFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read cmdline file: %w", err)
		}
		scanner := bufio.NewScanner(bytes.NewReader(data))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			bootArgs = append(bootArgs, line)
		}
	} else {
		im.PrintWarning("WARNING: no additional kernel cmdline params available, %s does not exist.\n", cmdlineFile)
	}

	return bootArgs, nil
}
