//go:build cgo

package vips

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import "unsafe"

// CreateSolidParams configures CreateSolid.
type CreateSolidParams struct {
	Width, Height int
	Bands         int     // 3 or 4
	BgR, BgG, BgB float64 // 0-255
	BgA           float64 // 0-255 (used only if Bands == 4)
}

// CreateSolid builds a solid-colour image.
func CreateSolid(p CreateSolidParams) (*Image, error) {
	if p.Bands != 3 && p.Bands != 4 {
		p.Bands = 3
	}
	var out *C.VipsImage
	rc := C.sharpgo_create_solid(&out,
		C.int(p.Width), C.int(p.Height), C.int(p.Bands),
		C.double(p.BgR), C.double(p.BgG), C.double(p.BgB), C.double(p.BgA),
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// CreateTextParams configures CreateText.
type CreateTextParams struct {
	Text     string
	Font     string // pango font spec, e.g. "sans 12"
	FontFile string // path to .ttf/.otf
	Width    int    // 0 = unconstrained
	Height   int    // 0 = unconstrained
	DPI      int    // default 72
	Spacing  int    // line spacing
	RGBA     bool
}

// CreateText renders text via libvips' pango integration.
func CreateText(p CreateTextParams) (*Image, error) {
	cText := C.CString(p.Text)
	defer C.free(unsafe.Pointer(cText))
	cFont := C.CString(p.Font)
	defer C.free(unsafe.Pointer(cFont))
	cFile := C.CString(p.FontFile)
	defer C.free(unsafe.Pointer(cFile))

	var out *C.VipsImage
	rc := C.sharpgo_create_text(&out,
		cText, cFont, cFile,
		C.int(p.Width), C.int(p.Height),
		C.int(p.DPI), C.int(p.Spacing),
		boolToC(p.RGBA),
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// ArrayJoinParams configures ArrayJoin.
type ArrayJoinParams struct {
	Across        int
	HSpacing      int
	VSpacing      int
	BgR, BgG, BgB float64
	BgA           float64
}

// ArrayJoin joins multiple images into a grid `across` columns wide.
func ArrayJoin(images []*Image, p ArrayJoinParams) (*Image, error) {
	if len(images) == 0 {
		return nil, lastError()
	}
	ptrs := make([]*C.VipsImage, len(images))
	for i, im := range images {
		ptrs[i] = im.ptr
	}
	var out *C.VipsImage
	rc := C.sharpgo_arrayjoin(&ptrs[0], C.int(len(images)), &out,
		C.int(p.Across), C.int(p.HSpacing), C.int(p.VSpacing),
		C.double(p.BgR), C.double(p.BgG), C.double(p.BgB), C.double(p.BgA),
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}
