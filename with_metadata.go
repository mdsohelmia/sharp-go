package sharp

import (
	"github.com/mdsohelmia/sharp-go/internal/vips"
)

// WithMetadataOptions configures WithMetadata.
type WithMetadataOptions struct {
	// Orientation EXIF tag value (1-8). 0 leaves untouched.
	Orientation int
	// Density (DPI). 0 leaves untouched.
	Density float64
	// ICC named profile to attach ("srgb"|"p3"|"cmyk"). Empty leaves untouched.
	ICC string
}

// WithMetadata writes metadata fields on the output image. Mirrors sharp's
// withMetadata({orientation, density, icc}). Implicitly retains all metadata
// categories at encode (equivalent to KeepMetadata).
func (im *Image) WithMetadata(opts WithMetadataOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.withMetadata = &opts
	im.opts.keepFlags |= keepAll
	return im
}

// WithExifOptions configures WithExif. Sharp groups tags by IFD; we accept a
// flat map where keys are the full libvips tag name, e.g.
//
//	"exif-ifd0-Make"        -> "Canon"
//	"exif-ifd0-Software"    -> "sharp-go 1.0"
//	"exif-ifd2-PixelXDimension" -> "1920"
//
// The Keep flag for EXIF is set automatically.
type WithExifOptions struct {
	Tags map[string]string
}

// WithExif sets EXIF tags on the output image.
func (im *Image) WithExif(opts WithExifOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.withExif = &opts
	im.opts.keepFlags |= keepEXIF
	return im
}

// WithICCProfileOptions configures WithICCProfile.
type WithICCProfileOptions struct {
	// Profile is a built-in name ("srgb"|"p3"|"cmyk") or a path to an
	// .icc/.icm file.
	Profile string
	// Embed only — skip the pixel-space conversion. Default false performs
	// vips_icc_transform from the embedded (or "srgb" fallback) profile into
	// the named output profile, then attaches it.
	AttachOnly bool
}

// WithICCProfile applies and/or embeds an ICC profile.
func (im *Image) WithICCProfile(opts WithICCProfileOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.withICCProfile = &opts
	im.opts.keepFlags |= keepICC
	return im
}

// WithIccProfile is the camelCase-with-lowercase-cc alias matching sharp's
// JS naming.
func (im *Image) WithIccProfile(opts WithICCProfileOptions) *Image {
	return im.WithICCProfile(opts)
}

// WithXmpOptions configures WithXmp.
type WithXmpOptions struct {
	XmpPacket []byte
}

// WithXmp attaches an XMP metadata packet to the output image.
func (im *Image) WithXmp(opts WithXmpOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.withXmp = &opts
	im.opts.keepFlags |= keepXMP
	return im
}

// applyWithMetadata applies all the With* metadata recorders to vimg.
func applyWithMetadata(vimg *vips.Image, opts *pipelineOpts) (*vips.Image, error) {
	out := vimg

	if opts.withICCProfile != nil {
		o := opts.withICCProfile
		if !o.AttachOnly && o.Profile != "" {
			converted, err := vips.ICCTransform(out, o.Profile, "")
			if err == nil {
				out = converted
			}
		}
	}

	if opts.withMetadata != nil {
		m := opts.withMetadata
		if m.Orientation > 0 {
			out.SetInt("orientation", m.Orientation)
		}
		if m.Density > 0 {
			// libvips xres/yres are pixels per millimetre; sharp's density is DPI.
			ppm := m.Density / 25.4
			out.SetResolution(ppm, ppm)
		}
	}

	if opts.withExif != nil {
		for name, val := range opts.withExif.Tags {
			out.SetString(name, val)
		}
	}

	if opts.withXmp != nil && len(opts.withXmp.XmpPacket) > 0 {
		out.SetBlob("xmp-data", opts.withXmp.XmpPacket)
	}

	return out, nil
}
