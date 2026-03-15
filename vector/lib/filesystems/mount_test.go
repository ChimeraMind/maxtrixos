package filesystems

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"golang.org/x/sys/unix"

	"matrixos/vector/lib/runner"
)

func TestMountpointToDevice(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/mnt", Source: "/dev/sda1", FSType: "ext4"},
		})

		device, err := MountpointToDevice("/mnt")
		if err != nil {
			t.Fatalf("MountpointToDevice failed: %v", err)
		}
		if device != "/dev/sda1" {
			t.Errorf("Expected device /dev/sda1, got %s", device)
		}
	})

	t.Run("NoMountpoint", func(t *testing.T) {
		_, err := MountpointToDevice("")
		if err == nil {
			t.Error("Expected error for missing mountpoint, got nil")
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{})
		_, err := MountpointToDevice("/mnt")
		if err == nil {
			t.Error("Expected error when mount not found")
		}
	})

	t.Run("MultipleOutputs", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/mnt", Source: "/dev/sda1", FSType: "ext4"},
			{Mountpoint: "/mnt", Source: "/dev/sda2", FSType: "ext4"},
		})
		device, err := MountpointToDevice("/mnt")
		if err != nil {
			t.Fatalf("MountpointToDevice failed: %v", err)
		}
		if device != "/dev/sda2" {
			t.Errorf("Expected most recent device /dev/sda2, got %s", device)
		}
	})
}

func TestMountpointToUUID(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		setupMockBlkid(t, map[string]string{
			"/dev/sda1:UUID": "abcd-1234-ef56-7890",
		})
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/mnt", Source: "/dev/sda1", FSType: "ext4"},
		})

		uuid, err := MountpointToUUID("/mnt")
		if err != nil {
			t.Fatalf("MountpointToUUID failed: %v", err)
		}
		if uuid != "abcd-1234-ef56-7890" {
			t.Errorf("Expected UUID abcd-1234-ef56-7890, got %s", uuid)
		}
	})

	t.Run("NoMountpoint", func(t *testing.T) {
		_, err := MountpointToUUID("")
		if err == nil {
			t.Error("Expected error for missing mountpoint, got nil")
		}
	})

	t.Run("MountNotFound", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{})
		_, err := MountpointToUUID("/mnt")
		if err == nil {
			t.Error("Expected error when mount not found")
		}
	})

	t.Run("NoUUID", func(t *testing.T) {
		setupMockBlkidFail(t)
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/mnt", Source: "tmpfs", FSType: "tmpfs"},
		})
		_, err := MountpointToUUID("/mnt")
		if err == nil {
			t.Error("Expected error for no UUID found, got nil")
		}
	})
}

func TestMountpointToFSType(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/mnt", Source: "/dev/sda1", FSType: "ext4"},
		})

		fstype, err := MountpointToFSType("/mnt")
		if err != nil {
			t.Fatalf("MountpointToFSType failed: %v", err)
		}
		if fstype != "ext4" {
			t.Errorf("Expected FSTYPE ext4, got %s", fstype)
		}
	})

	t.Run("SuccessVfat", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/boot/efi", Source: "/dev/sda2", FSType: "vfat"},
		})

		fstype, err := MountpointToFSType("/boot/efi")
		if err != nil {
			t.Fatalf("MountpointToFSType failed: %v", err)
		}
		if fstype != "vfat" {
			t.Errorf("Expected FSTYPE vfat, got %s", fstype)
		}
	})

	t.Run("NoMountpoint", func(t *testing.T) {
		_, err := MountpointToFSType("")
		if err == nil {
			t.Error("Expected error for missing mountpoint, got nil")
		}
	})

	t.Run("MountNotFound", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{})
		_, err := MountpointToFSType("/mnt")
		if err == nil {
			t.Error("Expected error when mount not found")
		}
	})
}

func TestListSubmounts(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/mnt/test"},
			{Mountpoint: "/mnt/test/sub"},
		})

		submounts, err := ListSubmounts("/mnt/test")
		if err != nil {
			t.Fatalf("ListSubmounts failed: %v", err)
		}
		if len(submounts) != 2 {
			t.Errorf("Expected 2 submounts, got %d", len(submounts))
		}
	})

	t.Run("NoMountpoint", func(t *testing.T) {
		_, err := ListSubmounts("")
		if err == nil {
			t.Error("Expected error for missing mountpoint, got nil")
		}
	})

	t.Run("ReadFail", func(t *testing.T) {
		setupMockMountInfoFail(t)
		_, err := ListSubmounts("/mnt")
		if err == nil {
			t.Error("Expected error from mountinfo read failure")
		}
	})
}

