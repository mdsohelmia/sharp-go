package sharp

import (
	"github.com/mdsohelmia/sharp-go/internal/vips"
)

// ExtendOptions configures Extend.
type ExtendOptions struct {
	Top, Bottom, Left, Right int
	Background               Color
}

// Extend pads the image with borders on each side, filling with Background.
func (im *Image) Extend(opts ExtendOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.extend = &opts
	return im
}

// TrimOptions configures Trim.
type TrimOptions struct {
	// Threshold 0-255 (sharp default 10). Pixels within this distance of the
	// border colour are treated as background.
	Threshold float64
	// LineArt uses pure black/white detection (sharp's lineArt option).
	LineArt bool
}

// Trim removes solid-colour borders.
func (im *Image) Trim(opts TrimOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Threshold == 0 {
		opts.Threshold = 10
	}
	im.opts.trim = &opts
	return im
}

// AffineOptions configures Affine.
type AffineOptions struct {
	// Matrix is the 2x2 affine matrix in row-major order: [a, b, c, d].
	// Output coordinates (X, Y) map to input via: X = a*x + b*y; Y = c*x + d*y.
	Matrix [4]float64
	// Background colour for areas outside the source image.
	Background Color
}

// Affine applies an affine transform.
func (im *Image) Affine(opts AffineOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.affine = &opts
	return im
}

// applyExtend wraps vips.Embed with sharp-style border offsets.
func applyExtend(vimg *vips.Image, e *ExtendOptions) (*vips.Image, error) {
	w := vimg.Width() + e.Left + e.Right
	h := vimg.Height() + e.Top + e.Bottom
	return vips.Embed(vimg, vips.EmbedParams{
		X: e.Left, Y: e.Top,
		Width: w, Height: h,
		BgR: e.Background.R, BgG: e.Background.G,
		BgB: e.Background.B, BgA: e.Background.A,
	})
}

// applyTrim runs find_trim + extract_area.
func applyTrim(vimg *vips.Image, t *TrimOptions) (*vips.Image, error) {
	l, top, w, h, err := vips.FindTrim(vimg, t.Threshold, t.LineArt)
	if err != nil {
		return nil, err
	}
	if w <= 0 || h <= 0 {
		// Nothing to trim — pass through.
		return vimg, nil
	}
	return vips.ExtractArea(vimg, l, top, w, h)
}

// applyAffine wraps vips.Affine.
func applyAffine(vimg *vips.Image, a *AffineOptions) (*vips.Image, error) {
	return vips.Affine(vimg, vips.AffineParams{
		A: a.Matrix[0], B: a.Matrix[1], C: a.Matrix[2], D: a.Matrix[3],
		BgR: a.Background.R, BgG: a.Background.G,
		BgB: a.Background.B, BgA: a.Background.A,
	})
}
