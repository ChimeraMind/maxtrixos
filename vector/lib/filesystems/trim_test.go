package filesystems

import (
	"bytes"
	"errors"
	"testing"

	"matrixos/vector/lib/runner"
)

var errTest = errors.New("test error")

func TestFstrim(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		mr := runner.NewMockRunner()
		var stdout, stderr bytes.Buffer

		err := Fstrim(mr.Run, &stdout, &stderr, "/mnt/rootfs")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(mr.Calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mr.Calls))
		}
		c := mr.Calls[0]
		if c.Name != "fstrim" {
			t.Errorf("expected command fstrim, got %q", c.Name)
		}
		if len(c.Args) != 2 || c.Args[0] != "-v" || c.Args[1] != "/mnt/rootfs" {
			t.Errorf("unexpected args: %v", c.Args)
		}
		if !bytes.Contains(stdout.Bytes(), []byte("/mnt/rootfs")) {
			t.Error("expected stdout to mention mount point")
		}
	})

	t.Run("EmptyMountPoint", func(t *testing.T) {
		mr := runner.NewMockRunner()
		var stdout, stderr bytes.Buffer

		err := Fstrim(mr.Run, &stdout, &stderr, "")
		if err == nil {
			t.Error("expected error for empty mount point")
		}
		if len(mr.Calls) != 0 {
			t.Errorf("expected no calls, got %d", len(mr.Calls))
		}
	})

	t.Run("RunnerError", func(t *testing.T) {
		mr := runner.NewMockRunnerFailOnCall(0, errTest)
		var stdout, stderr bytes.Buffer

		err := Fstrim(mr.Run, &stdout, &stderr, "/mnt/data")
		if err == nil {
			t.Error("expected error from runner")
		}
		if len(mr.Calls) != 1 {
			t.Fatalf("expected 1 call, got %d", len(mr.Calls))
		}
	})
}

func TestFstrimAll(t *testing.T) {
	t.Run("MultipleMountPoints", func(t *testing.T) {
		mr := runner.NewMockRunner()
		var stdout, stderr bytes.Buffer

		FstrimAll(mr.Run, &stdout, &stderr, "/mnt/rootfs", "/mnt/boot", "/mnt/efi")

		if len(mr.Calls) != 3 {
			t.Fatalf("expected 3 fstrim calls, got %d", len(mr.Calls))
		}
		for i, c := range mr.Calls {
			if c.Name != "fstrim" {
				t.Errorf("call %d: expected fstrim, got %q", i, c.Name)
			}
		}
		// Verify each mount point was passed.
		expected := []string{"/mnt/rootfs", "/mnt/boot", "/mnt/efi"}
		for i, c := range mr.Calls {
			if len(c.Args) < 2 || c.Args[1] != expected[i] {
				t.Errorf("call %d: expected mount point %q, got args %v", i, expected[i], c.Args)
			}
		}
	})

	t.Run("ErrorsIgnored", func(t *testing.T) {
		// Fail on every call – FstrimAll should still process all mount points.
		mr := runner.NewMockRunnerFailOnCall(-1, errTest)
		mr.Err = errTest
		mr.FailOn = -1
		var stdout, stderr bytes.Buffer

		// Should not panic or abort early.
		FstrimAll(mr.Run, &stdout, &stderr, "/mnt/a", "/mnt/b")

		if len(mr.Calls) != 2 {
			t.Fatalf("expected 2 calls even with errors, got %d", len(mr.Calls))
		}
	})

	t.Run("NoMountPoints", func(t *testing.T) {
		mr := runner.NewMockRunner()
		var stdout, stderr bytes.Buffer

		FstrimAll(mr.Run, &stdout, &stderr)

		if len(mr.Calls) != 0 {
			t.Errorf("expected no calls for empty mount point list, got %d", len(mr.Calls))
		}
	})
}
