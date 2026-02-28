package imager

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"matrixos/vector/lib/filesystems"
)

func (im *Image) GetKernelPath() (string, error) {
	if im.rootfs == "" {
		return "", errors.New("rootfs not set, call SetRootfs first")
	}

	modulesDir := filepath.Join(im.rootfs, "usr", "lib", "modules")
	entries, err := os.ReadDir(modulesDir)
	if err != nil {
		return "", fmt.Errorf("failed to read modules directory %s: %w", modulesDir, err)
	}

	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, e.Name())
		}
	}
	if len(dirs) == 0 {
		return "", fmt.Errorf("no kernel directory found in %s", modulesDir)
	}
	sort.Strings(dirs)
	return dirs[0], nil
}

func (im *Image) SetupPasswords() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}

	shadowFile := filepath.Join(im.rootfs, "etc", "shadow")

	cmd := exec.Command("openssl", "passwd", "-6", "matrix")
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("openssl passwd failed: %w", err)
	}
	passHash := strings.TrimSpace(string(out))
	lastChange := fmt.Sprintf("%d", time.Now().Unix()/86400)

	data, err := os.ReadFile(shadowFile)
	if err != nil {
		return fmt.Errorf("failed to read shadow file: %w", err)
	}

	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		// Remove existing matrix: and root: lines.
		if strings.HasPrefix(line, "matrix:") || strings.HasPrefix(line, "root:") {
			continue
		}
		lines = append(lines, line)
	}

	shadowEntry := func(user string) string {
		return fmt.Sprintf("%s:%s:%s:0:99999:7:::", user, passHash, lastChange)
	}

	im.Print("Setting the default password of matrix to matrix ...\n")
	lines = append(lines, shadowEntry("matrix"))
	im.Print("Setting the default password of root to matrix ...\n")
	lines = append(lines, shadowEntry("root"))

	return os.WriteFile(shadowFile, []byte(strings.Join(lines, "\n")+"\n"), 0640)
}

func (im *Image) ExtractPackageList() ([]string, error) {
	if im.rootfs == "" {
		return nil, errors.New("rootfs not set, call SetRootfs first")
	}

	roVdb, err := im.ReadOnlyVdb()
	if err != nil {
		return nil, err
	}

	vdb := filepath.Join(im.rootfs, roVdb)
	if !filesystems.DirectoryExists(vdb) {
		im.PrintError("%s does not exist. cannot generate pkglist\n", vdb)
		return nil, nil
	}

	var pkgList []string
	categories, err := os.ReadDir(vdb)
	if err != nil {
		return nil, fmt.Errorf("failed to read vdb directory %s: %w", vdb, err)
	}
	for _, cat := range categories {
		if !cat.IsDir() {
			continue
		}
		catPath := filepath.Join(vdb, cat.Name())
		pkgs, err := os.ReadDir(catPath)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to read category directory %s: %w", catPath, err)
		}
		for _, pkg := range pkgs {
			pkgList = append(pkgList, filepath.Join(cat.Name(), pkg.Name()))
		}
	}
	return pkgList, nil
}

func (im *Image) SetupHooks() error {
	if im.rootfs == "" {
		return errors.New("rootfs not set, call SetRootfs first")
	}

	ref, err := im.cleanAndStripRef()
	if err != nil {
		return fmt.Errorf("failed to clean ref: %w", err)
	}

	devDir, err := im.DevDir()
	if err != nil {
		return err
	}

	hooksSrcDir := filepath.Join(devDir, "image", "hooks")
	if !filesystems.DirectoryExists(hooksSrcDir) {
		im.PrintError("hooks source dir %s does not exist\n", hooksSrcDir)
		return nil
	}

	hookExec := filepath.Join(hooksSrcDir, ref+".sh")
	if !filesystems.FileExists(hookExec) {
		im.PrintError("hook script %s does not exist\n", hookExec)
		return nil
	}

	info, err := os.Stat(hookExec)
	if err != nil {
		return fmt.Errorf("failed to stat hook script: %w", err)
	}
	if info.Mode()&0111 == 0 {
		return fmt.Errorf("hook script %s is not executable", hookExec)
	}

	cmd := exec.Command(hookExec)
	cmd.Stdout = im.stdout
	cmd.Stderr = im.stderr
	cmd.Env = append(os.Environ(),
		"MATRIXOS_DEV_DIR="+devDir,
		"ROOTFS="+im.rootfs,
		"REF="+ref,
	)
	return cmd.Run()
}

func (im *Image) TestImage() error {
	if err := im.validateImageModeForCreation(); err != nil {
		return err
	}

	ref, err := im.cleanAndStripRef()
	if err != nil {
		return fmt.Errorf("failed to clean ref: %w", err)
	}

	devDir, err := im.DevDir()
	if err != nil {
		return err
	}

	testDir := filepath.Join(devDir, "image", "tests", ref)
	if !filesystems.DirectoryExists(testDir) {
		im.PrintError("test dir %s does not exist, skipping test\n", testDir)
		return nil
	}

	mountDir, err := im.MountDir()
	if err != nil {
		return err
	}

	imageTempDir, err := filesystems.CreateTempDir(mountDir, refToSuffix(ref))
	if err != nil {
		return fmt.Errorf("failed to create temp dir for testing: %w", err)
	}
	defer os.RemoveAll(imageTempDir)

	imageName := filepath.Base(im.imagePath)
	testImagePath := filepath.Join(imageTempDir, imageName)
	im.Print("Copying image to %s for testing ...\n", testImagePath)
	if err := filesystems.CopyFileReflink(im.imagePath, testImagePath); err != nil {
		return fmt.Errorf("failed to copy image for testing: %w", err)
	}

	logsDir, err := im.cfg.GetItem("matrixOS.LogsDir")
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(testDir)
	if err != nil {
		return fmt.Errorf("failed to read test dir: %w", err)
	}
	for _, entry := range entries {
		ts := filepath.Join(testDir, entry.Name())
		info, err := os.Stat(ts)
		if err != nil {
			continue
		}
		if !info.Mode().IsRegular() || info.Mode()&0111 == 0 {
			im.PrintError("Skipping non-executable test script %s\n", ts)
			continue
		}

		im.Print("Running test script %s ...\n", ts)
		cmd := exec.Command(ts)
		cmd.Stdout = im.stdout
		cmd.Stderr = im.stderr
		cmd.Env = append(os.Environ(),
			"MATRIXOS_DEV_DIR="+devDir,
			"MATRIXOS_LOGS_DIR="+logsDir,
			"IMAGE_PATH="+testImagePath,
			"REF="+ref,
		)
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("test script %s failed: %w", ts, err)
		}
	}
	return nil
}
