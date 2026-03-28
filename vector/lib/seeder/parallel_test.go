package seeder

import (
	"context"
	"fmt"
	"io"
	"sync"
	"testing"
)

// --- Test helpers ---

func noopWriter(label string) io.Writer { return io.Discard }
func noopCleanup(fn func())             {}

func defaultParallelOpts(
	sd *MockSeeder,
	seeders []SeederInfo,
	paramsByName map[string]*SeederParams,
) *ParallelSeedOptions {
	return &ParallelSeedOptions{
		Seeders:      seeders,
		ParamsByName: paramsByName,
		Parallelism:  1,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			return sd, nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
	}
}

// --- ResolveChrootDir ---

func TestResolveChrootDirPreferred(t *testing.T) {
	params := &SeederParams{PreferredChrootDir: "/chroot/a"}
	dir, err := ResolveChrootDir("test", params, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/chroot/a" {
		t.Errorf("got %q, want /chroot/a", dir)
	}
}

func TestResolveChrootDirOverride(t *testing.T) {
	params := &SeederParams{PreferredChrootDir: "/chroot/a"}
	dir, err := ResolveChrootDir("test", params, "/override")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir != "/override" {
		t.Errorf("got %q, want /override", dir)
	}
}

func TestResolveChrootDirEmpty(t *testing.T) {
	params := &SeederParams{}
	_, err := ResolveChrootDir("test", params, "")
	if err == nil {
		t.Fatal("expected error for empty chroot dir")
	}
}

// --- seedWorker tests (via ParallelSeed with parallelism=1) ---

func workerSetup(t *testing.T) (*MockSeeder, SeederInfo, *SeederParams) {
	t.Helper()
	chrootDir := t.TempDir()
	sd := DefaultMockSeeder()
	info := SeederInfo{
		Name:        "00-bedrock",
		Dir:         t.TempDir(),
		ChrootExec:  "/bin/chroot",
		PrepperExec: "/bin/prepper",
	}
	params := &SeederParams{
		ChrootName:         "bedrock-20260228",
		PreferredChrootDir: chrootDir,
	}
	return sd, info, params
}

func runSingleWorker(sd *MockSeeder, info SeederInfo, params *SeederParams, opts *ParallelSeedOptions) error {
	if opts == nil {
		opts = defaultParallelOpts(sd, []SeederInfo{info}, map[string]*SeederParams{info.Name: params})
	} else {
		opts.Seeders = []SeederInfo{info}
		opts.ParamsByName = map[string]*SeederParams{info.Name: params}
		if opts.NewSeeder == nil {
			opts.NewSeeder = func(_ *NewSeederOptions) (ISeeder, error) { return sd, nil }
		}
		if opts.NewStdoutWriter == nil {
			opts.NewStdoutWriter = noopWriter
		}
		if opts.NewStderrWriter == nil {
			opts.NewStderrWriter = noopWriter
		}
		if opts.PushCleanup == nil {
			opts.PushCleanup = noopCleanup
		}
	}
	return ParallelSeed(context.Background(), opts)
}

