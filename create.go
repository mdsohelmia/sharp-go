package sharp

import (
	"errors"
)

// CreateOptions configures FromCreate.
type CreateOptions struct {
	Width, Height int
	// Channels is 3 (RGB) or 4 (RGBA). Default 4.
	Channels   int
	Background Color
}

// FromCreate builds a solid-colour image of the given dimensions.
func FromCreate(opts CreateOptions) *Image {
	im := acquireImage()
	if opts.Width <= 0 || opts.Height <= 0 {
		im.stickyErr(errors.New("sharp: FromCreate requires positive Width and Height"))
		return im
	}
	if opts.Channels == 0 {
		opts.Channels = 4
	}
	if opts.Channels != 3 && opts.Channels != 4 {
		im.stickyErr(errors.New("sharp: FromCreate Channels must be 3 or 4"))
		return im
	}
	im.in.synth = &synthSpec{kind: synthCreate, create: opts}
	return im
}

// TextOptions configures FromText.
type TextOptions struct {
	Text     string
	Font     string // pango font spec, e.g. "sans 16"
	FontFile string // optional path to .ttf/.otf
	Width    int    // 0 = unconstrained
	Height   int    // 0 = unconstrained
	DPI      int    // default 72
	Spacing  int    // line spacing
	RGBA     bool
}

// FromText renders text into an image via pango.
func FromText(opts TextOptions) *Image {
	im := acquireImage()
	if opts.Text == "" {
		im.stickyErr(errors.New("sharp: FromText requires non-empty Text"))
		return im
	}
	im.in.synth = &synthSpec{kind: synthText, text: opts}
	return im
}

// JoinOptions configures Join.
type JoinOptions struct {
	Across     int // grid columns; default = len(images)
	HSpacing   int
	VSpacing   int
	Background Color
}

// Join builds a single image by joining multiple inputs in a grid.
func Join(images []*Image, opts JoinOptions) *Image {
	im := acquireImage()
	if len(images) == 0 {
		im.stickyErr(errors.New("sharp: Join requires at least one image"))
		return im
	}
	for i, x := range images {
		if x.err != nil {
			im.stickyErr(x.err)
			return im
		}
		if x.in.bytes == nil && x.in.path == "" && x.in.synth == nil {
			im.stickyErr(errors.New("sharp: Join element " + itoa(i) + " has no input"))
			return im
		}
	}
	im.in.synth = &synthSpec{kind: synthJoin, join: images, join2: opts}
	return im
}

func itoa(i int) string {
	const digits = "0123456789"
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = digits[i%10]
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}
