//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"unsafe"
)

// LoadBuffer decodes a byte slice into an Image with automatic format detection.
// The Go slice may be freed/GC'd immediately after this call returns — pixel
// data has been copied into libvips-managed memory.
func LoadBuffer(buf []byte) (*Image, error) {
	if len(buf) == 0 {
		return nil, errors.New("vips: empty input buffer")
	}
	var out *C.VipsImage
	rc := C.sharpgo_load_buffer(
		unsafe.Pointer(&buf[0]),
		C.size_t(len(buf)),
		&out,
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// LoadBufferPages decodes a multi-page image (animated GIF/WebP/HEIF/TIFF
// or multi-page TIFF/PDF). pages: 1 = first frame, -1 = all frames.
// page: starting page index (0-based).
func LoadBufferPages(buf []byte, pages, page int) (*Image, error) {
	if len(buf) == 0 {
		return nil, errors.New("vips: empty input buffer")
	}
	var out *C.VipsImage
	rc := C.sharpgo_load_buffer_pages(
		unsafe.Pointer(&buf[0]),
		C.size_t(len(buf)),
		C.int(pages), C.int(page),
		&out,
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// LoadRawBuffer wraps raw pixel data with explicit dimensions + band format.
// The Go slice may be freed/GC'd immediately after this call returns — the
// pixel data is copied into libvips-managed memory.
func LoadRawBuffer(buf []byte, width, height, bands int, format BandFormat) (*Image, error) {
	if len(buf) == 0 {
		return nil, errors.New("vips: empty raw input buffer")
	}
	var out *C.VipsImage
	rc := C.sharpgo_load_raw_buffer(
		unsafe.Pointer(&buf[0]),
		C.size_t(len(buf)),
		C.int(width), C.int(height), C.int(bands), C.int(format),
		&out,
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}
