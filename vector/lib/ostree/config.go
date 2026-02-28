package ostree

import (
	"errors"
	"fmt"
)

func (o *Ostree) GpgEnabled() (bool, error) {
	return o.cfg.GetBool("Ostree.Gpg")
}

func (o *Ostree) SetGpg(enabled bool) error {
	repoDir, err := o.RepoDir()
	if err != nil {
		return err
	}
	val := "false"
	if enabled {
		val = "true"
	}
	return o.ostreeRun("--repo="+repoDir, "config", "set", "core.gpg-verify", val)
}

func (o *Ostree) GpgPrivateKeyPath() (string, error) {
	pk, err := o.cfg.GetItem("Ostree.GpgPrivateKey")
	if err != nil {
		return "", err
	}
	if pk == "" {
		return "", errors.New("invalid Ostree.GpgPrivateKey")
	}
	return pk, nil
}

func (o *Ostree) GpgPublicKeyPath() (string, error) {
	pk, err := o.cfg.GetItem("Ostree.GpgPublicKey")
	if err != nil {
		return "", err
	}
	if pk == "" {
		return "", errors.New("invalid Ostree.GpgPublicKey")
	}
	return pk, nil
}

func (o *Ostree) GpgOfficialPubKeyPath() (string, error) {
	pk, err := o.cfg.GetItem("Ostree.GpgOfficialPublicKey")
	if err != nil {
		return "", err
	}
	if pk == "" {
		return "", errors.New("invalid Ostree.GpgOfficialPublicKey")
	}
	return pk, nil
}

func (o *Ostree) OsName() (string, error) {
	name, err := o.cfg.GetItem("matrixOS.OsName")
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", errors.New("invalid matrixOS.OsName")
	}
	return name, nil
}

func (o *Ostree) FancyOsName() (string, error) {
	name, err := o.cfg.GetItem("matrixOS.FancyOsName")
	if err != nil {
		return "", err
	}
	if name == "" {
		return "", errors.New("invalid matrixOS.FancyOsName")
	}
	return name, nil
}

func (o *Ostree) Arch() (string, error) {
	arch, err := o.cfg.GetItem("matrixOS.Arch")
	if err != nil {
		return "", err
	}
	if arch == "" {
		return "", errors.New("invalid matrixOS.Arch")
	}
	return arch, nil
}

func (o *Ostree) RepoDir() (string, error) {
	repoDir, err := o.cfg.GetItem("Ostree.RepoDir")
	if err != nil {
		return "", err
	}
	if repoDir == "" {
		return "", errors.New("invalid Ostree.RepoDir")
	}
	return repoDir, nil
}

func (o *Ostree) Sysroot() (string, error) {
	sysroot, err := o.cfg.GetItem("Ostree.Sysroot")
	if err != nil {
		return "", err
	}
	if sysroot == "" {
		return "", errors.New("invalid Ostree.Sysroot")
	}
	return sysroot, nil
}

func (o *Ostree) Root() (string, error) {
	root, err := o.cfg.GetItem("Ostree.Root")
	if err != nil {
		return "", err
	}
	if root == "" {
		return "", errors.New("invalid Ostree.Root")
	}
	return root, nil
}

func (o *Ostree) Remote() (string, error) {
	remote, err := o.cfg.GetItem("Ostree.Remote")
	if err != nil {
		return "", err
	}
	if remote == "" {
		return "", errors.New("invalid Ostree.Remote")
	}
	return remote, nil
}

func (o *Ostree) RemoteURL() (string, error) {
	url, err := o.cfg.GetItem("Ostree.RemoteUrl")
	if err != nil {
		return "", err
	}
	if url == "" {
		return "", errors.New("invalid Ostree.RemoteUrl")
	}
	return url, nil
}

func (o *Ostree) AvailableGpgPubKeyPaths() ([]string, error) {
	var candidates []string
	privatePubKeyPath, err := o.GpgPublicKeyPath()
	if err == nil {
		candidates = append(candidates, privatePubKeyPath)
	}
	officialPubKeyPath, err := o.GpgOfficialPubKeyPath()
	if err == nil {
		candidates = append(candidates, officialPubKeyPath)
	}

	var paths []string
	for _, path := range candidates {
		if fileExists(path) {
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return paths, fmt.Errorf(
			"unable to find a valid GPG pub key. Neither: %v nor %v exist",
			privatePubKeyPath,
			officialPubKeyPath,
		)
	}

	return paths, nil
}

func (o *Ostree) GpgBestPubKeyPath() (string, error) {
	paths, err := o.AvailableGpgPubKeyPaths()
	if err != nil {
		return "", err
	}
	// pick the first, it's the best always.
	return paths[0], nil
}

func (o *Ostree) ClientSideGpgArgs() ([]string, error) {
	gpgEnabled, err := o.GpgEnabled()
	if err != nil {
		return nil, err
	}
	var pubKeyPath string
	if gpgEnabled {
		pubKeyPath, err = o.GpgBestPubKeyPath()
		if err != nil {
			return nil, err
		}
	}
	return ClientSideGpgArgs(gpgEnabled, pubKeyPath)
}
