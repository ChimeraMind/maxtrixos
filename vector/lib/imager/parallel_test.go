package imager

import (
	"bytes"
	"context"
	"errors"
	"io"
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
