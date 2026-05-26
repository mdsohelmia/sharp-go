package sharp

import (
	"errors"

	"github.com/sohelmia/sharp-go/internal/vips"
)

// Gravity selects the overlay anchor when Top/Left are not set on a
// CompositeLayer.
type Gravity int

const (
	GravityCentre Gravity = iota
	GravityNorth
	GravityNorthEast
	GravityEast
	GravitySouthEast
	GravitySouth
	GravitySouthWest
	GravityWest
	GravityNorthWest
)

// BlendMode enumerates Porter-Duff + Photoshop-style blend modes for composite.
type BlendMode int

const (
	BlendOver BlendMode = iota
	BlendClear
	BlendSource
	BlendIn
	BlendOut
	BlendAtop
	BlendDest
	BlendDestOver
	BlendDestIn
	BlendDestOut
	BlendDestAtop
	BlendXor
	BlendAdd
	BlendSaturate
	BlendMultiply
	BlendScreen
	BlendOverlay
	BlendDarken
	BlendLighten
	BlendColourDodge
	BlendColourBurn
	BlendHardLight
	BlendSoftLight
	BlendDifference
	BlendExclusion
)

// CompositeLayer is one overlay in a Composite call.
type CompositeLayer struct {
	// Exactly one of Input, InputPath, or Prepared must be set. Prepared
	// takes precedence and is the recommended path for repeated watermark
	// overlays — it avoids re-decoding the overlay bytes on every call.
	Input     []byte
	InputPath string
	Prepared  *PreparedOverlay

	// Blend mode; default BlendOver.
	Blend BlendMode

	// Explicit offset. If HasOffset is false, Gravity is used instead.
	Top, Left int
	HasOffset bool

	// Gravity-based positioning, used when HasOffset is false.
	Gravity Gravity

	// Tile repeats the overlay to fill the base.
	Tile bool

	// Premultiplied skips the implicit unpremultiply step in libvips.
	Premultiplied bool
}

// PreparedOverlay is a decoded overlay image kept warm in memory so that
// many Composite calls can share it without re-decoding the source bytes.
// Construct once at server start with PrepareOverlay and reuse across
// requests; the underlying *vips.Image is reference-counted so concurrent
// composite calls each get an independent handle.
//
// Call Close when the overlay is no longer needed to release the libvips
// image immediately — otherwise it lives until GC reclaims the wrapper.
type PreparedOverlay struct {
	img *vips.Image
}

// PrepareOverlay decodes buf once and returns a sharable overlay handle.
// The image is upgraded to RGBA (alpha=1) so subsequent composite calls
// don't pay the EnsureAlpha cost.
func PrepareOverlay(buf []byte) (*PreparedOverlay, error) {
	if len(buf) == 0 {
		return nil, errors.New("sharp: PrepareOverlay requires non-empty buffer")
	}
	im, err := vips.LoadBuffer(buf)
	if err != nil {
		return nil, err
	}
	if !im.HasAlpha() {
		im, err = vips.EnsureAlpha(im, 1)
		if err != nil {
			return nil, err
		}
	}
	return &PreparedOverlay{img: im}, nil
}

// Close drops the wrapper's reference to the underlying image. After Close
// the overlay must not be used in further composites. Safe to call once;
// subsequent calls are no-ops.
func (p *PreparedOverlay) Close() {
	if p == nil {
		return
	}
	p.img = nil
}

// Composite stacks one or more overlays onto the current image.
func (im *Image) Composite(layers []CompositeLayer) *Image {
	if im.err != nil {
		return im
	}
	if len(layers) == 0 {
		return im
	}
	im.opts.composite = append([]CompositeLayer(nil), layers...)
	return im
}

