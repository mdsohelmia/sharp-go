package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func TestResizeCropModesAnimated(t *testing.T) {
	ctx := context.Background()
	buf := readFixture(t, "rgb-with-alpha.webp") // skips if fixtures absent
	_, info, err := sharp.FromBytes(buf).Animated().
		Resize(sharp.ResizeOptions{Width: 32, Height: 32, Fit: sharp.FitCover, Position: sharp.PositionHigh}).
		WebP(format.WebPOptions{}).ToBytes(ctx)
	if err != nil {
		t.Fatalf("animated crop: %v", err)
	}
	if info.Width != 32 {
		t.Fatalf("width = %d, want 32", info.Width)
	}
}

func TestResizeCropModesLowHighAll(t *testing.T) {
	ctx := context.Background()
	for _, pos := range []sharp.Position{sharp.PositionLow, sharp.PositionHigh, sharp.PositionAll} {
		src := sharp.FromCreate(sharp.CreateOptions{
			Width: 100, Height: 80, Channels: 3,
			Background: sharp.Color{R: 10, G: 20, B: 30},
		})
		_, info, err := src.Resize(sharp.ResizeOptions{
			Width: 40, Height: 40, Fit: sharp.FitCover, Position: pos,
		}).ToBytes(ctx)
		if err != nil {
			t.Fatalf("position %d: %v", pos, err)
		}
		if info.Width != 40 || info.Height != 40 {
			t.Fatalf("position %d: got %dx%d, want 40x40", pos, info.Width, info.Height)
		}
	}
}

// TestResizeCropModesBufferExactSize exercises the fused shrink-on-load path
// (encoded buffer input, default FastShrinkOnLoad) with a non-square source so
// FitCover must crop to exactly the target box. PositionAll in particular does
// not crop inside libvips' thumbnail, so the pipeline must enforce the size.
func TestResizeCropModesBufferExactSize(t *testing.T) {
	ctx := context.Background()
	src := jpegBuffer(t, 200, 100)
	for _, pos := range []sharp.Position{sharp.PositionLow, sharp.PositionHigh, sharp.PositionAll} {
		_, info, err := sharp.FromBytes(src).Resize(sharp.ResizeOptions{
			Width: 50, Height: 50, Fit: sharp.FitCover, Position: pos,
		}).ToBytes(ctx)
		if err != nil {
			t.Fatalf("position %d: %v", pos, err)
		}
		if info.Width != 50 || info.Height != 50 {
			t.Fatalf("position %d: got %dx%d, want 50x50", pos, info.Width, info.Height)
		}
	}
}
