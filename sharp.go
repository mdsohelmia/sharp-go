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
	"github.com/mdsohelmia/sharp-go/internal/vips"
)

// Version reports the underlying libvips version detected at init.
func Version() string { return vips.VersionString() }

// Concurrency returns the libvips worker thread count.
func Concurrency() int { return vips.Concurrency() }

// SetConcurrency sets the libvips worker thread count. n <= 0 selects NumCPU.
func SetConcurrency(n int) { vips.SetConcurrency(n) }

// Release returns an encoded-output slice obtained from ToBytes (or any
// other terminal that returns []byte) to a pool for reuse. After calling
// Release the slice must not be read or written.
//
// Calling Release is optional — the slice is plain Go memory and will be
// reclaimed by GC eventually. For high-throughput servers, recycling via
// Release eliminates the per-request encoder allocation entirely.
func Release(b []byte) { vips.ReleaseEncBuf(b) }
