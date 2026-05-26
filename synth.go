package sharp

import (
	"errors"

	"github.com/mdsohelmia/sharp-go/internal/vips"
)

// renderSynth materialises a synth-backed input into a *vips.Image at
// terminal-call time. Called from execute() before applying ops.
func renderSynth(s *synthSpec) (*vips.Image, error) {
	switch s.kind {
	case synthCreate:
		c := s.create
		return vips.CreateSolid(vips.CreateSolidParams{
			Width: c.Width, Height: c.Height, Bands: c.Channels,
			BgR: c.Background.R, BgG: c.Background.G,
			BgB: c.Background.B, BgA: c.Background.A,
		})
	case synthText:
		t := s.text
		return vips.CreateText(vips.CreateTextParams{
			Text: t.Text, Font: t.Font, FontFile: t.FontFile,
			Width: t.Width, Height: t.Height,
			DPI: t.DPI, Spacing: t.Spacing,
			RGBA: t.RGBA,
		})
	case synthJoin:
		return renderJoin(s.join, s.join2)
	default:
		return nil, errors.New("sharp: unknown synth kind")
	}
}

func renderJoin(images []*Image, opts JoinOptions) (*vips.Image, error) {
	// Recursively materialise each child input into a *vips.Image, applying
	// each child's own recorded operations. This gives sharp-style nested
	// pipelines per layer.
	vimgs := make([]*vips.Image, 0, len(images))
	for _, child := range images {
		vimg, err := materialiseInput(child)
		if err != nil {
			return nil, err
		}
		vimgs = append(vimgs, vimg)
	}
	return vips.ArrayJoin(vimgs, vips.ArrayJoinParams{
		Across:   opts.Across,
		HSpacing: opts.HSpacing,
		VSpacing: opts.VSpacing,
		BgR:      opts.Background.R, BgG: opts.Background.G,
		BgB: opts.Background.B, BgA: opts.Background.A,
	})
}

// materialiseInput resolves a child *Image to a fully-decoded *vips.Image.
// Used by Join to fold each layer's input into the parent pipeline.
func materialiseInput(im *Image) (*vips.Image, error) {
	if im.err != nil {
		return nil, im.err
	}
	if im.in.synth != nil {
		return renderSynth(im.in.synth)
	}
	buf, err := readInput(im.in)
	if err != nil {
		return nil, err
	}
	if im.in.raw != nil {
		r := im.in.raw
		return vips.LoadRawBuffer(buf, r.Width, r.Height, r.Channels, mapDepth(r.Depth))
	}
	if im.in.pages != 0 || im.in.page != 0 {
		pages := im.in.pages
		if pages == 0 {
			pages = 1
		}
		return vips.LoadBufferPages(buf, pages, im.in.page)
	}
	return vips.LoadBuffer(buf)
}
