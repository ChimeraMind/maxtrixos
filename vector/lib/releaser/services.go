package releaser

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"matrixos/vector/lib/filesystems"
)

func (r *Releaser) SetupHostname() error {
	hostname, err := r.Hostname()
	if err != nil {
		return err
	}

	r.Print("Setting hostname to: %s\n", hostname)
	data := []byte(hostname + "\n")
	return os.WriteFile(filepath.Join(r.imageDir, "etc/hostname"), data, 0644)
}

// serviceAction represents a parsed line from a services configuration file.
type serviceAction struct {
	action   string
	services []string
}

// parseServicesFile reads a services configuration file and returns parsed actions.
func parseServicesFile(path string) ([]serviceAction, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var actions []serviceAction
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}

		actions = append(actions, serviceAction{
			action:   fields[0],
			services: fields[1:],
		})
	}
	return actions, scanner.Err()
}

func (r *Releaser) SetupServices() error {
	imageDir := r.imageDir
	ref := r.ref

	hooksDir, err := r.HooksDir()
	if err != nil {
		return err
	}

	servicesFile := filepath.Join(hooksDir, ref+".conf")
	if !filesystems.FileExists(servicesFile) {
		// Try looking up by joining with the release/services structure.
		servicesDir, err := r.cfg.GetItem("Releaser.HooksDir")
		if err != nil {
			return err
		}
		// Fallback: check in the services dir relative to the same parent.
		parent := filepath.Dir(servicesDir)
		altPath := filepath.Join(parent, "services", ref+".conf")
		if !filesystems.FileExists(altPath) {
			r.PrintWarning("Services setup file %s does not exist. Skipping ...\n", servicesFile)
			return nil
		}
		servicesFile = altPath
	}

	actions, err := parseServicesFile(servicesFile)
	if err != nil {
		return fmt.Errorf("failed to parse services file: %w", err)
	}

	// Set up chroot mounts for systemctl execution.
	mounts, err := filesystems.NewCommonRootfsMounts(
		filesystems.CommonRootfsMountsOptions{
			MountPoint: imageDir,
			Mounting: func(mnt string) {
				r.Print("Mounting: %s ...\n", mnt)
				r.trackMount(mnt)
			},
			Mounted: func(mnt string) {
				r.Print("Mounted: %s\n", mnt)
			},
		},
	)
	if err != nil {
		return err
	}
	defer mounts.Cleanup()

	if err := mounts.Setup(); err != nil {
		return fmt.Errorf("failed to set up chroot mounts: %w", err)
	}

	// Group services by action type.
	var (
		enable        []string
		disable       []string
		mask          []string
		presetEnable  []string
		presetDisable []string
		presetMask    []string
		defaultTarget string
	)

	for _, a := range actions {
		switch a.action {
		case "enable":
			enable = append(enable, a.services...)
		case "disable":
			disable = append(disable, a.services...)
		case "mask":
			mask = append(mask, a.services...)
		case "preset-enable":
			presetEnable = append(presetEnable, a.services...)
		case "preset-disable":
			presetDisable = append(presetDisable, a.services...)
		case "preset-mask":
			presetMask = append(presetMask, a.services...)
		case "set-default":
			if len(a.services) > 0 {
				defaultTarget = a.services[len(a.services)-1]
			}
		default:
			r.PrintWarning("Unrecognized action in %s: %s\n", servicesFile, a.action)
		}
	}

	systemctl := func(args ...string) {
		cmd := strings.Join(args, " ")
		// Use /bin/sh -c to prevent systemctl from acting as PID 1.
		_ = filesystems.ChrootRun(imageDir, "/bin/sh", "-c", "systemctl "+cmd+"; exit $?")
	}

	for _, svc := range enable {
		r.Print("Enabling service: %s\n", svc)
		systemctl("enable", svc)
	}
	for _, svc := range disable {
		r.Print("Disabling service: %s\n", svc)
		systemctl("disable", svc)
	}
	for _, svc := range mask {
		r.Print("Masking service: %s\n", svc)
		systemctl("mask", svc)
	}
	for _, svc := range presetEnable {
		r.Print("Preset enabling for service: %s\n", svc)
		systemctl("--global", "enable", svc)
	}
	for _, svc := range presetDisable {
		r.Print("Preset disabling for service: %s\n", svc)
		systemctl("--global", "disable", svc)
	}
	for _, svc := range presetMask {
		r.Print("Preset masking for service: %s\n", svc)
		systemctl("--global", "mask", svc)
	}

	if defaultTarget != "" {
		r.Print("Setting default target to: %s\n", defaultTarget)
		systemctl("set-default", defaultTarget)
	}

	return nil
}

func (r *Releaser) ReleaseHook() error {
	ref := r.ref

	hooksDir, err := r.HooksDir()
	if err != nil {
		return err
	}

	hookPath := filepath.Join(hooksDir, ref+".sh")
	if !filesystems.FileExists(hookPath) {
		r.PrintWarning("Release hook %s does not exist. Skipping ...\n", hookPath)
		return nil
	}

	r.Print("Running release hook %s ...\n", hookPath)
	cmd := exec.Command(hookPath)
	cmd.Env = append(os.Environ(), "CHROOT_DIR="+r.imageDir)
	cmd.Stdout = r.stdout
	cmd.Stderr = r.stderr
	return cmd.Run()
}