func TestWorkerNoChrootDir(t *testing.T) {
	sd, info, _ := workerSetup(t)
	params := &SeederParams{PreferredChrootDir: ""}

	err := runSingleWorker(sd, info, params, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	// Error is wrapped by ParallelSeed
	if got := err.Error(); !contains(got, "no chroot dir specified") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorkerChrootDirOverride(t *testing.T) {
	sd, info, params := workerSetup(t)
	overrideDir := t.TempDir()
	opts := defaultParallelOpts(sd, nil, nil)
	opts.ChrootDir = overrideDir

	if err := runSingleWorker(sd, info, params, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sd.SeedCalled {
		t.Error("Seed should be called")
	}
}

func TestWorkerDoneFlagError(t *testing.T) {
	sd, info, params := workerSetup(t)
	sd.SeederDoneFlagFileErr = fmt.Errorf("flag file err")

	err := runSingleWorker(sd, info, params, nil)
	if err == nil || !contains(err.Error(), "flag file err") {
		t.Fatalf("expected flag file error, got: %v", err)
	}
}

func TestWorkerIsSeederDoneError(t *testing.T) {
	sd, info, params := workerSetup(t)
	sd.IsSeederDoneErr = fmt.Errorf("done check err")

	err := runSingleWorker(sd, info, params, nil)
	if err == nil || !contains(err.Error(), "done check err") {
		t.Fatalf("expected done check error, got: %v", err)
	}
}

func TestWorkerSkipsDoneSeeder(t *testing.T) {
	sd, info, params := workerSetup(t)
	sd.IsSeederDone_ = true

	if err := runSingleWorker(sd, info, params, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sd.ExecutePrepperCalled {
		t.Error("ExecutePrepper should not be called for done seeder")
	}
	if sd.SeedCalled {
		t.Error("Seed should not be called for done seeder")
	}
}

func TestWorkerPrepperError(t *testing.T) {
	sd, info, params := workerSetup(t)
	sd.ExecutePrepperErr = fmt.Errorf("prepper failed")

	err := runSingleWorker(sd, info, params, nil)
	if err == nil || !contains(err.Error(), "prepper failed") {
		t.Fatalf("expected prepper error, got: %v", err)
	}
}

func TestWorkerDNSError(t *testing.T) {
	sd, info, params := workerSetup(t)
	sd.SetupChrootDNSErr = fmt.Errorf("dns boom")

	err := runSingleWorker(sd, info, params, nil)
	if err == nil || !contains(err.Error(), "DNS setup failed") {
		t.Fatalf("expected DNS error, got: %v", err)
	}
}

func TestWorkerDirsError(t *testing.T) {
	sd, info, params := workerSetup(t)
	sd.SetupChrootDirsErr = fmt.Errorf("dirs boom")

	err := runSingleWorker(sd, info, params, nil)
	if err == nil || !contains(err.Error(), "dir setup failed") {
		t.Fatalf("expected dirs error, got: %v", err)
	}
}

func TestWorkerSeedError(t *testing.T) {
	sd, info, params := workerSetup(t)
	sd.SeedErr = fmt.Errorf("chroot exploded")

	err := runSingleWorker(sd, info, params, nil)
	if err == nil || !contains(err.Error(), "chroot execution failed") {
		t.Fatalf("expected seed error, got: %v", err)
	}
}

func TestWorkerMarkDoneError(t *testing.T) {
	sd, info, params := workerSetup(t)
	sd.MarkSeederDoneErr = fmt.Errorf("mark boom")

	err := runSingleWorker(sd, info, params, nil)
	if err == nil || !contains(err.Error(), "mark boom") {
		t.Fatalf("expected mark done error, got: %v", err)
	}
}

func TestWorkerOnSeederDoneError(t *testing.T) {
	sd, info, params := workerSetup(t)
	opts := defaultParallelOpts(sd, nil, nil)
	opts.OnSeederDone = func(_, _ string) error {
		return fmt.Errorf("record boom")
	}

	err := runSingleWorker(sd, info, params, opts)
	if err == nil || !contains(err.Error(), "failed to call OnSeederDone") {
		t.Fatalf("expected OnSeederDone error, got: %v", err)
	}
}

func TestWorkerResumeAndStage3Flags(t *testing.T) {
	sd, info, params := workerSetup(t)
	opts := defaultParallelOpts(sd, nil, nil)
	opts.Resume = true
	opts.Stage3File = "/tmp/stage3.tar.xz"

	if err := runSingleWorker(sd, info, params, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sd.ExecutePrepperCalled {
		t.Error("ExecutePrepper should be called")
	}
}

func TestWorkerOnSeederDoneCalled(t *testing.T) {
	sd, info, params := workerSetup(t)
	var calledName, calledDir string
	opts := defaultParallelOpts(sd, nil, nil)
	opts.OnSeederDone = func(name, chrootDir string) error {
		calledName = name
		calledDir = chrootDir
		return nil
	}

	if err := runSingleWorker(sd, info, params, opts); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calledName != info.Name {
		t.Errorf("OnSeederDone name: got %q, want %q", calledName, info.Name)
	}
	if calledDir != params.PreferredChrootDir {
		t.Errorf("OnSeederDone dir: got %q, want %q", calledDir, params.PreferredChrootDir)
	}
}

func TestWorkerLockError(t *testing.T) {
	sd, info, params := workerSetup(t)
	sd.ExecuteWithSeederLockErr = fmt.Errorf("lock failed")

	err := runSingleWorker(sd, info, params, nil)
	if err == nil || !contains(err.Error(), "lock failed") {
		t.Fatalf("expected lock error, got: %v", err)
	}
}

// --- Parallel execution tests ---

// parallelTestSeeder wraps MockSeeder to return per-seeder params and
// track execution order.
type parallelTestSeeder struct {
	*MockSeeder
	paramsMap map[string]*SeederParams
	mu        *sync.Mutex
	executed  *[]string
}

func (p *parallelTestSeeder) ParseSeederParams(info SeederInfo) (*SeederParams, error) {
	params, ok := p.paramsMap[info.Name]
	if !ok {
		return nil, fmt.Errorf("unknown seeder: %s", info.Name)
	}
	return params, nil
}

func (p *parallelTestSeeder) Seed(opts *SeedOptions) error {
	p.mu.Lock()
	*p.executed = append(*p.executed, opts.Info.Name)
	p.mu.Unlock()
	return p.MockSeeder.Seed(opts)
}

func setupParallelSeeders(t *testing.T, names []string) ([]SeederInfo, map[string]string) {
	t.Helper()
	var infos []SeederInfo
	chrootDirs := make(map[string]string)
	for _, name := range names {
		seederDir := t.TempDir()
		chrootDir := t.TempDir()
		chrootDirs[name] = chrootDir
		infos = append(infos, SeederInfo{
			Name:        name,
			Dir:         seederDir,
			ChrootExec:  "/bin/chroot",
			PrepperExec: "/bin/prepper",
		})
	}
	return infos, chrootDirs
}

func TestParallelSeedBasic(t *testing.T) {
	names := []string{"00-bedrock", "10-server", "20-gnome"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-bedrock": {
			ChrootName:         "bedrock",
			PreferredChrootDir: chrootDirs["00-bedrock"],
		},
		"10-server": {
			Depends:            []string{"00-bedrock"},
			ChrootName:         "server",
			PreferredChrootDir: chrootDirs["10-server"],
		},
		"20-gnome": {
			Depends:            []string{"00-bedrock"},
			ChrootName:         "gnome",
			PreferredChrootDir: chrootDirs["20-gnome"],
		},
	}

	var mu sync.Mutex
	executed := make([]string, 0)

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  2,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			sd := DefaultMockSeeder()
			return &parallelTestSeeder{
				MockSeeder: sd,
				paramsMap:  paramsMap,
				mu:         &mu,
				executed:   &executed,
			}, nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
	}

	if err := ParallelSeed(context.Background(), opts); err != nil {
		t.Fatalf("ParallelSeed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(executed) != 3 {
		t.Fatalf("expected 3 seeders executed, got %d: %v", len(executed), executed)
	}

	// bedrock must be before server and gnome.
	bedrockIdx := -1
	for i, name := range executed {
		if name == "00-bedrock" {
			bedrockIdx = i
			break
		}
	}
	if bedrockIdx == -1 {
		t.Fatal("00-bedrock not found in executed list")
	}
	for _, dep := range []string{"10-server", "20-gnome"} {
		for i, name := range executed {
			if name == dep && i < bedrockIdx {
				t.Errorf("%s executed before 00-bedrock", dep)
			}
		}
	}
}

func TestParallelSeedWorkerError(t *testing.T) {
	names := []string{"00-bedrock"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-bedrock": {
			ChrootName:         "bedrock",
			PreferredChrootDir: chrootDirs["00-bedrock"],
		},
	}

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  2,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			sd := DefaultMockSeeder()
			sd.SeedErr = fmt.Errorf("boom")
			return &parallelTestSeeder{
				MockSeeder: sd,
				paramsMap:  paramsMap,
				mu:         &sync.Mutex{},
				executed:   &[]string{},
			}, nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
	}

	err := ParallelSeed(context.Background(), opts)
	if err == nil || !contains(err.Error(), "boom") {
		t.Fatalf("expected boom error, got: %v", err)
	}
}

func TestParallelSeedNoDeps(t *testing.T) {
	names := []string{"00-alpha", "01-beta", "02-gamma"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-alpha": {ChrootName: "alpha", PreferredChrootDir: chrootDirs["00-alpha"]},
		"01-beta":  {ChrootName: "beta", PreferredChrootDir: chrootDirs["01-beta"]},
		"02-gamma": {ChrootName: "gamma", PreferredChrootDir: chrootDirs["02-gamma"]},
	}

	var mu sync.Mutex
	executed := make([]string, 0)

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  3,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			sd := DefaultMockSeeder()
			return &parallelTestSeeder{
				MockSeeder: sd,
				paramsMap:  paramsMap,
				mu:         &mu,
				executed:   &executed,
			}, nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
	}

	if err := ParallelSeed(context.Background(), opts); err != nil {
		t.Fatalf("ParallelSeed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(executed) != 3 {
		t.Fatalf("expected 3 seeders executed, got %d: %v", len(executed), executed)
	}
}

func TestParallelSeedSingleWorker(t *testing.T) {
	names := []string{"00-base", "01-app"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-base": {ChrootName: "base", PreferredChrootDir: chrootDirs["00-base"]},
		"01-app":  {ChrootName: "app", PreferredChrootDir: chrootDirs["01-app"], Depends: []string{"00-base"}},
	}

	var mu sync.Mutex
	var executed []string

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  1,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			sd := DefaultMockSeeder()
			return &parallelTestSeeder{
				MockSeeder: sd,
				paramsMap:  paramsMap,
				mu:         &mu,
				executed:   &executed,
			}, nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
	}

	if err := ParallelSeed(context.Background(), opts); err != nil {
		t.Fatalf("ParallelSeed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(executed) != 2 {
		t.Fatalf("expected 2 seeders executed, got %d: %v", len(executed), executed)
	}
	if executed[0] != "00-base" || executed[1] != "01-app" {
		t.Errorf("expected [00-base, 01-app], got %v", executed)
	}
}

func TestParallelSeedContextCancellation(t *testing.T) {
	names := []string{"00-bedrock"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-bedrock": {ChrootName: "bedrock", PreferredChrootDir: chrootDirs["00-bedrock"]},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  1,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			return DefaultMockSeeder(), nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
	}

	// With a cancelled context, ParallelSeed may return nil (workers
	// see ctx.Done and exit) or an error (worker started before cancel).
	// Either way it should not hang.
	_ = ParallelSeed(ctx, opts)
}

func TestParallelSeedNewSeederError(t *testing.T) {
	names := []string{"00-bedrock"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-bedrock": {ChrootName: "bedrock", PreferredChrootDir: chrootDirs["00-bedrock"]},
	}

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  1,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			return nil, fmt.Errorf("factory boom")
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
	}

	err := ParallelSeed(context.Background(), opts)
	if err == nil || !contains(err.Error(), "factory boom") {
		t.Fatalf("expected factory error, got: %v", err)
	}
}

// contains is a small helper to avoid importing strings in tests.
func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Boost / Unboost tests ---

// boostTracker records boost/unboost calls for testing.
type boostTracker struct {
	mu       sync.Mutex
	boosts   []int // worker indices that were boosted
	unboosts []int // worker indices that were unboosted
	boostErr error // if set, BoostWorker returns this error
}

func (bt *boostTracker) boost(idx int) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.boosts = append(bt.boosts, idx)
	return bt.boostErr
}

