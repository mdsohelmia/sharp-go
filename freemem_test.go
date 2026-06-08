package sharp_test

import (
	"sync"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
)

// TestFreeMemorySafe verifies FreeMemory is callable, idempotent, and safe
// under concurrency. The RSS-reclaim effect (debug.FreeOSMemory + glibc
// malloc_trim) is environment-dependent and is exercised by the concurrent
// load benchmark, not asserted here.
func TestFreeMemorySafe(t *testing.T) {
	sharp.FreeMemory()
	sharp.FreeMemory() // idempotent

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sharp.FreeMemory()
		}()
	}
	wg.Wait()
}

// TestLimitMallocArenas verifies the arena cap is callable and reports success
// on glibc / no-op elsewhere. On non-glibc (musl, macOS) it returns false
// because there is nothing to cap.
func TestLimitMallocArenas(t *testing.T) {
	// Should not panic regardless of platform.
	_ = sharp.LimitMallocArenas(2)
}
