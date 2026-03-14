package seeder

// Compile-time interface check.
var _ ISeederDetector = (*MockSeederDetector)(nil)

// MockSeederDetector implements ISeederDetector for testing.
type MockSeederDetector struct {
	Detect_   []SeederInfo
	DetectErr error
}

// DefaultMockSeederDetector returns a MockSeederDetector with safe zero values.
func DefaultMockSeederDetector() *MockSeederDetector {
	return &MockSeederDetector{}
}

func (m *MockSeederDetector) Detect(skip, only SeederFilterFunc) ([]SeederInfo, error) {
	return m.Detect_, m.DetectErr
}
