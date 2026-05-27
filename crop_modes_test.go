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
