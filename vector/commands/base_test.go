package commands

import (
	"fmt"
	"strings"
	"testing"

	"matrixos/vector/lib/config"
	"matrixos/vector/lib/ostree"
)

// --- detectRemotedAndPlainRefs unit tests ---

func TestDetectRemotedAndPlainRefs_NoRefs(t *testing.T) {
	cmd := &BaseCommand{
		ot: &ostree.MockOstree{LocalRefs_: nil},
	}

	var errs []string
	errf := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	if err := cmd.detectRemotedAndPlainRefs(errf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no error messages, got: %v", errs)
	}
}

func TestDetectRemotedAndPlainRefs_UniqueRefs(t *testing.T) {
	cmd := &BaseCommand{
		ot: &ostree.MockOstree{
			LocalRefs_: []string{
				"matrixos/x86_64/dev/gnome",
				"matrixos/x86_64/dev/cosmic",
			},
		},
	}

	var errs []string
	errf := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	if err := cmd.detectRemotedAndPlainRefs(errf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no error messages, got: %v", errs)
	}
}

func TestDetectRemotedAndPlainRefs_RemotedOnly(t *testing.T) {
	// All refs have the same remote prefix — no conflict because
	// CleanRemoteFromRef produces unique keys.
	cmd := &BaseCommand{
		ot: &ostree.MockOstree{
			LocalRefs_: []string{
				"origin:matrixos/x86_64/dev/gnome",
				"origin:matrixos/x86_64/dev/cosmic",
			},
		},
	}

	var errs []string
	errf := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	if err := cmd.detectRemotedAndPlainRefs(errf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no error messages, got: %v", errs)
	}
}

