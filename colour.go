package sharp

import (
	"github.com/sohelmia/sharp-go/internal/vips"
)

// TintOptions configures Tint.
type TintOptions struct {
	Colour Color // 0-255 RGB; alpha ignored
}

// Tint tints the image while preserving luminance.
func (im *Image) Tint(opts TintOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.tint = &opts
	return im
}

// Greyscale converts the image to single-band b/w. Alias for the US spelling
// Grayscale.
func (im *Image) Greyscale() *Image {
	if im.err != nil {
		return im
	}
	im.opts.greyscale = true
	return im
}

// Grayscale is an alias for Greyscale.
func (im *Image) Grayscale() *Image { return im.Greyscale() }

// ColourspaceID names a libvips colourspace interpretation that
// PipelineColourspace/Colourspace can target.
type ColourspaceID int

const (
	ColourspaceSRGB ColourspaceID = ColourspaceID(vips.InterpretationSRGB)
	ColourspaceBW   ColourspaceID = ColourspaceID(vips.InterpretationBW)
	ColourspaceCMYK ColourspaceID = ColourspaceID(vips.InterpretationCMYK)
	ColourspaceLAB  ColourspaceID = ColourspaceID(vips.InterpretationLAB)
	ColourspaceXYZ  ColourspaceID = ColourspaceID(vips.InterpretationXYZ)
)

// ToColourspace records an output colourspace conversion. Mirrors sharp's
// toColourspace(name). Applied after all other ops.
func (im *Image) ToColourspace(c ColourspaceID) *Image {
	if im.err != nil {
		return im
	}
	x := c
	im.opts.toColourspace = &x
	return im
}

// ToColorspace is the US-spelling alias.
func (im *Image) ToColorspace(c ColourspaceID) *Image { return im.ToColourspace(c) }

// PipelineColourspace sets the colourspace in which subsequent operations
// run. Mirrors sharp's pipelineColourspace(name) — applied before image-ops
// and reverted (where appropriate) before encoding.
func (im *Image) PipelineColourspace(c ColourspaceID) *Image {
	if im.err != nil {
		return im
	}
	x := c
	im.opts.pipelineColourspace = &x
	return im
}

// PipelineColorspace is the US-spelling alias.
func (im *Image) PipelineColorspace(c ColourspaceID) *Image {
	return im.PipelineColourspace(c)
}

// EnsureSRGB transforms the image into sRGB via its embedded ICC profile.
// Wide-gamut sources (Adobe RGB, Display P3, ProPhoto, …) display
// universally after this call — the pixel values are recalculated for
// sRGB rather than carried verbatim with an attached profile.
//
// Use this for CDN-style output where the encoded WebP/AVIF/JPEG must look
// identical on viewers that don't colour-manage embedded ICC profiles
// (Fastly Image Optimizer behaves this way by default). Pair with the
// default keep-no-metadata to drop the wide-gamut profile from the output.
//
// Recorded; the transform runs before resize so subsequent ops work in
// sRGB. Safe on inputs that are already sRGB or have no profile — libvips
// falls back to the input fallback profile (also sRGB) so the call is
// effectively a no-op for those.
func (im *Image) EnsureSRGB() *Image {
	if im.err != nil {
		return im
	}
	im.opts.ensureSRGB = true
	return im
}