func TestCheckDirNotFsRoot(t *testing.T) {
	err := CheckDirNotFsRoot("/")
	if err == nil {
		t.Error("CheckDirNotFsRoot(/) should fail")
	}

	tmpDir := t.TempDir()
	err = CheckDirNotFsRoot(tmpDir)
	if err != nil {
		t.Errorf("CheckDirNotFsRoot(tmpDir) failed: %v", err)
	}
}

func TestCleanupMounts(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	t.Run("SuccessfulUnmount", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/mnt/test", Source: "/dev/sda1"},
		})
		CleanupMounts(CleanupMountsOptions{
			Mounts: []string{"/mnt/test"},
		})
	})

	t.Run("MountNotExist", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{})
		CleanupMounts(CleanupMountsOptions{
			Mounts: []string{"/mnt/test"},
		})
	})

	t.Run("UnmountFail", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/mnt/fail", Source: "/dev/sda1"},
		})
		os.Setenv("MOCK_UMOUNT_FAIL", "1")
		defer os.Unsetenv("MOCK_UMOUNT_FAIL")
		// Should not panic or error out, just log
		CleanupMounts(CleanupMountsOptions{
			Mounts: []string{"/mnt/fail"},
		})
	})
}

func TestSetupCommonRootfsMounts(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatal(err)
	}

	var mountingCalls, mountedCalls []string
	mounter, err := NewCommonRootfsMounts(
		CommonRootfsMountsOptions{
			MountPoint: tmpDir,
			MountProc:  true,
			Mounting: func(tg string) {
				mountingCalls = append(mountingCalls, tg)
			},
			Mounted: func(tg string) {
				mountedCalls = append(mountedCalls, tg)
			},
		},
	)
	defer mounter.Cleanup()

	if err != nil {
		t.Fatalf("NewCommonRootfsMounts failed: %v", err)
	}
	if err := mounter.Setup(); err != nil {
		t.Errorf("Setup failed: %v", err)
	}
	if len(mountingCalls) != 6 {
		t.Errorf("Expected 6 mounting calls, got %d", len(mountingCalls))
	}
	if len(mountedCalls) != 6 {
		t.Errorf("Expected 6 mounted calls, got %d", len(mountedCalls))
	}
}

func TestSetupCommonRootfsMountsProcDisabled(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	tmpDir := t.TempDir()
	if err := os.MkdirAll(tmpDir, 0755); err != nil {
		t.Fatal(err)
	}

	var mountingCalls, mountedCalls []string
	mounter, err := NewCommonRootfsMounts(
		CommonRootfsMountsOptions{
			MountPoint: tmpDir,
			MountProc:  false,
			Mounting: func(tg string) {
				mountingCalls = append(mountingCalls, tg)
			},
			Mounted: func(tg string) {
				mountedCalls = append(mountedCalls, tg)
			},
		},
	)
	defer mounter.Cleanup()

	if err != nil {
		t.Fatalf("NewCommonRootfsMounts failed: %v", err)
	}
	if err := mounter.Setup(); err != nil {
		t.Errorf("Setup failed: %v", err)
	}
	// Without MountProc, expect 5 mounts: /dev, /dev/pts, /sys, dev/shm, run/lock
	if len(mountingCalls) != 5 {
		t.Errorf("Expected 5 mounting calls, got %d", len(mountingCalls))
	}
	if len(mountedCalls) != 5 {
		t.Errorf("Expected 5 mounted calls, got %d", len(mountedCalls))
	}
	// Verify proc was not mounted
	for _, call := range mountingCalls {
		if filepath.Base(call) == "proc" {
			t.Error("proc should not be mounted when MountProc is false")
		}
	}
}

