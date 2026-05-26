package format

// JXLOptions controls JPEG XL output.
type JXLOptions struct {
	Quality  int     // 1-100; default 75
	Tier     int     // 0-4; decode-speed tier
	Distance float64 // 0-25; butteraugli target; lower = better quality
	Effort   int     // 1-10; default 7
	Lossless bool
	Bitdepth int // 1-16; default 8
}
