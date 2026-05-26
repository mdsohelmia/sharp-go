package format

// RawDepth names the libvips band-format target for raw output.
type RawDepth int

const (
	RawDepthUchar RawDepth = iota // default
	RawDepthChar
	RawDepthUshort
	RawDepthShort
	RawDepthUint
	RawDepthInt
	RawDepthFloat
	RawDepthDouble
)

// RawOptions controls raw pixel output.
type RawOptions struct {
	// Depth selects the per-band storage format. Default Uchar (8-bit).
	Depth RawDepth
}

// RawInput describes the layout of raw pixel input bytes.
type RawInput struct {
	Width    int
	Height   int
	Channels int       // 1, 2, 3, or 4
	Depth    RawDepth  // default Uchar
	Premultiplied bool // hint for downstream resize ops; not yet used
}
