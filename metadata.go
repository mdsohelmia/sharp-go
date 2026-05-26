package sharp

import (
	"context"
	"math"
	"runtime"

	"github.com/sohelmia/sharp-go/internal/vips"
)

// Metadata mirrors sharp.metadata().
type Metadata struct {
	Format        string  // "jpeg", "png", ...
	Width         int
	Height        int
	Space         string  // "srgb", "cmyk", "b-w", ...
	Channels      int
	Depth         string  // "uchar", "ushort", ...
	Density       float64 // DPI
	HasAlpha      bool
	HasProfile    bool    // ICC profile present
	Orientation   int     // EXIF orientation tag (0 if absent)
	Pages         int     // animated frame count (1 if non-animated)
	IsProgressive bool    // JPEG/PNG progressive/interlaced

	// Raw metadata blobs. Empty when absent.
	Exif []byte
	ICC  []byte
	XMP  []byte
	IPTC []byte
}

// Metadata reads header information without decoding pixel data.
func (im *Image) Metadata(ctx context.Context) (Metadata, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if im.err != nil {
		return Metadata{}, im.err
	}
	if err := ctx.Err(); err != nil {
		return Metadata{}, err
	}
	if err := vips.InitError(); err != nil {
		return Metadata{}, err
	}

	buf, err := readInput(im.in)
	if err != nil {
		return Metadata{}, err
	}

	v, err := vips.LoadBufferLazy(buf)
	if err != nil {
		return Metadata{}, err
	}

	md := Metadata{
		Format:   vips.FindLoader(buf),
		Width:    v.Width(),
		Height:   v.Height(),
		Channels: v.Bands(),
		Space:    v.Interpretation().String(),
		Depth:    v.BandFormat().String(),
		HasAlpha: v.HasAlpha(),
	}

	// libvips xres/yres are pixels per millimetre. Convert to DPI.
	xres := v.XRes()
	if xres > 0 {
		md.Density = math.Round(xres*25.4*100) / 100
	}

	md.Orientation = v.Orientation()
	if n, ok := v.NPages(); ok && n > 0 {
		md.Pages = n
	} else {
		md.Pages = 1
	}
	if n, ok := v.InterlacedFlag(); ok {
		md.IsProgressive = n != 0
	} else if n, ok := v.JPEGProgressive(); ok {
		md.IsProgressive = n != 0
	}

	// Read raw blobs. Presence of ICC blob implies HasProfile.
	if b, ok := v.ICCBlob(); ok {
		md.ICC = b
		md.HasProfile = true
	}
	if b, ok := v.ExifBlob(); ok {
		md.Exif = b
	}
	if b, ok := v.XMPBlob(); ok {
		md.XMP = b
	}
	if b, ok := v.IPTCBlob(); ok {
		md.IPTC = b
	}

	return md, nil
}
