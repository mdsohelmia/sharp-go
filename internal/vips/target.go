//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

import (
	"io"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Target wraps a libvips VipsTargetCustom whose write signal pushes bytes
// into a Go io.Writer.
type Target struct {
	id  int64
	ptr *C.VipsTargetCustom
}

var (
	targetIDSeq atomic.Int64
	targetMu    sync.RWMutex
	targets     = map[int64]*targetSink{}
)

type targetSink struct {
	w   io.Writer
	err error
}

// NewTarget registers w as the sink for a new libvips streaming target.
// Caller must Close() to release the target's libvips reference.
func NewTarget(w io.Writer) (*Target, error) {
	id := targetIDSeq.Add(1)
	targetMu.Lock()
	targets[id] = &targetSink{w: w}
	targetMu.Unlock()

	t := C.sharpgo_target_new(C.longlong(id))
	if t == nil {
		targetMu.Lock()
		delete(targets, id)
		targetMu.Unlock()
		return nil, lastError()
	}
	return &Target{id: id, ptr: t}, nil
}

// Err returns the first write error encountered (if any) since creation.
func (t *Target) Err() error {
	targetMu.RLock()
	defer targetMu.RUnlock()
	if s, ok := targets[t.id]; ok {
		return s.err
	}
	return nil
}

// Close unrefs the libvips target and deregisters the sink. Idempotent.
func (t *Target) Close() {
	if t.ptr != nil {
		C.sharpgo_target_unref(t.ptr)
		t.ptr = nil
	}
	targetMu.Lock()
	delete(targets, t.id)
	targetMu.Unlock()
}

// SaveJPEGTarget streams a JPEG-encoded image into target.
func SaveJPEGTarget(im *Image, t *Target, p JPEGParams) error {
	rc := C.sharpgo_jpegsave_target(im.ptr, t.ptr,
		C.int(p.Quality),
		boolToC(p.Progressive),
		boolToC(p.OptimiseCoding),
		boolToC(p.TrellisQuantisation),
		boolToC(p.OvershootDeringing),
		boolToC(p.OptimiseScans),
		C.int(p.QuantisationTable),
		boolToC(p.ChromaSubsampling444),
	)
	if rc != 0 {
		if e := t.Err(); e != nil {
			return e
		}
		return lastError()
	}
	return t.Err()
}

// SavePNGTarget streams a PNG-encoded image into target.
func SavePNGTarget(im *Image, t *Target, p PNGParams) error {
	rc := C.sharpgo_pngsave_target(im.ptr, t.ptr,
		C.int(p.Compression),
		boolToC(p.Progressive),
		boolToC(p.Palette),
		C.int(p.Quality),
		C.int(p.Effort),
		C.int(p.Bitdepth),
	)
	if rc != 0 {
		if e := t.Err(); e != nil {
			return e
		}
		return lastError()
	}
	return t.Err()
}

//export sharpgoTargetWriteTrampoline
func sharpgoTargetWriteTrampoline(id int64, buf unsafe.Pointer, length int64) int64 {
	if length <= 0 {
		return 0
	}
	targetMu.RLock()
	sink, ok := targets[id]
	targetMu.RUnlock()
	if !ok || sink.w == nil {
		return -1
	}
	if sink.err != nil {
		return -1
	}
	chunk := C.GoBytes(buf, C.int(length))
	n, err := sink.w.Write(chunk)
	if err != nil {
		targetMu.Lock()
		if s, ok := targets[id]; ok {
			s.err = err
		}
		targetMu.Unlock()
		return -1
	}
	return int64(n)
}
