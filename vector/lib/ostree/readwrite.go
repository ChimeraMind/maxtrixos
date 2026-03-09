package ostree

// Readwrite unlocks the filesystem for writing using `ostree admin unlock`.
// If permanent is true, it uses --hotfix (persistent across reboots).
// If permanent is false, it uses --transient (lost on reboot).
func (o *Ostree) Readwrite(permanent bool) error {
	root, err := o.Root()
	if err != nil {
		return err
	}

	args := []string{"admin", "unlock", "--sysroot=" + root}
	if permanent {
		args = append(args, "--hotfix")
	} else {
		args = append(args, "--transient")
	}

	return o.ostreeRun(args...)
}
