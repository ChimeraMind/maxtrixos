package ostree

import (
	"matrixos/vector/lib/config"
	"matrixos/vector/lib/runner"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Unit tests (mocked runner, no ostree binary needed)
// ---------------------------------------------------------------------------

func TestCommitOptions_Validate(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		opts    CommitOptions
		wantErr string
	}{
		{
			name:    "missing RepoDir",
			opts:    CommitOptions{Branch: "b", ImageDir: tmpDir},
			wantErr: "missing RepoDir",
		},
		{
			name:    "missing Branch",
			opts:    CommitOptions{RepoDir: tmpDir, ImageDir: tmpDir},
			wantErr: "missing Branch",
		},
		{
			name:    "missing ImageDir",
			opts:    CommitOptions{RepoDir: tmpDir, Branch: "b"},
			wantErr: "missing ImageDir",
		},
		{
			name:    "non-existent RepoDir",
			opts:    CommitOptions{RepoDir: "/no/such/dir", Branch: "b", ImageDir: tmpDir},
			wantErr: "repo directory",
		},
		{
			name:    "non-existent ImageDir",
			opts:    CommitOptions{RepoDir: tmpDir, Branch: "b", ImageDir: "/no/such/dir"},
			wantErr: "image directory",
		},
		{
			name:    "non-existent BodyFile",
			opts:    CommitOptions{RepoDir: tmpDir, Branch: "b", ImageDir: tmpDir, BodyFile: "/no/such/file"},
			wantErr: "body file",
		},
		{
			name: "valid minimal",
			opts: CommitOptions{RepoDir: tmpDir, Branch: "b", ImageDir: tmpDir},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.opts.validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("error %q does not contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestCommitOptions_Args(t *testing.T) {
	opts := CommitOptions{
		RepoDir:  "/repo",
		Branch:   "test/branch",
		Subject:  "my subject",
		BodyFile: "/tmp/body",
		Parent:   "abc123",
		GpgArgs:  []string{"--gpg-sign=KEY", "--gpg-homedir=/home"},
		Consume:  true,
		ImageDir: "/image",
	}

	args := opts.args(false)
	got := strings.Join(args, " ")
	expected := "commit --consume --repo=/repo --parent=abc123 --branch=test/branch --gpg-sign=KEY --gpg-homedir=/home --subject=my subject --body-file=/tmp/body /image"
	if got != expected {
		t.Errorf("args mismatch:\n  got:  %s\n  want: %s", got, expected)
	}

	// With verbose
	args = opts.args(true)
	if args[1] != "--verbose" {
		t.Errorf("expected --verbose as second arg, got %q", args[1])
	}
}

func TestCommitOptions_ArgsMinimal(t *testing.T) {
	opts := CommitOptions{
		RepoDir:  "/repo",
		Branch:   "b",
		ImageDir: "/img",
	}
	args := opts.args(false)
	got := strings.Join(args, " ")
	expected := "commit --repo=/repo --branch=b /img"
	if got != expected {
		t.Errorf("args mismatch:\n  got:  %s\n  want: %s", got, expected)
	}
}

func TestOstreeCommit_MockedRunner(t *testing.T) {
	repoDir := t.TempDir()
	imageDir := t.TempDir()

	// Create dummy content.
	if err := os.WriteFile(filepath.Join(imageDir, "hello"), []byte("world"), 0644); err != nil {
		t.Fatal(err)
	}

	var captured []string
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.RepoDir": {repoDir},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg, Ref: "test/ref"})
	if err != nil {
		t.Fatalf("NewOstree: %v", err)
	}
	o.runner = func(cmd *runner.Cmd) error {
		args, name := cmd.Args, cmd.Name
		captured = append([]string{name}, args...)
		return nil
	}

	opts := CommitOptions{
		RepoDir:  repoDir,
		Branch:   "test/branch",
		Subject:  "test subject",
		Consume:  false,
		ImageDir: imageDir,
		GpgArgs:  []string{"--gpg-sign=ABC"},
	}

	if err := o.Commit(opts); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	cmdStr := strings.Join(captured, " ")
	for _, want := range []string{
		"ostree",
		"commit",
		"--repo=" + repoDir,
		"--branch=test/branch",
		"--gpg-sign=ABC",
		"--subject=test subject",
		imageDir,
	} {
		if !strings.Contains(cmdStr, want) {
			t.Errorf("command %q missing expected fragment %q", cmdStr, want)
		}
	}
}

func TestCommitBody_MockedRunner(t *testing.T) {
	repoDir := t.TempDir()
	imageDir := t.TempDir()

	var captured []string
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.RepoDir": {repoDir},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree: %v", err)
	}
	o.runner = func(cmd *runner.Cmd) error {
		args, name := cmd.Args, cmd.Name
		captured = append([]string{name}, args...)
		return nil
	}

	opts := CommitOptions{
		RepoDir:  repoDir,
		Branch:   "b",
		Body:     "hello body",
		ImageDir: imageDir,
	}

	if err := o.Commit(opts); err != nil {
		t.Fatalf("Commit with Body: %v", err)
	}

	cmdStr := strings.Join(captured, " ")
	if !strings.Contains(cmdStr, "--body-file=") {
		t.Errorf("expected --body-file in command: %s", cmdStr)
	}
}

func TestCommitEmptyBody_MockedRunner(t *testing.T) {
	repoDir := t.TempDir()
	imageDir := t.TempDir()

	var captured []string
	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.RepoDir": {repoDir},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree: %v", err)
	}
	o.runner = func(cmd *runner.Cmd) error {
		args, name := cmd.Args, cmd.Name
		captured = append([]string{name}, args...)
		return nil
	}

	opts := CommitOptions{
		RepoDir:  repoDir,
		Branch:   "b",
		ImageDir: imageDir,
	}

	if err := o.Commit(opts); err != nil {
		t.Fatalf("Commit empty body: %v", err)
	}

	cmdStr := strings.Join(captured, " ")
	if strings.Contains(cmdStr, "--body-file") {
		t.Errorf("empty body should not produce --body-file: %s", cmdStr)
	}
}

