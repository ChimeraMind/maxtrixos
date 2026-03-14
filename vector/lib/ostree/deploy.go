package ostree

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

type Deployment struct {
	Checksum  string `json:"checksum"`
	Stateroot string `json:"stateroot"`
	// Requires matrixOS ostree-2025.7-r1
	Refspec  string `json:"refspec"`
	Booted   bool   `json:"booted"`
	Pending  bool   `json:"pending"`
	Rollback bool   `json:"rollback"`
	Staged   bool   `json:"staged"`
	Index    int    `json:"index"`
	Serial   int    `json:"serial"`
}

// BuildDeploymentRootfs builds the path to the deployed rootfs given a sysroot, osName,
// commit and index.
func BuildDeploymentRootfs(sysroot, osName, commit string, index int) string {
	return filepath.Join(
		sysroot,
		"ostree",
		"deploy",
		osName,
		"deploy",
		commit+"."+strconv.Itoa(index),
	)
}

// DeployedRootfsWithSysroot returns the path to the deployed rootfs given a sysroot and repoDir.
func DeployedRootfsWithSysroot(sysroot, repoDir, osName, ref string, verbose bool) (string, error) {
	if sysroot == "" {
		return "", errors.New("invalid sysroot parameter")
	}
	if repoDir == "" {
		return "", errors.New("invalid repoDir parameter")
	}
	if osName == "" {
		return "", errors.New("invalid osName parameter")
	}
	if ref == "" {
		return "", errors.New("invalid ref parameter")
	}

	ostreeCommit, err := LastCommit(repoDir, ref, verbose)
	if err != nil {
		return "", fmt.Errorf("cannot get last ostree commit: %w", err)
	}

	rootfs := BuildDeploymentRootfs(sysroot, osName, ostreeCommit, 0)
	return rootfs, nil
}

func ListDeploymentsWithSysroot(sysroot string, verbose bool) ([]Deployment, error) {
	data, error := ostreeAdminStatusJson(sysroot, verbose)
	if error != nil {
		return nil, error
	}
	if data == nil {
		return nil, errors.New("failed to get ostree status")
	}

	var deployments struct {
		Deployments []Deployment `json:"deployments"`
	}

	if err := json.Unmarshal(*data, &deployments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ostree status: %w", err)
	}
	return deployments.Deployments, nil
}

func ostreeAdminStatusJson(sysroot string, verbose bool) (*[]byte, error) {
	if sysroot == "" {
		return nil, errors.New("invalid ostree sysroot parameter")
	}
	stdout, err := RunWithStdoutCapture(
		verbose,
		"--sysroot="+sysroot,
		"admin",
		"status",
		"--json",
	)
	if err != nil {
		return nil, err
	}

	data, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read ostree status: %w", err)
	}
	return &data, nil
}

// BootedRefWithSysroot returns the ref of the booted deployment.
func BootedRefWithSysroot(sysroot string, verbose bool) (string, error) {
	if sysroot == "" {
		return "", errors.New("invalid ostree sysroot parameter")
	}

	deployments, err := ListDeploymentsWithSysroot(sysroot, verbose)
	if err != nil {
		return "", err
	}

	for _, d := range deployments {
		if d.Booted {
			return d.Refspec, nil
		}
	}

	return "", errors.New("no booted deployment found")
}

// BootedHash returns the commit hash of the booted deployment.
func BootedHashWithSysroot(sysroot string, verbose bool) (string, error) {
	if sysroot == "" {
		return "", errors.New("invalid ostree sysroot parameter")
	}

	deployments, err := ListDeploymentsWithSysroot(sysroot, verbose)
	if err != nil {
		return "", err
	}

	for _, d := range deployments {
		if d.Booted {
			return d.Checksum, nil
		}
	}

	return "", errors.New("no booted deployment found")
}

// listDeploymentsFromSysroot lists deployments using the instance runner.
func (o *Ostree) listDeploymentsFromSysroot(sysroot string, verbose bool) ([]Deployment, error) {
	if sysroot == "" {
		return nil, errors.New("invalid ostree sysroot parameter")
	}
	stdout, err := o.ostreeRunCapture(verbose, "--sysroot="+sysroot, "admin", "status", "--json")
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read ostree status: %w", err)
	}
	var deployments struct {
		Deployments []Deployment `json:"deployments"`
	}
	if err := json.Unmarshal(data, &deployments); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ostree status: %w", err)
	}
	return deployments.Deployments, nil
}

// BootCommit returns the boot commit from an ostree sysroot.
func (o *Ostree) BootCommit(sysroot string) (string, error) {
	osName, err := o.OsName()
	if err != nil {
		return "", err
	}
	bootPrefix := filepath.Join(sysroot, "ostree", "boot.1", osName)
	files, err := os.ReadDir(bootPrefix)
	if err != nil {
		return "", err
	}
	if len(files) == 0 {
		return "", fmt.Errorf("no commit found in %s", bootPrefix)
	}
	return files[0].Name(), nil
}

// ListDeployments lists the deployments in the / filesystem.
func (o *Ostree) ListDeployments(verbose bool) ([]Deployment, error) {
	root, err := o.Root()
	if err != nil {
		return nil, err
	}
	return o.listDeploymentsFromSysroot(root, verbose)
}

