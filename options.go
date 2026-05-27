package sharp

// Color is an RGBA colour. Each channel is 0-255. Used for background fills
// and pad operations.
type Color struct {
	R, G, B, A float64
}

// Fit selects the resize fitting strategy. Mirrors sharp's `fit` option.
type Fit int

const (
	// FitCover (default) preserves aspect ratio and crops to fill the box.
	FitCover Fit = iota
	// FitContain preserves aspect ratio and pads to fit inside the box.
	FitContain
	// FitFill ignores aspect ratio and stretches to fill the box.
	FitFill
	// FitInside resizes to fit inside the box, no enlargement.
	FitInside
	// FitOutside resizes to fit outside the box, no reduction.
	FitOutside
)

// Position selects the crop anchor when Fit=FitCover. Mirrors sharp's
// `position` option (subset for v0.1; edge gravities land in v0.2).
type Position int

const (
	// PositionCentre is the default crop anchor.
	PositionCentre Position = iota
	// PositionEntropy uses libvips entropy-weighted smart crop.
	PositionEntropy
	// PositionAttention uses libvips attention-weighted smart crop.
	PositionAttention
	// Edge gravities. Naming matches sharp's gravity option.
	PositionNorth
	PositionNorthEast
	PositionEast
	PositionSouthEast
	PositionSouth
	PositionSouthWest
	PositionWest
	PositionNorthWest
	// PositionLow biases the smart crop toward low-value (dark) regions.
	// sharp-go extension: sharp's `position` exposes only entropy/attention.
	PositionLow
	// PositionHigh biases the smart crop toward high-value (bright) regions.
	PositionHigh
	// PositionAll treats the whole frame as interesting (near-centre crop).
	PositionAll
)

// Kernel selects the resize interpolation kernel. Mirrors sharp's `kernel`
// option.
type Kernel int

const (
	// KernelLanczos3 is the default resize kernel (sharp default).
	KernelLanczos3 Kernel = iota
	KernelNearest
	KernelLinear
	KernelCubic
	KernelMitchell
	KernelLanczos2
)
