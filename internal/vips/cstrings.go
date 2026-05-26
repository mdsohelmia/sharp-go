//go:build cgo

package vips

/*
#include <stdlib.h>
*/
import "C"

// Process-lifetime CString cache for libvips metadata keys that Get/Set
// callers reach for repeatedly. Allocated once at init; never freed. Eliminates
// the per-call C.CString + C.free round-trip that previously dominated the
// metadata-only hot path (sharp.Image.Metadata reads 7–9 keys per call).
var (
	cstrOrientation     = C.CString("orientation")
	cstrNPages          = C.CString("n-pages")
	cstrInterlaced      = C.CString("interlaced")
	cstrJPEGProgressive = C.CString("jpeg-progressive")
	cstrICCProfileData  = C.CString("icc-profile-data")
	cstrExifData        = C.CString("exif-data")
	cstrXmpData         = C.CString("xmp-data")
	cstrIPTCData        = C.CString("iptc-data")
	cstrLoop            = C.CString("loop")
	cstrPageHeight      = C.CString("page-height")
	cstrSRGB            = C.CString("srgb")
)
