package releaser

import (
	"fmt"
	"io"
)

// DetectLocalReleases lists local ostree refs, filtered by skip and only functions.
func (r *Releaser) DetectLocalReleases(skip, only RefFilterFunc, verbose bool) ([]string, error) {
	refs, err := r.ostree.LocalRefs()
	if err != nil {
		return nil, err
	}
	return filterRefs(refs, skip, only, r.stderr), nil
}

// DetectRemoteReleases lists remote ostree refs, filtered by skip and only functions.
func (r *Releaser) DetectRemoteReleases(skip, only RefFilterFunc, verbose bool) ([]string, error) {
	refs, err := r.ostree.RemoteRefs()
	if err != nil {
		return nil, err
	}
	return filterRefs(refs, skip, only, r.stderr), nil
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
