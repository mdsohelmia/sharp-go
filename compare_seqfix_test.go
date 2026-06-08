package sharp_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
)

func tallPNGCompare(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		c := color.RGBA{R: uint8(y % 256), G: uint8((y * 3) % 256), B: 200, A: 255}
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

// TestCompareDecodesRandomAccess guards against the sequential double-read
// regression: Compare realizes each source to pixels and reads it more than
// once (RMSE subtract + LAB deltaE), which a sequential-access decode cannot
// serve — it failed with "out of order read at line <height>". The decode must
// be random-access.
func TestCompareDecodesRandomAccess(t *testing.T) {
	p := tallPNGCompare(t, 600, 1100)

	res, err := sharp.Compare(context.Background(),
		sharp.FromBytes(p), sharp.FromBytes(p), sharp.CompareOptions{})
	if err != nil {
		t.Fatalf("Compare: %v", err)
	}
	if res.RMSE != 0 {
		t.Errorf("RMSE = %v, want 0 for identical inputs", res.RMSE)
	}
	if res.Width != 600 || res.Height != 1100 {
		t.Errorf("dims = %dx%d, want 600x1100", res.Width, res.Height)
	}
}
