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

// Image wraps a *VipsImage with Go-side lifetime management. Close drops the
// GObject reference deterministically; a runtime.AddCleanup finalizer is kept
// as a backstop for handles that are never explicitly closed (e.g. error
// paths) so the reference is still released once the Image becomes unreachable.
type Image struct {
	ptr     *C.VipsImage
	cleanup runtime.Cleanup
}

// wrap takes ownership of v (does not add a reference) and returns an Image
// that unrefs v on Close, or when garbage-collected if Close is never called.
func wrap(v *C.VipsImage) *Image {
	if v == nil {
		return nil
	}
	im := &Image{ptr: v}
	im.cleanup = runtime.AddCleanup(im, unrefVipsImage, unsafe.Pointer(v))
	return im
}

func unrefVipsImage(p unsafe.Pointer) {
	if p == nil {
		return
	}
	C.g_object_unref(C.gpointer(p))
}

// Close drops this handle's GObject reference now, instead of waiting for the
// GC to run the finalizer. Idempotent and safe on a nil *Image.
//
// This is the govips model — explicit, deterministic freeing — and it is what
// bounds RSS under server load. libvips images live in C memory the Go GC
// cannot see, so with a finalizer-only lifecycle dozens of finished pipelines'
// worth of C memory pile up between (rare, Go-heap-paced) GC cycles. Closing at
// end-of-pipeline returns that memory immediately.
//
// libvips refcounts an op's output against its input, so closing an image the
// moment its successor exists is safe: the underlying VipsImage survives
// (refcount ≥ 1) until the final image is closed and the lazy pipeline unwinds.
// After Close the handle must not be used again — its methods would dereference
// a freed pointer — so a caller frees an image only once its successor (or, for
// the final image, the encode/sink) no longer reads through it.
func (im *Image) Close() {
	if im == nil || im.ptr == nil {
		return
	}
	im.cleanup.Stop()
	C.g_object_unref(C.gpointer(im.ptr))
	im.ptr = nil
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
