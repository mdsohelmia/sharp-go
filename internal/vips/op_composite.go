//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

// BlendMode enumerates libvips' VipsBlendMode values (Porter-Duff + photo
// shop blends). Names match libvips.
type BlendMode int

const (
	BlendClear      BlendMode = C.VIPS_BLEND_MODE_CLEAR
	BlendSource     BlendMode = C.VIPS_BLEND_MODE_SOURCE
	BlendOver       BlendMode = C.VIPS_BLEND_MODE_OVER
	BlendIn         BlendMode = C.VIPS_BLEND_MODE_IN
	BlendOut        BlendMode = C.VIPS_BLEND_MODE_OUT
	BlendAtop       BlendMode = C.VIPS_BLEND_MODE_ATOP
	BlendDest       BlendMode = C.VIPS_BLEND_MODE_DEST
	BlendDestOver   BlendMode = C.VIPS_BLEND_MODE_DEST_OVER
	BlendDestIn     BlendMode = C.VIPS_BLEND_MODE_DEST_IN
	BlendDestOut    BlendMode = C.VIPS_BLEND_MODE_DEST_OUT
	BlendDestAtop   BlendMode = C.VIPS_BLEND_MODE_DEST_ATOP
	BlendXor        BlendMode = C.VIPS_BLEND_MODE_XOR
	BlendAdd        BlendMode = C.VIPS_BLEND_MODE_ADD
	BlendSaturate   BlendMode = C.VIPS_BLEND_MODE_SATURATE
	BlendMultiply   BlendMode = C.VIPS_BLEND_MODE_MULTIPLY
	BlendScreen     BlendMode = C.VIPS_BLEND_MODE_SCREEN
	BlendOverlay    BlendMode = C.VIPS_BLEND_MODE_OVERLAY
	BlendDarken     BlendMode = C.VIPS_BLEND_MODE_DARKEN
	BlendLighten    BlendMode = C.VIPS_BLEND_MODE_LIGHTEN
	BlendColourDodge BlendMode = C.VIPS_BLEND_MODE_COLOUR_DODGE
	BlendColourBurn  BlendMode = C.VIPS_BLEND_MODE_COLOUR_BURN
	BlendHardLight   BlendMode = C.VIPS_BLEND_MODE_HARD_LIGHT
	BlendSoftLight   BlendMode = C.VIPS_BLEND_MODE_SOFT_LIGHT
	BlendDifference  BlendMode = C.VIPS_BLEND_MODE_DIFFERENCE
	BlendExclusion   BlendMode = C.VIPS_BLEND_MODE_EXCLUSION
)

// Replicate tiles `in` to fill the given width x height canvas.
func Replicate(im *Image, width, height int) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_replicate(im.ptr, &out, C.int(width), C.int(height)); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Composite2Params configures Composite2.
type Composite2Params struct {
	Blend         BlendMode
	X, Y          int
	Premultiplied bool
}

// Composite2 composites overlay onto base.
func Composite2(base, overlay *Image, p Composite2Params) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_composite2(base.ptr, overlay.ptr, &out,
		C.int(p.Blend), C.int(p.X), C.int(p.Y), boolToC(p.Premultiplied),
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// ExtractBand returns a single-channel image containing band `band` (0-based).
func ExtractBand(im *Image, band int) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_extract_band(im.ptr, &out, C.int(band)); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Bandjoin band-joins `len(images)` images. They must share width/height.
func Bandjoin(images []*Image) (*Image, error) {
	if len(images) == 0 {
		return nil, lastError()
	}
	ptrs := make([]*C.VipsImage, len(images))
	for i, im := range images {
		ptrs[i] = im.ptr
	}
	var out *C.VipsImage
	rc := C.sharpgo_bandjoin(&ptrs[0], C.int(len(images)), &out)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Bandbool reduces all bands to a single boolean channel via op.
func Bandbool(im *Image, op BooleanOp) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_bandbool(im.ptr, &out, C.int(op)); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}
