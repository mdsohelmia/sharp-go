package format

// PNGOptions controls PNG output. Defaults match sharp's lib/output.js
// png() parser.
type PNGOptions struct {
	// Compression 0-9. Zero -> default 6.
	Compression int

	// Progressive enables interlaced PNG.
	Progressive bool

	// Palette quantises to a paletted (indexed) PNG. Implies libimagequant.
	Palette bool

	// Quality 1-100. Palette-only. Zero -> default 100.
	Quality int

	// Effort 1-10. Palette quantiser effort. Zero -> default 7.
	Effort int

	// Bitdepth 1/2/4/8/16. Zero selects auto (depends on Palette/source).
	Bitdepth int
}
