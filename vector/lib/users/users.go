package users

import (
	"fmt"
	"io"
	"os/user"
	"strconv"

	"matrixos/vector/lib/runner"
)

// SeedUserName is the system account used to run chroot operations under a
// user namespace. It is created on demand if it does not already exist.
const SeedUserName = "vector-seed"

// EnsureSystemUser ensures that the system account with the given name exists.
// If the account does not exist it is created via useradd as a no-login system
// user without a home directory.  The uid and gid of the account are returned.
func EnsureSystemUser(name string, run runner.Func, stdout, stderr io.Writer) (uid, gid uint32, err error) {
	uid, gid, err = lookupUser(name)
	if err == nil {
		return uid, gid, nil
	}
	if !isUnknownUser(err) {
		return 0, 0, fmt.Errorf("looking up user %q: %w", name, err)
	}

	fmt.Fprintf(stdout, "Creating system user %q ...\n", name)
	if runErr := run(&runner.Cmd{
		Name: "useradd",
		Args: []string{
			"--system",
			"--no-create-home",
			"--shell", "/sbin/nologin",
			name,
		},
		Stdout: stdout,
		Stderr: stderr,
	}); runErr != nil {
		return 0, 0, fmt.Errorf("useradd %q: %w", name, runErr)
	}

	uid, gid, err = lookupUser(name)
	if err != nil {
		return 0, 0, fmt.Errorf("looking up freshly created user %q: %w", name, err)
	}
	return uid, gid, nil
}

// lookupUser resolves a user name to its uid and gid.
func lookupUser(name string) (uid, gid uint32, err error) {
	u, err := user.Lookup(name)
	if err != nil {
		return 0, 0, err
	}
	uid64, err := strconv.ParseUint(u.Uid, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid uid %q for user %q: %w", u.Uid, name, err)
	}
	gid64, err := strconv.ParseUint(u.Gid, 10, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid gid %q for user %q: %w", u.Gid, name, err)
	}
	return uint32(uid64), uint32(gid64), nil
}

// isUnknownUser reports whether err is an os/user.UnknownUserError.
func isUnknownUser(err error) bool {
	_, ok := err.(user.UnknownUserError)
	return ok
}
