//go:build cgo

package vips

/*
#include <vips/vips.h>
#include <glib-object.h>
#include "bridge.h"
*/
import "C"

import (
	"runtime"
	"unsafe"
)

// Image wraps a *VipsImage with Go-side lifetime management. A finalizer
// installed via runtime.AddCleanup releases the GObject reference once the
// Image becomes unreachable.
type Image struct {
	ptr *C.VipsImage
}

// wrap takes ownership of v (does not add a reference) and returns an Image
// that will unref v when garbage-collected.
func wrap(v *C.VipsImage) *Image {
	if v == nil {
		return nil
	}
	im := &Image{ptr: v}
	runtime.AddCleanup(im, unrefVipsImage, unsafe.Pointer(v))
	return im
}

func unrefVipsImage(p unsafe.Pointer) {
	if p == nil {
		return
	}
	C.g_object_unref(C.gpointer(p))
}

// Width returns the image width in pixels.
func (im *Image) Width() int { return int(C.vips_image_get_width(im.ptr)) }

// Height returns the image height in pixels.
func (im *Image) Height() int { return int(C.vips_image_get_height(im.ptr)) }

// Bands returns the channel count.
func (im *Image) Bands() int { return int(C.vips_image_get_bands(im.ptr)) }

// Kill flags the image as cancelled. Any in-flight operation downstream of
// this image will abort at the next libvips checkpoint.
func (im *Image) Kill() {
	C.sharpgo_image_kill(im.ptr)
}

// Ref returns a fresh handle wrapping the same underlying VipsImage with an
// incremented GObject refcount. Used by PreparedOverlay to hand the same
// decoded image to many composite calls cheaply — each call gets its own
// finalizer-managed wrapper and the libvips image is only freed when the
// last wrapper goes out of scope.
func (im *Image) Ref() *Image {
	if im == nil || im.ptr == nil {
		return nil
	}
	C.sharpgo_image_ref(im.ptr)
	return wrap(im.ptr)
}
