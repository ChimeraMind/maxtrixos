package releaser

import (
	"fmt"
	"io"
	"os"
)

// Compile-time interface check.
var _ IRelease = (*MockReleaser)(nil)

// MockReleaser implements IRelease for testing commands.
// Only the fields/methods relevant to each test need to be configured;
// everything else returns safe zero values.
type MockReleaser struct {
	stdout io.Writer
	stderr io.Writer

	// Build / Cleanup
	BuildErr    error
	BuildCalled bool

	CleanupCalled bool

	// Locking
	ExecuteWithReleaseLockErr error
}

// DefaultMockReleaser returns a MockReleaser with sensible defaults for tests.
func DefaultMockReleaser() *MockReleaser {
	return &MockReleaser{}
}

func (m *MockReleaser) SetStdout(w io.Writer) { m.stdout = w }
func (m *MockReleaser) SetStderr(w io.Writer) { m.stderr = w }

func (m *MockReleaser) Print(format string, args ...any) {
	w := m.stdout
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, format, args...)
}

func (m *MockReleaser) ExecuteWithReleaseLock(fn func() error) error {
	if m.ExecuteWithReleaseLockErr != nil {
		return m.ExecuteWithReleaseLockErr
	}
	return fn()
}

func (m *MockReleaser) Build() error {
	m.BuildCalled = true
	return m.BuildErr
}

func (m *MockReleaser) Cleanup() {
	m.CleanupCalled = true
}
