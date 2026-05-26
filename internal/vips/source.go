//go:build cgo

package vips

/*
#include <stdint.h>
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"io"
	"runtime/cgo"
	"sync"
	"unsafe"
)

// Source wraps a libvips VipsSource subclass whose read/seek vfuncs pull bytes
// from a Go io.Reader (and io.Seeker when the wrapped reader supports it).
//
// The Go reader is referenced through a runtime/cgo.Handle stored on the C
// object; the handle is released in the GObject dispose handler. As a result
// the reader (and anything it retains, e.g. a backing []byte) lives exactly as
// long as the libvips source — which a loaded image keeps referenced for the
// lifetime of its (possibly lazy, shrink-on-load) pipeline. No global table or
// per-call mutex is needed for liveness; the only lock guards the read error.
type Source struct {
	h   cgo.Handle
	sr  *sourceReader
	ptr *C.VipsSource
}

// sourceReader is the value stored behind the cgo.Handle. It is referenced by
// both the handle table (for the C callbacks) and the Source (for Err()).
type sourceReader struct {
	r io.Reader
	s io.Seeker // optional; nil for non-seekable streams

	mu  sync.Mutex
	err error
}

func (sr *sourceReader) setErr(e error) {
	sr.mu.Lock()
	if sr.err == nil {
		sr.err = e
	}
	sr.mu.Unlock()
}

func (sr *sourceReader) getErr() error {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	return sr.err
}

// NewSource registers r as a streaming input. If r implements io.Seeker the
// seek vfunc is wired up too, enabling formats whose decoder rewinds the
// stream (TIFF, multi-page HEIF). The returned Source owns one reference (the
// creation reference); Close drops it. A loaded image takes its own reference,
// so it is safe — and expected — to Close right after the load call.
func NewSource(r io.Reader) (*Source, error) {
	if r == nil {
		return nil, errors.New("vips: nil reader")
	}
	sr := &sourceReader{r: r}
	if s, ok := r.(io.Seeker); ok {
		sr.s = s
	}
	h := cgo.NewHandle(sr)
	ptr := C.sharpgo_source_new(C.uintptr_t(h))
	if ptr == nil {
		h.Delete()
		return nil, loadError()
	}
	return &Source{h: h, sr: sr, ptr: ptr}, nil
}

// Err returns the first read/seek error encountered, if any.
func (s *Source) Err() error {
	if s.sr == nil {
		return nil
	}
	return s.sr.getErr()
}

// Close drops the creation reference on the libvips source. Idempotent. The
// cgo.Handle is NOT deleted here — that happens in the C dispose handler when
// the last reference (held by the image's pipeline) is dropped, ensuring the
// Go reader outlives any lazy reads.
func (s *Source) Close() {
	if s.ptr != nil {
		C.sharpgo_source_unref(s.ptr)
		s.ptr = nil
	}
}

// LoadSource decodes an image from a streaming source. Pixel data is copied
// into libvips-managed memory before return, so the source's creation
// reference may be dropped (Close) immediately afterwards.
func LoadSource(src *Source) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_load_source(src.ptr, &out)
	if rc != 0 {
		if e := src.Err(); e != nil {
			return nil, e
		}
		return nil, loadError()
	}
	return wrap(out), nil
}

// ThumbnailSource fuses load + resize into vips_thumbnail_source, activating
// format-native shrink-on-load while pulling bytes from src. The result is a
// lazy pipeline; src must outlive it, which is guaranteed because the returned
// image holds a reference to src (the caller may safely Close its own).
func ThumbnailSource(src *Source, p ThumbnailParams) (*Image, error) {
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
	rc := C.sharpgo_thumbnail_source(
		src.ptr,
		C.int(p.Width), C.int(p.Height),
		C.int(p.Size), C.int(p.Crop), boolToC(p.NoRotate),
		cImport, cExport, C.int(p.Intent),
		&out,
	)
	if rc != 0 {
		if e := src.Err(); e != nil {
			return nil, e
		}
		return nil, loadError()
	}
	return wrap(out), nil
}

//export sharpgoSourceRead
func sharpgoSourceRead(handle C.uintptr_t, buf unsafe.Pointer, length C.longlong) C.longlong {
	if length <= 0 {
		return 0
	}
	sr, ok := cgo.Handle(handle).Value().(*sourceReader)
	if !ok || sr.r == nil {
		return -1
	}
	// Read directly into the C buffer to avoid an intermediate copy.
	dst := unsafe.Slice((*byte)(buf), int(length))
	n, err := sr.r.Read(dst)
	if n > 0 {
		return C.longlong(n)
	}
	if errors.Is(err, io.EOF) {
		return 0
	}
	if err != nil {
		sr.setErr(err)
		return -1
	}
	return 0
}

//export sharpgoSourceSeek
func sharpgoSourceSeek(handle C.uintptr_t, offset C.longlong, whence C.int) C.longlong {
	sr, ok := cgo.Handle(handle).Value().(*sourceReader)
	if !ok || sr.s == nil {
		return -1
	}
	pos, err := sr.s.Seek(int64(offset), int(whence))
	if err != nil {
		sr.setErr(err)
		return -1
	}
	return C.longlong(pos)
}

//export sharpgoSourceClose
func sharpgoSourceClose(handle C.uintptr_t) {
	cgo.Handle(handle).Delete()
}
