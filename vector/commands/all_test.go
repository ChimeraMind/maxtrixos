package commands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"matrixos/vector/lib/filesystems"
)

func TestAllCommandName(t *testing.T) {
	cmd := NewAllCommand()
	if got := cmd.Name(); got != "all" {
		t.Errorf("Name() = %q, want %q", got, "all")
	}
}

func TestNewAllCommand(t *testing.T) {
	cmd := NewAllCommand()
	if cmd == nil {
		t.Fatal("NewAllCommand returned nil")
	}
}

func TestAllParseArgsNotRoot(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 1000 }
	defer func() { getEuid = origEuid }()

	cmd := NewAllCommand()
	err := cmd.parseArgs([]string{})
	if err == nil {
		t.Fatal("expected error for non-root, got nil")
	}
	if !strings.Contains(err.Error(), "must be run as root") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAllParseArgsDefaults(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewAllCommand()
	err := cmd.parseArgs([]string{})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	if cmd.buildName != "matrixOS weekly" {
		t.Errorf("buildName = %q, want %q", cmd.buildName, "matrixOS weekly")
	}
	if cmd.buildID != "weekly" {
		t.Errorf("buildID = %q, want %q", cmd.buildID, "weekly")
	}
	if cmd.forceRelease {
		t.Error("forceRelease should default to false")
	}
	if cmd.onlyImages {
		t.Error("onlyImages should default to false")
	}
	if cmd.forceImages {
		t.Error("forceImages should default to false")
	}
	if cmd.skipImages {
		t.Error("skipImages should default to false")
	}
	if cmd.onBuildServer {
		t.Error("onBuildServer should default to false")
	}
	if cmd.resumeSeeders {
		t.Error("resumeSeeders should default to false")
	}
	if cmd.disableJanitor {
		t.Error("disableJanitor should default to false")
	}
	if cmd.disableMail {
		t.Error("disableMail should default to false")
	}
	if cmd.cdnPusher != "" {
		t.Errorf("cdnPusher = %q, want empty", cmd.cdnPusher)
	}
	if cmd.mailUser != "root" {
		t.Errorf("mailUser = %q, want %q", cmd.mailUser, "root")
	}
}

func TestAllParseArgsAllFlags(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	tmp := t.TempDir()
	pusher := filepath.Join(tmp, "push.sh")
	if err := os.WriteFile(pusher, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := NewAllCommand()
	err := cmd.parseArgs([]string{
		"--force-release",
		"--only-images",
		"--force-images",
		"--skip-images",
		"--on-build-server",
		"--resume",
		"--build-name", "my build",
		"--build-id", "nightly",
		"--skip-seeders", "00-bedrock",
		"--only-seeders", "10-server",
		"--disable-janitor",
		"--disable-send-mail",
		"--mail-user", "ops@example.com",
		"--cdn-pusher", pusher,
		"--verbose",
	})
	if err != nil {
		t.Fatalf("parseArgs returned error: %v", err)
	}

	if !cmd.forceRelease {
		t.Error("forceRelease should be true")
	}
	if !cmd.onlyImages {
		t.Error("onlyImages should be true")
	}
	if !cmd.forceImages {
		t.Error("forceImages should be true")
	}
	if !cmd.skipImages {
		t.Error("skipImages should be true")
	}
	if !cmd.onBuildServer {
		t.Error("onBuildServer should be true")
	}
	if !cmd.resumeSeeders {
		t.Error("resumeSeeders should be true")
	}
	if cmd.buildName != "my build" {
		t.Errorf("buildName = %q, want %q", cmd.buildName, "my build")
	}
	if cmd.buildID != "nightly" {
		t.Errorf("buildID = %q, want %q", cmd.buildID, "nightly")
	}
	if cmd.skipSeedersRaw != "00-bedrock" {
		t.Errorf("skipSeedersRaw = %q, want %q", cmd.skipSeedersRaw, "00-bedrock")
	}
	if cmd.onlySeedersRaw != "10-server" {
		t.Errorf("onlySeedersRaw = %q, want %q", cmd.onlySeedersRaw, "10-server")
	}
	if !cmd.disableJanitor {
		t.Error("disableJanitor should be true")
	}
	if !cmd.disableMail {
		t.Error("disableMail should be true")
	}
	if cmd.cdnPusher != pusher {
		t.Errorf("cdnPusher = %q, want %q", cmd.cdnPusher, pusher)
	}
	if cmd.mailUser != "ops@example.com" {
		t.Errorf("mailUser = %q, want %q",
			cmd.mailUser, "ops@example.com")
	}
	if !cmd.verbose {
		t.Error("verbose should be true")
	}
}

func TestAllParseArgsEmptyBuildID(t *testing.T) {
	origEuid := getEuid
	getEuid = func() int { return 0 }
	defer func() { getEuid = origEuid }()

	cmd := NewAllCommand()
	err := cmd.parseArgs([]string{"--build-id", ""})
	if err == nil {
		t.Fatal("expected error for empty build ID")
	}
	if !strings.Contains(err.Error(), "build ID cannot be empty") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAllSeederFilterArgs(t *testing.T) {
	cmd := &AllCommand{}

	// No filters.
	if args := cmd.seederFilterArgs(); len(args) != 0 {
		t.Errorf("expected empty args, got %v", args)
	}

	// Skip only.
	cmd.skipSeedersRaw = "a,b"
	args := cmd.seederFilterArgs()
	if len(args) != 1 || args[0] != "--skip-seeders=a,b" {
		t.Errorf("unexpected args: %v", args)
	}

	// Both.
	cmd.onlySeedersRaw = "c"
	args = cmd.seederFilterArgs()
	if len(args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(args))
	}
	if args[0] != "--skip-seeders=a,b" {
		t.Errorf("args[0] = %q, want %q", args[0], "--skip-seeders=a,b")
	}
	if args[1] != "--only-seeders=c" {
		t.Errorf("args[1] = %q, want %q", args[1], "--only-seeders=c")
	}
}

func TestAllCDNPusherNotExecutable(t *testing.T) {
	tmp := t.TempDir()
	pusher := filepath.Join(tmp, "pusher.sh")
	if err := os.WriteFile(pusher, []byte("#!/bin/sh\n"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	cmd := &AllCommand{cdnPusher: pusher}
	cmd.StartUI()
	cmd.SetupPrinters("test")

	err := cmd.runCDNPusher(nil, false)
	if err == nil {
		t.Fatal("expected error for non-executable CDN pusher")
	}
	if !strings.Contains(err.Error(), "not executable") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAllCDNPusherMissing(t *testing.T) {
	cmd := &AllCommand{cdnPusher: "/nonexistent/pusher"}
	cmd.StartUI()
	cmd.SetupPrinters("test")

	err := cmd.runCDNPusher(nil, false)
	if err == nil {
		t.Fatal("expected error for missing CDN pusher")
	}
	if !strings.Contains(err.Error(), "file not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestAllCDNPusherEmpty(t *testing.T) {
	cmd := NewAllCommand()
	cmd.StartUI()
	cmd.SetupPrinters("test")

	err := cmd.runCDNPusher(nil, false)
	if err != nil {
		t.Errorf("expected nil error for empty CDN pusher, got: %v", err)
	}
}

func TestAcquireFileLock(t *testing.T) {
	tmp := t.TempDir()
	lockPath := filepath.Join(tmp, "test.lock")

	unlock, err := filesystems.AcquireFileLock(lockPath, 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireFileLock: %v", err)
	}
	defer unlock()

	// Lock file should exist.
	if _, err := os.Stat(lockPath); err != nil {
		t.Errorf("lock file should exist: %v", err)
	}
}
