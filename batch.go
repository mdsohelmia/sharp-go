package sharp

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// BatchOptions configures the parallel-terminal helpers (ToBytesAll,
// ToFilesAll, MetadataAll).
//
// Concurrency caps the number of pipelines running in parallel. Zero selects
// runtime.NumCPU(). libvips itself parallelises each operation across its
// own thread pool (see SetConcurrency); total CPU pressure is roughly
// BatchOptions.Concurrency × Concurrency() threads. For CPU-bound workloads,
// keep BatchOptions.Concurrency ≤ runtime.NumCPU() / Concurrency() to avoid
// oversubscription.
//
// StopOnFirstError cancels the shared context after the first job error.
// In-flight jobs run to completion; queued jobs return context.Canceled.
//
// PerJobTimeout applies a fresh context.WithTimeout to each pipeline. Zero
// means no per-job timeout.
type BatchOptions struct {
	Concurrency      int
	StopOnFirstError bool
	PerJobTimeout    time.Duration
}

// BatchResult is one entry in the output slice. Position matches the input.
type BatchResult struct {
	Data []byte // populated by ToBytesAll only
	Info Info
	Err  error
}

// MetadataResult pairs a Metadata with a per-job error.
type MetadataResult struct {
	Metadata Metadata
	Err      error
}

// ToBytesAll processes each input in parallel and returns results in input
// order. Each *Image is a fully-recorded pipeline ready for a terminal call.
//
// Distinct *Image values are safe to evaluate concurrently. Do not share a
// single *Image across calls.
func ToBytesAll(ctx context.Context, images []*Image, opts BatchOptions) []BatchResult {
	results := make([]BatchResult, len(images))
	runBatch(ctx, len(images), opts, func(jobCtx context.Context, i int) error {
		if images[i] == nil {
			err := errors.New("sharp: nil *Image at index " + itoa(i))
			results[i].Err = err
			return err
		}
		data, info, err := images[i].ToBytes(jobCtx)
		results[i] = BatchResult{Data: data, Info: info, Err: err}
		return err
	})
	return results
}

// ToFilesAll processes each image in parallel and writes each output to the
// matching path. paths must have the same length as images.
func ToFilesAll(ctx context.Context, images []*Image, paths []string, opts BatchOptions) []BatchResult {
	if len(images) != len(paths) {
		out := make([]BatchResult, len(images))
		err := errors.New("sharp: ToFilesAll: images/paths length mismatch")
		for i := range out {
			out[i].Err = err
		}
		return out
	}
	results := make([]BatchResult, len(images))
	runBatch(ctx, len(images), opts, func(jobCtx context.Context, i int) error {
		switch {
		case images[i] == nil:
			err := errors.New("sharp: nil *Image at index " + itoa(i))
			results[i].Err = err
			return err
		case paths[i] == "":
			err := errors.New("sharp: empty output path at index " + itoa(i))
			results[i].Err = err
			return err
		}
		info, err := images[i].ToFile(jobCtx, paths[i])
		results[i] = BatchResult{Info: info, Err: err}
		return err
	})
	return results
}

// MetadataAll reads header information from each input in parallel.
func MetadataAll(ctx context.Context, images []*Image, opts BatchOptions) []MetadataResult {
	results := make([]MetadataResult, len(images))
	runBatch(ctx, len(images), opts, func(jobCtx context.Context, i int) error {
		if images[i] == nil {
			err := errors.New("sharp: nil *Image at index " + itoa(i))
			results[i].Err = err
			return err
		}
		md, err := images[i].Metadata(jobCtx)
		results[i] = MetadataResult{Metadata: md, Err: err}
		return err
	})
	return results
}

// runBatch dispatches n jobs across bounded workers. fn is invoked for each
// i in [0, n) with a per-job ctx (timeout/cancellation-aware) and must record
// its own result; returning a non-nil error trips StopOnFirstError.
func runBatch(ctx context.Context, n int, opts BatchOptions, fn func(jobCtx context.Context, i int) error) {
	if n == 0 {
		return
	}
	workers := opts.Concurrency
	if workers <= 0 {
		workers = runtime.NumCPU()
	}
	if workers > n {
		workers = n
	}

	stopCtx, cancelAll := context.WithCancel(ctx)
	defer cancelAll()

	var firstErr atomic.Bool

	jobs := make(chan int, n)
	for i := 0; i < n; i++ {
		jobs <- i
	}
	close(jobs)

	var wg sync.WaitGroup
	wg.Add(workers)
	for w := 0; w < workers; w++ {
		go func() {
			defer wg.Done()
			for i := range jobs {
				if err := stopCtx.Err(); err != nil {
					// Synthesise a cancellation error in the slot.
					_ = fn(stopCtx, i)
					continue
				}
				jobCtx := stopCtx
				var cancel context.CancelFunc
				if opts.PerJobTimeout > 0 {
					jobCtx, cancel = context.WithTimeout(stopCtx, opts.PerJobTimeout)
				}
				err := fn(jobCtx, i)
				if cancel != nil {
					cancel()
				}
				if err != nil && opts.StopOnFirstError && firstErr.CompareAndSwap(false, true) {
					cancelAll()
				}
			}
		}()
	}
	wg.Wait()
}
