// Package format defines per-encoder option structs.
package format

// JPEGOptions controls JPEG output. Defaults match sharp's lib/output.js
// jpeg() parser.
type JPEGOptions struct {
	// Quality 1-100. Zero -> default 80.
	Quality int

	// Progressive enables interlaced (progressive) JPEG.
	Progressive bool

	// OptimiseCoding enables Huffman table optimisation. Default true.
	OptimiseCoding *bool

	// TrellisQuantisation, OvershootDeringing, OptimiseScans tune the
	// MozJPEG-style encoder. Setting MozJPEG=true enables all four
	// (trellis + overshoot + optimiseScans + progressive) at once.
	TrellisQuantisation bool
	OvershootDeringing  bool
	OptimiseScans       bool

	// MozJPEG is a composite flag that turns on
	// TrellisQuantisation + OvershootDeringing + OptimiseScans + Progressive.
	MozJPEG bool

	// QuantisationTable 0-8. Zero leaves libvips default.
	QuantisationTable int

	// ChromaSubsampling "4:4:4" disables chroma subsampling. Any other
	// value (including empty) selects libvips auto (4:2:0).
	ChromaSubsampling string
}
