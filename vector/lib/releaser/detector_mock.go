package releaser

// Compile-time interface check.
var _ IReleaseDetector = (*MockReleaseDetector)(nil)

// MockReleaseDetector implements IReleaseDetector for testing.
type MockReleaseDetector struct {
	DetectLocalReleases_    []string
	DetectLocalReleasesErr  error
	DetectRemoteReleases_   []string
	DetectRemoteReleasesErr error
}

// DefaultMockReleaseDetector returns a MockReleaseDetector with safe zero values.
func DefaultMockReleaseDetector() *MockReleaseDetector {
	return &MockReleaseDetector{}
}

func (m *MockReleaseDetector) DetectLocalReleases(skip, only RefFilterFunc) ([]string, error) {
	return m.DetectLocalReleases_, m.DetectLocalReleasesErr
}

func (m *MockReleaseDetector) DetectRemoteReleases(skip, only RefFilterFunc) ([]string, error) {
	return m.DetectRemoteReleases_, m.DetectRemoteReleasesErr
}
