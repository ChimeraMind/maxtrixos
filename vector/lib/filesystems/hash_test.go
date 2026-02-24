package filesystems

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSha256(t *testing.T) {
	tmpDir := t.TempDir()
	name := "test.img"
	content := []byte("test data")
	inputPath := filepath.Join(tmpDir, name)
	if err := os.WriteFile(inputPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	sha256Path := filepath.Join(tmpDir, name+".sha256")

	if err := Sha256(inputPath, sha256Path); err != nil {
		t.Fatalf("Sha256 failed: %v", err)
	}

	data, err := os.ReadFile(sha256Path)
	if err != nil {
		t.Fatalf("Failed to read sha256 file: %v", err)
	}

	expectedHash := sha256.Sum256(content)
	expectedHex := hex.EncodeToString(expectedHash[:])
	expectedLine := fmt.Sprintf("%s  %s\n", expectedHex, name)
	if string(data) != expectedLine {
		t.Errorf("sha256 file content mismatch:\ngot:  %q\nwant: %q", string(data), expectedLine)
	}
}

func TestSha256_ContainsFilename(t *testing.T) {
	tmpDir := t.TempDir()
	name := "myimage.img.xz"
	inputPath := filepath.Join(tmpDir, name)
	if err := os.WriteFile(inputPath, []byte("compressed"), 0644); err != nil {
		t.Fatal(err)
	}
	sha256Path := filepath.Join(tmpDir, name+".sha256")

	if err := Sha256(inputPath, sha256Path); err != nil {
		t.Fatalf("Sha256 failed: %v", err)
	}

	data, err := os.ReadFile(sha256Path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), name) {
		t.Errorf("sha256 file doesn't contain filename %q: %s", name, string(data))
	}
}

func TestSha256_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	err := Sha256(filepath.Join(tmpDir, "nonexistent.img"), filepath.Join(tmpDir, "out.sha256"))
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}
}

func TestSha256_EmptyFile(t *testing.T) {
	tmpDir := t.TempDir()
	name := "empty.img"
	inputPath := filepath.Join(tmpDir, name)
	if err := os.WriteFile(inputPath, []byte{}, 0644); err != nil {
		t.Fatal(err)
	}
	sha256Path := filepath.Join(tmpDir, name+".sha256")

	if err := Sha256(inputPath, sha256Path); err != nil {
		t.Fatalf("Sha256 failed: %v", err)
	}

	data, err := os.ReadFile(sha256Path)
	if err != nil {
		t.Fatal(err)
	}

	expectedHash := sha256.Sum256([]byte{})
	expectedHex := hex.EncodeToString(expectedHash[:])
	expectedLine := fmt.Sprintf("%s  %s\n", expectedHex, name)
	if string(data) != expectedLine {
		t.Errorf("sha256 file content mismatch:\ngot:  %q\nwant: %q", string(data), expectedLine)
	}
}

func TestSha256_LargeFile(t *testing.T) {
	tmpDir := t.TempDir()
	name := "large.img"
	// 1MB of data
	content := make([]byte, 1024*1024)
	for i := range content {
		content[i] = byte(i % 256)
	}
	inputPath := filepath.Join(tmpDir, name)
	if err := os.WriteFile(inputPath, content, 0644); err != nil {
		t.Fatal(err)
	}
	sha256Path := filepath.Join(tmpDir, name+".sha256")

	if err := Sha256(inputPath, sha256Path); err != nil {
		t.Fatalf("Sha256 failed: %v", err)
	}

	data, err := os.ReadFile(sha256Path)
	if err != nil {
		t.Fatal(err)
	}

	expectedHash := sha256.Sum256(content)
	expectedHex := hex.EncodeToString(expectedHash[:])
	expectedLine := fmt.Sprintf("%s  %s\n", expectedHex, name)
	if string(data) != expectedLine {
		t.Errorf("sha256 file content mismatch:\ngot:  %q\nwant: %q", string(data), expectedLine)
	}
}

func TestSha256_OutputDirNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	name := "test.img"
	inputPath := filepath.Join(tmpDir, name)
	if err := os.WriteFile(inputPath, []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	err := Sha256(inputPath, filepath.Join(tmpDir, "nodir", "out.sha256"))
	if err == nil {
		t.Fatal("expected error when output directory does not exist")
	}
}

func TestSha256_Sha256sumFormat(t *testing.T) {
	// Verify the output format has two spaces between hash and filename,
	// matching the sha256sum(1) utility format.
	tmpDir := t.TempDir()
	name := "format-test.img"
	inputPath := filepath.Join(tmpDir, name)
	if err := os.WriteFile(inputPath, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	sha256Path := filepath.Join(tmpDir, name+".sha256")

	if err := Sha256(inputPath, sha256Path); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(sha256Path)
	if err != nil {
		t.Fatal(err)
	}

	line := string(data)
	parts := strings.SplitN(line, "  ", 2)
	if len(parts) != 2 {
		t.Fatalf("expected format '<hash>  <name>\\n', got: %q", line)
	}
	if len(parts[0]) != 64 {
		t.Errorf("expected 64-char hex hash, got %d chars: %q", len(parts[0]), parts[0])
	}
	if strings.TrimRight(parts[1], "\n") != name {
		t.Errorf("expected filename %q, got %q", name, strings.TrimRight(parts[1], "\n"))
	}
}
