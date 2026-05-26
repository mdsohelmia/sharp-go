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
