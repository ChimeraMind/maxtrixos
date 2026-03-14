package seeder

import "io"

// Compile-time interface check.
var _ ISeederDetector = (*MockSeederDetector)(nil)

// MockSeederDetector implements ISeederDetector for testing.
type MockSeederDetector struct {
	Detect_    []SeederInfo
	DetectErr  error
	SetStderr_ io.Writer
	Stderr_    io.Writer
}

// DefaultMockSeederDetector returns a MockSeederDetector with safe zero values.
func DefaultMockSeederDetector() *MockSeederDetector {
	return &MockSeederDetector{}
}

func (m *MockSeederDetector) Detect(skip, only SeederFilterFunc) ([]SeederInfo, error) {
	return m.Detect_, m.DetectErr
}

func (m *MockSeederDetector) SetStderr(w io.Writer) {
	m.SetStderr_ = w
}

func (m *MockSeederDetector) Stderr() io.Writer {
	return m.Stderr_
}
