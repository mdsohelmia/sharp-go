//go:build cgo

package vips

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Source wraps a libvips VipsSourceCustom whose read/seek signals pull bytes
// from a Go io.Reader (and io.Seeker when the wrapped reader supports it).
type Source struct {
	id  int64
	ptr *C.VipsSourceCustom
}

var (
	sourceIDSeq atomic.Int64
	sourceMu    sync.RWMutex
	sources     = map[int64]*sourceSink{}
)

type sourceSink struct {
	r   io.Reader
	s   io.Seeker // optional
	err error
}

// NewSource registers r as a streaming input. If r implements io.Seeker the
// seek signal is wired up too, enabling formats whose decoder rewinds the
// stream (TIFF, multi-page HEIF). Caller must Close() to drop the libvips
// reference.
func NewSource(r io.Reader) (*Source, error) {
	if r == nil {
		return nil, errors.New("vips: nil reader")
	}
	id := sourceIDSeq.Add(1)
	sink := &sourceSink{r: r}
	seekable := 0
	if s, ok := r.(io.Seeker); ok {
		sink.s = s
		seekable = 1
	}
	sourceMu.Lock()
	sources[id] = sink
	sourceMu.Unlock()

	ptr := C.sharpgo_source_new(C.longlong(id), C.int(seekable))
	if ptr == nil {
		sourceMu.Lock()
		delete(sources, id)
		sourceMu.Unlock()
		return nil, lastError()
	}
	return &Source{id: id, ptr: ptr}, nil
}

// Err returns the first read error encountered, if any.
func (s *Source) Err() error {
	sourceMu.RLock()
	defer sourceMu.RUnlock()
	if sink, ok := sources[s.id]; ok {
		return sink.err
	}
	return nil
}

// Close unrefs the libvips source and deregisters the sink. Idempotent.
func (s *Source) Close() {
	if s.ptr != nil {
		C.sharpgo_source_unref(s.ptr)
		s.ptr = nil
	}
	sourceMu.Lock()
	delete(sources, s.id)
	sourceMu.Unlock()
}

// LoadSource decodes an image from a streaming source. Pixel data is copied
// to libvips-managed memory before return so the underlying Go reader may be
// closed afterwards.
func LoadSource(src *Source) (*Image, error) {
	var out *C.VipsImage
	rc := C.sharpgo_load_source(src.ptr, &out)
	if rc != 0 {
		if e := src.Err(); e != nil {
			return nil, e
		}
		return nil, lastError()
	}
	return wrap(out), nil
}

// ThumbnailSource fuses load + resize into vips_thumbnail_source, activating
// format-native shrink-on-load while pulling bytes from src.
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
		return nil, lastError()
	}
	return wrap(out), nil
}

//export sharpgoSourceReadTrampoline
func sharpgoSourceReadTrampoline(id int64, buf unsafe.Pointer, length int64) int64 {
	if length <= 0 {
		return 0
	}
	sourceMu.RLock()
	sink, ok := sources[id]
	sourceMu.RUnlock()
	if !ok || sink.r == nil {
		return -1
	}
	if sink.err != nil {
		return -1
	}
	// Read directly into the C buffer to avoid an intermediate copy.
	dst := unsafe.Slice((*byte)(buf), int(length))
	n, err := sink.r.Read(dst)
	if n > 0 {
		return int64(n)
	}
	if errors.Is(err, io.EOF) {
		return 0
	}
	if err != nil {
		sourceMu.Lock()
		if s, ok := sources[id]; ok {
			s.err = err
		}
		sourceMu.Unlock()
		return -1
	}
	return 0
}

//export sharpgoSourceSeekTrampoline
func sharpgoSourceSeekTrampoline(id int64, offset int64, whence int32) int64 {
	sourceMu.RLock()
	sink, ok := sources[id]
	sourceMu.RUnlock()
	if !ok || sink.s == nil {
		return -1
	}
	pos, err := sink.s.Seek(offset, int(whence))
	if err != nil {
		sourceMu.Lock()
		if s, ok := sources[id]; ok {
			s.err = err
		}
		sourceMu.Unlock()
		return -1
	}
	return pos
}
