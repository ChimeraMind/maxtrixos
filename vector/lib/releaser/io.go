package releaser

import (
	"fmt"
	"io"
)

// SetStdout replaces the writer used for informational output.
func (r *Releaser) SetStdout(w io.Writer) { r.stdout = w }

// SetStderr replaces the writer used for warnings and errors.
func (r *Releaser) SetStderr(w io.Writer) { r.stderr = w }

// Stdout returns the current informational output writer.
func (r *Releaser) Stdout() io.Writer { return r.stdout }

// Stderr returns the current warning/error output writer.
func (r *Releaser) Stderr() io.Writer { return r.stderr }

// SetChrootDir sets the source chroot directory.
func (r *Releaser) SetChrootDir(dir string) { r.chrootDir = dir }

// ChrootDir returns the source chroot directory.
func (r *Releaser) ChrootDir() string { return r.chrootDir }

// SetImageDir sets the destination image directory.
// It validates that dir is a non-empty existing directory.
func (r *Releaser) SetImageDir(dir string) error {
	if err := checkImageDir(dir); err != nil {
		return err
	}
	r.imageDir = dir
	return nil
}

// ImageDir returns the destination image directory.
func (r *Releaser) ImageDir() string { return r.imageDir }

// SetRef sets the ostree ref (branch).
func (r *Releaser) SetRef(ref string) { r.ref = ref }

// Ref returns the ostree ref (branch).
func (r *Releaser) Ref() string { return r.ref }

// Print writes a formatted informational message to stdout.
func (r *Releaser) Print(format string, args ...any) {
	fmt.Fprintf(r.stdout, format, args...)
}

// PrintWarning writes a formatted warning message to stderr.
func (r *Releaser) PrintWarning(format string, args ...any) {
	fmt.Fprintf(r.stderr, format, args...)
}

// PrintError writes a formatted error/diagnostic message to stderr.
func (r *Releaser) PrintError(format string, args ...any) {
	fmt.Fprintf(r.stderr, format, args...)
}
