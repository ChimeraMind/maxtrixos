package filesystems

import "testing"

func TestParseHumanSize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		{"Bytes", "1024", 1024, false},
		{"Kilobytes", "1K", 1024, false},
		{"KilobytesLower", "1k", 1024, false},
		{"Megabytes", "200M", 200 * 1024 * 1024, false},
		{"MegabytesLower", "200m", 200 * 1024 * 1024, false},
		{"Gigabytes", "32G", 32 * 1024 * 1024 * 1024, false},
		{"GigabytesLower", "32g", 32 * 1024 * 1024 * 1024, false},
		{"Terabytes", "1T", 1024 * 1024 * 1024 * 1024, false},
		{"TerabytesLower", "1t", 1024 * 1024 * 1024 * 1024, false},
		{"Empty", "", 0, true},
		{"Invalid", "abc", 0, true},
		{"InvalidWithSuffix", "abcG", 0, true},
		{"Whitespace", "  32G  ", 32 * 1024 * 1024 * 1024, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseHumanSize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseHumanSize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseHumanSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
