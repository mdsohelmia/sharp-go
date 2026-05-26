//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

import "unsafe"

// Tint colour-tints the image while preserving luminance.
func Tint(im *Image, r, g, b float64) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_tint(im.ptr, &out, C.double(r), C.double(g), C.double(b)); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// Greyscale converts to single-band b/w.
func Greyscale(im *Image) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_greyscale(im.ptr, &out); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// Colourspace converts to the given libvips interpretation.
func Colourspace(im *Image, interp Interpretation) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_colourspace(im.ptr, &out, C.int(interp)); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// RemoveAlpha drops the alpha channel if present.
func RemoveAlpha(im *Image) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_remove_alpha(im.ptr, &out); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// EnsureAlpha adds an alpha channel of the given value (0-1 normalised against
// libvips' format range) if one is not already present.
func EnsureAlpha(im *Image, alpha float64) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_ensure_alpha(im.ptr, &out, C.double(alpha)); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// ConvolveParams configures Convolve.
type ConvolveParams struct {
	Kernel []float64
	Width  int
	Height int
	Scale  float64
	Offset float64
}

// Convolve applies a 2D convolution.
func Convolve(im *Image, p ConvolveParams) (*Image, error) {
	if len(p.Kernel) != p.Width*p.Height {
		return nil, loadError()
	}
	var out *C.VipsImage
	rc := C.sharpgo_convolve(im.ptr, &out,
		(*C.double)(unsafe.Pointer(&p.Kernel[0])),
		C.int(p.Width), C.int(p.Height),
		C.double(p.Scale), C.double(p.Offset),
	)
	if rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// BooleanOp selects the boolean operation for BooleanConst.
type BooleanOp int

const (
	BooleanAnd BooleanOp = C.VIPS_OPERATION_BOOLEAN_AND
	BooleanOr  BooleanOp = C.VIPS_OPERATION_BOOLEAN_OR
	BooleanEor BooleanOp = C.VIPS_OPERATION_BOOLEAN_EOR
)

// BooleanConst applies a bitwise op against a scalar constant.
func BooleanConst(im *Image, op BooleanOp, constant float64) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_boolean_const(im.ptr, &out, C.int(op), C.double(constant)); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// Recomb applies a band-recombination matrix (N x N).
func Recomb(im *Image, matrix []float64, n int) (*Image, error) {
	if len(matrix) != n*n {
		return nil, loadError()
	}
	var out *C.VipsImage
	rc := C.sharpgo_recomb(im.ptr, &out,
		(*C.double)(unsafe.Pointer(&matrix[0])),
		C.int(n),
	)
	if rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// MorphMode selects dilate vs erode.
type MorphMode int

const (
	MorphDilate MorphMode = 0
	MorphErode  MorphMode = 1
)

// Morph applies a morphological dilate or erode `size` times.
func Morph(im *Image, size int, mode MorphMode) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_morph(im.ptr, &out, C.int(size), C.int(mode)); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// Flatten composites the alpha channel onto background.
func Flatten(im *Image, r, g, b float64) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_flatten(im.ptr, &out, C.double(r), C.double(g), C.double(b)); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// Clahe applies Contrast-Limited Adaptive Histogram Equalisation.
func Clahe(im *Image, width, height, maxSlope int) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_clahe(im.ptr, &out, C.int(width), C.int(height), C.int(maxSlope))
	if rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// Normalise stretches the dynamic range to 0-255.
func Normalise(im *Image, lowerPct, upperPct int) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_normalise(im.ptr, &out, C.int(lowerPct), C.int(upperPct)); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// Modulate scales brightness/saturation and rotates hue.
func Modulate(im *Image, brightness, saturation, hueDeg, lightnessAdd float64) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_modulate(im.ptr, &out,
		C.double(brightness), C.double(saturation),
		C.double(hueDeg), C.double(lightnessAdd),
	)
	if rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}
