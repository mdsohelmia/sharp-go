//go:build cgo

package vips

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import (
	"errors"
	"strings"
	"unsafe"
)

// FindLoader returns a short format name ("jpeg", "png", ...) for the given
// in-memory image, or an empty string if no loader matches.
func FindLoader(buf []byte) string {
	if len(buf) == 0 {
		return ""
	}
	cs := C.sharpgo_find_load_buffer(unsafe.Pointer(&buf[0]), C.size_t(len(buf)))
	if cs == nil {
		return ""
	}
	name := C.GoString(cs)
	// e.g. "VipsForeignLoadJpegBuffer" -> "jpeg"
	const prefix = "VipsForeignLoad"
	if !strings.HasPrefix(name, prefix) {
		return strings.ToLower(name)
	}
	short := strings.TrimSuffix(name[len(prefix):], "Buffer")
	short = strings.TrimSuffix(short, "Source")
	short = strings.TrimSuffix(short, "File")
	return strings.ToLower(short)
}

// LoadBufferLazy loads an image header without forcing a pixel decode. The
// caller must keep buf alive until the returned Image is unref'd (handled by
// AddCleanup). Use only for header/metadata reads.
func LoadBufferLazy(buf []byte) (*Image, error) {
	if len(buf) == 0 {
		return nil, errors.New("vips: empty input buffer")
	}
	var out *C.VipsImage
	rc := C.sharpgo_load_buffer_lazy(
		unsafe.Pointer(&buf[0]),
		C.size_t(len(buf)),
		&out,
	)
	if rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// HasAlpha reports whether the image has an alpha channel.
func (im *Image) HasAlpha() bool {
	return C.sharpgo_has_alpha(im.ptr) != 0
}

// Interpretation returns the libvips colourspace interpretation enum value.
func (im *Image) Interpretation() Interpretation {
	return Interpretation(C.sharpgo_get_interpretation(im.ptr))
}

// BandFormat returns the libvips band format enum value.
func (im *Image) BandFormat() BandFormat {
	return BandFormat(C.sharpgo_get_band_format(im.ptr))
}

// XRes returns horizontal resolution in pixels per millimetre.
func (im *Image) XRes() float64 { return float64(C.sharpgo_get_xres(im.ptr)) }

// YRes returns vertical resolution in pixels per millimetre.
func (im *Image) YRes() float64 { return float64(C.sharpgo_get_yres(im.ptr)) }

// GetInt reads an integer header field. Returns ok=false if absent.
//
// For the well-known metadata keys exposed by sharp's Metadata API, prefer
// the typed accessors below — they skip the per-call C.CString alloc by
// using process-lifetime interned C strings.
func (im *Image) GetInt(name string) (int, bool) {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	return im.getIntC(cs)
}

func (im *Image) getIntC(cs *C.char) (int, bool) {
	var v C.int
	if C.sharpgo_get_int(im.ptr, cs, &v) != 0 {
		return 0, false
	}
	return int(v), true
}

// GetBlob reads a blob metadata field (EXIF, ICC, XMP, IPTC). Returns
// ok=false if absent. The returned slice is a copy and safe to retain.
func (im *Image) GetBlob(name string) ([]byte, bool) {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	return im.getBlobC(cs)
}

func (im *Image) getBlobC(cs *C.char) ([]byte, bool) {
	var data unsafe.Pointer
	var size C.size_t
	if C.sharpgo_get_blob(im.ptr, cs, &data, &size) != 0 {
		return nil, false
	}
	if size == 0 || data == nil {
		return nil, false
	}
	return C.GoBytes(data, C.int(size)), true
}

// GetString reads a string header field. Returns ok=false if absent.
func (im *Image) GetString(name string) (string, bool) {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	var out *C.char
	if C.sharpgo_get_string(im.ptr, cs, &out) != 0 {
		return "", false
	}
	return C.GoString(out), true
}

// Interned-key fast accessors. These mirror what sharp.Image.Metadata pulls
// per request; the keys are static, so the C.CString allocation amortises
// to zero across the program's lifetime.

// Orientation returns the EXIF orientation tag, or 0 if absent.
func (im *Image) Orientation() int {
	v, _ := im.getIntC(cstrOrientation)
	return v
}

// NPages returns the n-pages metadata (animated frame count); 0 if absent.
func (im *Image) NPages() (int, bool) { return im.getIntC(cstrNPages) }

// InterlacedFlag returns the libvips "interlaced" hint (true when set,
// false when absent or zero).
func (im *Image) InterlacedFlag() (int, bool) { return im.getIntC(cstrInterlaced) }

// JPEGProgressive returns the libvips "jpeg-progressive" flag.
func (im *Image) JPEGProgressive() (int, bool) { return im.getIntC(cstrJPEGProgressive) }

// ICCBlob returns the embedded ICC profile bytes, or nil if absent.
func (im *Image) ICCBlob() ([]byte, bool) { return im.getBlobC(cstrICCProfileData) }

// ExifBlob returns the embedded EXIF blob, or nil if absent.
func (im *Image) ExifBlob() ([]byte, bool) { return im.getBlobC(cstrExifData) }

// XMPBlob returns the embedded XMP packet bytes, or nil if absent.
func (im *Image) XMPBlob() ([]byte, bool) { return im.getBlobC(cstrXmpData) }

// IPTCBlob returns the embedded IPTC blob, or nil if absent.
func (im *Image) IPTCBlob() ([]byte, bool) { return im.getBlobC(cstrIPTCData) }

// SetInt writes integer metadata.
func (im *Image) SetInt(name string, value int) {
	cs := C.CString(name)
	defer C.free(unsafe.Pointer(cs))
	C.sharpgo_set_int(im.ptr, cs, C.int(value))
}

// SetString writes string metadata.
func (im *Image) SetString(name, value string) {
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	cv := C.CString(value)
	defer C.free(unsafe.Pointer(cv))
	C.sharpgo_set_string(im.ptr, cn, cv)
}

// SetBlob writes a blob metadata field (libvips makes its own copy).
func (im *Image) SetBlob(name string, data []byte) {
	cn := C.CString(name)
	defer C.free(unsafe.Pointer(cn))
	if len(data) == 0 {
		C.sharpgo_set_blob(im.ptr, cn, nil, 0)
		return
	}
	C.sharpgo_set_blob(im.ptr, cn, unsafe.Pointer(&data[0]), C.size_t(len(data)))
}

// SetResolution writes xres/yres in pixels-per-millimetre.
func (im *Image) SetResolution(xres, yres float64) {
	C.sharpgo_set_resolution(im.ptr, C.double(xres), C.double(yres))
}

// SetICCProfileBlob embeds an ICC profile as metadata without converting
// pixels. Use ICCTransform if you need a colour-space conversion as well.
func (im *Image) SetICCProfileBlob(data []byte) {
	if len(data) == 0 {
		return
	}
	C.sharpgo_set_icc_profile_blob(im.ptr, unsafe.Pointer(&data[0]), C.size_t(len(data)))
}

// ICCTransform converts pixels into the named output profile. output may be
// a built-in name ("srgb"|"p3"|"cmyk") or a file path to a .icc/.icm.
// input is an optional fallback profile path when the image has none embedded.
//
// The "srgb" target — by far the common case — is served from a process-
// lifetime interned C string; other targets pay one C.CString per call.
func ICCTransform(im *Image, output, input string) (*Image, error) {
	var co *C.char
	if output == "srgb" {
		co = cstrSRGB
	} else {
		co = C.CString(output)
		defer C.free(unsafe.Pointer(co))
	}
	var ci *C.char
	if input == "srgb" {
		ci = cstrSRGB
	} else if input != "" {
		ci = C.CString(input)
		defer C.free(unsafe.Pointer(ci))
	}
	var out *C.VipsImage
	if rc := C.sharpgo_icc_transform(im.ptr, &out, co, ci); rc != 0 {
		return nil, lastError()
	}
	return wrap(out), nil
}

// Interpretation enumerates libvips' colourspace interpretation values.
type Interpretation int

const (
	InterpretationMultiband Interpretation = C.VIPS_INTERPRETATION_MULTIBAND
	InterpretationBW        Interpretation = C.VIPS_INTERPRETATION_B_W
	InterpretationSRGB      Interpretation = C.VIPS_INTERPRETATION_sRGB
	InterpretationRGB16     Interpretation = C.VIPS_INTERPRETATION_RGB16
	InterpretationGrey16    Interpretation = C.VIPS_INTERPRETATION_GREY16
	InterpretationCMYK      Interpretation = C.VIPS_INTERPRETATION_CMYK
	InterpretationLAB       Interpretation = C.VIPS_INTERPRETATION_LAB
	InterpretationXYZ       Interpretation = C.VIPS_INTERPRETATION_XYZ
)

// String returns sharp's name for the interpretation (e.g. "srgb", "cmyk").
func (i Interpretation) String() string {
	switch i {
	case InterpretationMultiband:
		return "multiband"
	case InterpretationBW:
		return "b-w"
	case InterpretationSRGB:
		return "srgb"
	case InterpretationRGB16:
		return "rgb16"
	case InterpretationGrey16:
		return "grey16"
	case InterpretationCMYK:
		return "cmyk"
	case InterpretationLAB:
		return "lab"
	case InterpretationXYZ:
		return "xyz"
	default:
		return "unknown"
	}
}

// BandFormat enumerates libvips' per-band storage formats.
type BandFormat int

const (
	BandFormatUchar  BandFormat = C.VIPS_FORMAT_UCHAR
	BandFormatChar   BandFormat = C.VIPS_FORMAT_CHAR
	BandFormatUshort BandFormat = C.VIPS_FORMAT_USHORT
	BandFormatShort  BandFormat = C.VIPS_FORMAT_SHORT
	BandFormatUint   BandFormat = C.VIPS_FORMAT_UINT
	BandFormatInt    BandFormat = C.VIPS_FORMAT_INT
	BandFormatFloat  BandFormat = C.VIPS_FORMAT_FLOAT
	BandFormatDouble BandFormat = C.VIPS_FORMAT_DOUBLE
)

// String returns sharp's name for the band format ("uchar", "ushort", ...).
func (b BandFormat) String() string {
	switch b {
	case BandFormatUchar:
		return "uchar"
	case BandFormatChar:
		return "char"
	case BandFormatUshort:
		return "ushort"
	case BandFormatShort:
		return "short"
	case BandFormatUint:
		return "uint"
	case BandFormatInt:
		return "int"
	case BandFormatFloat:
		return "float"
	case BandFormatDouble:
		return "double"
	default:
		return "unknown"
	}
}
