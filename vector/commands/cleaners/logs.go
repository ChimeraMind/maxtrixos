package cleaners

import (
	"fmt"
	"io"
	"matrixos/vector/lib/config"
	"matrixos/vector/lib/filesystems"
	"path"
	"time"
)

const (
	logsCutoffAge = 30 * 24 * time.Hour
)

type LogsCleaner struct {
	cfg    config.IConfig
	stdout io.Writer
	stderr io.Writer
}

func (c *LogsCleaner) Name() string {
	return "logs"
}

func (c *LogsCleaner) Init(cfg config.IConfig, stdout, stderr io.Writer) error {
	c.cfg = cfg
	c.stdout = stdout
	c.stderr = stderr
	return nil
}

func (c *LogsCleaner) isDryRun() (bool, error) {
	val, err := c.cfg.GetItem("LogsCleaner.DryRun")
	if err != nil {
		return false, err
	}
	return val == "true", nil
}

func (c *LogsCleaner) getLogsDir() (string, error) {
	val, err := c.cfg.GetItem("matrixOS.LogsDir")
	if err != nil {
		return "", err
	}
	if val == "" {
		return "", fmt.Errorf("matrixOS.LogsDir is not set")
	}
	return val, nil
}

func (c *LogsCleaner) Run() error {
	logsDir, err := c.getLogsDir()
	if err != nil {
		return err
	}
	if !filesystems.DirectoryExists(logsDir) {
		fmt.Fprintf(c.stderr, "Logs directory %s does not exist. Nothing to do.\n", logsDir)
		return nil
	}

	dryRun, err := c.isDryRun()
	if err != nil {
		return err
	}

	fmt.Fprintf(c.stdout, "Cleaning old logs from %s ...\n", logsDir)

	dirs := []string{
		path.Join(logsDir, "weekly-builder"),
	}
	for _, dir := range dirs {
		err := cleanDirectoryBasedOnMtime(dir, logsCutoffAge, dryRun, c.stdout, c.stderr)
		if err != nil {
			return err
		}
	}
	return nil
}
