//go:build cgo

package vips

/*
#include <stdlib.h>
#include <string.h>
#include <glib.h>
#include "bridge.h"
*/
import "C"

import (
	"sync"
	"unsafe"
)

// encBufPool recycles the Go-side []byte that holds an encoded image. The
// libvips encoder writes into a g_malloc'd buffer; the Go wrapper memcpys
// out so the libvips memory can be released immediately. The pool is
// populated lazily by ReleaseEncBuf — callers that opt out simply allocate
// fresh slices, matching the prior C.GoBytes cost profile. Callers that
// recycle via sharp.Release pay zero allocations on the encode hot path.
var encBufPool sync.Pool // no New: empty pool yields a nil Get

// acquireEncBuf returns a slice sized to n bytes, reusing pooled capacity
// when something Released has the room. The returned slice is uninitialised
// — callers must write to all n bytes.
func acquireEncBuf(n int) []byte {
	if v := encBufPool.Get(); v != nil {
		if p, ok := v.(*[]byte); ok && p != nil && cap(*p) >= n {
			return (*p)[:n]
		}
	}
	return make([]byte, n)
}

// ReleaseEncBuf returns a slice obtained from SaveJPEG/SavePNG/etc. to the
// pool for reuse. Safe with nil or non-pooled slices. Callers must not use
// the slice after release.
func ReleaseEncBuf(b []byte) {
	if cap(b) == 0 {
		return
	}
	b = b[:0]
	encBufPool.Put(&b)
}

// copyToPooledBuf draws a slice from the encoder pool, memcpys size bytes
// from cBuf into it, and returns the slice. Replaces C.GoBytes on the
// encoder hot path to avoid per-encode heap allocation.
func copyToPooledBuf(cBuf unsafe.Pointer, size C.size_t) []byte {
	out := acquireEncBuf(int(size))
	if size > 0 {
		C.memcpy(unsafe.Pointer(&out[0]), cBuf, size)
	}
	return out
}

// KeepFlags is a bitset of metadata categories to retain through encoding.
// Zero strips everything (sharp default). The values are libvips VipsForeignKeep
// flags.
type KeepFlags int

const (
	KeepNone  KeepFlags = 0
	KeepEXIF  KeepFlags = 1
	KeepXMP   KeepFlags = 2
	KeepIPTC  KeepFlags = 4
	KeepICC   KeepFlags = 8
	KeepOther KeepFlags = 16
	KeepAll   KeepFlags = 31
)

// ApplyKeep strips metadata fields from im that are not retained by flags.
// Call immediately before SaveXxx; the modification is in-place.
func ApplyKeep(im *Image, flags KeepFlags) {
	C.sharpgo_apply_keep(im.ptr, C.int(flags))
}

// JPEGParams mirrors the subset of vips_jpegsave_buffer options that sharp
// exposes. Fields use libvips defaults when zero unless documented otherwise.
type JPEGParams struct {
	Quality              int  // 1-100; default 80
	Progressive          bool // interlace
	OptimiseCoding       bool // default true
	TrellisQuantisation  bool
	OvershootDeringing   bool
	OptimiseScans        bool
	QuantisationTable    int  // 0-8
	ChromaSubsampling444 bool // false = 4:2:0 (libvips auto)
}

// PNGParams mirrors the subset of vips_pngsave_buffer options that sharp
// exposes.
type PNGParams struct {
	Compression int  // 0-9; default 6
	Progressive bool // interlace
	Palette     bool
	Quality     int  // 1-100, palette only; default 100
	Effort      int  // 1-10; default 7
	Bitdepth    int  // 0=auto, 1/2/4/8/16
}

// WebPParams mirrors the subset of vips_webpsave_buffer options that sharp
// exposes. See WebPSharpYUVParams for the libwebp-direct encoder that exposes
// use_sharp_yuv, autofilter, sns_strength, target_psnr, and segments —
// knobs libvips's webpsave wrapper hides.
type WebPParams struct {
	Quality        int  // 1-100; default 80
	AlphaQuality   int  // 0-100; default 100
	Lossless       bool
	NearLossless   bool
	SmartSubsample bool
	SmartDeblock   bool
	Passes         int  // 1-10; 0 = libwebp default
	Preset         WebPPreset
	Effort         int  // 0-6; default 4
	Loop           int  // animated; 0=infinite
	MinSize        bool
	Mixed          bool
}

