package sharp

import (
	"errors"
	"math"

	"github.com/sohelmia/sharp-go/internal/vips"
)

// applyRotate maps the public RotateOptions to the internal vips ops. Uses
// vips_rot for the lossless 90/180/270 cases and vips_rotate for arbitrary
// angles.
func applyRotate(vimg *vips.Image, r *RotateOptions) (*vips.Image, error) {
	angle := math.Mod(r.Angle, 360)
	if angle < 0 {
		angle += 360
	}
	switch angle {
	case 0:
		return vimg, nil
	case 90:
		return vips.Rot90(vimg, 1)
	case 180:
		return vips.Rot90(vimg, 2)
	case 270:
		return vips.Rot90(vimg, 3)
	default:
		return vips.Rotate(vimg, vips.RotateParams{
			Angle: r.Angle,
			BgR:   r.Background.R, BgG: r.Background.G, BgB: r.Background.B, BgA: r.Background.A,
		})
	}
}

// RotateOptions configures Rotate.
type RotateOptions struct {
	// Angle in degrees. 0/90/180/270 are lossless (no interpolation).
	// Other angles trigger interpolation with Background fill.
	Angle float64

	// Background colour for non-orthogonal rotations. Default transparent black.
	Background Color
}

// Rotate records a rotate operation.
//
// Sharp behaviour:
//   - Angle == 0 with no other options is a no-op.
//   - Multiples of 90 use lossless vips_rot.
//   - Other angles use vips_rotate with Background fill.
func (im *Image) Rotate(opts RotateOptions) *Image {
	if im.err != nil {
		return im
	}
	r := opts
	im.opts.rotate = &r
	return im
}

// AutoOrient applies the EXIF orientation tag and clears it.
func (im *Image) AutoOrient() *Image {
	if im.err != nil {
		return im
	}
	im.opts.autoOrient = true
	return im
}

// Flip mirrors the image top-to-bottom (vertical flip across the X axis).
func (im *Image) Flip() *Image {
	if im.err != nil {
		return im
	}
	im.opts.flip = true
	return im
}

// Flop mirrors the image left-to-right (horizontal flip across the Y axis).
func (im *Image) Flop() *Image {
	if im.err != nil {
		return im
	}
	im.opts.flop = true
	return im
}

// ExtractRegion configures Extract.
type ExtractRegion struct {
	Left, Top, Width, Height int
}

// Extract records a crop to the given sub-rectangle. The crop is applied
// before resize (matching sharp's pipeline order: extract -> autoOrient ->
// rotate -> resize).
func (im *Image) Extract(r ExtractRegion) *Image {
	if im.err != nil {
		return im
	}
	if r.Width <= 0 || r.Height <= 0 {
		im.stickyErr(errors.New("sharp: Extract requires positive Width and Height"))
		return im
	}
	if r.Left < 0 || r.Top < 0 {
		im.stickyErr(errors.New("sharp: Extract requires non-negative Left and Top"))
		return im
	}
	rr := r
	im.opts.extract = &rr
	return im
}
