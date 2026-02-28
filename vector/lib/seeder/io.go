package seeder

import (
	"fmt"
	"io"
)

func (s *Seeder) SetStdout(w io.Writer) { s.stdout = w }
func (s *Seeder) SetStderr(w io.Writer) { s.stderr = w }
func (s *Seeder) Stdout() io.Writer     { return s.stdout }
func (s *Seeder) Stderr() io.Writer     { return s.stderr }

func (s *Seeder) Print(format string, args ...any) {
	fmt.Fprintf(s.stdout, format, args...)
}

func (s *Seeder) PrintWarning(format string, args ...any) {
	fmt.Fprintf(s.stderr, format, args...)
}

func (s *Seeder) PrintError(format string, args ...any) {
	fmt.Fprintf(s.stderr, format, args...)
}