func TestNewCommonRootfsMounts_SkipIfMounted(t *testing.T) {
	t.Run("Skips if already mounted", func(t *testing.T) {
		setupMockExec(t)
		setupMockSyscalls(t)

		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		preMountedDev := filepath.Join(tmpDir, "dev")
		if err := os.MkdirAll(preMountedDev, 0755); err != nil {
			t.Fatal(err)
		}

		// Mock that /dev is already mounted
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: preMountedDev},
		})

		var mountingCalls, skippingCalls []string
		mounter, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				MountPoint:    tmpDir,
				SkipIfMounted: true,
				Mounting: func(tg string) {
					mountingCalls = append(mountingCalls, tg)
				},
				Skipping: func(tg string) {
					skippingCalls = append(skippingCalls, tg)
				},
			},
		)
		if err != nil {
			t.Fatalf("NewCommonRootfsMounts failed: %v", err)
		}
		defer mounter.Cleanup()

		if err := mounter.Setup(); err != nil {
			t.Errorf("Setup failed: %v", err)
		}

		if len(skippingCalls) != 1 {
			t.Errorf("Expected 1 skipping call, got %d: %v", len(skippingCalls), skippingCalls)
		}
		if skippingCalls[0] != preMountedDev {
			t.Errorf("Expected to skip %s, but skipped %s", preMountedDev, skippingCalls[0])
		}

		for _, mnt := range mountingCalls {
			if mnt == preMountedDev {
				t.Errorf("Should not have tried to mount %s", preMountedDev)
			}
		}
	})

	t.Run("Skips dev/shm if already mounted", func(t *testing.T) {
		setupMockExec(t)
		setupMockSyscalls(t)

		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		preMountedDevShm := filepath.Join(tmpDir, "dev", "shm")
		if err := os.MkdirAll(preMountedDevShm, 0755); err != nil {
			t.Fatal(err)
		}

		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: preMountedDevShm},
		})

		var mountingCalls, skippingCalls []string
		mounter, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				MountPoint:    tmpDir,
				SkipIfMounted: true,
				Mounting: func(tg string) {
					mountingCalls = append(mountingCalls, tg)
				},
				Skipping: func(tg string) {
					skippingCalls = append(skippingCalls, tg)
				},
			},
		)
		if err != nil {
			t.Fatalf("NewCommonRootfsMounts failed: %v", err)
		}
		defer mounter.Cleanup()

		if err := mounter.Setup(); err != nil {
			t.Errorf("Setup failed: %v", err)
		}

		if len(skippingCalls) != 1 {
			t.Errorf("Expected 1 skipping call for dev/shm, got %d: %v", len(skippingCalls), skippingCalls)
		}
		if len(skippingCalls) > 0 && skippingCalls[0] != preMountedDevShm {
			t.Errorf("Expected to skip %s, but skipped %s", preMountedDevShm, skippingCalls[0])
		}
		for _, mnt := range mountingCalls {
			if mnt == preMountedDevShm {
				t.Errorf("Should not have tried to mount %s", preMountedDevShm)
			}
		}
	})

	t.Run("Skips proc if already mounted", func(t *testing.T) {
		setupMockExec(t)
		setupMockSyscalls(t)

		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		preMountedProc := filepath.Join(tmpDir, "proc")
		if err := os.MkdirAll(preMountedProc, 0755); err != nil {
			t.Fatal(err)
		}

		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: preMountedProc},
		})

		var mountingCalls, skippingCalls []string
		mounter, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				MountPoint:    tmpDir,
				MountProc:     true,
				SkipIfMounted: true,
				Mounting: func(tg string) {
					mountingCalls = append(mountingCalls, tg)
				},
				Skipping: func(tg string) {
					skippingCalls = append(skippingCalls, tg)
				},
			},
		)
		if err != nil {
			t.Fatalf("NewCommonRootfsMounts failed: %v", err)
		}
		defer mounter.Cleanup()

		if err := mounter.Setup(); err != nil {
			t.Errorf("Setup failed: %v", err)
		}

		if len(skippingCalls) != 1 {
			t.Errorf("Expected 1 skipping call for proc, got %d: %v", len(skippingCalls), skippingCalls)
		}
		if len(skippingCalls) > 0 && skippingCalls[0] != preMountedProc {
			t.Errorf("Expected to skip %s, but skipped %s", preMountedProc, skippingCalls[0])
		}
		for _, mnt := range mountingCalls {
			if mnt == preMountedProc {
				t.Errorf("Should not have tried to mount %s", preMountedProc)
			}
		}
	})

	t.Run("Skips run/lock if already mounted", func(t *testing.T) {
		setupMockExec(t)
		setupMockSyscalls(t)

		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		preMountedRunLock := filepath.Join(tmpDir, "run", "lock")
		if err := os.MkdirAll(preMountedRunLock, 0755); err != nil {
			t.Fatal(err)
		}

		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: preMountedRunLock},
		})

		var mountingCalls, skippingCalls []string
		mounter, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				MountPoint:    tmpDir,
				SkipIfMounted: true,
				Mounting: func(tg string) {
					mountingCalls = append(mountingCalls, tg)
				},
				Skipping: func(tg string) {
					skippingCalls = append(skippingCalls, tg)
				},
			},
		)
		if err != nil {
			t.Fatalf("NewCommonRootfsMounts failed: %v", err)
		}
		defer mounter.Cleanup()

		if err := mounter.Setup(); err != nil {
			t.Errorf("Setup failed: %v", err)
		}

		if len(skippingCalls) != 1 {
			t.Errorf("Expected 1 skipping call for run/lock, got %d: %v", len(skippingCalls), skippingCalls)
		}
		if len(skippingCalls) > 0 && skippingCalls[0] != preMountedRunLock {
			t.Errorf("Expected to skip %s, but skipped %s", preMountedRunLock, skippingCalls[0])
		}
		for _, mnt := range mountingCalls {
			if mnt == preMountedRunLock {
				t.Errorf("Should not have tried to mount %s", preMountedRunLock)
			}
		}
	})

	t.Run("Skips all mounts if everything pre-mounted", func(t *testing.T) {
		setupMockExec(t)
		setupMockSyscalls(t)

		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		allMounts := []string{
			filepath.Join(tmpDir, "dev"),
			filepath.Join(tmpDir, "dev", "pts"),
			filepath.Join(tmpDir, "sys"),
			filepath.Join(tmpDir, "dev", "shm"),
			filepath.Join(tmpDir, "proc"),
			filepath.Join(tmpDir, "run", "lock"),
		}
		var entries []*MountInfoEntry
		for _, m := range allMounts {
			if err := os.MkdirAll(m, 0755); err != nil {
				t.Fatal(err)
			}
			entries = append(entries, &MountInfoEntry{Mountpoint: m})
		}
		setupMockMountInfo(t, entries)

		var mountingCalls, skippingCalls []string
		mounter, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				MountPoint:    tmpDir,
				MountProc:     true,
				SkipIfMounted: true,
				Mounting: func(tg string) {
					mountingCalls = append(mountingCalls, tg)
				},
				Skipping: func(tg string) {
					skippingCalls = append(skippingCalls, tg)
				},
			},
		)
		if err != nil {
			t.Fatalf("NewCommonRootfsMounts failed: %v", err)
		}
		defer mounter.Cleanup()

		if err := mounter.Setup(); err != nil {
			t.Errorf("Setup failed: %v", err)
		}

		if len(mountingCalls) != 0 {
			t.Errorf("Expected 0 mounting calls when all pre-mounted, got %d: %v",
				len(mountingCalls), mountingCalls)
		}
		if len(skippingCalls) != 6 {
			t.Errorf("Expected 6 skipping calls, got %d: %v",
				len(skippingCalls), skippingCalls)
		}
	})

	t.Run("Fails if already mounted and not skipping", func(t *testing.T) {
		setupMockExec(t)
		// Don't use setupMockSyscalls, we need a custom Mount mock
		origMount := Mount
		t.Cleanup(func() { Mount = origMount })

		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		preMountedDev := filepath.Join(tmpDir, "dev")
		if err := os.MkdirAll(preMountedDev, 0755); err != nil {
			t.Fatal(err)
		}

		// Mock that /dev is already mounted
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: preMountedDev},
		})

		Mount = func(source, target, fstype string, flags uintptr, data string) error {
			if target == preMountedDev {
				return fmt.Errorf("mock mount failed: already mounted")
			}
			return nil
		}

		mounter, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				MountPoint:    tmpDir,
				SkipIfMounted: false, // This is the default, but let's be explicit
			},
		)
		if err != nil {
			t.Fatalf("NewCommonRootfsMounts failed: %v", err)
		}
		defer mounter.Cleanup()

		err = mounter.Setup()
		if err == nil {
			t.Error("Setup should have failed, but it didn't")
		}
		if err != nil {
			expectedError := "mock mount failed: already mounted"
			if !strings.Contains(err.Error(), expectedError) {
				t.Errorf("Setup() failed with wrong error: got %q, want something containing %q", err, expectedError)
			}
		}
	})
}

