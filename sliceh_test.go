package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestGreyscale(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Greyscale().
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Greyscale: %v", err)
	}
	if info.Channels != 1 {
		t.Errorf("Channels = %d, want 1", info.Channels)
	}
}

func TestTint(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Tint(sharp.TintOptions{Colour: sharp.Color{R: 255, G: 100, B: 50}}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Tint: %v", err)
	}
}

func TestModulate(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Modulate(sharp.ModulateOptions{Brightness: 1.2, Saturation: 0.8, Hue: 30}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Modulate: %v", err)
	}
}

func TestNormalise(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Normalise(sharp.NormaliseOptions{}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Normalise: %v", err)
	}
}

func TestClahe(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Clahe(sharp.ClaheOptions{Width: 8, Height: 8, MaxSlope: 3}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Clahe: %v", err)
	}
}

func TestConvolveBlur(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	box := []float64{
		1, 1, 1,
		1, 1, 1,
		1, 1, 1,
	}
	_, _, err := sharp.FromBytes(in).
		Convolve(sharp.ConvolveOptions{Kernel: box, Width: 3, Height: 3, Scale: 9}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Convolve: %v", err)
	}
}

func TestRecombSwapRG(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	swap := []float64{
		0, 1, 0,
		1, 0, 0,
		0, 0, 1,
	}
	_, info, err := sharp.FromBytes(in).
		Recomb(sharp.RecombOptions{Matrix: swap, N: 3}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Recomb: %v", err)
	}
	if info.Channels != 3 {
		t.Errorf("Channels = %d, want 3", info.Channels)
	}
}

func TestDilateErode(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Threshold(sharp.ThresholdOptions{Value: 128, Grayscale: true}).
		Dilate(sharp.MorphOptions{Size: 1}).
		Erode(sharp.MorphOptions{Size: 1}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Dilate/Erode: %v", err)
	}
}

func TestFlatten(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands-alpha.png")
	_, info, err := sharp.FromBytes(in).
		Flatten(sharp.FlattenOptions{Background: sharp.Color{R: 255, G: 255, B: 255}}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Flatten: %v", err)
	}
	if info.Channels != 3 {
		t.Errorf("after flatten Channels = %d, want 3", info.Channels)
	}
}

func TestBooleanAnd(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Boolean(sharp.BooleanOptions{Op: sharp.BooleanAnd, Constant: 0xF0}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Boolean: %v", err)
	}
}

func TestRemoveAlpha(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands-alpha.png")
	_, info, err := sharp.FromBytes(in).
		RemoveAlpha().
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("RemoveAlpha: %v", err)
	}
	if info.Channels != 3 {
		t.Errorf("Channels = %d, want 3", info.Channels)
	}
}

func TestEnsureAlpha(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		EnsureAlpha(sharp.EnsureAlphaOptions{Alpha: 1}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("EnsureAlpha: %v", err)
	}
	if info.Channels != 4 {
		t.Errorf("Channels = %d, want 4", info.Channels)
	}
}

func TestToColourspace(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		ToColourspace(sharp.ColourspaceBW).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToColourspace: %v", err)
	}
	if info.Channels != 1 {
		t.Errorf("Channels = %d, want 1", info.Channels)
	}
}
