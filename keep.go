package sharp

import (
	"github.com/sohelmia/sharp-go/internal/vips"
)

// Keep flag values for use with WithKeep and the dedicated Keep* methods.
const (
	keepEXIF  = int(vips.KeepEXIF)
	keepXMP   = int(vips.KeepXMP)
	keepIPTC  = int(vips.KeepIPTC)
	keepICC   = int(vips.KeepICC)
	keepOther = int(vips.KeepOther)
	keepAll   = int(vips.KeepAll)
)

// KeepMetadata retains all metadata categories (EXIF, XMP, IPTC, ICC, other)
// through encoding. Mirrors sharp's keepMetadata().
func (im *Image) KeepMetadata() *Image {
	if im.err != nil {
		return im
	}
	im.opts.keepFlags |= keepAll
	return im
}

// KeepExif retains EXIF metadata through encoding.
func (im *Image) KeepExif() *Image {
	if im.err != nil {
		return im
	}
	im.opts.keepFlags |= keepEXIF
	return im
}

// KeepXmp retains XMP metadata through encoding.
func (im *Image) KeepXmp() *Image {
	if im.err != nil {
		return im
	}
	im.opts.keepFlags |= keepXMP
	return im
}

// KeepIptc retains IPTC metadata through encoding.
func (im *Image) KeepIptc() *Image {
	if im.err != nil {
		return im
	}
	im.opts.keepFlags |= keepIPTC
	return im
}

// KeepIccProfile retains the ICC colour profile through encoding.
func (im *Image) KeepIccProfile() *Image {
	if im.err != nil {
		return im
	}
	im.opts.keepFlags |= keepICC
	return im
}
