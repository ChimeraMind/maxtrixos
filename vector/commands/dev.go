package commands

import (
	"flag"
	"fmt"
	"strings"
)

// DevCommand is a uber command for orchestrating the development toolkit and its workflow.
type DevCommand struct {
	fs          *flag.FlagSet
	subcommands map[string]func() ICommand
	sub         string
	args        []string
}

// NewDevCommand creates a new DevCommand
func NewDevCommand() *DevCommand {
	subcommands := map[string]func() ICommand{
		"check":   func() ICommand { return NewCheckCommand() },
		"enter":   func() ICommand { return NewEnterCommand() },
		"janitor": func() ICommand { return NewJanitorCommand() },
		"vm":      func() ICommand { return NewVMCommand() },
	}
	return &DevCommand{
		fs:          flag.NewFlagSet("dev", flag.ExitOnError),
		subcommands: subcommands,
	}
}

func (c *DevCommand) Name() string {
	return c.fs.Name()
}

func (c *DevCommand) Init(args []string) error {
	var names []string
	for name := range c.subcommands {
		names = append(names, name)
	}
	c.fs.Usage = func() {
		fmt.Printf("Usage: vector dev <subcommand>\n")
		fmt.Println("Subcommands: " + strings.Join(names, ", "))
		c.fs.PrintDefaults()
	}
	err := c.fs.Parse(args)
	if err != nil {
		return err
	}
	if c.fs.NArg() < 1 {
		c.fs.Usage()
		return fmt.Errorf("no subcommand provided")
	}
	c.sub = c.fs.Arg(0)
	c.args = c.fs.Args()[1:]
	return nil
}

func (c *DevCommand) Run() error {
	sf, ok := c.subcommands[c.sub]
	if !ok {
		return fmt.Errorf("unknown subcommand: %s", c.sub)
	}
	subcommand := sf()

	if err := subcommand.Init(c.args); err != nil {
		return fmt.Errorf("failed to initialize subcommand: %w", err)
	}
	if err := subcommand.Run(); err != nil {
		return fmt.Errorf("failed to run subcommand: %w", err)
	}

	return nil
}