func TestBindMount(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	src := t.TempDir()
	dst := t.TempDir()

	bm, err := NewBindMount(BindMountOptions{
		Src: src,
		Dst: dst,
	})
	if err != nil {
		t.Errorf("NewBindMount failed: %v", err)
	}
	if bm == nil {
		t.Fatal("NewBindMount returned nil")
	}
	if bm.Dst() != dst {
		t.Errorf("Dst() = %q, want %q", bm.Dst(), dst)
	}
	if err := bm.Mount(); err != nil {
		t.Errorf("Mount() failed: %v", err)
	}
}

func TestBindMountReadOnly(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	src := t.TempDir()
	dst := t.TempDir()

	t.Run("ReadOnlyMounts", func(t *testing.T) {
		var mountCalls []uintptr
		origMount := Mount
		Mount = func(source, target, fstype string, flags uintptr, data string) error {
			mountCalls = append(mountCalls, flags)
			return nil
		}
		t.Cleanup(func() { Mount = origMount })

		bm, err := NewBindMount(BindMountOptions{
			Src:      src,
			Dst:      dst,
			ReadOnly: true,
		})
		if err != nil {
			t.Fatalf("NewBindMount failed: %v", err)
		}
		if err := bm.Mount(); err != nil {
			t.Fatalf("Mount() failed: %v", err)
		}

		// Expect 3 mount calls: MS_BIND, MS_SLAVE, MS_REMOUNT|MS_RDONLY|MS_BIND.
		if len(mountCalls) != 3 {
			t.Fatalf("expected 3 mount calls, got %d", len(mountCalls))
		}
		roFlags := uintptr(unix.MS_REMOUNT | unix.MS_RDONLY | unix.MS_BIND)
		if mountCalls[2] != roFlags {
			t.Errorf("third mount flags = %#x, want %#x", mountCalls[2], roFlags)
		}
	})

	t.Run("ReadOnlyRemountFails", func(t *testing.T) {
		callIdx := 0
		origMount := Mount
		Mount = func(source, target, fstype string, flags uintptr, data string) error {
			callIdx++
			if callIdx == 3 {
				return fmt.Errorf("mock remount RO fail")
			}
			return nil
		}
		t.Cleanup(func() { Mount = origMount })

		newSrc := t.TempDir()
		newDst := t.TempDir()
		bm, err := NewBindMount(BindMountOptions{
			Src:      newSrc,
			Dst:      newDst,
			ReadOnly: true,
		})
		if err != nil {
			t.Fatalf("NewBindMount failed: %v", err)
		}
		if err := bm.Mount(); err == nil {
			t.Error("Mount() should fail when RO remount fails")
		}
	})

	t.Run("WithoutReadOnly", func(t *testing.T) {
		var mountCalls []uintptr
		origMount := Mount
		Mount = func(source, target, fstype string, flags uintptr, data string) error {
			mountCalls = append(mountCalls, flags)
			return nil
		}
		t.Cleanup(func() { Mount = origMount })

		newSrc := t.TempDir()
		newDst := t.TempDir()
		bm, err := NewBindMount(BindMountOptions{
			Src: newSrc,
			Dst: newDst,
		})
		if err != nil {
			t.Fatalf("NewBindMount failed: %v", err)
		}
		if err := bm.Mount(); err != nil {
			t.Fatalf("Mount() failed: %v", err)
		}

		// Expect 2 mount calls: MS_BIND, MS_SLAVE (no RO remount).
		if len(mountCalls) != 2 {
			t.Fatalf("expected 2 mount calls, got %d", len(mountCalls))
		}
	})
}

