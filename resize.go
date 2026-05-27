package sharp

import (
	"errors"

	"github.com/mdsohelmia/sharp-go/internal/vips"
)

// ResizeOptions configures Resize. Mirrors the option object that sharp's
// resize(width, height, opts) accepts.
type ResizeOptions struct {
	Width    int
	Height   int
	Fit      Fit
	Position Position
	Kernel   Kernel

	// Background colour for FitContain padding. Default: transparent black.
	Background Color

	// WithoutEnlargement disables upscaling.
	WithoutEnlargement bool
	// WithoutReduction disables downscaling.
	WithoutReduction bool
}

// Resize records a resize operation.
//
// At least one of Width or Height must be set. A zero dimension is treated
// as "match the other dimension while preserving aspect ratio".
func (im *Image) Resize(opts ResizeOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Width <= 0 && opts.Height <= 0 {
		im.stickyErr(errors.New("sharp: Resize requires Width or Height"))
		return im
	}
	r := opts
	im.opts.resize = &r
	return im
}

// resizeDimensions resolves the (possibly partial) width/height from
// ResizeOptions against a known source size, preserving aspect ratio when
// only one dimension is supplied.
func resizeDimensions(r *ResizeOptions, sw, sh int) (width, height int) {
	width = r.Width
	height = r.Height
	switch {
	case width <= 0 && height > 0:
		width = int(float64(sw)*float64(height)/float64(sh) + 0.5)
	case height <= 0 && width > 0:
		height = int(float64(sh)*float64(width)/float64(sw) + 0.5)
	case width <= 0 && height <= 0:
		width, height = sw, sh
	}
	return
}

// resizeThumbnailParams maps ResizeOptions onto ThumbnailParams. Returns the
// fully-resolved width/height alongside.
func resizeThumbnailParams(r *ResizeOptions, width, height int) vips.ThumbnailParams {
	p := vips.ThumbnailParams{
		Width:    width,
		Height:   height,
		Kernel:   mapKernel(r.Kernel),
		NoRotate: true, // sharp does autoOrient as a separate pipeline step
	}
	switch r.Fit {
	case FitFill:
		p.Size = vips.SizeForce
		p.Crop = vips.InterestingNone
	case FitInside:
		p.Size = vips.SizeDown
		p.Crop = vips.InterestingNone
	case FitOutside:
		p.Size = vips.SizeUp
		p.Crop = vips.InterestingNone
	case FitContain:
		p.Size = vips.SizeBoth
		p.Crop = vips.InterestingNone
	case FitCover:
		fallthrough
	default:
		p.Size = vips.SizeBoth
		p.Crop = mapPosition(r.Position)
	}
	if r.WithoutEnlargement {
		if p.Size == vips.SizeBoth || p.Size == vips.SizeUp {
			p.Size = vips.SizeDown
		}
	}
	if r.WithoutReduction {
		if p.Size == vips.SizeBoth || p.Size == vips.SizeDown {
			p.Size = vips.SizeUp
		}
	}
	return p
}

// applyResizeContainPadding pads a thumbnail to the requested box when Fit is
// FitContain and the result fell short on either dimension. Shared between
// the post-decode and fused-thumbnail paths.
func applyResizeContainPadding(out *vips.Image, r *ResizeOptions) (*vips.Image, error) {
	if r.Fit != FitContain {
		return out, nil
	}
	ow, oh := out.Width(), out.Height()
	if ow >= r.Width && oh >= r.Height {
		return out, nil
	}
	x := (r.Width - ow) / 2
	y := (r.Height - oh) / 2
	return vips.Embed(out, vips.EmbedParams{
		X: x, Y: y,
		Width: r.Width, Height: r.Height,
		BgR: r.Background.R, BgG: r.Background.G, BgB: r.Background.B, BgA: r.Background.A,
	})
}

