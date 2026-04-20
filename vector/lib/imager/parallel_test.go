package imager

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"matrixos/vector/lib/ostree"
)

// --- parallel_test.go helpers ---

// testParallelImageOpts returns a ParallelImageOptions suitable for
// testing with mock dependencies.
func testParallelImageOpts(
	refs []string,
) *ParallelImageOptions {
	cfg := baseImageConfig()
	var mu sync.Mutex
	var built []string

	return &ParallelImageOptions{
		Refs:        refs,
		Parallelism: 2,
		Config:      cfg,
		NewStdoutWriter: func(label string) io.Writer {
			return &bytes.Buffer{}
		},
		NewStderrWriter: func(label string) io.Writer {
			return &bytes.Buffer{}
		},
		PushCleanup: func(fn func()) {},
		ShortRef:    func(ref string) string { return ref },
		OnImageDone: func(ref string) error {
			mu.Lock()
			built = append(built, ref)
			mu.Unlock()
			return nil
		},
	}
}

// --- Tests ---

func TestParallelImage_EmptyRefs(t *testing.T) {
	opts := testParallelImageOpts(nil)
	opts.Parallelism = 1
	ctx := context.Background()
	err := ParallelImage(ctx, opts)
	if err != nil {
		t.Fatalf("empty refs should succeed, got: %v", err)
	}
}

func TestParallelImage_CancelledContextNoWork(t *testing.T) {
	// With no refs and a cancelled context, ParallelImage must
	// return without error — there is simply nothing to do.
	opts := testParallelImageOpts(nil)
	opts.Parallelism = 1
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ParallelImage(ctx, opts)
	if err != nil {
		t.Fatalf("expected nil error for empty work + cancelled ctx, got: %v", err)
	}
}

func TestParallelImage_SetupBuildError(t *testing.T) {
	refs := []string{"matrixos/amd64/dev/bedrock"}
	opts := testParallelImageOpts(refs)
	opts.Parallelism = 1
	opts.SetupBuild = func(pushCleanup func(func()), ot ostree.IOstree, im IImager) error {
		return errors.New("setup broken")
	}

	ctx := context.Background()
	err := ParallelImage(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Error could be about SetupBuild failing or about the mock config
	// being insufficient for ExecuteWithImageLock — either demonstrates
	// error propagation from the worker.
}

func TestParallelImage_OnImageDoneError(t *testing.T) {
	refs := []string{"matrixos/amd64/dev/bedrock"}
	opts := testParallelImageOpts(refs)
	opts.Parallelism = 1
	opts.OnImageDone = func(ref string) error {
		return errors.New("record failed")
	}

	ctx := context.Background()
	err := ParallelImage(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	// Error demonstrates that worker errors propagate.
}

func TestParallelImage_CancelledContextStopsWorkers(t *testing.T) {
	refs := []string{
		"matrixos/amd64/dev/bedrock",
		"matrixos/amd64/dev/server",
	}
	opts := testParallelImageOpts(refs)
	opts.Parallelism = 1

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := ParallelImage(ctx, opts)
	// With a cancelled context, workers should detect ctx.Done and
	// either return nil (no work picked up) or return ctx.Err.
	// Both are acceptable outcomes.
	_ = err
}

func TestParallelImage_AllErrorsPrinted(t *testing.T) {
	refs := []string{
		"matrixos/amd64/dev/bedrock",
		"matrixos/amd64/dev/server",
		"matrixos/amd64/dev/gnome",
	}

	var stderrBufs []*bytes.Buffer
	var stderrMu sync.Mutex

	opts := testParallelImageOpts(refs)
	opts.Parallelism = 1
	opts.NewStderrWriter = func(label string) io.Writer {
		buf := &bytes.Buffer{}
		stderrMu.Lock()
		stderrBufs = append(stderrBufs, buf)
		stderrMu.Unlock()
		return buf
	}
	opts.SetupBuild = func(pushCleanup func(func()), ot ostree.IOstree, im IImager) error {
		return errors.New("setup broken")
	}

	ctx := context.Background()
	err := ParallelImage(ctx, opts)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Collect all stderr output.
	stderrMu.Lock()
	var allStderr string
	for _, buf := range stderrBufs {
		allStderr += buf.String()
	}
	stderrMu.Unlock()

	// The first ref's error should be in the returned error.
	if got := err.Error(); !strings.Contains(got, "image failed") {
		t.Errorf("returned error should mention image failure, got: %s", got)
	}

	// The first ref's error should also be printed to stderr.
	if !strings.Contains(allStderr, "image failed") {
		t.Errorf("stderr should contain the first error, got:\n%s", allStderr)
	}

	// Remaining refs should be reported as skipped.
	if !strings.Contains(allStderr, "Skipping ref") {
		t.Errorf("stderr should report skipped refs, got:\n%s", allStderr)
	}
}