func TestBindMountMkdirAll(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "sub", "dir")

	bm, err := NewBindMount(BindMountOptions{
		Src:      src,
		Dst:      dst,
		MkdirAll: true,
	})
	if err != nil {
		t.Fatalf("NewBindMount with MkdirAll failed: %v", err)
	}
	if !DirectoryExists(dst) {
		t.Error("MkdirAll did not create destination directory")
	}
	if err := bm.Mount(); err != nil {
		t.Errorf("Mount() failed: %v", err)
	}
}

func TestBindMountDoubleMount(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	src := t.TempDir()
	dst := t.TempDir()

	bm, err := NewBindMount(BindMountOptions{
		Src: src,
		Dst: dst,
	})
	if err != nil {
		t.Fatalf("NewBindMount failed: %v", err)
	}
	if err := bm.Mount(); err != nil {
		t.Fatalf("first Mount() failed: %v", err)
	}
	if err := bm.Mount(); err == nil {
		t.Error("second Mount() should fail but didn't")
	}
}

func TestCleanupLoopDevices(t *testing.T) {
	setupMockExec(t)

	f, err := os.CreateTemp("", "loop")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	defer os.Remove(f.Name())

	// Mock sysfs backing_file read to make the device look attached.
	setupMockLoop(t)
	loopDir := filepath.Join(sysBlockPrefix, filepath.Base(f.Name()), "loop")
	if err := os.MkdirAll(loopDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(loopDir, "backing_file"), []byte("/path/to/backing/file\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// openFile for Detach returns a temp file, ioctl succeeds via setupMockLoop defaults.
	tmp, err2 := os.CreateTemp("", "loopdev")
	if err2 != nil {
		t.Fatal(err2)
	}
	defer os.Remove(tmp.Name())
	openFile = func(name string, flag int, perm os.FileMode) (*os.File, error) {
		return tmp, nil
	}

	CleanupLoopDevices([]string{f.Name()})
}

func TestCheckActiveMounts(t *testing.T) {
	t.Run("NoActiveMounts", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{})
		if err := CheckActiveMounts("/mnt/test"); err != nil {
			t.Errorf("CheckActiveMounts failed: %v", err)
		}
	})

	t.Run("ActiveMountsDetected", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: "/mnt/test/proc", Source: "proc", FSType: "proc"},
		})
		if err := CheckActiveMounts("/mnt/test"); err == nil {
			t.Error("CheckActiveMounts should fail when mounts are detected")
		}
	})
}

