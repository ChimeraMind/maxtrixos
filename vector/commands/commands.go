package commands

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

// ICommand defines the interface for a vector command
type ICommand interface {
	// Name returns the name of the command.
	Name() string
	// Init initializes the command with the given arguments.
	Init(args []string) error
	// Run executes the command.
	Run() error
}

// SignalGuard manages a LIFO stack of cleanup functions that are executed
// on process termination signals (SIGINT, SIGTERM) or when RunWithGuard
// catches a panic.  It mirrors the bash `trap clean_exit EXIT` pattern
// used, for example, in image_main.sh.
//
// Embed it in any command struct and call Arm() + PushCleanup() to register
// cleanup work.  Disarm() stops the signal listener (idempotent).
//
// Usage:
//
//	func (c *MyCommand) Run() error {
//	    c.Arm()
//	    defer c.Disarm()
//
//	    c.PushCleanup(func() { unmountAll() })
//	    c.PushCleanup(func() { removeTemp() })
//	    ...
//	}
//
// Or use RunWithGuard for automatic panic recovery:
//
//	func (c *MyCommand) Run() error {
//	    return c.RunWithGuard(func() error {
//	        c.PushCleanup(func() { unmountAll() })
//	        ...
//	    })
//	}
type SignalGuard struct {
	mu       sync.Mutex
	cleanups []func()
	armed    bool
	sigCh    chan os.Signal
	done     chan struct{}
}

// Arm starts listening for SIGINT and SIGTERM.  On receipt the full
// cleanup stack is executed (LIFO) and the process exits with code 1.
// Arm is idempotent — calling it on an already-armed guard is a no-op.
func (sg *SignalGuard) Arm() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if sg.armed {
		return
	}
	sg.armed = true
	sg.sigCh = make(chan os.Signal, 1)
	sg.done = make(chan struct{})

	signal.Notify(sg.sigCh, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		select {
		case sig := <-sg.sigCh:
			fmt.Fprintf(os.Stderr, "\nReceived signal %s, running cleanup ...\n", sig)
			sg.runCleanups()
			os.Exit(1)
		case <-sg.done:
			return
		}
	}()
}

// Disarm stops the signal listener and clears the cleanup stack.
// It is safe to call multiple times.
func (sg *SignalGuard) Disarm() {
	sg.mu.Lock()
	defer sg.mu.Unlock()

	if !sg.armed {
		return
	}
	sg.armed = false
	signal.Stop(sg.sigCh)
	close(sg.done)
	sg.cleanups = nil
}

// PushCleanup adds a cleanup function to the top of the LIFO stack.
// Cleanups are executed in reverse order of registration (last-in, first-out)
// exactly like nested bash trap handlers.
func (sg *SignalGuard) PushCleanup(fn func()) {
	sg.mu.Lock()
	defer sg.mu.Unlock()
	sg.cleanups = append(sg.cleanups, fn)
}

// RunCleanups executes all registered cleanup functions in LIFO order
// and clears the stack.  It can be called explicitly (e.g. from defer)
// in addition to being invoked automatically by the signal handler.
// Each cleanup runs in its own recovery block so a panicking cleanup
// does not prevent the remaining ones from executing.
func (sg *SignalGuard) RunCleanups() {
	sg.runCleanups()
}

// runCleanups is the internal, lock-free implementation.
func (sg *SignalGuard) runCleanups() {
	sg.mu.Lock()
	fns := make([]func(), len(sg.cleanups))
	copy(fns, sg.cleanups)
	sg.cleanups = nil
	sg.mu.Unlock()

	// Execute in LIFO order.
	for i := len(fns) - 1; i >= 0; i-- {
		func() {
			defer func() {
				if r := recover(); r != nil {
					fmt.Fprintf(os.Stderr, "panic during cleanup: %v\n", r)
				}
			}()
			fns[i]()
		}()
	}
}

// RunWithGuard is a convenience wrapper that arms the guard, runs fn,
// ensures cleanups run on return (normal or panic), and then disarms.
func (sg *SignalGuard) RunWithGuard(fn func() error) (retErr error) {
	sg.Arm()

	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic caught by SignalGuard: %v\n", r)
			retErr = fmt.Errorf("panic: %v", r)
		}
		sg.runCleanups()
		sg.Disarm()
	}()

	return fn()
}

// UI provides common UI styles and icons for commands
type UI struct {
	// UI Styles
	cReset, cRed, cGreen, cYellow, cBlue string
	cMagenta, cCyan, cWhite, cBold       string

	// UI Icons
	iconSearch, iconDownload, iconCheck         string
	iconUpdate, iconPackage                     string
	iconQuestion, iconRocket, iconGear, iconDoc string
	iconNew, iconError, iconWarn                string
	separator                                   string

	// TTY detection result (set by StartUI)
	isTTY bool

	// Per-command styled printers initialised by SetupPrinters.
	printer    *styledWriter
	errPrinter *styledWriter
}

// IsTTY returns whether stdout was detected as a terminal.
func (ui *UI) IsTTY() bool {
	return ui.isTTY
}

