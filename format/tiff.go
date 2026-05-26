package format

// TIFFCompression names libvips' tiffsave compression mode.
type TIFFCompression int

const (
	TIFFCompressionNone TIFFCompression = iota
	TIFFCompressionJPEG
	TIFFCompressionDeflate
	TIFFCompressionPackbits
	TIFFCompressionCCITTFAX4
	TIFFCompressionLZW
	TIFFCompressionWebP
	TIFFCompressionZSTD
	TIFFCompressionJP2K
)

// TIFFPredictor names libvips' tiffsave predictor.
type TIFFPredictor int

const (
	TIFFPredictorNone       TIFFPredictor = iota
	TIFFPredictorHorizontal               // default
	TIFFPredictorFloat
)

// TIFFOptions controls TIFF output. Defaults match sharp's lib/output.js
// tiff() parser.
type TIFFOptions struct {
	Compression TIFFCompression // default Deflate via sharp; libvips default None
	Quality     int             // JPEG-in-TIFF Q; default 80
	Predictor   TIFFPredictor   // default Horizontal
	Tile        bool
	TileWidth   int  // default 256
	TileHeight  int  // default 256
	Pyramid     bool // multi-resolution pyramid
	Bitdepth    int  // 0=auto, 1/2/4/8
	BigTIFF     bool
}
