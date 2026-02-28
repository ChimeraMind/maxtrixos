package releaser

import (
	"slices"

	"matrixos/vector/lib/filesystems"
)

// trackMount appends a single mount point to the tracked list.
func (r *Releaser) trackMount(mnt string) {
	r.trackedMountsMu.Lock()
	defer r.trackedMountsMu.Unlock()
	r.trackedMounts = append(r.trackedMounts, mnt)
}

func (r *Releaser) Cleanup() {
	r.trackedMountsMu.Lock()
	mounts := slices.Clone(r.trackedMounts)
	r.trackedMounts = nil
	r.trackedMountsMu.Unlock()

	opts := filesystems.CleanupMountsOptions{
		Mounts: mounts,
		Stdout: r.stdout,
		Stderr: r.stderr,
	}
	filesystems.CleanupMounts(opts)
}