func TestCommonRootfsMountsCleanup(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	t.Run("Success", func(t *testing.T) {
		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		// Mock that the mounts exist
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: filepath.Join(tmpDir, "dev")},
			{Mountpoint: filepath.Join(tmpDir, "dev", "pts")},
			{Mountpoint: filepath.Join(tmpDir, "sys")},
			{Mountpoint: filepath.Join(tmpDir, "dev", "shm")},
			{Mountpoint: filepath.Join(tmpDir, "proc")},
			{Mountpoint: filepath.Join(tmpDir, "run", "lock")},
		})
		noop := func(string) {}
		mounter, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				MountPoint: tmpDir,
				MountProc:  true,
				Mounting:   noop,
				Mounted:    noop,
			},
		)
		if err != nil {
			t.Fatalf("NewCommonRootfsMounts failed: %v", err)
		}
		if err := mounter.Setup(); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
		if err := mounter.Cleanup(); err != nil {
			t.Errorf("Cleanup failed: %v", err)
		}
	})

	t.Run("SuccessProcDisabled", func(t *testing.T) {
		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		// Mock mounts without proc
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: filepath.Join(tmpDir, "dev")},
			{Mountpoint: filepath.Join(tmpDir, "dev", "pts")},
			{Mountpoint: filepath.Join(tmpDir, "sys")},
			{Mountpoint: filepath.Join(tmpDir, "dev", "shm")},
			{Mountpoint: filepath.Join(tmpDir, "run", "lock")},
		})
		noop := func(string) {}
		mounter, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				MountPoint: tmpDir,
				MountProc:  false,
				Mounting:   noop,
				Mounted:    noop,
			},
		)
		if err != nil {
			t.Fatalf("NewCommonRootfsMounts failed: %v", err)
		}
		if err := mounter.Setup(); err != nil {
			t.Fatalf("Setup failed: %v", err)
		}
		if err := mounter.Cleanup(); err != nil {
			t.Errorf("Cleanup failed: %v", err)
		}
	})

	t.Run("MissingMnt", func(t *testing.T) {
		noop := func(string) {}
		_, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				Mounting: noop,
				Mounted:  noop,
			},
		)
		if err == nil {
			t.Error("Expected error for missing mnt, got nil")
		}
	})

	t.Run("NonExistentMnt", func(t *testing.T) {
		noop := func(string) {}
		mounter, err := NewCommonRootfsMounts(
			CommonRootfsMountsOptions{
				MountPoint: "/non/existent/path",
				Mounting:   noop,
				Mounted:    noop,
			},
		)
		if err != nil {
			t.Fatalf("NewCommonRootfsMounts failed: %v", err)
		}
		if err := mounter.Setup(); err == nil {
			t.Error("Expected error for non-existent mnt, got nil")
		}
	})
}

