package releaser

import (
	"errors"
	"fmt"
	"io"
	"os"

	"matrixos/vector/lib/ostree"
)

// Compile-time interface check.
var _ IReleaseDetector = (*ReleaseDetector)(nil)

// IReleaseDetector defines the interface for detecting available releases.
type IReleaseDetector interface {
	// DetectLocalReleases lists local ostree refs, filtered by skip and only functions.
	DetectLocalReleases(skip, only RefFilterFunc) ([]string, error)
	// DetectRemoteReleases lists remote ostree refs, filtered by skip and only functions.
	DetectRemoteReleases(skip, only RefFilterFunc) ([]string, error)
}

// ReleaseDetector discovers available ostree refs (releases) from local
// or remote repositories and applies caller-provided filters.
type ReleaseDetector struct {
	ostree ostree.IOstree
	stderr io.Writer
}

// NewReleaseDetector creates a new ReleaseDetector instance.
func NewReleaseDetector(ot ostree.IOstree) (*ReleaseDetector, error) {
	if ot == nil {
		return nil, errors.New("missing ostree parameter")
	}
	return &ReleaseDetector{
		ostree: ot,
		stderr: os.Stderr,
	}, nil
}

// SetStderr replaces the writer used for filter-skip messages.
func (d *ReleaseDetector) SetStderr(w io.Writer) { d.stderr = w }

// Stderr returns the current warning/error output writer.
func (d *ReleaseDetector) Stderr() io.Writer { return d.stderr }

// DetectLocalReleases lists local ostree refs, filtered by skip and only functions.
func (d *ReleaseDetector) DetectLocalReleases(skip, only RefFilterFunc) ([]string, error) {
	refs, err := d.ostree.LocalRefs()
	if err != nil {
		return nil, err
	}
	return filterRefs(refs, skip, only, d.stderr), nil
}

// DetectRemoteReleases lists remote ostree refs, filtered by skip and only functions.
func (d *ReleaseDetector) DetectRemoteReleases(skip, only RefFilterFunc) ([]string, error) {
	refs, err := d.ostree.RemoteRefs()
	if err != nil {
		return nil, err
	}
	return filterRefs(refs, skip, only, d.stderr), nil
}

// filterRefs applies skip/only filter functions to a list of refs.
func filterRefs(refs []string, skip, only RefFilterFunc, w io.Writer) []string {
	var filtered []string
	for _, ref := range refs {
		if skip != nil && skip(ref) {
			fmt.Fprintf(w, "Skipping release: %s as requested by flags.\n", ref)
			continue
		}
		if only != nil && !only(ref) {
			fmt.Fprintf(w, "Skipping release: %s not in list of releases to create.\n", ref)
			continue
		}
		filtered = append(filtered, ref)
	}
	return filtered
}