// WebPSharpYUVParams configures the libwebp-direct encoder (SaveWebPSharpYUV).
// All zero-valued fields fall through to libwebp's internal defaults so a
// minimal {Quality, Effort, UseSharpYUV} call works.
type WebPSharpYUVParams struct {
	Quality      int        // 1-100
	Effort       int        // 0-6 (libwebp "method")
	UseSharpYUV  bool       // sharper RGB→YUV conversion (libvips can't set)
	AutoFilter   bool       // auto-tune deblocking filter
	SNSStrength  int        // 0-100; 0 leaves libwebp default (50)
	TargetSize   int        // exact byte budget; 0 = ignore
	TargetPSNR   float32    // dB; 0 = ignore (target_size wins if both set)
	Passes       int        // 1-10; 0 = libwebp default
	Preset       WebPPreset // libwebp WebPPreset enum
	Segments     int        // 1-4; 0 = libwebp default (4)
}

// WebPPreset names libwebp's content-type presets. Values match
// VipsForeignWebpPreset.
type WebPPreset int

const (
	WebPPresetDefault WebPPreset = iota
	WebPPresetPicture
	WebPPresetPhoto
	WebPPresetDrawing
	WebPPresetIcon
	WebPPresetText
)

// GIFParams mirrors the subset of vips_gifsave_buffer options that sharp
// exposes.
type GIFParams struct {
	Dither              float64 // 0.0-1.0
	Effort              int     // 1-10; default 7
	Bitdepth            int     // 1-8; default 8
	InterframeMaxError  int     // 0-32
	InterpaletteMaxError int    // 0-256; default 3
	Interlace           bool
	Reuse               bool
	KeepDuplicateFrames bool
}

// SaveJPEG encodes the image as JPEG and returns the bytes.
func SaveJPEG(im *Image, p JPEGParams) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_jpegsave_buffer(
		im.ptr,
		&buf, &size,
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
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return copyToPooledBuf(buf, size), nil
}

// SavePNG encodes the image as PNG and returns the bytes.
func SavePNG(im *Image, p PNGParams) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_pngsave_buffer(
		im.ptr,
		&buf, &size,
		C.int(p.Compression),
		boolToC(p.Progressive),
		boolToC(p.Palette),
		C.int(p.Quality),
		C.int(p.Effort),
		C.int(p.Bitdepth),
	)
	if rc != 0 {
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return copyToPooledBuf(buf, size), nil
}

// SaveWebPSharpYUV encodes via libwebp directly (bypasses libvips's
// webpsave). Used to access WebPConfig fields libvips doesn't expose:
// use_sharp_yuv, autofilter, sns_strength, target_psnr, segments.
func SaveWebPSharpYUV(im *Image, p WebPSharpYUVParams) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_webpsave_sharp_yuv(
		im.ptr,
		&buf, &size,
		C.int(p.Quality),
		C.int(p.Effort),
		boolToC(p.UseSharpYUV),
		boolToC(p.AutoFilter),
		C.int(p.SNSStrength),
		C.int(p.TargetSize),
		C.float(p.TargetPSNR),
		C.int(p.Passes),
		C.int(p.Preset),
		C.int(p.Segments),
	)
	if rc != 0 {
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return C.GoBytes(buf, C.int(size)), nil
}

// SaveWebP encodes the image as WebP and returns the bytes.
func SaveWebP(im *Image, p WebPParams) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_webpsave_buffer(
		im.ptr,
		&buf, &size,
		C.int(p.Quality),
		C.int(p.AlphaQuality),
		boolToC(p.Lossless),
		boolToC(p.NearLossless),
		boolToC(p.SmartSubsample),
		C.int(p.Effort),
		C.int(p.Loop),
		boolToC(p.MinSize),
		boolToC(p.Mixed),
		boolToC(p.SmartDeblock),
		C.int(p.Passes),
		C.int(p.Preset),
	)
	if rc != 0 {
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return copyToPooledBuf(buf, size), nil
}

// TIFFParams mirrors the subset of vips_tiffsave_buffer options that sharp
// exposes.
type TIFFParams struct {
	Compression int  // VipsForeignTiffCompression enum
	Quality     int  // JPEG-in-TIFF Q
	Predictor   int  // VipsForeignTiffPredictor enum
	Tile        bool
	TileWidth   int
	TileHeight  int
	Pyramid     bool
	Bitdepth    int
	BigTIFF     bool
}

// HEIFParams mirrors the subset of vips_heifsave_buffer options that sharp
// exposes.
type HEIFParams struct {
	Compression       int  // VipsForeignHeifCompression (HEVC/AVC/JPEG/AV1)
	Quality           int
	Lossless          bool
	Effort            int
	Bitdepth          int
	ChromaSubsample444 bool
}