func (bt *boostTracker) unboost(idx int) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.unboosts = append(bt.unboosts, idx)
	return nil
}

func TestParallelSeedBoostSingleRunnable(t *testing.T) {
	// 00-bedrock has no deps, 10-server and 20-gnome depend on it.
	// descCount[bedrock]=2, remaining=3 → 2+1=3 → boost bedrock.
	// When bedrock completes and server/gnome both become ready,
	// descCount[server]=0, descCount[gnome]=0, remaining=2 → no boost.
	names := []string{"00-bedrock", "10-server", "20-gnome"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-bedrock": {
			ChrootName:         "bedrock",
			PreferredChrootDir: chrootDirs["00-bedrock"],
		},
		"10-server": {
			Depends:            []string{"00-bedrock"},
			ChrootName:         "server",
			PreferredChrootDir: chrootDirs["10-server"],
		},
		"20-gnome": {
			Depends:            []string{"00-bedrock"},
			ChrootName:         "gnome",
			PreferredChrootDir: chrootDirs["20-gnome"],
		},
	}

	bt := &boostTracker{}

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  2,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			return DefaultMockSeeder(), nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
		BoostWorker:     bt.boost,
		UnboostWorker:   bt.unboost,
	}

	if err := ParallelSeed(context.Background(), opts); err != nil {
		t.Fatalf("ParallelSeed: %v", err)
	}

	bt.mu.Lock()
	defer bt.mu.Unlock()

	// Exactly one boost (bedrock) and one matching unboost.
	if len(bt.boosts) != 1 {
		t.Fatalf("expected exactly 1 boost call, got %d", len(bt.boosts))
	}
	if len(bt.unboosts) != 1 {
		t.Fatalf("expected exactly 1 unboost call, got %d", len(bt.unboosts))
	}
}

