package releaser

import (
	"bytes"
	"errors"
	"testing"

	"matrixos/vector/lib/ostree"
)

func TestFilterRefs_NilFilters(t *testing.T) {
	refs := []string{"a", "b", "c"}
	got := filterRefs(refs, nil, nil, &bytes.Buffer{})
	if len(got) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(got))
	}
}

func TestFilterRefs_SkipFilter(t *testing.T) {
	refs := []string{"a", "b", "c"}
	skip := func(ref string) bool { return ref == "b" }
	var buf bytes.Buffer
	got := filterRefs(refs, skip, nil, &buf)

	if len(got) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(got))
	}
	if got[0] != "a" || got[1] != "c" {
		t.Errorf("unexpected refs: %v", got)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Skipping release: b as requested by flags.")) {
		t.Error("expected skip message in output")
	}
}

func TestFilterRefs_OnlyFilter(t *testing.T) {
	refs := []string{"a", "b", "c"}
	only := func(ref string) bool { return ref == "b" }
	var buf bytes.Buffer
	got := filterRefs(refs, nil, only, &buf)

	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("expected [b], got %v", got)
	}
	if !bytes.Contains(buf.Bytes(), []byte("Skipping release: a not in list")) {
		t.Error("expected only-filter skip message in output")
	}
}

func TestFilterRefs_SkipTakesPrecedenceOverOnly(t *testing.T) {
	refs := []string{"a", "b"}
	skip := func(ref string) bool { return ref == "b" }
	only := func(ref string) bool { return ref == "b" }
	got := filterRefs(refs, skip, only, &bytes.Buffer{})

	// "a" is excluded by only, "b" is excluded by skip
	if len(got) != 0 {
		t.Fatalf("expected 0 refs, got %v", got)
	}
}

func TestFilterRefs_EmptyInput(t *testing.T) {
	got := filterRefs(nil, nil, nil, &bytes.Buffer{})
	if got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
}

// --- newTestDetector helper ---

func newTestDetector(ot ostree.IOstree) *ReleaseDetector {
	return &ReleaseDetector{
		ostree: ot,
		stderr: &bytes.Buffer{},
	}
}

// --- ReleaseDetector tests ---

func TestNewReleaseDetector_NilOstree(t *testing.T) {
	_, err := NewReleaseDetector(nil)
	if err == nil {
		t.Fatal("expected error for nil ostree")
	}
}

func TestNewReleaseDetector_HappyPath(t *testing.T) {
	d, err := NewReleaseDetector(&ostree.MockOstree{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d == nil {
		t.Fatal("expected non-nil ReleaseDetector")
	}
}

func TestNewReleaseDetector_ImplementsIReleaseDetector(t *testing.T) {
	d, err := NewReleaseDetector(&ostree.MockOstree{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var _ IReleaseDetector = d
}

func TestReleaseDetector_SetStderr(t *testing.T) {
	d, _ := NewReleaseDetector(&ostree.MockOstree{})
	var buf bytes.Buffer
	d.SetStderr(&buf)
	if d.Stderr() != &buf {
		t.Error("SetStderr did not update the writer")
	}
}

func TestDetectLocalReleases_Success(t *testing.T) {
	ot := &ostree.MockOstree{LocalRefs_: []string{"ref/a", "ref/b", "ref/c"}}
	d := newTestDetector(ot)

	got, err := d.DetectLocalReleases(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(got))
	}
}

func TestDetectLocalReleases_WithSkip(t *testing.T) {
	ot := &ostree.MockOstree{LocalRefs_: []string{"ref/a", "ref/b"}}
	d := newTestDetector(ot)

	skip := func(ref string) bool { return ref == "ref/a" }
	got, err := d.DetectLocalReleases(skip, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "ref/b" {
		t.Fatalf("expected [ref/b], got %v", got)
	}
}

func TestDetectLocalReleases_Error(t *testing.T) {
	ot := &ostree.MockOstree{LocalRefsErr: errors.New("boom")}
	d := newTestDetector(ot)

	_, err := d.DetectLocalReleases(nil, nil)
	if err == nil || err.Error() != "boom" {
		t.Fatalf("expected 'boom' error, got %v", err)
	}
}

func TestDetectRemoteReleases_Success(t *testing.T) {
	ot := &ostree.MockOstree{Refs: []string{"remote/a", "remote/b"}}
	d := newTestDetector(ot)

	got, err := d.DetectRemoteReleases(nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 refs, got %d", len(got))
	}
}

func TestDetectRemoteReleases_WithOnly(t *testing.T) {
	ot := &ostree.MockOstree{Refs: []string{"remote/a", "remote/b"}}
	d := newTestDetector(ot)

	only := func(ref string) bool { return ref == "remote/b" }
	got, err := d.DetectRemoteReleases(nil, only)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0] != "remote/b" {
		t.Fatalf("expected [remote/b], got %v", got)
	}
}

func TestDetectRemoteReleases_Error(t *testing.T) {
	ot := &ostree.MockOstree{RefsErr: errors.New("network fail")}
	d := newTestDetector(ot)

	_, err := d.DetectRemoteReleases(nil, nil)
	if err == nil || err.Error() != "network fail" {
		t.Fatalf("expected 'network fail' error, got %v", err)
	}
}
