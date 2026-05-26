//go:build cgo

package vips

/*
#include <vips/vips.h>
*/
import "C"

import (
	"errors"
	"strings"
)

// lastError reads and clears the libvips per-thread error buffer.
// Returns nil if the buffer is empty.
func lastError() error {
	buf := C.GoString(C.vips_error_buffer())
	C.vips_error_clear()
	msg := strings.TrimSpace(buf)
	if msg == "" {
		return nil
	}
	return errors.New(msg)
}

// loadError returns the libvips error after a failed load/op, falling back to a
// generic message when libvips reported failure WITHOUT setting the error
// buffer (some loaders reject malformed input silently). Used only on the
// failure path; it never returns nil, so callers can't mistake a failure for
// success and dereference a nil *Image.
func loadError() error {
	if e := lastError(); e != nil {
		return e
	}
	return errors.New("vips: operation failed")
}
