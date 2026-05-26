package sharp

import (
	"fmt"
	"strings"

	"github.com/sohelmia/sharp-go/format"
)

// FormatID is a string identifier for ToFormat. Matches sharp's accepted
// names ("jpeg", "jpg", "png", "webp", "avif", "gif", "tiff", "heif",
// "jxl", "jp2", "raw").
type FormatID string

const (
	FormatJPEG FormatID = "jpeg"
	FormatPNG  FormatID = "png"
	FormatWebP FormatID = "webp"
	FormatGIF  FormatID = "gif"
	FormatTIFF FormatID = "tiff"
	FormatHEIF FormatID = "heif"
	FormatAVIF FormatID = "avif"
	FormatJXL  FormatID = "jxl"
	FormatJP2  FormatID = "jp2"
	FormatRaw  FormatID = "raw"
)

// ToFormat dispatches to the format method named by id. opts may be the
// matching format options struct (format.JPEGOptions, format.PNGOptions, ...)
// or nil for defaults. Returns the image with a sticky error if opts has the
// wrong type for the format.
func (im *Image) ToFormat(id FormatID, opts any) *Image {
	if im.err != nil {
		return im
	}
	switch strings.ToLower(string(id)) {
	case "jpeg", "jpg":
		return im.JPEG(coerce[format.JPEGOptions](opts, im))
	case "png":
		return im.PNG(coerce[format.PNGOptions](opts, im))
	case "webp":
		return im.WebP(coerce[format.WebPOptions](opts, im))
	case "gif":
		return im.GIF(coerce[format.GIFOptions](opts, im))
	case "tiff", "tif":
		return im.TIFF(coerce[format.TIFFOptions](opts, im))
	case "heif", "heic":
		return im.HEIF(coerce[format.HEIFOptions](opts, im))
	case "avif":
		return im.AVIF(coerce[format.AVIFOptions](opts, im))
	case "jxl":
		return im.JXL(coerce[format.JXLOptions](opts, im))
	case "jp2", "j2k", "jpx", "jpf":
		return im.JP2(coerce[format.JP2Options](opts, im))
	case "raw":
		return im.Raw(coerce[format.RawOptions](opts, im))
	default:
		im.stickyErr(fmt.Errorf("sharp: unsupported output format %q", id))
		return im
	}
}

func coerce[T any](opts any, im *Image) T {
	var zero T
	if opts == nil {
		return zero
	}
	if v, ok := opts.(T); ok {
		return v
	}
	if pv, ok := opts.(*T); ok && pv != nil {
		return *pv
	}
	im.stickyErr(fmt.Errorf("sharp: ToFormat opts has wrong type %T, want %T", opts, zero))
	return zero
}
