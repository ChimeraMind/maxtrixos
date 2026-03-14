package ostree

import (
	"matrixos/vector/lib/config"
	"testing"
)

func TestBranchHelpers(t *testing.T) {
	tests := []struct {
		name     string
		ref      string
		remote   string
		cleanRef string
		isShort  bool
	}{
		{"Full Ref", "origin:matrixos/dev", "origin", "matrixos/dev", false},
		{"Short Ref", "gnome", "", "gnome", true},
		{"Ref No Remote", "matrixos/dev", "", "matrixos/dev", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ExtractRemoteFromRef(tt.ref); got != tt.remote {
				t.Errorf("ExtractRemoteFromRef(%q) = %q, want %q", tt.ref, got, tt.remote)
			}
			if got := CleanRemoteFromRef(tt.ref); got != tt.cleanRef {
				t.Errorf("CleanRemoteFromRef(%q) = %q, want %q", tt.ref, got, tt.cleanRef)
			}
			if got := IsBranchShortName(tt.cleanRef); got != tt.isShort {
				t.Errorf("IsBranchShortName(%q) = %v, want %v", tt.cleanRef, got, tt.isShort)
			}
		})
	}
}

func TestBranchShortnameToNormal(t *testing.T) {
	got, err := BranchShortnameToNormal("dev", "gnome", "matrixos", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "matrixos/amd64/dev/gnome"
	if got != want {
		t.Errorf("BranchShortnameToNormal = %q, want %q", got, want)
	}

	gotProd, err := BranchShortnameToNormal("prod", "gnome", "matrixos", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	wantProd := "matrixos/amd64/gnome"
	if gotProd != wantProd {
		t.Errorf("BranchShortnameToNormal(prod) = %q, want %q", gotProd, wantProd)
	}
}

func TestBranchContainsRemote(t *testing.T) {
	tests := []struct {
		branch string
		want   bool
	}{
		{"origin:branch", true},
		{"branch", false},
		{"remote:group/branch", true},
	}
	for _, tt := range tests {
		if got := BranchContainsRemote(tt.branch); got != tt.want {
			t.Errorf("BranchContainsRemote(%q) = %v, want %v", tt.branch, got, tt.want)
		}
	}
}

func TestFullBranchHelpers(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.FullBranchSuffix": {"full"},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	// IsBranchFullSuffixed
	if isFull, _ := o.IsBranchFullSuffixed("branch-full"); !isFull {
		t.Error("IsBranchFullSuffixed(branch-full) = false, want true")
	}
	if isFull, _ := o.IsBranchFullSuffixed("branch"); isFull {
		t.Error("IsBranchFullSuffixed(branch) = true, want false")
	}

	// BranchToFull
	if full, _ := o.BranchToFull("branch"); full != "branch-full" {
		t.Errorf("BranchToFull(branch) = %q, want branch-full", full)
	}
	if full, _ := o.BranchToFull("branch-full"); full != "branch-full" {
		t.Errorf("BranchToFull(branch-full) = %q, want branch-full", full)
	}

	// RemoveFullFromBranch
	if clean, _ := o.RemoveFullFromBranch("branch-full"); clean != "branch" {
		t.Errorf("RemoveFullFromBranch(branch-full) = %q, want branch", clean)
	}
	if clean, _ := o.RemoveFullFromBranch("branch"); clean != "branch" {
		t.Errorf("RemoveFullFromBranch(branch) = %q, want branch", clean)
	}

	// BranchShortnameToFull
	fullRef, err := o.BranchShortnameToFull("gnome", "dev", "matrixos", "amd64")
	if err != nil {
		t.Errorf("BranchShortnameToFull failed: %v", err)
	}
	wantFullRef := "matrixos/amd64/dev/gnome-full"
	if fullRef != wantFullRef {
		t.Errorf("BranchShortnameToFull = %q, want %q", fullRef, wantFullRef)
	}
}

func TestBranchHelpersErrors(t *testing.T) {
	if _, err := BranchShortnameToNormal("", "short", "os", "arch"); err == nil {
		t.Error("Should fail empty stage")
	}
	if _, err := BranchShortnameToNormal("stage", "", "os", "arch"); err == nil {
		t.Error("Should fail empty shortname")
	}
	if _, err := BranchShortnameToNormal("stage", "short", "", "arch"); err == nil {
		t.Error("Should fail empty os")
	}
	if _, err := BranchShortnameToNormal("stage", "short", "os", ""); err == nil {
		t.Error("Should fail empty arch")
	}
}

func TestOstreeBranchMethodsErrors(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.FullBranchSuffix": {"full"},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree failed: %v", err)
	}

	if _, err := o.IsBranchFullSuffixed(""); err == nil {
		t.Error("IsBranchFullSuffixed should fail empty ref")
	}
	if _, err := o.BranchShortnameToFull("", "stage", "os", "arch"); err == nil {
		t.Error("BranchShortnameToFull should fail empty shortname")
	}
	if _, err := o.BranchToFull(""); err == nil {
		t.Error("BranchToFull should fail empty ref")
	}
	if _, err := o.RemoveFullFromBranch(""); err == nil {
		t.Error("RemoveFullFromBranch should fail empty ref")
	}
}
