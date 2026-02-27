package commands

import (
	"fmt"
	"matrixos/vector/lib/ostree"
	"matrixos/vector/lib/config"
	"strings"
)

type BaseCommand struct {
	cfg config.IConfig
	ot  ostree.IOstree
}

// shortRef returns a short version of the ref for display purposes (e.g. "fcos" for "fcos/36/x86_64").
func (c *BaseCommand) shortRef(ref string) string {
	// remove the remote, get the first char.
	remote := ostree.ExtractRemoteFromRef(ref)
	if remote != "" {
		ref = ostree.CleanRemoteFromRef(ref)
		remote = fmt.Sprintf("%s:", string(remote[0]))
	}

	// for each element /, get the first letter.
	parts := strings.Split(ref, "/")
	var srefs []string
	for _, part := range parts {
		if part != "" {
			srefs = append(srefs, string(part[0]))
		}
	}
	return remote + strings.Join(srefs, "/")

}

// initBaseConfig initializes the base configuration for the command.
func (c *BaseCommand) initBaseConfig() error {
	cfg, err := config.NewBaseConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	c.cfg = cfg
	return nil
}

// initClientConfig initializes the client configuration for the command.
func (c *BaseCommand) initClientConfig() error {
	cfg, err := config.NewClientConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	if err := cfg.Load(); err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	c.cfg = cfg
	return nil
}

// initOstree initializes the ostree client for the command.
func (c *BaseCommand) initOstree() error {
	if c.cfg == nil {
		return fmt.Errorf("config not initialized")
	}
	opts := ostree.NewOstreeOptions{
		Config: c.cfg,
	}
	ot, err := ostree.NewOstree(opts)
	if err != nil {
		return fmt.Errorf("failed to initialize ostree: %w", err)
	}
	c.ot = ot
	return nil
}
