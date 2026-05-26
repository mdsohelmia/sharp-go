//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

// Direction selects the axis for Flip.
type Direction int

const (
	DirectionHorizontal Direction = 0 // flop (left-right mirror)
	DirectionVertical   Direction = 1 // flip (up-down mirror)
)

// Rot90 rotates by 90, 180, or 270 degrees (lossless).
//
// quarter: 1=90, 2=180, 3=270. Any other value is a no-op identity copy.
func Rot90(im *Image, quarter int) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_rot(im.ptr, &out, C.int(quarter)); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// RotateParams configures arbitrary-angle rotation.
type RotateParams struct {
	Angle       float64
	BgR, BgG, BgB, BgA float64
}

// Rotate rotates by an arbitrary angle (degrees), padding with background.
func Rotate(im *Image, p RotateParams) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_rotate(
		im.ptr, &out,
		C.double(p.Angle),
		C.double(p.BgR), C.double(p.BgG), C.double(p.BgB), C.double(p.BgA),
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Flip flips across an axis.
func Flip(im *Image, d Direction) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_flip(im.ptr, &out, C.int(d)); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Autorot applies EXIF orientation and clears the orientation tag.
func Autorot(im *Image) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_autorot(im.ptr, &out); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// ExtractArea crops to a sub-rectangle.
func ExtractArea(im *Image, left, top, width, height int) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_extract_area(im.ptr, &out, C.int(left), C.int(top), C.int(width), C.int(height))
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}
