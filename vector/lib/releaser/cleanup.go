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

// Cleanup unmounts all mount points tracked by this Releaser instance
// in reverse order. It is safe to call multiple times.
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
