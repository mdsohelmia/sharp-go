package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func TestExtend(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Extend(sharp.ExtendOptions{
			Top: 10, Bottom: 20, Left: 30, Right: 40,
			Background: sharp.Color{R: 255, G: 0, B: 0, A: 255},
		}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Extend: %v", err)
	}
	if info.Width != 320+30+40 {
		t.Errorf("Width = %d, want %d", info.Width, 320+30+40)
	}
	if info.Height != 240+10+20 {
		t.Errorf("Height = %d, want %d", info.Height, 240+10+20)
	}
}

func TestTrim(t *testing.T) {
	// 320x240 has uniform borders only sporadically; use a synthetic-ish path.
	// We at least verify no error + non-zero output for a JPEG with borders.
	in := readFixture(t, "Flag_of_the_Netherlands.png")
	_, info, err := sharp.FromBytes(in).
		Trim(sharp.TrimOptions{Threshold: 10}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Trim: %v", err)
	}
	if info.Width <= 0 || info.Height <= 0 {
		t.Errorf("trim produced empty image: %dx%d", info.Width, info.Height)
	}
}

func TestAffineScale(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	// Scale by 0.5 via affine.
	_, info, err := sharp.FromBytes(in).
		Affine(sharp.AffineOptions{
			Matrix: [4]float64{0.5, 0, 0, 0.5},
		}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Affine: %v", err)
	}
	if info.Width != 160 || info.Height != 120 {
		t.Errorf("Affine 0.5 scale: dims = %dx%d, want 160x120", info.Width, info.Height)
	}
}