func TestParallelSeedNoBoostWhenMultipleRunnable(t *testing.T) {
	// All seeds are independent → descCount[each]=0, remaining=3 →
	// 0+1=1≠3 → no boost for any seed.
	names := []string{"00-alpha", "01-beta", "02-gamma"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-alpha": {ChrootName: "alpha", PreferredChrootDir: chrootDirs["00-alpha"]},
		"01-beta":  {ChrootName: "beta", PreferredChrootDir: chrootDirs["01-beta"]},
		"02-gamma": {ChrootName: "gamma", PreferredChrootDir: chrootDirs["02-gamma"]},
	}

	bt := &boostTracker{}

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  3,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			return DefaultMockSeeder(), nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
		BoostWorker:     bt.boost,
		UnboostWorker:   bt.unboost,
	}

	if err := ParallelSeed(context.Background(), opts); err != nil {
		t.Fatalf("ParallelSeed: %v", err)
	}

	bt.mu.Lock()
	defer bt.mu.Unlock()

	// Graph structure says no seed is a bottleneck → zero boosts.
	if len(bt.boosts) != 0 {
		t.Errorf("expected 0 boosts for independent seeds, got %d", len(bt.boosts))
	}
	if len(bt.unboosts) != 0 {
		t.Errorf("expected 0 unboosts, got %d", len(bt.unboosts))
	}
}

