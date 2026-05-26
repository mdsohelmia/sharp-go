package format

// HEIFCompression names libvips' heifsave compression mode.
type HEIFCompression int

const (
	HEIFCompressionHEVC HEIFCompression = iota + 1 // default
	HEIFCompressionAVC
	HEIFCompressionJPEG
	HEIFCompressionAV1 // used by AVIF
)

// HEIFOptions controls HEIF/HEIC output (HEIF container with HEVC/AVC/JPEG).
type HEIFOptions struct {
	Quality           int // 1-100; default 50
	Lossless          bool
	Compression       HEIFCompression
	Effort            int // 0-9; default 4
	Bitdepth          int // 8/10/12; default 12 for HEIF, 8 for AVIF
	ChromaSubsampling string // "4:4:4" disables; anything else uses libvips auto
}

// AVIFOptions controls AVIF output. AVIF is HEIF container + AV1 codec.
// Sharp exposes a separate avif() method; we route to the same encoder
// with Compression=AV1.
type AVIFOptions struct {
	Quality           int  // default 50
	Lossless          bool
	Effort            int  // 0-9; default 4
	Bitdepth          int  // 8/10/12; default 8
	ChromaSubsampling string
}
