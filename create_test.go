package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func TestFromCreate(t *testing.T) {
	_, info, err := sharp.FromCreate(sharp.CreateOptions{
		Width: 64, Height: 48, Channels: 3,
		Background: sharp.Color{R: 255, G: 100, B: 0},
	}).PNG(format.PNGOptions{}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("FromCreate: %v", err)
	}
	if info.Width != 64 || info.Height != 48 {
		t.Errorf("dims = %dx%d, want 64x48", info.Width, info.Height)
	}
	if info.Channels != 3 {
		t.Errorf("Channels = %d, want 3", info.Channels)
	}
}

func TestFromCreateRGBA(t *testing.T) {
	_, info, err := sharp.FromCreate(sharp.CreateOptions{
		Width: 100, Height: 80,
		Background: sharp.Color{R: 0, G: 0, B: 255, A: 128},
	}).PNG(format.PNGOptions{}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("FromCreate: %v", err)
	}
	if info.Channels != 4 {
		t.Errorf("Channels = %d, want 4 (default)", info.Channels)
	}
}

func TestFromText(t *testing.T) {
	_, info, err := sharp.FromText(sharp.TextOptions{
		Text:    "hello",
		Font:    "sans 24",
		DPI:     150,
		RGBA:    true,
	}).PNG(format.PNGOptions{}).ToBytes(context.Background())
	if err != nil {
		t.Skipf("FromText: %v (libvips may lack pango/fontconfig)", err)
	}
	if info.Width <= 0 || info.Height <= 0 {
		t.Errorf("FromText dims = %dx%d", info.Width, info.Height)
	}
}

func TestJoin(t *testing.T) {
	a := sharp.FromCreate(sharp.CreateOptions{Width: 50, Height: 50, Background: sharp.Color{R: 255, A: 255}})
	b := sharp.FromCreate(sharp.CreateOptions{Width: 50, Height: 50, Background: sharp.Color{G: 255, A: 255}})
	c := sharp.FromCreate(sharp.CreateOptions{Width: 50, Height: 50, Background: sharp.Color{B: 255, A: 255}})

	_, info, err := sharp.Join([]*sharp.Image{a, b, c}, sharp.JoinOptions{Across: 3}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Join: %v", err)
	}
	// 3 across, 1 tall.
	if info.Width != 150 || info.Height != 50 {
		t.Errorf("Join dims = %dx%d, want 150x50", info.Width, info.Height)
	}
}

func TestJoinGrid(t *testing.T) {
	a := sharp.FromCreate(sharp.CreateOptions{Width: 32, Height: 32, Background: sharp.Color{R: 255, A: 255}})
	b := sharp.FromCreate(sharp.CreateOptions{Width: 32, Height: 32, Background: sharp.Color{G: 255, A: 255}})
	c := sharp.FromCreate(sharp.CreateOptions{Width: 32, Height: 32, Background: sharp.Color{B: 255, A: 255}})
	d := sharp.FromCreate(sharp.CreateOptions{Width: 32, Height: 32, Background: sharp.Color{R: 255, G: 255, A: 255}})

	_, info, err := sharp.Join([]*sharp.Image{a, b, c, d}, sharp.JoinOptions{Across: 2}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Join grid: %v", err)
	}
	if info.Width != 64 || info.Height != 64 {
		t.Errorf("Join 2x2 dims = %dx%d, want 64x64", info.Width, info.Height)
	}
}

func TestFromCreateThenResize(t *testing.T) {
	_, info, err := sharp.FromCreate(sharp.CreateOptions{
		Width: 200, Height: 100,
		Background: sharp.Color{R: 200, G: 50, B: 100, A: 255},
	}).
		Resize(sharp.ResizeOptions{Width: 50, Height: 50, Fit: sharp.FitFill}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("FromCreate+Resize: %v", err)
	}
	if info.Width != 50 || info.Height != 50 {
		t.Errorf("dims = %dx%d, want 50x50", info.Width, info.Height)
	}
}