// SaveTIFF encodes the image as TIFF and returns the bytes.
func SaveTIFF(im *Image, p TIFFParams) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_tiffsave_buffer(
		im.ptr,
		&buf, &size,
		C.int(p.Compression),
		C.int(p.Quality),
		C.int(p.Predictor),
		boolToC(p.Tile),
		C.int(p.TileWidth),
		C.int(p.TileHeight),
		boolToC(p.Pyramid),
		C.int(p.Bitdepth),
		boolToC(p.BigTIFF),
	)
	if rc != 0 {
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return copyToPooledBuf(buf, size), nil
}

// SaveHEIF encodes the image as HEIF/AVIF/HEVC and returns the bytes.
func SaveHEIF(im *Image, p HEIFParams) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_heifsave_buffer(
		im.ptr,
		&buf, &size,
		C.int(p.Compression),
		C.int(p.Quality),
		boolToC(p.Lossless),
		C.int(p.Effort),
		C.int(p.Bitdepth),
		boolToC(p.ChromaSubsample444),
	)
	if rc != 0 {
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return copyToPooledBuf(buf, size), nil
}

// HEIFCompressionHEVC etc enumerate libvips' VipsForeignHeifCompression.
const (
	HEIFCompressionHEVC = 1
	HEIFCompressionAVC  = 2
	HEIFCompressionJPEG = 3
	HEIFCompressionAV1  = 4
)

// TIFFCompressionNone etc enumerate libvips' VipsForeignTiffCompression.
const (
	TIFFCompressionNone     = 0
	TIFFCompressionJPEG     = 1
	TIFFCompressionDeflate  = 2
	TIFFCompressionPackbits = 3
	TIFFCompressionCCITTFAX4 = 4
	TIFFCompressionLZW      = 5
	TIFFCompressionWebP     = 6
	TIFFCompressionZSTD     = 7
	TIFFCompressionJP2K     = 8
)

// JXLParams mirrors the subset of vips_jxlsave_buffer options that sharp
// exposes.
type JXLParams struct {
	Quality  int     // 0-100; default 75
	Tier     int     // 0-4
	Distance float64 // 0-25; lower = better
	Effort   int     // 1-10; default 7
	Lossless bool
	Bitdepth int // 1-16; default 8
}

// JP2Params mirrors the subset of vips_jp2ksave_buffer options that sharp
// exposes.
type JP2Params struct {
	Quality            int  // 1-100; default 48
	Lossless           bool
	TileWidth          int  // default 512
	TileHeight         int  // default 512
	ChromaSubsample444 bool
}

// SaveJXL encodes the image as JPEG XL.
func SaveJXL(im *Image, p JXLParams) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_jxlsave_buffer(
		im.ptr, &buf, &size,
		C.int(p.Quality),
		C.int(p.Tier),
		C.double(p.Distance),
		C.int(p.Effort),
		boolToC(p.Lossless),
		C.int(p.Bitdepth),
	)
	if rc != 0 {
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return copyToPooledBuf(buf, size), nil
}

// SaveJP2 encodes the image as JPEG 2000.
func SaveJP2(im *Image, p JP2Params) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_jp2ksave_buffer(
		im.ptr, &buf, &size,
		C.int(p.Quality),
		boolToC(p.Lossless),
		C.int(p.TileWidth),
		C.int(p.TileHeight),
		boolToC(p.ChromaSubsample444),
	)
	if rc != 0 {
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return copyToPooledBuf(buf, size), nil
}

// SaveRaw returns the raw pixel bytes, casting to the given band format.
func SaveRaw(im *Image, format BandFormat) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_rawsave_buffer(im.ptr, C.int(format), &buf, &size)
	if rc != 0 {
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return copyToPooledBuf(buf, size), nil
}

// SaveGIF encodes the image as GIF and returns the bytes.
func SaveGIF(im *Image, p GIFParams) ([]byte, error) {
	var buf unsafe.Pointer
	var size C.size_t
	rc := C.sharpgo_gifsave_buffer(
		im.ptr,
		&buf, &size,
		C.double(p.Dither),
		C.int(p.Effort),
		C.int(p.Bitdepth),
		C.int(p.InterframeMaxError),
		C.int(p.InterpaletteMaxError),
		boolToC(p.Interlace),
		boolToC(p.Reuse),
		boolToC(p.KeepDuplicateFrames),
	)
	if rc != 0 {
		return nil, lastError()
	}
	defer C.g_free(C.gpointer(buf))
	return copyToPooledBuf(buf, size), nil
}

func boolToC(b bool) C.int {
	if b {
		return 1
	}
	return 0
}
