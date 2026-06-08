package sharp_test

import (
	"context"
	"io"
	"runtime"
	"runtime/debug"
	"testing"
	"time"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

// TestDeterministicFreeStreamWriterWithoutGC covers the streaming encode path
// (ToWriter → streamTo) used for JPEG/PNG output to an io.Writer. Like the
// buffer path it must release its final image at end-of-pipeline rather than
// relying on the GC.
func TestDeterministicFreeStreamWriterWithoutGC(t *testing.T) {
	in := readFixture(t, "320x240.jpg")

	run := func() {
		_, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 80, Height: 60}).
			JPEG(format.JPEGOptions{Quality: 70}).
			ToWriter(context.Background(), io.Discard)
		if err != nil {
			t.Fatalf("pipeline: %v", err)
		}
	}

	for i := 0; i < 5; i++ {
		run()
	}
	settleGC()

	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	baseAllocs := sharp.TrackedAllocs()

	const iterations = 200
	for i := 0; i < iterations; i++ {
		run()
	}

	leaked := sharp.TrackedAllocs() - baseAllocs
	if leaked > 16 {
		t.Errorf("with GC frozen, tracked libvips allocations grew by %d over %d "+
			"iterations — the streaming encode does not free its image "+
			"deterministically. Expected near-zero growth.",
			leaked, iterations)
	}
}

// TestDeterministicFreeWithoutGC verifies that a completed pipeline releases its
// libvips images at end-of-pipeline, WITHOUT waiting for the Go GC — the govips
// model (explicit Close + per-op free). This is the property that bounds RSS
// under server load: libvips images live in C memory the Go GC cannot see, so a
// finalizer-only lifecycle lets dozens of finished pipelines' worth of C memory
// pile up between GC cycles.
//
// The GC is frozen for the measurement window. With finalizer-only cleanup,
// nothing libvips-side can be released while frozen, so tracked allocations grow
// in proportion to the iteration count. Deterministic freeing keeps it flat.
func TestDeterministicFreeWithoutGC(t *testing.T) {
	in := readFixture(t, "320x240.jpg")

	run := func() {
		_, _, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 80, Height: 60}).
			JPEG(format.JPEGOptions{Quality: 70}).
			ToBytes(context.Background())
		if err != nil {
			t.Fatalf("pipeline: %v", err)
		}
	}

	// Warm up one-time libvips allocations (type system, profiles), then settle.
	for i := 0; i < 5; i++ {
		run()
	}
	settleGC()

	// Freeze the GC: no automatic collection, so no AddCleanup callbacks fire
	// during the window. Anything freed here was freed deterministically.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	baseAllocs := sharp.TrackedAllocs()

	const iterations = 200
	for i := 0; i < iterations; i++ {
		run()
	}

	leaked := sharp.TrackedAllocs() - baseAllocs
	// Deterministic freeing keeps live allocations flat regardless of GC. A
	// small constant of slack absorbs any one-time lazy allocation; growth
	// proportional to iterations means images are held until GC (finalizer-only).
	if leaked > 16 {
		t.Errorf("with GC frozen, tracked libvips allocations grew by %d over %d "+
			"iterations — images are not freed deterministically (finalizer-only). "+
			"Expected near-zero growth (govips-style explicit free).",
			leaked, iterations)
	}
}

