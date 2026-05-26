package format

// WebPOptions controls WebP output. Defaults match sharp's lib/output.js
// webp() parser.
type WebPOptions struct {
	// Quality 1-100. Zero -> default 80.
	Quality int

	// AlphaQuality 0-100. Zero -> default 100.
	AlphaQuality int

	// Lossless enables lossless compression.
	Lossless bool

	// NearLossless enables near-lossless compression (use with Quality).
	NearLossless bool

	// SmartSubsample enables high-quality chroma subsampling.
	SmartSubsample bool

	// Effort 0 (fastest) to 6 (slowest). Zero -> default 4.
	Effort int

	// Loop count for animated WebP. 0 = infinite.
	Loop int

	// MinSize enables minimum size mode for animated WebP (drops frames where
	// allowed). Set together with Mixed for best results.
	MinSize bool

	// Mixed allows mixing lossy + lossless frames in animated WebP.
	Mixed bool

	// SmartDeblock auto-adjusts libwebp's deblocking filter strength. Small
	// quality win at high efforts on photographic content; rarely worth
	// the extra encode time for low/medium effort.
	SmartDeblock bool

	// Passes is the number of entropy-analysis passes, 1-10. Higher = better
	// compression, slower. Zero defaults to libwebp's internal value (1 for
	// effort ≤ 3, scales up otherwise).
	Passes int

	// Preset hints libwebp at the content type for better entropy choices.
	// Valid: "default" (or "") | "picture" | "photo" | "drawing" | "icon" |
	// "text". `photo` typically cuts an additional 5-8% off natural images
	// at the same Q vs the default preset.
	Preset string

	// UseSharpYUV switches to a libwebp-direct encoder path that sets
	// WebPConfig.use_sharp_yuv = 1 — a sharper (slower) RGB→YUV
	// conversion that better preserves chroma detail under 4:2:0
	// subsampling. libvips's vips_webpsave_buffer doesn't expose this
	// flag; setting it routes through sharp-go's custom libwebp wrapper.
	// Measured: -0.10 butteraugli + 1-2 ssimulacra2 points at same
	// bytes on photographic content.
	UseSharpYUV bool

	// SNSStrength tunes libwebp's Spatial Noise Shaping (only used when
	// UseSharpYUV is on, since that's the direct-libwebp path).
	// 0 = libwebp default (50); valid 0-100.
	SNSStrength int

	// AutoFilter auto-adjusts the deblocking filter strength per-frame
	// (only used when UseSharpYUV is on).
	AutoFilter bool

	// TargetSize, in bytes, asks libwebp to bisect Q to hit a byte
	// budget (only used when UseSharpYUV is on). Zero = ignore.
	TargetSize int

	// Multithread lets libwebp encode token partitions in parallel
	// (WebPConfig.thread_level = 1), trading a sub-0.1% size delta for
	// faster encoding on multicore hosts. Only used when UseSharpYUV is on.
	Multithread bool
}
