package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestRotate90(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Rotate(sharp.RotateOptions{Angle: 90}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 240 || info.Height != 320 {
		t.Errorf("Rotate(90): dims = %dx%d, want 240x320", info.Width, info.Height)
	}
}

func TestRotate180(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Rotate(sharp.RotateOptions{Angle: 180}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("Rotate(180): dims = %dx%d, want 320x240", info.Width, info.Height)
	}
}

func TestRotateArbitrary(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Rotate(sharp.RotateOptions{Angle: 45, Background: sharp.Color{R: 0, G: 0, B: 0, A: 255}}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	// 45-degree rotation grows the bounding box (~ width * sqrt(2)).
	if info.Width <= 320 || info.Height <= 240 {
		t.Errorf("Rotate(45): dims = %dx%d, want larger than 320x240", info.Width, info.Height)
	}
}

func TestFlipFlop(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Flip().Flop().
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("Flip+Flop: dims = %dx%d, want 320x240", info.Width, info.Height)
	}
}

func TestExtract(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Extract(sharp.ExtractRegion{Left: 10, Top: 20, Width: 100, Height: 50}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 100 || info.Height != 50 {
		t.Errorf("Extract: dims = %dx%d, want 100x50", info.Width, info.Height)
	}
}

func TestExtractThenResize(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Extract(sharp.ExtractRegion{Left: 0, Top: 0, Width: 160, Height: 120}).
		Resize(sharp.ResizeOptions{Width: 80, Height: 60, Fit: sharp.FitFill}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 80 || info.Height != 60 {
		t.Errorf("dims = %dx%d, want 80x60", info.Width, info.Height)
	}
}

func TestAutoOrient(t *testing.T) {
	// Landscape_1 is the canonical orientation (orientation=1, no rotation).
	in := readFixture(t, "Landscape_1.jpg")
	_, info, err := sharp.FromBytes(in).
		AutoOrient().
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	// Should be landscape-oriented: width > height.
	if info.Width <= info.Height {
		t.Errorf("AutoOrient Landscape_1: dims = %dx%d, expected landscape", info.Width, info.Height)
	}
}
