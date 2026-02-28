package seeder

import (
	"bytes"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/runner"
)

// newTestSeeder returns a Seeder with mock dependencies suitable for unit tests.
func newTestSeeder() *Seeder {
	mr := runner.NewMockRunner()
	return &Seeder{
		cfg:    &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}},
		runner: mr.Run,
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
	}
}
