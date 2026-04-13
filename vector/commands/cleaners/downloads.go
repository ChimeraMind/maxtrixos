package cleaners

import (
	"io"
	"matrixos/vector/lib/config"
	"time"
)

const (
	downloadsCutoffAge = 30 * 24 * time.Hour
)

type DownloadsCleaner struct {
	cfg    config.IConfig
	stdout io.Writer
	stderr io.Writer
}

func (c *DownloadsCleaner) Name() string {
	return "downloads"
}

func (c *DownloadsCleaner) Init(cfg config.IConfig, stdout, stderr io.Writer) error {
	c.cfg = cfg
	c.stdout = stdout
	c.stderr = stderr
	return nil
}

func (c *DownloadsCleaner) isDryRun() (bool, error) {
	val, err := c.cfg.GetItem("DownloadsCleaner.DryRun")
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

func (c *DownloadsCleaner) getDownloadsDir() (string, error) {
	val, err := c.cfg.GetItem("Seeder.DownloadsDir")
	if err != nil {
		return "", err
	}
	return val, nil
}

func (c *DownloadsCleaner) Run() error {
	downloadsDir, err := c.getDownloadsDir()
	if err != nil {
		return err
	}
	dryRun, err := c.isDryRun()
	if err != nil {
		return err
	}

	return cleanDirectoryBasedOnMtime(downloadsDir, downloadsCutoffAge, dryRun, c.stdout, c.stderr)
}