// applyComposite applies recorded composite layers in order.
func applyComposite(base *vips.Image, layers []CompositeLayer) (*vips.Image, error) {
	if !base.HasAlpha() {
		var err error
		base, err = vips.EnsureAlpha(base, 1)
		if err != nil {
			return nil, err
		}
	}
	for _, layer := range layers {
		var (
			overlay *vips.Image
			err     error
		)
		switch {
		case layer.Prepared != nil && layer.Prepared.img != nil:
			// Reuse the pre-decoded overlay via a refcount bump — zero
			// alloc on the Go side, zero decode work in libvips.
			overlay = layer.Prepared.img.Ref()
		case layer.Input != nil:
			overlay, err = vips.LoadBuffer(layer.Input)
		case layer.InputPath != "":
			buf, ferr := readInput(inputSource{path: layer.InputPath})
			if ferr != nil {
				return nil, ferr
			}
			overlay, err = vips.LoadBuffer(buf)
		default:
			return nil, errors.New("sharp: composite layer needs Input, InputPath, or Prepared")
		}
		if err != nil {
			return nil, err
		}
		if !overlay.HasAlpha() {
			overlay, err = vips.EnsureAlpha(overlay, 1)
			if err != nil {
				return nil, err
			}
		}

		if layer.Tile {
			overlay, err = vips.Replicate(overlay, base.Width(), base.Height())
			if err != nil {
				return nil, err
			}
		}

		x, y := positionLayer(base, overlay, layer)
		base, err = vips.Composite2(base, overlay, vips.Composite2Params{
			Blend:         mapBlend(layer.Blend),
			X:             x,
			Y:             y,
			Premultiplied: layer.Premultiplied,
		})
		if err != nil {
			return nil, err
		}
	}
	return base, nil
}

func positionLayer(base, overlay *vips.Image, layer CompositeLayer) (x, y int) {
	if layer.HasOffset {
		return layer.Left, layer.Top
	}
	if layer.Tile {
		return 0, 0
	}
	bw, bh := base.Width(), base.Height()
	ow, oh := overlay.Width(), overlay.Height()
	switch layer.Gravity {
	case GravityNorth:
		return (bw - ow) / 2, 0
	case GravityNorthEast:
		return bw - ow, 0
	case GravityEast:
		return bw - ow, (bh - oh) / 2
	case GravitySouthEast:
		return bw - ow, bh - oh
	case GravitySouth:
		return (bw - ow) / 2, bh - oh
	case GravitySouthWest:
		return 0, bh - oh
	case GravityWest:
		return 0, (bh - oh) / 2
	case GravityNorthWest:
		return 0, 0
	case GravityCentre:
		fallthrough
	default:
		return (bw - ow) / 2, (bh - oh) / 2
	}
}

func mapBlend(b BlendMode) vips.BlendMode {
	switch b {
	case BlendClear:
		return vips.BlendClear
	case BlendSource:
		return vips.BlendSource
	case BlendIn:
		return vips.BlendIn
	case BlendOut:
		return vips.BlendOut
	case BlendAtop:
		return vips.BlendAtop
	case BlendDest:
		return vips.BlendDest
	case BlendDestOver:
		return vips.BlendDestOver
	case BlendDestIn:
		return vips.BlendDestIn
	case BlendDestOut:
		return vips.BlendDestOut
	case BlendDestAtop:
		return vips.BlendDestAtop
	case BlendXor:
		return vips.BlendXor
	case BlendAdd:
		return vips.BlendAdd
	case BlendSaturate:
		return vips.BlendSaturate
	case BlendMultiply:
		return vips.BlendMultiply
	case BlendScreen:
		return vips.BlendScreen
	case BlendOverlay:
		return vips.BlendOverlay
	case BlendDarken:
		return vips.BlendDarken
	case BlendLighten:
		return vips.BlendLighten
	case BlendColourDodge:
		return vips.BlendColourDodge
	case BlendColourBurn:
		return vips.BlendColourBurn
	case BlendHardLight:
		return vips.BlendHardLight
	case BlendSoftLight:
		return vips.BlendSoftLight
	case BlendDifference:
		return vips.BlendDifference
	case BlendExclusion:
		return vips.BlendExclusion
	case BlendOver:
		fallthrough
	default:
		return vips.BlendOver
	}
}
