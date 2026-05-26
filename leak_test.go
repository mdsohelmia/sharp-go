package sharp_test

import (
	"context"
	"runtime"
	"testing"
	"time"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

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

// settleGC nudges the runtime to run finalizers and AddCleanup callbacks so
// that libvips refs held by *vips.Image wrappers get unref'd. Multiple
// rounds + sleeps because runtime.AddCleanup uses a background goroutine.
func settleGC() {
	for i := 0; i < 10; i++ {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
}