func TestDetectRemotedAndPlainRefs_DuplicateRemotedAndPlain(t *testing.T) {
	// Both "origin:matrixos/x86_64/dev/gnome" and "matrixos/x86_64/dev/gnome"
	// exist — after stripping the remote they collide.
	cmd := &BaseCommand{
		ot: &ostree.MockOstree{
			LocalRefs_: []string{
				"origin:matrixos/x86_64/dev/gnome",
				"matrixos/x86_64/dev/gnome",
			},
		},
	}

	var errs []string
	errf := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	err := cmd.detectRemotedAndPlainRefs(errf)
	if err == nil {
		t.Fatal("expected error for ambiguous refs, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous refs detected") {
		t.Errorf("unexpected error message: %v", err)
	}
	if len(errs) == 0 {
		t.Fatal("expected error messages from errf callback")
	}
	// Should mention both variants.
	combined := strings.Join(errs, " ")
	if !strings.Contains(combined, "origin:matrixos/x86_64/dev/gnome") {
		t.Errorf("error message should mention the remoted ref, got: %s", combined)
	}
	if !strings.Contains(combined, "ostree refs --delete") {
		t.Errorf("error message should suggest ostree refs --delete, got: %s", combined)
	}
}

func TestDetectRemotedAndPlainRefs_MultipleDuplicates(t *testing.T) {
	// Multiple refs have both remoted and plain variants.
	cmd := &BaseCommand{
		ot: &ostree.MockOstree{
			LocalRefs_: []string{
				"origin:matrixos/x86_64/dev/gnome",
				"matrixos/x86_64/dev/gnome",
				"origin:matrixos/x86_64/dev/cosmic",
				"matrixos/x86_64/dev/cosmic",
				"matrixos/x86_64/dev/server", // no duplicate
			},
		},
	}

	var errs []string
	errf := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	err := cmd.detectRemotedAndPlainRefs(errf)
	if err == nil {
		t.Fatal("expected error for ambiguous refs, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous refs detected") {
		t.Errorf("unexpected error message: %v", err)
	}
	// At least 4 error lines (2 ERROR + 2 "Please remove" per duplicate set).
	if len(errs) < 4 {
		t.Errorf("expected at least 4 error messages, got %d: %v", len(errs), errs)
	}
}

func TestDetectRemotedAndPlainRefs_LocalRefsError(t *testing.T) {
	cmd := &BaseCommand{
		ot: &ostree.MockOstree{
			LocalRefsErr: fmt.Errorf("mock localrefs failure"),
		},
	}

	errf := func(format string, args ...any) {
		t.Errorf("errf should not be called, got: %s", fmt.Sprintf(format, args...))
	}

	err := cmd.detectRemotedAndPlainRefs(errf)
	if err == nil {
		t.Fatal("expected error from LocalRefs failure, got nil")
	}
	if !strings.Contains(err.Error(), "failed to list local refs") {
		t.Errorf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "mock localrefs failure") {
		t.Errorf("expected wrapped error, got: %v", err)
	}
}

func TestDetectRemotedAndPlainRefs_SameRemoteDifferentBranches(t *testing.T) {
	// Two remoted refs from different remotes that clean to different branches — no problem.
	cmd := &BaseCommand{
		ot: &ostree.MockOstree{
			LocalRefs_: []string{
				"origin:matrixos/x86_64/dev/gnome",
				"upstream:matrixos/x86_64/dev/cosmic",
			},
		},
	}

	var errs []string
	errf := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	if err := cmd.detectRemotedAndPlainRefs(errf); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(errs) != 0 {
		t.Errorf("expected no error messages, got: %v", errs)
	}
}

func TestDetectRemotedAndPlainRefs_DifferentRemotesSameBranch(t *testing.T) {
	// Two different remotes mapping to the same cleaned branch -> ambiguous.
	cmd := &BaseCommand{
		ot: &ostree.MockOstree{
			LocalRefs_: []string{
				"origin:matrixos/x86_64/dev/gnome",
				"upstream:matrixos/x86_64/dev/gnome",
			},
		},
	}

	var errs []string
	errf := func(format string, args ...any) {
		errs = append(errs, fmt.Sprintf(format, args...))
	}

	err := cmd.detectRemotedAndPlainRefs(errf)
	if err == nil {
		t.Fatal("expected error for ambiguous refs from different remotes, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous refs detected") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// --- shortRef unit tests ---

func TestShortRef(t *testing.T) {
	cmd := &BaseCommand{}

	tests := []struct {
		name string
		ref  string
		want string
	}{
		{
			name: "multi-segment ref",
			ref:  "matrixos/x86_64/dev/gnome",
			want: "m/x/d/gnome",
		},
		{
			name: "remoted multi-segment ref",
			ref:  "origin:matrixos/x86_64/dev/gnome",
			want: "o:m/x/d/gnome",
		},
		{
			name: "single segment",
			ref:  "matrixos",
			want: "matrixos",
		},
		{
			name: "two segments",
			ref:  "matrixos/x86_64",
			want: "m/x86_64",
		},
		{
			name: "remoted single segment",
			ref:  "origin:matrixos",
			want: "o:matrixos",
		},
		{
			name: "empty ref",
			ref:  "",
			want: "",
		},
		{
			name: "different remote prefix",
			ref:  "upstream:matrixos/x86_64/dev/cosmic",
			want: "u:m/x/d/cosmic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmd.shortRef(tt.ref)
			if got != tt.want {
				t.Errorf(
					"shortRef(%q) = %q, want %q",
					tt.ref, got, tt.want,
				)
			}
		})
	}
}

// --- resolveRefRemote tests ---

func TestResolveRefRemote_PlainRef(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Remote": {"origin"},
		},
	}
	cmd := &BaseCommand{
		cfg: cfg,
		ot: &ostree.MockOstree{
			Remote_: "origin",
		},
	}

	var warns []string
	warnf := func(format string, args ...any) {
		warns = append(warns, fmt.Sprintf(format, args...))
	}

	rr, err := cmd.resolveRefRemote("matrixos/x86_64/dev/gnome", warnf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rr.Ref != "matrixos/x86_64/dev/gnome" {
		t.Errorf("expected unchanged ref, got %q", rr.Ref)
	}
	if rr.Remote != "origin" {
		t.Errorf("expected remote 'origin', got %q", rr.Remote)
	}
	if len(warns) != 0 {
		t.Errorf("expected no warnings, got: %v", warns)
	}
}

func TestResolveRefRemote_RemotedRef(t *testing.T) {
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.Remote": {"origin"},
		},
	}
	cmd := &BaseCommand{
		cfg: cfg,
		ot: &ostree.MockOstree{
			Remote_: "origin",
		},
	}

	var warns []string
	warnf := func(format string, args ...any) {
		warns = append(warns, fmt.Sprintf(format, args...))
	}

	rr, err := cmd.resolveRefRemote("custom:matrixos/x86_64/dev/gnome", warnf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rr.Ref != "matrixos/x86_64/dev/gnome" {
		t.Errorf("expected cleaned ref, got %q", rr.Ref)
	}
	if rr.Remote != "custom" {
		t.Errorf("expected remote 'custom', got %q", rr.Remote)
	}
	if len(warns) == 0 {
		t.Error("expected a warning about embedded remote")
	}
}

func TestResolveRefRemote_RemoteError(t *testing.T) {
	cmd := &BaseCommand{
		ot: &ostree.MockOstree{
			RemoteErr: fmt.Errorf("mock remote failure"),
		},
	}

	warnf := func(format string, args ...any) {}

	_, err := cmd.resolveRefRemote("matrixos/x86_64/dev/gnome", warnf)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "mock remote failure") {
		t.Errorf("unexpected error: %v", err)
	}
}
