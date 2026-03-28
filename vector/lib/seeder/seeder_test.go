package seeder

import (
	"bytes"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/runner"
)

// newTestSeeder returns a Seeder with mock dependencies suitable for unit tests.
func newTestSeeder() *Seeder {
	mr := runner.NewMockRunner()
	cfg := &config.MockConfig{Items: map[string][]string{}, Bools: map[string]bool{}}
	return &Seeder{
		SeederConfig: NewSeederConfig(cfg),
		runner:       mr.Run,
		chrootRunner: mr.ChrootRun,
		stdout:       &bytes.Buffer{},
		stderr:       &bytes.Buffer{},
	}
}
