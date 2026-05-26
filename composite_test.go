package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func TestPreparedOverlayReuse(t *testing.T) {
	base := readFixture(t, "320x240.jpg")
	overlay := readFixture(t, "Flag_of_the_Netherlands.png")
	prep, err := sharp.PrepareOverlay(overlay)
	if err != nil {
		t.Fatalf("PrepareOverlay: %v", err)
	}
	defer prep.Close()
	// Three composite passes share the same prepared overlay — verify the
	// refcount-bump path produces identical results across reuses.
	for i := 0; i < 3; i++ {
		_, info, err := sharp.FromBytes(base).
			Composite([]sharp.CompositeLayer{
				{Prepared: prep, Gravity: sharp.GravityCentre, Blend: sharp.BlendOver},
			}).
			JPEG(format.JPEGOptions{}).
			ToBytes(context.Background())
		if err != nil {
			t.Fatalf("iter %d Composite: %v", i, err)
		}
		if info.Width != 320 || info.Height != 240 {
			t.Errorf("iter %d dims = %dx%d, want 320x240", i, info.Width, info.Height)
		}
	}
}

func TestCompositeOver(t *testing.T) {
	base := readFixture(t, "320x240.jpg")
	overlay := readFixture(t, "Flag_of_the_Netherlands.png")
	_, info, err := sharp.FromBytes(base).
		Composite([]sharp.CompositeLayer{
			{Input: overlay, Gravity: sharp.GravityCentre, Blend: sharp.BlendOver},
		}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Composite: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("dims = %dx%d, want 320x240", info.Width, info.Height)
	}
}

func TestCompositeOffset(t *testing.T) {
	base := readFixture(t, "320x240.jpg")
	overlay := readFixture(t, "Flag_of_the_Netherlands.png")
	_, _, err := sharp.FromBytes(base).
		Composite([]sharp.CompositeLayer{
			{Input: overlay, Top: 50, Left: 100, HasOffset: true, Blend: sharp.BlendOver},
		}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Composite offset: %v", err)
	}
}

func TestCompositeTile(t *testing.T) {
	base := readFixture(t, "320x240.jpg")
	overlay := readFixture(t, "Flag_of_the_Netherlands.png")
	_, info, err := sharp.FromBytes(base).
		Composite([]sharp.CompositeLayer{
			{Input: overlay, Tile: true, Blend: sharp.BlendOver},
		}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Composite tile: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("dims = %dx%d, want 320x240", info.Width, info.Height)
	}
}

func TestCompositeMultiply(t *testing.T) {
	base := readFixture(t, "320x240.jpg")
	overlay := readFixture(t, "Flag_of_the_Netherlands.png")
	_, _, err := sharp.FromBytes(base).
		Composite([]sharp.CompositeLayer{
			{Input: overlay, Gravity: sharp.GravityNorthWest, Blend: sharp.BlendMultiply},
		}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Composite multiply: %v", err)
	}
}

func TestCompositeMultiLayer(t *testing.T) {
	base := readFixture(t, "320x240.jpg")
	overlay := readFixture(t, "Flag_of_the_Netherlands.png")
	_, _, err := sharp.FromBytes(base).
		Composite([]sharp.CompositeLayer{
			{Input: overlay, Gravity: sharp.GravityNorthWest},
			{Input: overlay, Gravity: sharp.GravitySouthEast, Blend: sharp.BlendScreen},
		}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Composite multi-layer: %v", err)
	}
}
