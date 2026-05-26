package sharp

import (
	"github.com/mdsohelmia/sharp-go/internal/vips"
)

// Versions reports the libvips version detected at init.
type Versions struct {
	Major int
	Minor int
	Micro int
}

// V reports the libvips version components.
func V() Versions {
	maj, min, mic := vips.Version()
	return Versions{Major: maj, Minor: min, Micro: mic}
}

// FormatSupport describes which loaders/savers libvips offers for a format.
type FormatSupport struct {
	Load bool
	Save bool
}

// SupportedFormats probes libvips for the set of formats it can load/save.
// The keys are sharp-style format names ("jpeg", "png", "webp", ...).
func SupportedFormats() map[string]FormatSupport {
	probes := []struct {
		name   string
		loader string
		saver  string
	}{
		{"jpeg", "jpegload", "jpegsave"},
		{"png", "pngload", "pngsave"},
		{"webp", "webpload", "webpsave"},
		{"avif", "heifload", "heifsave"},
		{"gif", "gifload", "gifsave"},
		{"tiff", "tiffload", "tiffsave"},
		{"heif", "heifload", "heifsave"},
		{"jxl", "jxlload", "jxlsave"},
		{"jp2", "jp2kload", "jp2ksave"},
		{"svg", "svgload", ""},
		{"pdf", "pdfload", ""},
		{"raw", "rawload", "rawsave"},
	}
	out := make(map[string]FormatSupport, len(probes))
	for _, p := range probes {
		out[p.name] = FormatSupport{
			Load: p.loader != "" && vips.HasOperation(p.loader),
			Save: p.saver != "" && vips.HasOperation(p.saver),
		}
	}
	return out
}

// Block disables a libvips operation by class name. Useful for sandboxing
// untrusted input — e.g. Block("VipsForeignLoadHeif") refuses HEIF input.
func Block(name string) {
	vips.BlockOperation(name, true)
}

// Unblock re-enables a previously blocked operation.
func Unblock(name string) {
	vips.BlockOperation(name, false)
}

// SetCache configures the libvips operation cache. Zero on all params
// disables caching entirely (sharp default).
func SetCache(maxOps, maxFiles int, maxMem uint64) {
	vips.SetCache(maxOps, maxFiles, maxMem)
}

// TrackedMem returns libvips' currently-tracked allocated memory in bytes.
// Pair with runtime.GC + a few cleanup goroutine yields to verify no leaks.
func TrackedMem() int64 { return vips.TrackedMem() }

// TrackedAllocs returns the count of currently-tracked libvips allocations.
func TrackedAllocs() int { return vips.TrackedAllocs() }

// TrackedFiles returns the count of currently-tracked file descriptors.
func TrackedFiles() int { return vips.TrackedFiles() }