func TestParallelSeedNilBoostCallbacks(t *testing.T) {
	// Verify that nil BoostWorker/UnboostWorker is handled gracefully.
	names := []string{"00-bedrock"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-bedrock": {ChrootName: "bedrock", PreferredChrootDir: chrootDirs["00-bedrock"]},
	}

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  2,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			return DefaultMockSeeder(), nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
		// BoostWorker and UnboostWorker intentionally nil.
	}

	if err := ParallelSeed(context.Background(), opts); err != nil {
		t.Fatalf("ParallelSeed with nil boost callbacks: %v", err)
	}
}

func TestParallelSeedBoostError(t *testing.T) {
	names := []string{"00-bedrock"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-bedrock": {ChrootName: "bedrock", PreferredChrootDir: chrootDirs["00-bedrock"]},
	}

	bt := &boostTracker{boostErr: fmt.Errorf("boost failed")}

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  1,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			return DefaultMockSeeder(), nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
		BoostWorker:     bt.boost,
		UnboostWorker:   bt.unboost,
	}

	err := ParallelSeed(context.Background(), opts)
	if err == nil || !contains(err.Error(), "boost failed") {
		t.Fatalf("expected boost error, got: %v", err)
	}
}

// --- computeDescendantCounts ---

func TestComputeDescendantCounts(t *testing.T) {
	tests := []struct {
		name       string
		seeders    []SeederInfo
		dependents map[string][]string
		want       map[string]int
	}{
		{
			name:       "linear_chain",
			seeders:    []SeederInfo{{Name: "A"}, {Name: "B"}, {Name: "C"}},
			dependents: map[string][]string{"A": {"B"}, "B": {"C"}},
			want:       map[string]int{"A": 2, "B": 1, "C": 0},
		},
		{
			name:       "fan_out",
			seeders:    []SeederInfo{{Name: "A"}, {Name: "B"}, {Name: "C"}},
			dependents: map[string][]string{"A": {"B", "C"}},
			want:       map[string]int{"A": 2, "B": 0, "C": 0},
		},
		{
			name:       "diamond",
			seeders:    []SeederInfo{{Name: "A"}, {Name: "B"}, {Name: "C"}, {Name: "D"}},
			dependents: map[string][]string{"A": {"B", "C"}, "B": {"D"}, "C": {"D"}},
			want:       map[string]int{"A": 3, "B": 1, "C": 1, "D": 0},
		},
		{
			name:       "independent",
			seeders:    []SeederInfo{{Name: "A"}, {Name: "B"}, {Name: "C"}},
			dependents: map[string][]string{},
			want:       map[string]int{"A": 0, "B": 0, "C": 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeDescendantCounts(tt.dependents, tt.seeders)
			for name, wantCount := range tt.want {
				if got[name] != wantCount {
					t.Errorf("%s: got %d, want %d", name, got[name], wantCount)
				}
			}
		})
	}
}

