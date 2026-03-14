package commands

import (
	"flag"
	"fmt"
	"strings"
)

// BuildCommand is an uber command for orchestrating build workflow and tools
// (image, seed, release, etc.).
type BuildCommand struct {
	fs          *flag.FlagSet
	subcommands map[string]func() ICommand
	sub         string
	args        []string
}

// NewBuildCommand creates a new BuildCommand
func NewBuildCommand() *BuildCommand {
	subcommands := map[string]func() ICommand{
		"image":   func() ICommand { return NewImageCommand() },
		"images":  func() ICommand { return NewImagesCommand() },
		"release": func() ICommand { return NewReleaseCommand() },
	}
	return &BuildCommand{
		fs:          flag.NewFlagSet("build", flag.ExitOnError),
		subcommands: subcommands,
	}
}

// Name returns the name of the command
func (c *BuildCommand) Name() string {
	return c.fs.Name()
}

// Init initializes the command
func (c *BuildCommand) Init(args []string) error {
	var names []string
	for name := range c.subcommands {
		names = append(names, name)
	}
	c.fs.Usage = func() {
		fmt.Printf("Usage: vector build <subcommand>\n")
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

// Run runs the command
func (c *BuildCommand) Run() error {
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