func TestBindUmount(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	t.Run("Success", func(t *testing.T) {
		tmpDir, _ := filepath.EvalSymlinks(t.TempDir())
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: tmpDir},
		})
		src, _ := filepath.EvalSymlinks(t.TempDir())
		bm, err := NewBindMount(BindMountOptions{
			Src: src,
			Dst: tmpDir,
		})
		if err != nil {
			t.Fatalf("NewBindMount failed: %v", err)
		}
		if err := bm.Mount(); err != nil {
			t.Fatalf("Mount() failed: %v", err)
		}
		if err := bm.Unmount(); err != nil {
			t.Errorf("Unmount failed: %v", err)
		}
		// Double unmount should be a no-op.
		if err := bm.Unmount(); err != nil {
			t.Errorf("second Unmount failed: %v", err)
		}
	})

	t.Run("MissingSrc", func(t *testing.T) {
		_, err := NewBindMount(BindMountOptions{
			Dst: "/tmp",
		})
		if err == nil {
			t.Error("Expected error for missing src, got nil")
		}
	})

	t.Run("MissingDst", func(t *testing.T) {
		_, err := NewBindMount(BindMountOptions{
			Src: "/tmp",
		})
		if err == nil {
			t.Error("Expected error for missing dst, got nil")
		}
	})

	t.Run("NonExistentSrc", func(t *testing.T) {
		_, err := NewBindMount(BindMountOptions{
			Src: "/non/existent/path",
			Dst: t.TempDir(),
		})
		if err == nil {
			t.Error("Expected error for non-existent src, got nil")
		}
	})

	t.Run("NonExistentDst", func(t *testing.T) {
		_, err := NewBindMount(BindMountOptions{
			Src: t.TempDir(),
			Dst: "/non/existent/path",
		})
		if err == nil {
			t.Error("Expected error for non-existent dst, got nil")
		}
	})
}

func TestBindMountDistdir(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	t.Run("Success", func(t *testing.T) {
		distfilesDir := t.TempDir()
		rootfs := t.TempDir()
		bm, err := BindMountDistdir(BindMountDistdirOptions{
			DistfilesDir: distfilesDir,
			Rootfs:       rootfs,
		})
		if err != nil {
			t.Errorf("BindMountDistdir failed: %v", err)
		}
		if bm == nil {
			t.Fatal("BindMountDistdir returned nil")
		}
		expected := filepath.Join(
			rootfs, "var", "cache", "distfiles",
		)
		if bm.Dst() != expected {
			t.Errorf("Dst() = %q, want %q", bm.Dst(), expected)
		}
		if err := bm.Mount(); err != nil {
			t.Errorf("Mount() failed: %v", err)
		}
	})
}

func TestBindUmountDistdir(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	t.Run("Success", func(t *testing.T) {
		rootfs := t.TempDir()
		distfilesDir := t.TempDir()
		bm, err := BindMountDistdir(BindMountDistdirOptions{
			DistfilesDir: distfilesDir,
			Rootfs:       rootfs,
		})
		if err != nil {
			t.Fatalf("BindMountDistdir failed: %v", err)
		}
		if err := bm.Mount(); err != nil {
			t.Fatalf("Mount() failed: %v", err)
		}
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: bm.Dst()},
		})
		if err := bm.Unmount(); err != nil {
			t.Errorf("Unmount failed: %v", err)
		}
	})
}

func TestBindMountBinpkgs(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	t.Run("Success", func(t *testing.T) {
		binpkgsDir := t.TempDir()
		rootfs := t.TempDir()
		bm, err := BindMountBinpkgs(BindMountBinpkgsOptions{
			BinpkgsDir: binpkgsDir,
			Rootfs:     rootfs,
		})
		if err != nil {
			t.Errorf("BindMountBinpkgs failed: %v", err)
		}
		if bm == nil {
			t.Fatal("BindMountBinpkgs returned nil")
		}
		expected := filepath.Join(
			rootfs, "var", "cache", "binpkgs",
		)
		if bm.Dst() != expected {
			t.Errorf("Dst() = %q, want %q", bm.Dst(), expected)
		}
		if err := bm.Mount(); err != nil {
			t.Errorf("Mount() failed: %v", err)
		}
	})
}

func TestBindUmountBinpkgs(t *testing.T) {
	setupMockExec(t)
	setupMockSyscalls(t)

	t.Run("Success", func(t *testing.T) {
		rootfs := t.TempDir()
		binpkgsDir := t.TempDir()
		bm, err := BindMountBinpkgs(BindMountBinpkgsOptions{
			BinpkgsDir: binpkgsDir,
			Rootfs:     rootfs,
		})
		if err != nil {
			t.Fatalf("BindMountBinpkgs failed: %v", err)
		}
		if err := bm.Mount(); err != nil {
			t.Fatalf("Mount() failed: %v", err)
		}
		setupMockMountInfo(t, []*MountInfoEntry{
			{Mountpoint: bm.Dst()},
		})
		if err := bm.Unmount(); err != nil {
			t.Errorf("Unmount failed: %v", err)
		}
	})
}

