//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

// DeltaEMethod selects the CIE colour-difference formula.
type DeltaEMethod int

const (
	DeltaE2000 DeltaEMethod = iota // vips_dE00
	DeltaE76                       // vips_dE76
	DeltaECMC                      // vips_dECMC
)

// Subtract returns a float image of (a - b). a and b must share dimensions
// and band count.
func Subtract(a, b *Image) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_subtract(a.ptr, b.ptr, &out); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// DeltaE returns a 1-band per-pixel CIE colour-difference image between a and
// b using the given formula. Inputs are converted to LAB by libvips.
func DeltaE(a, b *Image, method DeltaEMethod) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_delta_e(a.ptr, b.ptr, &out, C.int(method)); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}
