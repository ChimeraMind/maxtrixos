package filesystems

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Sha256 generates a sha256sum file for a given file.
// The output format matches the sha256sum(1) utility: "<hash>  <name>\n".
// inputPath is the full path to the file to hash,
// and outputPath is where the checksum file will be written.
func Sha256(inputPath, outputPath string) error {
	f, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", inputPath, err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash %s: %w", inputPath, err)
	}

	name := filepath.Base(inputPath)
	line := fmt.Sprintf("%s  %s\n", hex.EncodeToString(h.Sum(nil)), name)
	return os.WriteFile(outputPath, []byte(line), 0644)
}
