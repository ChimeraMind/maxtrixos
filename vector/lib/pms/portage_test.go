package pms

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// mkCatPkg creates a category/package directory inside vdb.
func mkCatPkg(t *testing.T, vdb, cat, pkg string) {
	t.Helper()
	dir := filepath.Join(vdb, cat, pkg)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
}

// ---------- PackageList ----------

func TestPackageList_EmptyVdb(t *testing.T) {
	vdb := t.TempDir()

	pkgs, err := PackageList(vdb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 0 {
		t.Fatalf("expected empty list, got %v", pkgs)
	}
}

func TestPackageList_SingleCategorySinglePackage(t *testing.T) {
	vdb := t.TempDir()
	mkCatPkg(t, vdb, "sys-apps", "systemd-256")

	pkgs, err := PackageList(vdb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}
	want := "sys-apps/systemd-256"
	if pkgs[0] != want {
		t.Fatalf("expected %q, got %q", want, pkgs[0])
	}
}

func TestPackageList_MultipleCategories(t *testing.T) {
	vdb := t.TempDir()
	mkCatPkg(t, vdb, "sys-apps", "systemd-256")
	mkCatPkg(t, vdb, "dev-libs", "openssl-3.1")
	mkCatPkg(t, vdb, "app-misc", "screen-4.9")

	pkgs, err := PackageList(vdb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Strings(pkgs)
	want := []string{
		"app-misc/screen-4.9",
		"dev-libs/openssl-3.1",
		"sys-apps/systemd-256",
	}
	if len(pkgs) != len(want) {
		t.Fatalf("expected %d packages, got %d: %v", len(want), len(pkgs), pkgs)
	}
	for i := range want {
		if pkgs[i] != want[i] {
			t.Fatalf("index %d: expected %q, got %q", i, want[i], pkgs[i])
		}
	}
}

func TestPackageList_MultiplePackagesInCategory(t *testing.T) {
	vdb := t.TempDir()
	mkCatPkg(t, vdb, "dev-libs", "openssl-3.1")
	mkCatPkg(t, vdb, "dev-libs", "libxml2-2.12")

	pkgs, err := PackageList(vdb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Strings(pkgs)
	want := []string{
		"dev-libs/libxml2-2.12",
		"dev-libs/openssl-3.1",
	}
	if len(pkgs) != len(want) {
		t.Fatalf("expected %d packages, got %d: %v", len(want), len(pkgs), pkgs)
	}
	for i := range want {
		if pkgs[i] != want[i] {
			t.Fatalf("index %d: expected %q, got %q", i, want[i], pkgs[i])
		}
	}
}

func TestPackageList_SkipsRegularFilesInVdb(t *testing.T) {
	vdb := t.TempDir()
	mkCatPkg(t, vdb, "sys-apps", "util-linux-2.39")

	// Create a regular file at top level of vdb (should be skipped).
	f := filepath.Join(vdb, "world")
	if err := os.WriteFile(f, []byte("sys-apps/util-linux"), 0644); err != nil {
		t.Fatal(err)
	}

	pkgs, err := PackageList(vdb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 package, got %d: %v", len(pkgs), pkgs)
	}
	want := "sys-apps/util-linux-2.39"
	if pkgs[0] != want {
		t.Fatalf("expected %q, got %q", want, pkgs[0])
	}
}

func TestPackageList_NonexistentVdb(t *testing.T) {
	_, err := PackageList("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Fatal("expected error for nonexistent vdb directory")
	}
}

func TestPackageList_UnreadableCategoryDir(t *testing.T) {
	vdb := t.TempDir()
	catDir := filepath.Join(vdb, "broken-cat")
	if err := os.MkdirAll(catDir, 0755); err != nil {
		t.Fatal(err)
	}
	// Remove read permission so ReadDir inside the category fails.
	if err := os.Chmod(catDir, 0000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(catDir, 0755) })

	_, err := PackageList(vdb)
	if err == nil {
		t.Fatal("expected error for unreadable category directory")
	}
}

func TestPackageList_IncludesFilesInsideCategory(t *testing.T) {
	vdb := t.TempDir()
	catDir := filepath.Join(vdb, "dev-libs")
	if err := os.MkdirAll(catDir, 0755); err != nil {
		t.Fatal(err)
	}
	// A regular file inside a category dir is included (mirrors real behavior
	// where ReadDir returns all entries).
	f := filepath.Join(catDir, ".keep")
	if err := os.WriteFile(f, nil, 0644); err != nil {
		t.Fatal(err)
	}
	mkCatPkg(t, vdb, "dev-libs", "glib-2.78")

	pkgs, err := PackageList(vdb)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sort.Strings(pkgs)
	want := []string{
		"dev-libs/.keep",
		"dev-libs/glib-2.78",
	}
	if len(pkgs) != len(want) {
		t.Fatalf("expected %d entries, got %d: %v", len(want), len(pkgs), pkgs)
	}
	for i := range want {
		if pkgs[i] != want[i] {
			t.Fatalf("index %d: expected %q, got %q", i, want[i], pkgs[i])
		}
	}
}
