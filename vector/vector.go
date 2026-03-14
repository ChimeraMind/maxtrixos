// Package main is the main entry point for vector. Vector is the (future) matrixOS
// management toolkit for development, building, releasing, installing and managing
// matrixOS.
package main

import (
	"fmt"
	"matrixos/vector/commands"
	"matrixos/vector/lib/ostree"
	"os"
)

const (
	helpMessage = `matrixos' vector - Your OS handy tool (in the future...).
Usage:

  PROTOTYPE! Some features are not fully tested yet!

  help        - this command.
  branch      - vector branch command. Operates on OS ostree branches.
    show         show current OS ostree branch.
    list         list all the available OS branches.
    switch       switch to a new branch.
  upgrade     - system upgrade tool, wraps ostree.
  setupOS     - setup tool, configures passwords, accounts, languages, etc.
  readwrite   - temporarily (until next upgrade) turn OS into a (mutable) read-write system.
  jailbreak   - permanently turns this system into a regular mutable Gentoo.
  dev 	      - development toolkit command, orchestrates development workflow and tools.
    janitor      cleans up development toolkit artifacts, such as old images and downloads.
    vm           runs generated image tests using QEMU.
  build       - build toolkit command, orchestrates building OS artifacts.
    seeds        builds chroot filesystems using the configured seeders.
    release      generates a single OS release (ostree commit).
    releases     generates multiple OS releases across all detected seeders.
    image        generates a single OS image.
    images 	     generates multiple OS images based on released branches.
`
)

func main() {
	if len(os.Args) < 2 {
		fmt.Print(helpMessage)
		os.Exit(1)
	}

	ostree.SetupEnvironment()

	cmds := []commands.ICommand{
		commands.NewBranchCommand(),
		commands.NewUpgradeCommand(),
		commands.NewFlashCommand(),
		commands.NewReadWriteCommand(),
		commands.NewSetupOSCommand(),
		commands.NewJailbreakCommand(),
		commands.NewDevCommand(),
		commands.NewBuildCommand(),
	}

	cmdStr := os.Args[1]
	subcmdArgs := os.Args[2:]

	if cmdStr == "help" || cmdStr == "--help" || cmdStr == "-h" {
		fmt.Print(helpMessage)
		os.Exit(0)
	}

	for _, cmd := range cmds {
		if cmd.Name() == cmdStr {
			if err := cmd.Init(subcmdArgs); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			if err := cmd.Run(); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			os.Exit(0)
		}
	}

	fmt.Printf("Unknown command: %s\n", cmdStr)
	os.Exit(1)
}
