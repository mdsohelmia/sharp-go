package format

// JP2Options controls JPEG 2000 output.
type JP2Options struct {
	Quality           int  // 1-100; default 48
	Lossless          bool
	TileWidth         int  // default 512
	TileHeight        int  // default 512
	ChromaSubsampling string // "4:4:4" disables chroma subsampling
}
