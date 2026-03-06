package commands

import (
	"flag"
	"fmt"

	"matrixos/vector/lib/validation"
)

// CheckCommand verifies that the host has all the required binaries
// and directories to run the matrixOS build, release and image
// workflows.
type CheckCommand struct {
	BaseCommand
	UI
	fs *flag.FlagSet

	qa *validation.QA
}

// NewCheckCommand creates a new CheckCommand.
func NewCheckCommand() *CheckCommand {
	return &CheckCommand{
		fs: flag.NewFlagSet("check", flag.ContinueOnError),
	}
}

func (c *CheckCommand) Name() string {
	return c.fs.Name()
}

func (c *CheckCommand) Init(args []string) error {
	if err := c.initBaseConfig(); err != nil {
		return err
	}
	c.fs.Usage = func() {
		fmt.Printf("Usage: vector dev %s\n", c.Name())
		c.fs.PrintDefaults()
	}
	if err := c.fs.Parse(args); err != nil {
		return err
	}

	qa, err := validation.New(c.cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize QA: %w", err)
	}
	c.qa = qa

	c.StartUI()

	return nil
}

func (c *CheckCommand) Run() error {
	c.SetupPrinters("check")
	defer c.FlushPrinters()

	c.Println("If the following lines contain errors, install the respective packages.")
	c.Println("")

	allOk := true

	// Verify seeder environment.
	c.Printf("%sChecking for seeders support (to build root filesystem from gentoo stage3):%s\n",
		c.cBold, c.cReset)
	if err := c.qa.VerifySeederEnvironmentSetup("/"); err != nil {
		c.PrintErrf("%s%s%s\n", c.cRed, err, c.cReset)
		allOk = false
	} else {
		c.Printf("  %s%sOK%s\n", c.cGreen, c.iconCheck, c.cReset)
	}
	c.Println("")

	// Verify releaser environment.
	c.Printf("%sChecking for releaser support (to release chroots to ostree):%s\n",
		c.cBold, c.cReset)
	if err := c.qa.VerifyReleaserEnvironmentSetup("/"); err != nil {
		c.PrintErrf("%s%s%s\n", c.cRed, err, c.cReset)
		allOk = false
	} else {
		c.Printf("  %s%sOK%s\n", c.cGreen, c.iconCheck, c.cReset)
	}
	c.Println("")

	// Verify imager environment.
	c.Printf("%sChecking for imager support (to create bootable images):%s\n",
		c.cBold, c.cReset)
	if err := c.qa.VerifyImagerEnvironmentSetup("/"); err != nil {
		c.PrintErrf("%s%s%s\n", c.cRed, err, c.cReset)
		allOk = false
	} else {
		c.Printf("  %s%sOK%s\n", c.cGreen, c.iconCheck, c.cReset)
	}
	c.Println("")

	if allOk {
		c.Printf("%s%sAll environment checks passed.%s\n",
			c.cGreen, c.iconCheck, c.cReset)
	} else {
		return fmt.Errorf("one or more environment checks failed")
	}

	return nil
}