func TestCleanupCryptsetupDevices(t *testing.T) {
	setupMockExec(t)

	t.Run("Success", func(t *testing.T) {
		tmpDir := t.TempDir()

		originalDevMapperPrefix := devMapperPrefix
		t.Cleanup(func() { devMapperPrefix = originalDevMapperPrefix })
		devMapperPrefix = tmpDir

		devPath := filepath.Join(tmpDir, "mycrypt")
		if _, err := os.Create(devPath); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(devPath)
		CleanupCryptsetupDevices([]string{"mycrypt"})
	})

	t.Run("CloseFail", func(t *testing.T) {
		setupMockMountInfo(t, []*MountInfoEntry{})
		os.Setenv("MOCK_CRYPTSETUP_FAIL", "1")
		defer os.Unsetenv("MOCK_CRYPTSETUP_FAIL")
		tmpDir := t.TempDir()

		originalDevMapperPrefix := devMapperPrefix
		t.Cleanup(func() { devMapperPrefix = originalDevMapperPrefix })
		devMapperPrefix = tmpDir

		devPath := filepath.Join(tmpDir, "mycrypt")
		if _, err := os.Create(devPath); err != nil {
			t.Fatal(err)
		}
		defer os.Remove(devPath)

		// Should not panic or error out, just log
		CleanupCryptsetupDevices([]string{"mycrypt"})
	})
}

func TestRemountReadWriteIntegration(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("skipping integration test: requires root")
	}

	tmpDir := t.TempDir()

	// Mount a tmpfs.
	if err := unix.Mount("tmpfs", tmpDir, "tmpfs", 0, "size=1M"); err != nil {
		t.Fatalf("mount tmpfs: %v", err)
	}
	t.Cleanup(func() { unix.Unmount(tmpDir, 0) })

	// Remount read-only.
	if err := unix.Mount("", tmpDir, "", unix.MS_REMOUNT|unix.MS_RDONLY, ""); err != nil {
		t.Fatalf("remount ro: %v", err)
	}

	// Verify it's read-only.
	testFile := filepath.Join(tmpDir, "probe")
	if err := os.WriteFile(testFile, []byte("x"), 0644); err == nil {
		t.Fatal("expected write to fail on read-only mount")
	}

	// Use the real RemountReadWrite.
	if err := RemountReadWrite(tmpDir); err != nil {
		t.Fatalf("RemountReadWrite: %v", err)
	}

	// Verify it's writable.
	if err := os.WriteFile(testFile, []byte("x"), 0644); err != nil {
		t.Fatalf("write after RemountReadWrite failed: %v", err)
	}
}

func TestRemountReadWrite(t *testing.T) {
	setupMockExec(t)

	t.Run("EmptyMountpoint", func(t *testing.T) {
		err := RemountReadWrite("")
		if err == nil {
			t.Fatal("expected error for empty mountpoint")
		}
		if !strings.Contains(err.Error(), "missing mnt parameter") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("ExecFails", func(t *testing.T) {
		origExecRun := execRun
		t.Cleanup(func() { execRun = origExecRun })
		execRun = func(c *runner.Cmd) error {
			return fmt.Errorf("mount failed")
		}

		err := RemountReadWrite("/mnt")
		if err == nil {
			t.Fatal("expected error when mount command fails")
		}
		if !strings.Contains(err.Error(), "mount failed") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Success", func(t *testing.T) {
		var capturedArgs []string
		origExecRun := execRun
		t.Cleanup(func() { execRun = origExecRun })
		execRun = func(c *runner.Cmd) error {
			capturedArgs = append([]string{c.Name}, c.Args...)
			return nil
		}

		if err := RemountReadWrite("/mnt"); err != nil {
			t.Fatalf("RemountReadWrite failed: %v", err)
		}

		expected := []string{"mount", "-o", "remount,rw", "--", "/mnt"}
		if len(capturedArgs) != len(expected) {
			t.Fatalf("expected args %v, got %v", expected, capturedArgs)
		}
		for i, arg := range expected {
			if capturedArgs[i] != arg {
				t.Errorf("arg[%d]: expected %q, got %q", i, arg, capturedArgs[i])
			}
		}
	})
}
