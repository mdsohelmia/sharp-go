package sharp

// BlurOptions configures Blur. Sharp accepts a bare sigma or {sigma, precision}.
type BlurOptions struct {
	// Sigma in pixels. Sharp defaults to 1.0 when called with no argument.
	Sigma float64
}

// Blur applies a Gaussian blur.
func (im *Image) Blur(opts BlurOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Sigma == 0 {
		opts.Sigma = 1.0
	}
	im.opts.blur = &opts
	return im
}

// SharpenOptions configures Sharpen. Mirrors sharp's sharpen({sigma,m1,m2,x1,y2,y3}).
type SharpenOptions struct {
	Sigma float64 // default 1.0
	M1    float64 // default 1.0 (flat-area boost)
	M2    float64 // default 2.0 (jagged-area boost)
	X1    float64 // default 2.0 (threshold between flat and jagged)
	Y2    float64 // default 10.0 (maximum flat boost)
	Y3    float64 // default 20.0 (maximum jagged boost)
}

// Sharpen applies an unsharp-mask sharpen.
func (im *Image) Sharpen(opts SharpenOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Sigma == 0 {
		opts.Sigma = 1.0
	}
	if opts.M1 == 0 {
		opts.M1 = 1.0
	}
	if opts.M2 == 0 {
		opts.M2 = 2.0
	}
	if opts.X1 == 0 {
		opts.X1 = 2.0
	}
	if opts.Y2 == 0 {
		opts.Y2 = 10.0
	}
	if opts.Y3 == 0 {
		opts.Y3 = 20.0
	}
	im.opts.sharpen = &opts
	return im
}

// GammaOptions configures Gamma. Mirrors sharp's gamma(exponent[, exponentOut]).
type GammaOptions struct {
	Exponent    float64 // 1.0-3.0; default 2.2
	ExponentOut float64 // 0 = use Exponent
}

// Gamma applies gamma correction.
func (im *Image) Gamma(opts GammaOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Exponent == 0 {
		opts.Exponent = 2.2
	}
	im.opts.gamma = &opts
	return im
}

// NegateOptions configures Negate.
type NegateOptions struct {
	// KeepAlpha leaves the alpha channel untouched when negating. Sharp's
	// equivalent is .negate({alpha:false}); default sharp behaviour is to
	// negate alpha too, which corresponds to KeepAlpha=false.
	KeepAlpha bool
}

// Negate inverts pixel values.
func (im *Image) Negate(opts NegateOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.negate = &opts
	return im
}

// ThresholdOptions configures Threshold.
type ThresholdOptions struct {
	// Value 0-255. Default 128.
	Value float64
	// Grayscale converts to b-w before thresholding (matches sharp default).
	Grayscale bool
}

// Threshold thresholds pixel values.
func (im *Image) Threshold(opts ThresholdOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Value == 0 {
		opts.Value = 128
	}
	im.opts.threshold = &opts
	return im
}

// LinearOptions configures Linear. Mirrors sharp's linear(a, b) — either
// scalars (len 1) or per-channel arrays.
type LinearOptions struct {
	A []float64 // multiplier
	B []float64 // offset
}

// Linear applies out = in * a + b.
func (im *Image) Linear(opts LinearOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.linear = &opts
	return im
}

// MedianOptions configures Median.
type MedianOptions struct {
	// Size of the square window in pixels. Default 3.
	Size int
}

// Median applies a median filter.
func (im *Image) Median(opts MedianOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Size <= 0 {
		opts.Size = 3
	}
	im.opts.median = &opts
	return im
}

// ModulateOptions configures Modulate. Default values are pass-through
// (no-op): brightness=1, saturation=1, hue=0, lightness=0.
type ModulateOptions struct {
	Brightness float64 // multiplier; 1.0 = no change
	Saturation float64 // multiplier; 1.0 = no change
	Hue        float64 // rotation in degrees
	Lightness  float64 // additive L* (Lab)
}

// Modulate scales brightness/saturation and rotates hue.
func (im *Image) Modulate(opts ModulateOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Brightness == 0 {
		opts.Brightness = 1
	}
	if opts.Saturation == 0 {
		opts.Saturation = 1
	}
	im.opts.modulate = &opts
	return im
}

// NormaliseOptions configures Normalise. Percentile clipping (sharp's
// {lower, upper} parameters) is deferred to a later milestone.
type NormaliseOptions struct {
	Lower int // percentile, 0-100; default 1
	Upper int // percentile, 0-100; default 99
}

// Normalise stretches the dynamic range to 0-255.
func (im *Image) Normalise(opts NormaliseOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.normalise = &opts
	return im
}

// ClaheOptions configures Clahe.
type ClaheOptions struct {
	Width    int // tile width in pixels; required
	Height   int // tile height in pixels; required
	MaxSlope int // slope clip; 0 means no clip
}

// Clahe applies Contrast-Limited Adaptive Histogram Equalisation.
func (im *Image) Clahe(opts ClaheOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.clahe = &opts
	return im
}

// ConvolveOptions configures Convolve.
type ConvolveOptions struct {
	Kernel []float64
	Width  int
	Height int
	Scale  float64 // 0 = auto
	Offset float64
}

// Convolve applies a 2D convolution.
func (im *Image) Convolve(opts ConvolveOptions) *Image {
	if im.err != nil {
		return im
	}
	c := opts
	im.opts.convolve = &c
	return im
}

// BooleanOp selects the bitwise operation for Boolean.
type BooleanOp int

const (
	BooleanAnd BooleanOp = iota
	BooleanOr
	BooleanEor
)

// BooleanOptions configures Boolean.
type BooleanOptions struct {
	Op       BooleanOp
	Constant float64
}

// Boolean applies a bitwise op against a scalar constant.
func (im *Image) Boolean(opts BooleanOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.boolean = &opts
	return im
}

// RecombOptions configures Recomb. Matrix is row-major NxN where N is the
// band count.
type RecombOptions struct {
	Matrix []float64
	N      int
}

// Recomb applies a band-recombination matrix.
func (im *Image) Recomb(opts RecombOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.recomb = &opts
	return im
}

// MorphOptions configures Dilate/Erode.
type MorphOptions struct {
	Size int // iteration count; default 1
}

// Dilate applies a morphological dilate.
func (im *Image) Dilate(opts MorphOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Size <= 0 {
		opts.Size = 1
	}
	im.opts.dilate = &opts
	return im
}

// Erode applies a morphological erode.
func (im *Image) Erode(opts MorphOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Size <= 0 {
		opts.Size = 1
	}
	im.opts.erode = &opts
	return im
}

// FlattenOptions configures Flatten.
type FlattenOptions struct {
	Background Color // alpha channel is ignored; 0-255 RGB
}

// Flatten composites the alpha channel onto background.
func (im *Image) Flatten(opts FlattenOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.flatten = &opts
	return im
}
