package sharp_test

import (
	"context"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

// maxRSSBytes returns the process peak resident set size (high-watermark) in
// bytes. ru_maxrss is bytes on darwin, kilobytes on linux.
func maxRSSBytes() int64 {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	if runtime.GOOS == "linux" {
		return int64(ru.Maxrss) * 1024
	}
	return int64(ru.Maxrss)
}

// TestConcurrentLoadMemoryBounded simulates the proxy's real workload — many
// concurrent resize+encode pipelines bounded by an in-flight semaphore — and
// verifies that libvips' tracked memory stays bounded by the in-flight count,
// NOT by the total number of requests. The GC is frozen so the only thing that
// can release C memory is sharp-go's deterministic end-of-pipeline free; a
// finalizer-only lifecycle would let tracked memory grow ~linearly with total
// requests and blow up here.
//
// It also logs peak process RSS, which surfaces the separate allocator-retention
// effect (glibc keeps freed arenas; tracked memory drops but RSS may not).
func TestConcurrentLoadMemoryBounded(t *testing.T) {
	if testing.Short() {
		t.Skip("heavy concurrent load test; skipped in -short")
	}
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg") // 2725x2225 photo

	pipeline := func() error {
		_, _, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 800}).
			AutoOrient().
			WebP(format.WebPOptions{Quality: 75, Effort: 4, Preset: "photo"}).
			ToBytes(context.Background())
		return err
	}

	// Warm up one-time libvips allocations, then settle.
	for i := 0; i < 8; i++ {
		if err := pipeline(); err != nil {
			t.Fatalf("warmup: %v", err)
		}
	}
	settleGC()
	rssBefore := maxRSSBytes()

	// Freeze the GC for the load window: deterministic free is the only path
	// that can release libvips C memory now.
	defer debug.SetGCPercent(debug.SetGCPercent(-1))
	baseTracked := sharp.TrackedMem()

	const (
		inflight = 12  // bounded concurrency, like the proxy's encodeSem
		total    = 600 // far more than inflight — exposes per-request accumulation
	)

	var peakTracked int64
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			default:
				if m := sharp.TrackedMem(); m > atomic.LoadInt64(&peakTracked) {
					atomic.StoreInt64(&peakTracked, m)
				}
				time.Sleep(time.Millisecond)
			}
		}
	}()

	sem := make(chan struct{}, inflight)
	var wg sync.WaitGroup
	var failed atomic.Int64
	for i := 0; i < total; i++ {
		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			if err := pipeline(); err != nil {
				failed.Add(1)
			}
		}()
	}
	wg.Wait()
	close(stop)

	if n := failed.Load(); n != 0 {
		t.Fatalf("%d/%d pipelines failed", n, total)
	}

	afterTracked := sharp.TrackedMem()
	peak := atomic.LoadInt64(&peakTracked)

	// Re-enable GC + return memory to the OS, then sample RSS again.
	debug.SetGCPercent(100)
	settleGC()
	debug.FreeOSMemory()
	rssAfter := maxRSSBytes()

	t.Logf("libvips tracked mem: baseline=%dKB peak=%dKB after-load(no GC)=%dKB",
		baseTracked>>10, peak>>10, afterTracked>>10)
	t.Logf("process peak RSS: before=%dMB after=%dMB (ru_maxrss is monotonic high-water)",
		rssBefore>>20, rssAfter>>20)
	t.Logf("inflight=%d total=%d — peak tracked should track inflight, not total", inflight, total)

	// PRIMARY: with the GC frozen, tracked memory must return to ~baseline after
	// the load. This is the deterministic-free property — a finalizer-only
	// lifecycle would leave ~total pipelines' worth resident here.
	if afterTracked > baseTracked+(4<<20) {
		t.Errorf("after concurrent load (GC frozen) tracked mem is %dKB vs baseline %dKB "+
			"— images not freed deterministically under concurrency",
			afterTracked>>10, baseTracked>>10)
	}

	// SECONDARY: peak tracked memory is bounded by the in-flight count, not the
	// request count. Generous per-pipeline ceiling for a large-source decode;
	// a request-count-scaling regression would be orders of magnitude over this.
	const perImageCeil = 32 << 20 // 32 MiB per in-flight pipeline, generous
	ceiling := baseTracked + int64(inflight)*perImageCeil
	if peak > ceiling {
		t.Errorf("peak tracked mem %dMB exceeded inflight-bounded ceiling %dMB "+
			"— memory scales with request count, not concurrency (lifecycle leak)",
			peak>>20, ceiling>>20)
	}
}
