package users

import (
	"bytes"
	"errors"
	"os/user"
	"strconv"
	"strings"
	"testing"

	"matrixos/vector/lib/runner"
)

// currentUser returns the OS user running the test, skipping if unavailable.
func currentUser(t *testing.T) *user.User {
	t.Helper()
	u, err := user.Current()
	if err != nil {
		t.Skipf("cannot get current user: %v", err)
	}
	return u
}

// ---------- isUnknownUser ----------

func TestIsUnknownUser_True(t *testing.T) {
	err := user.UnknownUserError("nobody")
	if !isUnknownUser(err) {
		t.Error("expected true for UnknownUserError, got false")
	}
}

func TestIsUnknownUser_False(t *testing.T) {
	err := errors.New("some other error")
	if isUnknownUser(err) {
		t.Error("expected false for non-UnknownUserError, got true")
	}
}

// ---------- lookupUser ----------

func TestLookupUser_ExistingUser(t *testing.T) {
	u := currentUser(t)

	wantUID, _ := strconv.ParseUint(u.Uid, 10, 32)
	wantGID, _ := strconv.ParseUint(u.Gid, 10, 32)

	uid, gid, err := lookupUser(u.Username)
	if err != nil {
		t.Fatalf("unexpected error looking up %q: %v", u.Username, err)
	}
	if uid != uint32(wantUID) {
		t.Errorf("uid = %d, want %d", uid, wantUID)
	}
	if gid != uint32(wantGID) {
		t.Errorf("gid = %d, want %d", gid, wantGID)
	}
}

func TestLookupUser_NonExistentUser(t *testing.T) {
	_, _, err := lookupUser("nonexistent-user-xyz-vector")
	if err == nil {
		t.Fatal("expected error for non-existent user, got nil")
	}
	if !isUnknownUser(err) {
		t.Errorf("expected UnknownUserError, got %T: %v", err, err)
	}
}

// ---------- EnsureSystemUser ----------

// TestEnsureSystemUser_UserAlreadyExists uses the current OS user, which is
// guaranteed to exist. The runner must not be invoked and no stdout output
// should appear.
func TestEnsureSystemUser_UserAlreadyExists(t *testing.T) {
	u := currentUser(t)
	wantUID, _ := strconv.ParseUint(u.Uid, 10, 32)
	wantGID, _ := strconv.ParseUint(u.Gid, 10, 32)

	mr := runner.NewMockRunner()
	var stdout, stderr bytes.Buffer

	uid, gid, err := EnsureSystemUser(u.Username, mr.Run, &stdout, &stderr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if uid != uint32(wantUID) {
		t.Errorf("uid = %d, want %d", uid, wantUID)
	}
	if gid != uint32(wantGID) {
		t.Errorf("gid = %d, want %d", gid, wantGID)
	}
	if len(mr.Calls) != 0 {
		t.Errorf("runner should not be called when user exists, got %d calls", len(mr.Calls))
	}
	if stdout.Len() != 0 {
		t.Errorf("expected no stdout output, got %q", stdout.String())
	}
}

// TestEnsureSystemUser_UseraaddFails verifies that a runner error from useradd
// is propagated correctly.
func TestEnsureSystemUser_UseraaddFails(t *testing.T) {
	wantErr := errors.New("useradd failed")
	mr := &runner.MockRunner{FailOn: -1, Err: wantErr}
	var stdout, stderr bytes.Buffer

	_, _, err := EnsureSystemUser("nonexistent-user-xyz-vector", mr.Run, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, wantErr) {
		t.Errorf("got %v, want error wrapping %v", err, wantErr)
	}
	if len(mr.Calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(mr.Calls))
	}
	if mr.Calls[0].Name != "useradd" {
		t.Errorf("expected useradd call, got %q", mr.Calls[0].Name)
	}
}

// TestEnsureSystemUser_UseraaddArgs verifies the exact arguments passed to useradd.
func TestEnsureSystemUser_UseraaddArgs(t *testing.T) {
	mr := runner.NewMockRunner()
	var stdout, stderr bytes.Buffer
	const name = "nonexistent-user-xyz-vector"

	EnsureSystemUser(name, mr.Run, &stdout, &stderr) //nolint:errcheck

	if len(mr.Calls) == 0 {
		t.Fatal("no runner calls recorded")
	}
	call := mr.Calls[0]
	if call.Name != "useradd" {
		t.Fatalf("expected useradd call, got %q", call.Name)
	}
	wantArgs := []string{"--system", "--no-create-home", "--shell", "/sbin/nologin", name}
	if len(call.Args) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", call.Args, wantArgs)
	}
	for i, a := range wantArgs {
		if call.Args[i] != a {
			t.Errorf("args[%d] = %q, want %q", i, call.Args[i], a)
		}
	}
}

// TestEnsureSystemUser_StdoutNotified checks that a creation message mentioning
// the username is written to stdout before invoking useradd.
func TestEnsureSystemUser_StdoutNotified(t *testing.T) {
	mr := runner.NewMockRunner()
	var stdout, stderr bytes.Buffer
	const name = "nonexistent-user-xyz-vector"

	EnsureSystemUser(name, mr.Run, &stdout, &stderr) //nolint:errcheck

	if !strings.Contains(stdout.String(), name) {
		t.Errorf("expected username %q in stdout, got %q", name, stdout.String())
	}
}

// TestEnsureSystemUser_UseraaddSucceedsButLookupFails exercises the error path
// where useradd appears to succeed but the user still cannot be found afterwards
// (the mock does not create a real OS user).
func TestEnsureSystemUser_UseraaddSucceedsButLookupFails(t *testing.T) {
	mr := runner.NewMockRunner()
	var stdout, stderr bytes.Buffer

	_, _, err := EnsureSystemUser("nonexistent-user-xyz-vector", mr.Run, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error after failed post-create lookup, got nil")
	}
	if !strings.Contains(err.Error(), "looking up freshly created user") {
		t.Errorf("unexpected error message: %v", err)
	}
	if len(mr.Calls) != 1 {
		t.Fatalf("expected 1 runner call, got %d", len(mr.Calls))
	}
}
