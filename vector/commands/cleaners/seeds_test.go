package cleaners

import (
	"errors"
	"matrixos/vector/lib/config"
	"os"
	"path/filepath"
	"testing"
)

func TestSeedsCleaner_Name(t *testing.T) {
	cleaner := &SeedsCleaner{}
	if cleaner.Name() != "seeds" {
		t.Errorf("Expected name to be 'seeds', but got '%s'", cleaner.Name())
	}
}

func TestSeedsCleaner_Init(t *testing.T) {
	cleaner := &SeedsCleaner{}
	mockCfg := &config.MockConfig{Items: map[string][]string{}}
	err := cleaner.Init(mockCfg)
	if err != nil {
		t.Errorf("Init should not return an error, but got: %v", err)
	}
	if cleaner.cfg != mockCfg {
		t.Error("cfg should be initialized with the provided config")
	}
}

func TestSeedsCleaner_isDryRun(t *testing.T) {
	tests := []struct {
		name     string
		dryRun   string
		expected bool
		wantErr  bool
	}{
		{"DryRunTrue", "true", true, false},
		{"DryRunFalse", "false", false, false},
		{"DryRunNotSet", "", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCfg := &config.MockConfig{Items: map[string][]string{}}
			if tt.dryRun != "" {
				mockCfg.Items["SeedsCleaner.DryRun"] = []string{tt.dryRun}
			}
			cleaner := &SeedsCleaner{cfg: mockCfg}
			got, err := cleaner.isDryRun()
			if (err != nil) != tt.wantErr {
				t.Errorf("isDryRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("isDryRun() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSeedsCleaner_isDryRun_Error(t *testing.T) {
	errCfg := &config.ErrConfig{Err: errors.New("config error")}
	cleaner := &SeedsCleaner{cfg: errCfg}
	_, err := cleaner.isDryRun()
	if err == nil {
		t.Error("isDryRun() should return an error when config fails")
	}
}

func TestSeedsCleaner_MinAmountOfSeeds(t *testing.T) {
	tests := []struct {
		name     string
		val      string
		expected int
		wantErr  bool
	}{
		{"Valid", "5", 5, false},
		{"ValidZero", "0", 0, false},
		{"ValidLarge", "100", 100, false},
		{"Invalid", "abc", 0, true},
		{"NotSet", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCfg := &config.MockConfig{Items: map[string][]string{}}
			if tt.name != "NotSet" {
				mockCfg.Items["SeedsCleaner.MinAmountOfSeeds"] = []string{tt.val}
			}
			cleaner := &SeedsCleaner{cfg: mockCfg}
			got, err := cleaner.MinAmountOfSeeds()
			if (err != nil) != tt.wantErr {
				t.Errorf("MinAmountOfSeeds() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.expected {
				t.Errorf("MinAmountOfSeeds() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSeedsCleaner_MinAmountOfSeeds_Error(t *testing.T) {
	errCfg := &config.ErrConfig{Err: errors.New("config error")}
	cleaner := &SeedsCleaner{cfg: errCfg}
	_, err := cleaner.MinAmountOfSeeds()
	if err == nil {
		t.Error("MinAmountOfSeeds() should return an error when config fails")
	}
}

func TestChrootDirNamePattern(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		matches bool
		groups  []string // [full, name, date]
	}{
		{"ValidBedrock", "bedrock-20260101", true, []string{"bedrock-20260101", "bedrock", "20260101"}},
		{"ValidServer", "server-20261231", true, []string{"server-20261231", "server", "20261231"}},
		{"ValidGnome", "gnome-20260305", true, []string{"gnome-20260305", "gnome", "20260305"}},
		{"ValidUnderscores", "my_seed-20260101", true, []string{"my_seed-20260101", "my_seed", "20260101"}},
		{"ValidNumbers", "seed123-20260101", true, []string{"seed123-20260101", "seed123", "20260101"}},
		{"NoDate", "bedrock-abc", false, nil},
		{"NoDash", "bedrock20260101", false, nil},
		{"Empty", "", false, nil},
		{"ShortDate", "bedrock-2026010", false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := ChrootDirNamePattern.FindStringSubmatch(tt.input)
			if tt.matches && matches == nil {
				t.Errorf("Expected %q to match ChrootDirNamePattern", tt.input)
			}
			if !tt.matches && matches != nil {
				t.Errorf("Expected %q to NOT match ChrootDirNamePattern, got %v", tt.input, matches)
			}
			if tt.matches && matches != nil {
				if len(matches) < 3 {
					t.Fatalf("Expected at least 3 groups, got %d: %v", len(matches), matches)
				}
				for i, expected := range tt.groups {
					if matches[i] != expected {
						t.Errorf("Group %d: expected %q, got %q", i, expected, matches[i])
					}
				}
			}
		})
	}
}

func TestSortChrootDirs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			"ChronologicalOrder",
			[]string{
				"/chroots/bedrock-20260103",
				"/chroots/bedrock-20260101",
				"/chroots/bedrock-20260102",
			},
			[]string{
				"/chroots/bedrock-20260101",
				"/chroots/bedrock-20260102",
				"/chroots/bedrock-20260103",
			},
		},
		{
			"AlreadySorted",
			[]string{
				"/chroots/bedrock-20260101",
				"/chroots/bedrock-20260102",
				"/chroots/bedrock-20260103",
			},
			[]string{
				"/chroots/bedrock-20260101",
				"/chroots/bedrock-20260102",
				"/chroots/bedrock-20260103",
			},
		},
		{
			"ReverseOrder",
			[]string{
				"/chroots/bedrock-20260103",
				"/chroots/bedrock-20260102",
				"/chroots/bedrock-20260101",
			},
			[]string{
				"/chroots/bedrock-20260101",
				"/chroots/bedrock-20260102",
				"/chroots/bedrock-20260103",
			},
		},
		{
			"SingleElement",
			[]string{"/chroots/bedrock-20260101"},
			[]string{"/chroots/bedrock-20260101"},
		},
		{
			"Empty",
			[]string{},
			[]string{},
		},
		{
			"MixedSeeds",
			[]string{
				"/chroots/gnome-20260201",
				"/chroots/bedrock-20260101",
				"/chroots/gnome-20260101",
				"/chroots/bedrock-20260201",
			},
			[]string{
				"/chroots/bedrock-20260101",
				"/chroots/gnome-20260101",
				"/chroots/gnome-20260201",
				"/chroots/bedrock-20260201",
			},
		},
		{
			"NonMatchingEntries",
			[]string{
				"/chroots/invalid",
				"/chroots/bedrock-20260101",
				"/chroots/other",
			},
			// Non-matching entries keep relative order (stable behavior not guaranteed,
			// but they compare as equal, so the sort should not rearrange them).
			[]string{
				"/chroots/invalid",
				"/chroots/bedrock-20260101",
				"/chroots/other",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sortChrootDirs(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("Length mismatch: got %d, want %d", len(got), len(tt.expected))
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("Index %d: got %q, want %q", i, got[i], tt.expected[i])
				}
			}
		})
	}
}

func TestFilterChrootEntry(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-filter-chroot-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a regular file that matches the pattern.
	matchingFile := "bedrock-20260101.img.xz"
	matchingPath := filepath.Join(tempDir, matchingFile)
	if err := os.WriteFile(matchingPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create matching file: %v", err)
	}

	// Create a regular file that does NOT match the pattern.
	nonMatchingFile := "random.txt"
	nonMatchingPath := filepath.Join(tempDir, nonMatchingFile)
	if err := os.WriteFile(nonMatchingPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create non-matching file: %v", err)
	}

	// Create a directory.
	subDir := "bedrock-20260102.img.xz"
	subDirPath := filepath.Join(tempDir, subDir)
	if err := os.Mkdir(subDirPath, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp dir: %v", err)
	}

	entryMap := make(map[string]os.DirEntry)
	for _, e := range entries {
		entryMap[e.Name()] = e
	}

	tests := []struct {
		name     string
		fileName string
		expected bool
	}{
		{"MatchingRegularFile", matchingFile, true},
		{"NonMatchingFile", nonMatchingFile, false},
		{"Directory", subDir, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry, ok := entryMap[tt.fileName]
			if !ok {
				t.Fatalf("Entry %s not found in temp dir", tt.fileName)
			}
			path := filepath.Join(tempDir, tt.fileName)
			got := filterChrootEntry(ChrootDirNamePattern, path, entry)
			if got != tt.expected {
				t.Errorf("filterChrootEntry(%q) = %v, want %v", tt.fileName, got, tt.expected)
			}
		})
	}
}

func TestFilterChrootEntry_NonExistent(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-filter-nonexistent-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a temp file so we can get a valid DirEntry, then delete it.
	tmpFile := filepath.Join(tempDir, "gone-20260101.img.xz")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}
	// Remove the file so Lstat will fail.
	os.Remove(tmpFile)

	got := filterChrootEntry(ChrootDirNamePattern, tmpFile, entries[0])
	if got {
		t.Error("filterChrootEntry should return false for non-existent path")
	}
}

func TestFilterChrootEntry_Symlink(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "test-filter-symlink-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a target file.
	target := filepath.Join(tempDir, "target.img.xz")
	if err := os.WriteFile(target, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	// Create a symlink with a matching name.
	symlinkName := "bedrock-20260101.img.xz"
	symlinkPath := filepath.Join(tempDir, symlinkName)
	if err := os.Symlink(target, symlinkPath); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	entries, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read dir: %v", err)
	}

	var symlinkEntry os.DirEntry
	for _, e := range entries {
		if e.Name() == symlinkName {
			symlinkEntry = e
			break
		}
	}
	if symlinkEntry == nil {
		t.Fatal("Symlink entry not found in directory listing")
	}

	// Lstat on a symlink returns the symlink itself, which is not a regular file.
	got := filterChrootEntry(ChrootDirNamePattern, symlinkPath, symlinkEntry)
	if got {
		t.Error("filterChrootEntry should return false for symlinks (Lstat reports non-regular)")
	}
}
