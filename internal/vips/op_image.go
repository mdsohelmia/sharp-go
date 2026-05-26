//go:build cgo

package vips

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import "unsafe"

// Gaussblur applies a Gaussian blur of the given sigma.
func Gaussblur(im *Image, sigma float64) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_gaussblur(im.ptr, &out, C.double(sigma)); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// SharpenParams configures unsharp-mask sharpening.
type SharpenParams struct {
	Sigma float64
	M1    float64
	M2    float64
	X1    float64
	Y2    float64
	Y3    float64
}

// Sharpen applies an unsharp-mask sharpen.
func Sharpen(im *Image, p SharpenParams) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_sharpen(
		im.ptr, &out,
		C.double(p.Sigma),
		C.double(p.M1), C.double(p.M2),
		C.double(p.X1), C.double(p.Y2), C.double(p.Y3),
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Gamma corrects gamma. ExponentOut is optional (0 = use Exponent for both).
func Gamma(im *Image, exponent, exponentOut float64) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_gamma(im.ptr, &out, C.double(exponent), C.double(exponentOut))
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Negate inverts pixel values. If keepAlpha and the input has an alpha
// channel, the alpha is left untouched.
func Negate(im *Image, keepAlpha bool) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_negate(im.ptr, &out, boolToC(keepAlpha)); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Threshold thresholds pixels at value. If grayscale, the image is converted
// to b-w first.
func Threshold(im *Image, value float64, grayscale bool) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_threshold(im.ptr, &out, C.double(value), boolToC(grayscale)); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Linear applies out = in * a + b. a/b may be length 1 or band-count.
func Linear(im *Image, a, b []float64) (*Image, error) {
	if len(a) == 0 {
		a = []float64{1}
	}
	if len(b) == 0 {
		b = []float64{0}
	}
	var out *C.VipsImage
	rc := C.sharpgo_linear(
		im.ptr, &out,
		(*C.double)(unsafe.Pointer(&a[0])), C.int(len(a)),
		(*C.double)(unsafe.Pointer(&b[0])), C.int(len(b)),
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Median applies a median filter with a square window of width pixels.
func Median(im *Image, size int) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_median(im.ptr, &out, C.int(size)); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}
