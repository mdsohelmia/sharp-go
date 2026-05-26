package format

// GIFOptions controls GIF output. Defaults match sharp's lib/output.js
// gif() parser.
type GIFOptions struct {
	// Dither 0.0 (off) - 1.0 (full Floyd-Steinberg).
	Dither float64

	// Effort 1-10. Zero -> default 7.
	Effort int

	// Bitdepth 1-8. Zero -> default 8.
	Bitdepth int

	// InterframeMaxError 0-32. Zero = no allowance.
	InterframeMaxError int

	// InterpaletteMaxError 0-256. Zero -> default 3.
	InterpaletteMaxError int

	// Interlace produces interlaced GIF.
	Interlace bool

	// Reuse the palette across frames where possible. Default true (set
	// ForceNoReuse to disable).
	ForceNoReuse bool

	// KeepDuplicateFrames retains duplicate frames in animated output.
	KeepDuplicateFrames bool
}