// ---------------------------------------------------------------------------
// Integration test — requires a real ostree binary.
// ---------------------------------------------------------------------------

func TestCommitIntegration(t *testing.T) {
	repoDir := setupTestRepo(t)

	imageDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(imageDir, "usr", "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(imageDir, "usr", "bin", "hello"), []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
		t.Fatal(err)
	}
	// Add a symlink to verify it is committed.
	if err := os.Symlink("hello", filepath.Join(imageDir, "usr", "bin", "hi")); err != nil {
		t.Fatal(err)
	}

	branch := "test/integration/commit"

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.RepoDir": {repoDir},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg, Ref: branch})
	if err != nil {
		t.Fatalf("NewOstree: %v", err)
	}

	// --- First commit ---
	err = o.Commit(CommitOptions{
		RepoDir:  repoDir,
		Branch:   branch,
		Subject:  "integration test commit",
		ImageDir: imageDir,
	})
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// Verify the ref exists.
	refs, err := o.LocalRefs()
	if err != nil {
		t.Fatalf("LocalRefs: %v", err)
	}
	found := false
	for _, r := range refs {
		if r == branch {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("branch %q not found in refs: %v", branch, refs)
	}

	// Verify a second commit (with parent) works.
	if err := os.WriteFile(filepath.Join(imageDir, "usr", "bin", "world"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	parentRev, err := LastCommit(repoDir, branch, false)
	if err != nil {
		t.Fatalf("LastCommit: %v", err)
	}

	err = o.Commit(CommitOptions{
		RepoDir:  repoDir,
		Branch:   branch,
		Subject:  "second commit",
		Parent:   parentRev,
		ImageDir: imageDir,
	})
	if err != nil {
		t.Fatalf("second Commit: %v", err)
	}

	// --- Test Commit with body file ---
	bodyFile := filepath.Join(t.TempDir(), "body.txt")
	if err := os.WriteFile(bodyFile, []byte("detailed body\nwith lines\n"), 0644); err != nil {
		t.Fatal(err)
	}

	err = o.Commit(CommitOptions{
		RepoDir:  repoDir,
		Branch:   branch,
		Subject:  "instance commit",
		BodyFile: bodyFile,
		ImageDir: imageDir,
	})
	if err != nil {
		t.Fatalf("instance Commit: %v", err)
	}

	// Verify via ostree log that the subject is present.
	cmd := exec.Command("ostree", "log", "--repo="+repoDir, branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ostree log: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "instance commit") {
		t.Errorf("ostree log missing expected subject:\n%s", out)
	}
}

func TestCommitIntegration_Consume(t *testing.T) {
	repoDir := setupTestRepo(t)

	imageDir := t.TempDir()
	marker := filepath.Join(imageDir, "marker")
	if err := os.WriteFile(marker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.RepoDir": {repoDir},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree: %v", err)
	}

	err = o.Commit(CommitOptions{
		RepoDir:  repoDir,
		Branch:   "test/consume",
		Subject:  "consume commit",
		Consume:  true,
		ImageDir: imageDir,
	})
	if err != nil {
		t.Fatalf("Commit with consume: %v", err)
	}

	// After --consume, the source directory content should be removed.
	entries, err := os.ReadDir(imageDir)
	if err != nil && !os.IsNotExist(err) {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Name()
		}
		t.Errorf("expected empty imageDir after --consume, got: %v", names)
	}
}

func TestCommitIntegration_InlineBody(t *testing.T) {
	repoDir := setupTestRepo(t)

	imageDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(imageDir, "f"), []byte("data"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.MockConfig{
		Items: map[string][]string{
			"Ostree.RepoDir": {repoDir},
		},
	}
	o, err := NewOstree(NewOstreeOptions{Config: cfg})
	if err != nil {
		t.Fatalf("NewOstree: %v", err)
	}

	branch := "test/inlinebody"
	err = o.Commit(CommitOptions{
		RepoDir:  repoDir,
		Branch:   branch,
		Subject:  "inline body test",
		Body:     "the body content\nline2\n",
		ImageDir: imageDir,
	})
	if err != nil {
		t.Fatalf("Commit with Body: %v", err)
	}

	cmd := exec.Command("ostree", "log", "--repo="+repoDir, branch)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ostree log: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "the body content") {
		t.Errorf("ostree log missing expected body:\n%s", out)
	}
}
