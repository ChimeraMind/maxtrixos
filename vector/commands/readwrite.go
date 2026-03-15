package commands

import (
	"flag"
	"fmt"
)

// ReadWriteCommand is a command for unlocking the system filesystem
// for writing. By default the unlock is transient (lost on reboot).
// Use --permanent to persist the overlay across reboots (hotfix mode).
type ReadWriteCommand struct {
	BaseCommand
	UI
	fs        *flag.FlagSet
	permanent bool
}

// NewReadWriteCommand creates a new ReadWriteCommand
func NewReadWriteCommand() *ReadWriteCommand {
	return &ReadWriteCommand{}
}

func (c *ReadWriteCommand) Name() string {
	return "readwrite"
}

func (c *ReadWriteCommand) Init(args []string) error {
	if err := c.parseArgs(args); err != nil {
		return err
	}

	if err := c.initClientConfig(); err != nil {
		return err
	}

	if err := c.initOstree(); err != nil {
		return err
	}

	c.StartUI()

	return nil
}

// parseArgs parses the command-line arguments without initializing config or ostree.
func (c *ReadWriteCommand) parseArgs(args []string) error {
	c.fs = flag.NewFlagSet("readwrite", flag.ContinueOnError)
	c.fs.BoolVar(&c.permanent, "permanent", false,
		"Make the unlock permanent (hotfix mode, persists across reboots)")
	c.fs.Usage = func() {
		fmt.Printf("Usage: vector %s [options]\n", c.Name())
		c.fs.PrintDefaults()
	}
	return c.fs.Parse(args)
}

func (c *ReadWriteCommand) Run() error {
	// Check if we are running as root. If running as user, exit with error.
	if getEuid() != 0 {
		return fmt.Errorf("this command must be run as root")
	}

	mode := "transient"
	if c.permanent {
		mode = "permanent (hotfix)"
	}

	fmt.Printf("%s%sUnlocking filesystem in %s mode...%s\n",
		c.cBold, c.iconGear, mode, c.cReset)

	if err := c.ot.Readwrite(c.permanent); err != nil {
		return fmt.Errorf("failed to unlock filesystem: %w", err)
	}

	fmt.Printf("%s%sFilesystem unlocked successfully.%s\n",
		c.cGreen, c.iconCheck, c.cReset)

	if !c.permanent {
		fmt.Printf("%s%sNote: changes will be lost on reboot. Use --permanent for hotfix mode.%s\n",
			c.cYellow, c.iconWarn, c.cReset)
	} else {
		fmt.Printf("%s%sWarning: hotfix mode is active. Changes persist across reboots until the next upgrade.%s\n",
			c.cYellow, c.iconWarn, c.cReset)
	}

	return nil
}