// DeployedRootfs returns the path to the deployed rootfs.
func (o *Ostree) DeployedRootfs(ref string, verbose bool) (string, error) {
	sysroot, err := o.Sysroot()
	if err != nil {
		return "", err
	}

	if ref == "" {
		return "", errors.New("invalid ref parameter")
	}
	osName, err := o.OsName()
	if err != nil {
		return "", err
	}

	ostreeCommit, err := o.LastCommit(ref, verbose)
	if err != nil {
		return "", fmt.Errorf("cannot get last ostree commit: %w", err)
	}

	rootfs := BuildDeploymentRootfs(sysroot, osName, ostreeCommit, 0)
	return rootfs, nil
}

// BootedRef returns the ref of the booted deployment.
func (o *Ostree) BootedRef(verbose bool) (string, error) {
	root, err := o.Root()
	if err != nil {
		return "", err
	}
	deployments, err := o.listDeploymentsFromSysroot(root, verbose)
	if err != nil {
		return "", err
	}
	for _, d := range deployments {
		if d.Booted {
			return d.Refspec, nil
		}
	}
	return "", errors.New("no booted deployment found")
}

// BootedHash returns the commit hash of the booted deployment.
func (o *Ostree) BootedHash(verbose bool) (string, error) {
	root, err := o.Root()
	if err != nil {
		return "", err
	}
	deployments, err := o.listDeploymentsFromSysroot(root, verbose)
	if err != nil {
		return "", err
	}
	for _, d := range deployments {
		if d.Booted {
			return d.Checksum, nil
		}
	}
	return "", errors.New("no booted deployment found")
}

// Switch runs `ostree admin switch` to switch to the given ref.
func (o *Ostree) Switch(ref string, verbose bool) error {
	sysroot, err := o.Sysroot()
	if err != nil {
		return err
	}
	return o.ostreeRun(verbose, "admin", "switch", "--sysroot="+sysroot, ref)
}

// Deploy deploys an ostree commit.
func (o *Ostree) Deploy(ref, sysroot string, bootArgs []string, verbose bool) error {
	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}
	remote, err := o.Remote()
	if err != nil {
		return err
	}

	o.Print("Creating %s ...\n", sysroot)
	if err := os.MkdirAll(sysroot, 0755); err != nil {
		return err
	}

	ostreeCommit, err := o.lastCommitFromRepo(repoDir, ref, verbose)
	if err != nil {
		return fmt.Errorf("cannot get last ostree commit: %w", err)
	}

	o.Print("Initializing ostree dir structure into %s ...\n", sysroot)
	if err := o.ostreeRun(verbose, "admin", "init-fs", sysroot); err != nil {
		return err
	}

	osName, err := o.OsName()
	if err != nil {
		return err
	}

	o.Print("Initializing OS %s into %s...\n", osName, sysroot)
	osInitArgs := []string{
		"admin", "os-init",
		osName,
		"--sysroot=" + sysroot,
	}
	if err := o.ostreeRun(verbose, osInitArgs...); err != nil {
		return err
	}

	sysrootRepo := filepath.Join(sysroot, "ostree", "repo")
	o.Print("Pulling local ostree commit %s into %s ...\n", ostreeCommit, sysrootRepo)
	pullArgs := []string{
		"--repo=" + sysrootRepo,
		"pull-local",
		repoDir,
		ostreeCommit,
	}
	if err := o.ostreeRun(verbose, pullArgs...); err != nil {
		return err
	}

	createRefArgs := []string{
		"refs",
		"--repo=" + sysrootRepo,
		"--create=" + remote + ":" + ref,
		ostreeCommit,
	}
	o.Print("ostree creating ref %s in sysroot repo ...\n", remote+":"+ref)
	if err := o.ostreeRun(verbose, createRefArgs...); err != nil {
		return err
	}

	o.Print("ostree setting bootloader to none (using blscfg instead) ...")
	blArgs := []string{
		"config",
		"--repo=" + sysrootRepo,
		"set",
		"sysroot.bootloader",
		"none",
	}
	if err := o.ostreeRun(verbose, blArgs...); err != nil {
		return err
	}

	o.Print("ostree setting bootprefix = false, given separate boot partition ...")
	bootprefixArgs := []string{
		"config",
		"--repo=" + sysrootRepo,
		"set",
		"sysroot.bootprefix",
		"false",
	}
	if err := o.ostreeRun(verbose, bootprefixArgs...); err != nil {
		return err
	}

	o.Print("Deploying %s to %s...\n", ref, sysroot)
	deployArgs := []string{
		"admin", "deploy",
		"--sysroot=" + sysroot,
		"--os=" + osName,
	}
	for _, ba := range bootArgs {
		deployArgs = append(deployArgs, "--karg-append="+ba)
	}
	deployArgs = append(deployArgs, remote+":"+ref)

	if err := o.ostreeRun(verbose, deployArgs...); err != nil {
		return err
	}

	o.Print("Deployed filesystem at %s for commit %s.\n", sysroot, ostreeCommit)
	return nil
}

// Upgrade runs `ostree admin upgrade`.
func (o *Ostree) Upgrade(args []string, verbose bool) error {
	root, err := o.Root()
	if err != nil {
		return err
	}

	cmdArgs := []string{"admin", "upgrade", "--sysroot=" + root}
	cmdArgs = append(cmdArgs, args...)

	return o.ostreeRun(verbose, cmdArgs...)
}
