package sharp

import (
	"github.com/sohelmia/sharp-go/internal/vips"
)

// applyJoinChannel decodes each input and band-joins them with the current
// pipeline image.
func applyJoinChannel(base *vips.Image, opts *JoinChannelOptions) (*vips.Image, error) {
	if len(opts.Inputs) == 0 {
		return base, nil
	}
	all := make([]*vips.Image, 0, len(opts.Inputs)+1)
	all = append(all, base)
	for _, b := range opts.Inputs {
		img, err := vips.LoadBuffer(b)
		if err != nil {
			return nil, err
		}
		all = append(all, img)
	}
	return vips.Bandjoin(all)
}

// RemoveAlpha drops the alpha channel if present.
func (im *Image) RemoveAlpha() *Image {
	if im.err != nil {
		return im
	}
	im.opts.removeAlpha = true
	return im
}

// EnsureAlphaOptions configures EnsureAlpha.
type EnsureAlphaOptions struct {
	// Alpha value to use when adding a new alpha channel. 0-1 normalised.
	// Default 1 (fully opaque).
	Alpha float64
}

// EnsureAlpha adds an alpha channel if one is not already present.
func (im *Image) EnsureAlpha(opts EnsureAlphaOptions) *Image {
	if im.err != nil {
		return im
	}
	if opts.Alpha == 0 {
		opts.Alpha = 1
	}
	im.opts.ensureAlpha = &opts
	return im
}

// ExtractChannel selects a single band (0-based). Subsequent operations
// operate on the single-channel result.
func (im *Image) ExtractChannel(band int) *Image {
	if im.err != nil {
		return im
	}
	b := band
	im.opts.extractChannel = &b
	return im
}

// JoinChannelOptions configures JoinChannel.
type JoinChannelOptions struct {
	// Inputs are extra single-channel images to bandjoin with the pipeline's
	// current image. Each must match the current width/height. Supplied as
	// raw bytes (decoded as a normal image; only its first band is used if
	// it has multiple).
	Inputs [][]byte
}

// JoinChannel appends one or more channels.
func (im *Image) JoinChannel(opts JoinChannelOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.joinChannel = &opts
	return im
}

// BandboolOptions configures Bandbool.
type BandboolOptions struct {
	Op BooleanOp
}

// Bandbool reduces all bands to a single channel via a bitwise op.
func (im *Image) Bandbool(opts BandboolOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.bandbool = &opts
	return im
}