// TestLeakResize runs a tight loop of resize-encode and verifies that libvips'
// tracked memory does not grow without bound. Allows some slack for caches.
func TestLeakResize(t *testing.T) {
	in := readFixture(t, "320x240.jpg")

	// Warm-up + GC.
	for i := 0; i < 5; i++ {
		_, _, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 80, Height: 60}).
			JPEG(format.JPEGOptions{Quality: 70}).
			ToBytes(context.Background())
		if err != nil {
			t.Fatalf("warmup: %v", err)
		}
	}
	settleGC()
	baseline := sharp.TrackedMem()
	baselineAllocs := sharp.TrackedAllocs()

	const iterations = 200
	for i := 0; i < iterations; i++ {
		_, _, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 80, Height: 60}).
			JPEG(format.JPEGOptions{Quality: 70}).
			ToBytes(context.Background())
		if err != nil {
			t.Fatalf("iter %d: %v", i, err)
		}
	}
	settleGC()
	after := sharp.TrackedMem()
	afterAllocs := sharp.TrackedAllocs()

	memDelta := after - baseline
	allocDelta := afterAllocs - baselineAllocs

	// Allow some delta — libvips caches operations and Go's GC is async — but
	// growth proportional to iteration count is a leak.
	if memDelta > 32<<20 { // 32 MiB
		t.Errorf("tracked memory grew by %d bytes over %d iterations (baseline %d, after %d)",
			memDelta, iterations, baseline, after)
	}
	if allocDelta > iterations {
		t.Errorf("tracked allocs grew by %d over %d iterations (baseline %d, after %d)",
			allocDelta, iterations, baselineAllocs, afterAllocs)
	}
}

// TestDeterministicFreeRandomAccessWithoutGC covers the random-access op chain.
// Ops like rotate/trim/normalise call vips.StaySequential, which materialises a
// full-raster intermediate. Freeing only the final image leaves that
// intermediate pinned at refcount 1 by its (uncollected) Go wrapper until GC —
// so deterministic freeing requires releasing intermediates as the chain
// advances (the govips per-op setImage model).
func TestDeterministicFreeRandomAccessWithoutGC(t *testing.T) {
	in := readFixture(t, "320x240.jpg")

	run := func() {
		_, _, err := sharp.FromBytes(in).
			Rotate(sharp.RotateOptions{Angle: 90}). // forces StaySequential materialisation
			JPEG(format.JPEGOptions{Quality: 70}).
			ToBytes(context.Background())
		if err != nil {
			t.Fatalf("pipeline: %v", err)
		}
	}

	for i := 0; i < 5; i++ {
		run()
	}
	settleGC()

	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	baseAllocs := sharp.TrackedAllocs()

	const iterations = 200
	for i := 0; i < iterations; i++ {
		run()
	}

	leaked := sharp.TrackedAllocs() - baseAllocs
	if leaked > 16 {
		t.Errorf("with GC frozen, tracked libvips allocations grew by %d over %d "+
			"iterations — materialised intermediates are not freed deterministically. "+
			"Expected near-zero growth (govips-style per-op free).",
			leaked, iterations)
	}
}

// TestDeterministicFreeStatsWithoutGC covers the Stats terminal method, which
// fully decodes the image (random access). It must free that raster at the end
// of the call rather than leaving it for the GC.
func TestDeterministicFreeStatsWithoutGC(t *testing.T) {
	in := readFixture(t, "320x240.jpg")

	run := func() {
		_, err := sharp.FromBytes(in).Stats(context.Background())
		if err != nil {
			t.Fatalf("stats: %v", err)
		}
	}

	for i := 0; i < 5; i++ {
		run()
	}
	settleGC()

	defer debug.SetGCPercent(debug.SetGCPercent(-1))

	baseAllocs := sharp.TrackedAllocs()

	const iterations = 200
	for i := 0; i < iterations; i++ {
		run()
	}

	leaked := sharp.TrackedAllocs() - baseAllocs
	if leaked > 16 {
		t.Errorf("with GC frozen, tracked libvips allocations grew by %d over %d "+
			"iterations — Stats does not free its decoded image deterministically.",
			leaked, iterations)
	}
}

// settleGC nudges the runtime to run finalizers and AddCleanup callbacks so
// that libvips refs held by *vips.Image wrappers get unref'd. Multiple
// rounds + sleeps because runtime.AddCleanup uses a background goroutine.
func settleGC() {
	for i := 0; i < 10; i++ {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
}