// applyResize maps the public ResizeOptions to internal vips parameters and
// runs the thumbnail (+ optional embed) pipeline against vimg. This is the
// post-decode path; the fused shrink-on-load path runs in buildPipelineImage.
func applyResize(vimg *vips.Image, r *ResizeOptions) (*vips.Image, error) {
	sw, sh := vimg.Width(), vimg.Height()
	width, height := resizeDimensions(r, sw, sh)

	// Edge gravities (north/south/east/west/...) for FitCover need a two-step
	// resize+extract that libvips' thumbnail doesn't expose directly.
	if r.Fit == FitCover && isEdgeGravity(r.Position) {
		return applyResizeCoverEdge(vimg, r, width, height)
	}

	p := resizeThumbnailParams(r, width, height)
	out, err := vips.ThumbnailImage(vimg, p)
	if err != nil {
		return nil, err
	}
	// VIPS_INTERESTING_ALL asks libvips to treat the whole frame as interesting,
	// so it preserves the source aspect ratio (no cropping) and may return an
	// image larger than the target box on one axis. For FitCover semantics we
	// centre-crop any overshoot back to the exact target size.
	if r.Fit == FitCover && r.Position == PositionAll {
		ow, oh := out.Width(), out.Height()
		if ow > width || oh > height {
			x := (ow - width) / 2
			y := (oh - height) / 2
			cw, ch := width, height
			if ow < cw {
				cw = ow
			}
			if oh < ch {
				ch = oh
			}
			out, err = vips.ExtractArea(out, x, y, cw, ch)
			if err != nil {
				return nil, err
			}
		}
	}
	return applyResizeContainPadding(out, r)
}

func mapKernel(k Kernel) vips.Kernel {
	switch k {
	case KernelNearest:
		return vips.KernelNearest
	case KernelLinear:
		return vips.KernelLinear
	case KernelCubic:
		return vips.KernelCubic
	case KernelMitchell:
		return vips.KernelMitchell
	case KernelLanczos2:
		return vips.KernelLanczos2
	case KernelLanczos3:
		fallthrough
	default:
		return vips.KernelLanczos3
	}
}

func mapPosition(p Position) vips.Interesting {
	switch p {
	case PositionEntropy:
		return vips.InterestingEntropy
	case PositionAttention:
		return vips.InterestingAttention
	case PositionLow:
		return vips.InterestingLow
	case PositionHigh:
		return vips.InterestingHigh
	case PositionAll:
		return vips.InterestingAll
	case PositionCentre:
		fallthrough
	default:
		return vips.InterestingCentre
	}
}

func isEdgeGravity(p Position) bool {
	switch p {
	case PositionNorth, PositionNorthEast, PositionEast, PositionSouthEast,
		PositionSouth, PositionSouthWest, PositionWest, PositionNorthWest:
		return true
	}
	return false
}

// applyResizeCoverEdge handles FitCover with an edge/corner gravity. libvips
// thumbnail's crop modes don't cover these, so we resize to "cover" with
// crop=NONE (no crop) and then extract the requested sub-rectangle.
func applyResizeCoverEdge(vimg *vips.Image, r *ResizeOptions, targetW, targetH int) (*vips.Image, error) {
	sw, sh := vimg.Width(), vimg.Height()
	// Scale such that the smaller dimension fills the box.
	scaleW := float64(targetW) / float64(sw)
	scaleH := float64(targetH) / float64(sh)
	scale := scaleW
	if scaleH > scale {
		scale = scaleH
	}
	intermediateW := int(float64(sw)*scale + 0.5)
	intermediateH := int(float64(sh)*scale + 0.5)

	resized, err := vips.ThumbnailImage(vimg, vips.ThumbnailParams{
		Width:    intermediateW,
		Height:   intermediateH,
		Size:     vips.SizeForce,
		Crop:     vips.InterestingNone,
		NoRotate: true,
	})
	if err != nil {
		return nil, err
	}

	x, y := edgeCropOffset(r.Position, intermediateW, intermediateH, targetW, targetH)
	return vips.ExtractArea(resized, x, y, targetW, targetH)
}

func edgeCropOffset(p Position, iw, ih, tw, th int) (x, y int) {
	dx := iw - tw
	dy := ih - th
	if dx < 0 {
		dx = 0
	}
	if dy < 0 {
		dy = 0
	}
	switch p {
	case PositionNorth:
		return dx / 2, 0
	case PositionNorthEast:
		return dx, 0
	case PositionEast:
		return dx, dy / 2
	case PositionSouthEast:
		return dx, dy
	case PositionSouth:
		return dx / 2, dy
	case PositionSouthWest:
		return 0, dy
	case PositionWest:
		return 0, dy / 2
	case PositionNorthWest:
		return 0, 0
	default:
		return dx / 2, dy / 2
	}
}
