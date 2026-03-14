package commands

import (
	"bufio"
	"fmt"
	"io"
	"matrixos/vector/lib/config"
	"matrixos/vector/lib/ostree"
	"regexp"
	"strings"
)

// Prompter provides interactive input prompting with validation.
// It is designed to be reusable across any command that needs user input.
type Prompter struct {
	Scanner *bufio.Scanner
	Stdout  io.Writer
	Stderr  io.Writer
	UI      *UI
}

// NewPrompter creates a Prompter from a reader and writers.
func NewPrompter(stdin io.Reader, stdout, stderr io.Writer, ui *UI) *Prompter {
	return &Prompter{
		Scanner: bufio.NewScanner(stdin),
		Stdout:  stdout,
		Stderr:  stderr,
		UI:      ui,
	}
}

// AskInput prompts the user for input with a default value and optional
// regex validation.  Returns the user's input or the default value if
// empty input is given (or on EOF).
func (p *Prompter) AskInput(prompt, defaultVal string, pattern *regexp.Regexp) (string, error) {
	for {
		defDisplay := defaultVal
		if defDisplay == "" {
			defDisplay = "none"
		}
		fmt.Fprintf(p.Stdout, "   %s%s%s (default: %s): %s",
			p.UI.cYellow, p.UI.iconQuestion, prompt, defDisplay, p.UI.cReset)

		if !p.Scanner.Scan() {
			if err := p.Scanner.Err(); err != nil {
				return "", fmt.Errorf("failed to read input: %w", err)
			}
			// EOF — use default.
			return defaultVal, nil
		}
		input := strings.TrimSpace(p.Scanner.Text())

		if input == "" {
			return defaultVal, nil
		}

		if pattern != nil && !pattern.MatchString(input) {
			fmt.Fprintf(p.Stderr, "   %s%sInvalid input format. Please try again.%s\n",
				p.UI.cRed, p.UI.iconError, p.UI.cReset)
			continue
		}
		return input, nil
	}
}

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

// splitCSV splits a comma-separated string into a trimmed slice of strings.
// Empty input returns nil.
func SplitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	var result []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// makeSkipFilter returns a filter function that returns true for names
// present in the skip list.  Returns nil when the list is empty.
func makeSkipFilter(skip []string) func(string) bool {
	if len(skip) == 0 {
		return nil
	}
	set := make(map[string]bool, len(skip))
	for _, s := range skip {
		set[s] = true
	}
	return func(name string) bool { return set[name] }
}

// makeOnlyFilter returns a filter function that returns true for names
// present in the allow list.  Returns nil when the list is empty
// (meaning all names pass).
func makeOnlyFilter(only []string) func(string) bool {
	if len(only) == 0 {
		return nil
	}
	set := make(map[string]bool, len(only))
	for _, s := range only {
		set[s] = true
	}
	return func(name string) bool { return set[name] }
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

// resolveRefRemoteResult holds the result of resolveRefRemote.
type resolveRefRemoteResult struct {
	Ref    string // cleaned ref (remote prefix stripped if present)
	Remote string // resolved remote name
}

// resolveRefRemote checks whether ref contains a remote prefix
// (e.g. "origin:matrixos/...").  When it does the remote is extracted,
// a warning is emitted to warnf, and the Ostree.Remote config key is
// overridden via an overlay.  The returned result always contains the
// cleaned ref and the resolved remote, regardless of whether the ref
// contained an embedded remote.
func (c *BaseCommand) resolveRefRemote(ref string, warnf func(format string, args ...any)) (*resolveRefRemoteResult, error) {
	remote, err := c.ot.Remote()
	if err != nil {
		return nil, err
	}

	if remoted := ostree.ExtractRemoteFromRef(ref); remoted != "" {
		remote = remoted
		ref = ostree.CleanRemoteFromRef(ref)
		warnf(
			"WARNING: %s contains the remote reference, using remote=%s and ref=%s\n",
			ref, remote, ref)

		overlay := map[string][]string{
			"Ostree.Remote": {remote},
		}
		if err := c.cfg.AddOverlay(overlay); err != nil {
			return nil, fmt.Errorf("failed to add config overlay: %w", err)
		}
	}

	return &resolveRefRemoteResult{Ref: ref, Remote: remote}, nil
}

// initOstree initializes the ostree client for the command.
func (c *BaseCommand) initOstree() error {
	if c.cfg == nil {
		return fmt.Errorf("config not initialized")
	}
	ot, err := cds.NewOstree(cds.NewOstreeOptions{Config: c.cfg})
	if err != nil {
		return fmt.Errorf("failed to initialize ostree: %w", err)
	}
	c.ot = ot
	return nil
}

// initGpg initializes the GPG keychain for the command.
func (c *BaseCommand) initGpg() error {
	if err := c.ot.MaybeInitializeRemote(); err != nil {
		return fmt.Errorf("failed to initialize remote: %w", err)
	}
	if err := c.ot.MaybeInitializeGpg(); err != nil {
		return fmt.Errorf("failed to initialize GPG: %w", err)
	}
	return nil
}
