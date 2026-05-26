//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

// FindTrim computes the bounding box of non-background content using the
// libvips find_trim algorithm.
//
// threshold: 0-255 sensitivity (sharp default 10).
// lineArt:   true uses pure black/white detection (skips alpha-based logic).
func FindTrim(im *Image, threshold float64, lineArt bool) (left, top, width, height int, err error) {
	var l, t, w, h C.int
	rc := C.sharpgo_find_trim(im.ptr,
		C.double(threshold), boolToC(lineArt),
		&l, &t, &w, &h)
	if rc != 0 {
		return 0, 0, 0, 0, lastError()
	}
	return int(l), int(t), int(w), int(h), nil
}

// AffineParams configures Affine.
type AffineParams struct {
	A, B, C, D float64
	BgR, BgG, BgB, BgA float64
}

// Affine applies an affine transform with the given 2x2 matrix.
func Affine(im *Image, p AffineParams) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_affine(im.ptr, &out,
		C.double(p.A), C.double(p.B), C.double(p.C), C.double(p.D),
		C.double(p.BgR), C.double(p.BgG), C.double(p.BgB), C.double(p.BgA),
	)
	if rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}
