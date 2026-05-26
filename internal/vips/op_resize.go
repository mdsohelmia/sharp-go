//go:build cgo

package vips

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// Kernel selects the resize interpolation kernel.
type Kernel int

const (
	KernelNearest  Kernel = C.VIPS_KERNEL_NEAREST
	KernelLinear   Kernel = C.VIPS_KERNEL_LINEAR
	KernelCubic    Kernel = C.VIPS_KERNEL_CUBIC
	KernelMitchell Kernel = C.VIPS_KERNEL_MITCHELL
	KernelLanczos2 Kernel = C.VIPS_KERNEL_LANCZOS2
	KernelLanczos3 Kernel = C.VIPS_KERNEL_LANCZOS3
)

// Size selects the thumbnail size-fitting mode.
type Size int

const (
	SizeBoth  Size = C.VIPS_SIZE_BOTH
	SizeUp    Size = C.VIPS_SIZE_UP
	SizeDown  Size = C.VIPS_SIZE_DOWN
	SizeForce Size = C.VIPS_SIZE_FORCE
)

// Interesting selects the crop strategy when fit=cover.
type Interesting int

const (
	InterestingNone      Interesting = C.VIPS_INTERESTING_NONE
	InterestingCentre    Interesting = C.VIPS_INTERESTING_CENTRE
	InterestingEntropy   Interesting = C.VIPS_INTERESTING_ENTROPY
	InterestingAttention Interesting = C.VIPS_INTERESTING_ATTENTION
	InterestingLow       Interesting = C.VIPS_INTERESTING_LOW
	InterestingHigh      Interesting = C.VIPS_INTERESTING_HIGH
	InterestingAll       Interesting = C.VIPS_INTERESTING_ALL
)

// ThumbnailParams are the inputs to ThumbnailImage.
type ThumbnailParams struct {
	Width    int
	Height   int
	Kernel   Kernel
	Size     Size
	Crop     Interesting
	NoRotate bool

	// Optional ICC profile names/paths honoured by the fused thumbnail-buffer
	// path. Ignored by ThumbnailImage (post-decode resize).
	ImportProfile string
	ExportProfile string
	Intent        Intent
}

// Intent selects the ICC rendering intent passed to thumbnail-buffer.
type Intent int

const (
	IntentPerceptual           Intent = C.VIPS_INTENT_PERCEPTUAL
	IntentRelativeColorimetric Intent = C.VIPS_INTENT_RELATIVE
	IntentSaturation           Intent = C.VIPS_INTENT_SATURATION
	IntentAbsoluteColorimetric Intent = C.VIPS_INTENT_ABSOLUTE
)

// ThumbnailImage produces a resized variant of the input image using libvips'
// optimised thumbnail pipeline (no shrink-on-load — input is already decoded).
func ThumbnailImage(im *Image, p ThumbnailParams) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_thumbnail_image(
		im.ptr, &out,
		C.int(p.Width), C.int(p.Height),
		C.int(p.Kernel), C.int(p.Size), C.int(p.Crop),
		boolToC(p.NoRotate),
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// ThumbnailBuffer fuses load + resize into a single vips_thumbnail_buffer
// call. The decoder uses format-native shrink-on-load (JPEG DCT scale,
// PNG/WebP/HEIF native subsample) so libvips never materialises the full
// source — large source / small target workloads see 3-5× speedup and ~10×
// peak RSS reduction versus LoadBuffer + ThumbnailImage.
//
// The Kernel field is ignored: libvips' thumbnail pipeline uses lanczos3
// internally and does not expose a kernel knob. The buffer must remain valid
// for the duration of the call; libvips copies pixel data before return.
func ThumbnailBuffer(buf []byte, p ThumbnailParams) (*Image, error) {
	if len(buf) == 0 {
		return nil, errors.New("vips: empty input buffer")
	}
	var cImport, cExport *C.char
	if p.ImportProfile != "" {
		cImport = C.CString(p.ImportProfile)
		defer C.free(unsafe.Pointer(cImport))
	}
	if p.ExportProfile != "" {
		cExport = C.CString(p.ExportProfile)
		defer C.free(unsafe.Pointer(cExport))
	}
	var out *C.VipsImage
	rc := C.sharpgo_thumbnail_buffer(
		unsafe.Pointer(&buf[0]),
		C.size_t(len(buf)),
		C.int(p.Width), C.int(p.Height),
		C.int(p.Size), C.int(p.Crop), boolToC(p.NoRotate),
		cImport, cExport, C.int(p.Intent),
		&out,
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// EmbedParams are the inputs to Embed.
type EmbedParams struct {
	X, Y          int
	Width, Height int
	BgR, BgG, BgB float64 // 0-255
	BgA           float64 // 0-255
}

// Embed places the source image into a larger canvas at (x, y), padding with
// background colour.
func Embed(im *Image, p EmbedParams) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_embed(
		im.ptr, &out,
		C.int(p.X), C.int(p.Y),
		C.int(p.Width), C.int(p.Height),
		C.double(p.BgR), C.double(p.BgG), C.double(p.BgB), C.double(p.BgA),
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// silence "unused" if cgo defines unused helpers
var _ = unsafe.Pointer(nil)
