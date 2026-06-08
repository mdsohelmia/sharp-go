// Package sharp is a Go port of the sharp Node.js image library, built on
// libvips via cgo. It provides high-performance resize, format conversion,
// composite, colour, channel, and metadata operations.
//
// sharp-go calls into libvips through its C API only. No C++ code is used —
// the libvips C++ wrapper (vips-cpp) is deliberately avoided to keep the
// binary smaller, simplify cross-compilation, and remove the libstdc++ link.
//
// Operations are recorded on an *Image and applied in a single ordered
// pipeline by terminal methods (ToBytes, ToFile, ToWriter, Metadata, Stats).
// A *Image is not safe for concurrent option mutation; for parallel variants,
// Clone first.
package sharp

import (
	"runtime/debug"

	"github.com/mdsohelmia/sharp-go/internal/vips"
)

// Version reports the underlying libvips version detected at init.
func Version() string { return vips.VersionString() }

// Concurrency returns the libvips worker thread count.
func Concurrency() int { return vips.Concurrency() }

// SetConcurrency sets the libvips worker thread count. n <= 0 selects NumCPU.
func SetConcurrency(n int) { vips.SetConcurrency(n) }

// ShutdownThread releases the per-thread resources libvips lazily attaches to
// the calling OS thread (its buffer cache and thread-local error buffer).
//
// Terminal methods do NOT pin to an OS thread, so normal use needs no manual
// cleanup — libvips work runs on the Go scheduler's small, reused thread pool
// and the per-thread state is reused, not multiplied. Call this only if you
// run libvips from your own long-lived OS thread (one you pinned with
// runtime.LockOSThread) that is about to stop: invoke it on that thread, while
// still locked, before unlocking. Mirrors govips' vips.ShutdownThread.
func ShutdownThread() { vips.ThreadShutdown() }

// Release returns an encoded-output slice obtained from ToBytes (or any
// other terminal that returns []byte) to a pool for reuse. After calling
// Release the slice must not be read or written.
//
// Calling Release is optional — the slice is plain Go memory and will be
// reclaimed by GC eventually. For high-throughput servers, recycling via
// Release eliminates the per-request encoder allocation entirely.
func Release(b []byte) { vips.ReleaseEncBuf(b) }

// FreeMemory returns process memory to the OS. It runs a Go GC and releases
// unused Go-heap spans (debug.FreeOSMemory), then asks the C allocator to give
// back its free arenas (glibc malloc_trim; a no-op on musl and macOS).
//
// For a cgo image pipeline the C heap dominates RSS, and after libvips frees a
// buffer glibc tends to keep it on a per-arena free list rather than returning
// it — so RSS stays high under sustained load even though nothing is leaked
// (libvips' own tracked memory returns to baseline; see TrackedMem). Servers
// processing a high volume should call FreeMemory on a periodic ticker
// (imgproxy uses ~10s), NOT per request: it forces a full GC and heap walk,
// which is far too costly on the hot path.
//
// The single biggest steady-state-RSS lever is upstream of this call: set
// MALLOC_ARENA_MAX=2 in the environment (or LD_PRELOAD jemalloc/tcmalloc) to
// stop glibc from fragmenting across many arenas under concurrency.
func FreeMemory() {
	debug.FreeOSMemory()
	vips.TrimMemory()
}

// LimitMallocArenas caps how many memory arenas the C allocator (glibc) creates.
// glibc defaults to 8×NumCPU arenas; under many concurrent libvips pipelines
// that fragments and inflates RSS. Capping to a small number — imgproxy uses 2
// — is the single biggest, cheapest steady-state-RSS win for a high-volume
// image service.
//
// Call once at startup, before serving load. Returns true when applied (glibc),
// false on musl/macOS where there are no per-CPU arenas to cap. Equivalent to
// setting MALLOC_ARENA_MAX=2 in the environment, but self-contained so it works
// without controlling the deployment image.
func LimitMallocArenas(n int) bool { return vips.LimitMallocArenas(n) }