// NewStdoutWriter creates a styled writer for os.Stdout using the UI
// theme.  The returned writer can be passed to sub-processes or
// libraries that need a prefixed, TTY-aware io.Writer.
func (ui *UI) NewStdoutWriter(name string) *styledWriter {
	prefix := fmt.Sprintf("%s%s%s[%s]%s", ui.cBold, ui.cGreen, ui.iconGear, name, ui.cReset)
	return newStyledWriter(os.Stdout, prefix, ui.cGreen, ui.cReset, ui.isTTY)
}

// NewStderrWriter creates a styled writer for os.Stderr using the UI
// theme.  See NewStdoutWriter for details.
func (ui *UI) NewStderrWriter(name string) *styledWriter {
	prefix := fmt.Sprintf("%s%s%s[%s]%s", ui.cBold, ui.cRed, ui.iconError, name, ui.cReset)
	return newStyledWriter(os.Stderr, prefix, ui.cYellow, ui.cReset, ui.isTTY)
}

// SetupPrinters creates the default stdout/stderr styled writers for
// the given command name.  After calling this, Println / Printf /
// PrintErr / PrintErrf write through these writers.
func (ui *UI) SetupPrinters(name string) {
	ui.printer = ui.NewStdoutWriter(name)
	ui.errPrinter = ui.NewStderrWriter(name)
}

// Println prints a styled line to the command's stdout writer.
// If SetupPrinters has not been called it falls back to fmt.Println.
func (ui *UI) Println(args ...interface{}) {
	if ui.printer == nil {
		fmt.Println(args...)
		return
	}
	fmt.Fprintln(ui.printer, fmt.Sprint(args...))
}

// Printf prints a styled formatted message to the command's stdout
// writer.  Include a trailing \n if you want the line flushed
// immediately.
func (ui *UI) Printf(format string, args ...interface{}) {
	if ui.printer == nil {
		fmt.Printf(format, args...)
		return
	}
	fmt.Fprintf(ui.printer, format, args...)
}

// PrintErr prints a styled line to the command's stderr writer.
func (ui *UI) PrintErr(args ...interface{}) {
	if ui.errPrinter == nil {
		fmt.Fprintln(os.Stderr, args...)
		return
	}
	fmt.Fprintln(ui.errPrinter, fmt.Sprint(args...))
}

// PrintErrf prints a styled formatted message to the command's stderr writer.
func (ui *UI) PrintErrf(format string, args ...interface{}) {
	if ui.errPrinter == nil {
		fmt.Fprintf(os.Stderr, format, args...)
		return
	}
	fmt.Fprintf(ui.errPrinter, format, args...)
}

// FlushPrinters flushes any buffered content in both styled writers.
func (ui *UI) FlushPrinters() {
	if ui.printer != nil {
		ui.printer.Flush()
	}
	if ui.errPrinter != nil {
		ui.errPrinter.Flush()
	}
}

// StartUI initializes the UI component with environment detection
func (ui *UI) StartUI() {
	useColor := false
	useEmoji := false

	// Check if stdout is a terminal
	_, err := unix.IoctlGetTermios(int(os.Stdout.Fd()), unix.TCGETS)
	isTerm := err == nil
	ui.isTTY = isTerm

	if isTerm {
		termEnv := os.Getenv("TERM")
		if termEnv != "dumb" {
			useColor = true
		}
		// Linux console has limited font support
		if termEnv != "linux" {
			useEmoji = true
		}
	}

	if useColor {
		ui.cReset = "\033[0m"
		ui.cRed = "\033[31m"
		ui.cGreen = "\033[32m"
		ui.cYellow = "\033[33m"
		ui.cBlue = "\033[34m"
		ui.cMagenta = "\033[35m"
		ui.cCyan = "\033[36m"
		ui.cWhite = "\033[37m"
		ui.cBold = "\033[1m"
	}

	if useEmoji {
		ui.iconSearch = "○ "
		ui.iconDownload = "⇩ "
		ui.iconCheck = "✔ "
		ui.iconUpdate = "↻ "
		ui.iconPackage = "▤ "
		ui.iconQuestion = "? "
		ui.iconRocket = "➤ "
		ui.iconGear = "⚙ "
		ui.iconDoc = "≡ "
		ui.iconNew = "★ "
		ui.iconError = "✖ "
		ui.iconWarn = "⚠ "
		ui.separator = "   ───────────────────────────────────────────────────"
	} else {
		ui.iconSearch = "[?] "
		ui.iconDownload = "[v] "
		ui.iconCheck = "[OK] "
		ui.iconUpdate = "[~] "
		ui.iconPackage = "[#] "
		ui.iconQuestion = "[?] "
		ui.iconRocket = "[>] "
		ui.iconGear = "[*] "
		ui.iconDoc = "[f] "
		ui.iconNew = "[+] "
		ui.iconError = "[X] "
		ui.iconWarn = "[!] "
		ui.separator = "   ---------------------------------------------------"
	}
}

var execCommand = exec.Command
var getEuid = os.Geteuid