// --- Linear chain: every seed in the chain is a bottleneck ---

func TestParallelSeedBoostLinearChain(t *testing.T) {
	// A→B→C: descCount[A]=2, descCount[B]=1, descCount[C]=0.
	// At each stage only one seed can run → all 3 should be boosted.
	names := []string{"00-base", "01-mid", "02-top"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-base": {ChrootName: "base", PreferredChrootDir: chrootDirs["00-base"]},
		"01-mid":  {ChrootName: "mid", PreferredChrootDir: chrootDirs["01-mid"], Depends: []string{"00-base"}},
		"02-top":  {ChrootName: "top", PreferredChrootDir: chrootDirs["02-top"], Depends: []string{"01-mid"}},
	}

	bt := &boostTracker{}

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  3,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			return DefaultMockSeeder(), nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
		BoostWorker:     bt.boost,
		UnboostWorker:   bt.unboost,
	}

	if err := ParallelSeed(context.Background(), opts); err != nil {
		t.Fatalf("ParallelSeed: %v", err)
	}

	bt.mu.Lock()
	defer bt.mu.Unlock()

	// Every seed in a linear chain is a bottleneck.
	if len(bt.boosts) != 3 {
		t.Fatalf("expected 3 boosts (every seed in chain), got %d", len(bt.boosts))
	}
	if len(bt.unboosts) != 3 {
		t.Fatalf("expected 3 unboosts, got %d", len(bt.unboosts))
	}
}

func TestParallelSeedBoostDiamond(t *testing.T) {
	// A→{B,C}→D: A is a bottleneck (desc=3, remaining=4 → 3+1=4).
	// B and C run concurrently → not boosted.
	// D is the last seed (desc=0, remaining=1 → 0+1=1) → boosted.
	names := []string{"00-root", "01-left", "02-right", "03-join"}
	infos, chrootDirs := setupParallelSeeders(t, names)

	paramsMap := map[string]*SeederParams{
		"00-root":  {ChrootName: "root", PreferredChrootDir: chrootDirs["00-root"]},
		"01-left":  {ChrootName: "left", PreferredChrootDir: chrootDirs["01-left"], Depends: []string{"00-root"}},
		"02-right": {ChrootName: "right", PreferredChrootDir: chrootDirs["02-right"], Depends: []string{"00-root"}},
		"03-join":  {ChrootName: "join", PreferredChrootDir: chrootDirs["03-join"], Depends: []string{"01-left", "02-right"}},
	}

	bt := &boostTracker{}

	opts := &ParallelSeedOptions{
		Seeders:      infos,
		ParamsByName: paramsMap,
		Parallelism:  2,
		NewSeeder: func(_ *NewSeederOptions) (ISeeder, error) {
			return DefaultMockSeeder(), nil
		},
		NewStdoutWriter: noopWriter,
		NewStderrWriter: noopWriter,
		PushCleanup:     noopCleanup,
		BoostWorker:     bt.boost,
		UnboostWorker:   bt.unboost,
	}

	if err := ParallelSeed(context.Background(), opts); err != nil {
		t.Fatalf("ParallelSeed: %v", err)
	}

	bt.mu.Lock()
	defer bt.mu.Unlock()

	// A (bottleneck, descCount=3) and D (last seed, descCount=0) → 2 boosts.
	if len(bt.boosts) != 2 {
		t.Fatalf("expected 2 boosts (root + join), got %d", len(bt.boosts))
	}
	if len(bt.unboosts) != 2 {
		t.Fatalf("expected 2 unboosts, got %d", len(bt.unboosts))
	}
}
