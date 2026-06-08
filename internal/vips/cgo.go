//go:build cgo

// Package vips is the cgo binding to the libvips C API.
// Public sharp-go code never imports this package directly; it sits behind
// internal/ to enforce the language boundary. No C++ is used — vips-cpp is
// deliberately avoided. All libvips access is via vips_* C functions.
package vips

/*
#cgo pkg-config: vips libwebp
#include <stdlib.h>
#include <vips/vips.h>
#include "bridge.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"sync"
	"unsafe"
)

const (
	minMajor = 8
	minMinor = 16
)

var (
	initOnce sync.Once
	initErr  error
	vMajor   int
	vMinor   int
	vMicro   int
)

func init() {
	initOnce.Do(start)
}

func start() {
	name := C.CString("sharp-go")
	defer C.free(unsafe.Pointer(name))

	if rc := C.vips_init(name); rc != 0 {
		initErr = fmt.Errorf("vips_init failed: %w", lastError())
		return
	}

	vMajor = int(C.vips_version(0))
	vMinor = int(C.vips_version(1))
	vMicro = int(C.vips_version(2))

	if vMajor < minMajor || (vMajor == minMajor && vMinor < minMinor) {
		initErr = fmt.Errorf("libvips %d.%d.%d is too old; require >= %d.%d",
			vMajor, vMinor, vMicro, minMajor, minMinor)
		return
	}

	// Server-oriented defaults, matching govips. Per-op concurrency = 1: each
	// pipeline runs single-threaded, so parallelism comes from running many
	// pipelines (one goroutine per request) which the Go scheduler spreads over
	// a small, reused pool of OS threads — no thread-per-core fan-out per
	// request, no oversubscription under load. Operation cache stays OFF
	// (unlike govips' 50 MB default): a per-image proxy almost never re-runs an
	// identical op, so the cache would only hold stale results at a memory cost.
	// Tune both via SetConcurrency / SetCache.
	C.vips_concurrency_set(1)
	C.vips_cache_set_max(0)
	C.vips_cache_set_max_mem(0)
	C.vips_cache_set_max_files(0)
	C.vips_leak_set(C.gboolean(0))
}

// InitError returns the libvips initialization error, if any. Public sharp-go
// constructors should surface this on first use.
func InitError() error { return initErr }

// Version returns the libvips major, minor, micro version detected at init.
func Version() (major, minor, micro int) { return vMajor, vMinor, vMicro }

// VersionString returns the libvips version as "major.minor.micro".
func VersionString() string {
	return fmt.Sprintf("%d.%d.%d", vMajor, vMinor, vMicro)
}

// SetConcurrency sets the libvips worker thread count. n <= 0 selects NumCPU.
func SetConcurrency(n int) {
	if n <= 0 {
		n = runtime.NumCPU()
	}
	C.vips_concurrency_set(C.int(n))
}

// Concurrency returns the current libvips worker thread count.
func Concurrency() int { return int(C.vips_concurrency_get()) }

// ThreadShutdown releases the per-thread resources libvips lazily attaches to
// the calling OS thread (its buffer cache and thread-local error buffer).
//
// The library does NOT call this per operation (matching govips): terminal
// methods no longer pin to an OS thread, so libvips work runs on the Go
// scheduler's small, reused pool of Ms and the per-thread state is reused
// across requests rather than multiplied per distinct thread. Call this only
// from a goroutine that pinned itself with runtime.LockOSThread and is about
// to stop driving libvips for good — e.g. a dedicated long-lived worker thread
// shutting down. It is exposed publicly via sharp.ShutdownThread.
func ThreadShutdown() { C.vips_thread_shutdown() }

// SetCache configures the libvips operation cache. Zero disables.
func SetCache(maxOps, maxFiles int, maxMem uint64) {
	C.vips_cache_set_max(C.int(maxOps))
	C.vips_cache_set_max_files(C.int(maxFiles))
	C.vips_cache_set_max_mem(C.size_t(maxMem))
}

// BlockOperation blocks (or unblocks) a libvips operation by class name.
// Used to disable risky loaders (e.g. "VipsForeignLoadPdf") in sandboxed
// environments.
func BlockOperation(name string, blocked bool) {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	C.sharpgo_block_operation(cs, boolToC(blocked))
}

// HasOperation reports whether an operation is registered (and not blocked).
func HasOperation(name string) bool {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	base := C.CString("VipsOperation")
	defer C.free(unsafe.Pointer(base))
	return C.vips_type_find(base, cs) != 0
}

// TrimMemory asks the C allocator to return free heap memory to the OS (glibc
// malloc_trim; a no-op on musl and macOS). libvips frees its buffers back to
// the allocator, which may keep them on per-arena free lists — inflating RSS
// under sustained load even though nothing is leaked. Call periodically (e.g.
// on a ~10s ticker), never per request. Surfaced publicly via sharp.FreeMemory.
func TrimMemory() { C.sharpgo_malloc_trim() }

// LimitMallocArenas caps the number of glibc malloc arenas (mallopt
// M_ARENA_MAX). glibc defaults to 8*NumCPU arenas, which fragment and inflate
// RSS under concurrent pipelines; capping to a small value (imgproxy uses 2) is
// a large, cheap RSS win. Call once at startup, before heavy load. Returns true
// on glibc when applied; false on musl/macOS (no arenas to cap) or on error.
func LimitMallocArenas(n int) bool { return C.sharpgo_malloc_arena_max(C.int(n)) != 0 }

// TrackedMem returns libvips' currently-tracked allocated memory in bytes.
// Useful for leak tests — should return to baseline after pipeline runs +
// runtime.GC.
func TrackedMem() int64 { return int64(C.vips_tracked_get_mem()) }

// TrackedAllocs returns the count of currently-tracked allocations.
func TrackedAllocs() int { return int(C.vips_tracked_get_allocs()) }

// TrackedFiles returns the count of currently-tracked file descriptors.
func TrackedFiles() int { return int(C.vips_tracked_get_files()) }
