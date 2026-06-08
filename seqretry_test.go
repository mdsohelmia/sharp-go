package sharp_test

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

// tallPNG builds a baseline (non-interlaced) RGB PNG h rows tall. Go's encoder
// never interlaces, so this isolates the *access mode* as the only variable.
func tallPNG(t *testing.T, w, h int) []byte {
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

// TestSequentialOutOfOrderRecovers exercises the failure class behind the prod
// metric `vipspng: out of order read`. Extract of a non-top region pulls source
// rows out of order; under v1.2.0's sequential decode the loader rejects it.
// The pipeline must recover (retry with random access) and still encode.
func TestSequentialOutOfOrderRecovers(t *testing.T) {
	in := tallPNG(t, 400, 1200)

	out, info, err := sharp.FromBytes(in).
		Extract(sharp.ExtractRegion{Left: 0, Top: 800, Width: 400, Height: 300}).
		WebP(format.WebPOptions{Quality: 80}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("empty output")
	}
	if info.Width != 400 || info.Height != 300 {
		t.Errorf("dimensions = %dx%d, want 400x300", info.Width, info.Height)
	}
}
