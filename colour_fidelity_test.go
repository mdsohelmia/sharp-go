package sharp_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"math"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

// decodeToImage round-trips arbitrary encoded bytes to pixels by asking the
// library to emit lossless PNG, then decoding with the stdlib. This lets us
// read pixels of formats Go can't decode natively (AVIF/WebP) on the same
// footing as the source.
func decodeToImage(t *testing.T, data []byte) image.Image {
	t.Helper()
	pngBytes, _, err := sharp.FromBytes(data).PNG(format.PNGOptions{Compression: 6}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("decode->png: %v", err)
	}
	img, err := png.Decode(bytes.NewReader(pngBytes))
	if err != nil {
		t.Fatalf("png.Decode: %v", err)
	}
	return img
}

// toneStats returns the mean signed per-channel difference (b-a) — the tone
// bias, in 0-255 units — and the overall RMSE. A tone SHIFT shows up as a
// non-zero bias; lossy compression alone shows up as RMSE with ~zero bias.
func toneStats(t *testing.T, a, b image.Image) (bias [3]float64, rmse float64) {
	t.Helper()
	if a.Bounds() != b.Bounds() {
		t.Fatalf("size mismatch: %v vs %v", a.Bounds(), b.Bounds())
	}
	var sumsq float64
	n := 0
	bnd := a.Bounds()
	for y := bnd.Min.Y; y < bnd.Max.Y; y++ {
		for x := bnd.Min.X; x < bnd.Max.X; x++ {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			dr := float64(int(br>>8) - int(ar>>8))
			dg := float64(int(bg>>8) - int(ag>>8))
			db := float64(int(bb>>8) - int(ab>>8))
			bias[0] += dr
			bias[1] += dg
			bias[2] += db
			sumsq += dr*dr + dg*dg + db*db
			n++
		}
	}
	for i := range bias {
		bias[i] /= float64(n)
	}
	rmse = math.Sqrt(sumsq / float64(3*n))
	return bias, rmse
}

func enc(t *testing.T, im *sharp.Image) []byte {
	t.Helper()
	b, _, err := im.ToBytes(context.Background())
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return b
}

// TestColourToneFidelity proves the pipeline does not shift colour tone.
//   - Lossless path (EnsureSRGB + PNG) on an sRGB source must be pixel-faithful.
//   - Lossy paths (WebP/AVIF) add compression noise (RMSE) but must NOT
//     introduce a tone bias (mean per-channel offset stays ~0).
func TestColourToneFidelity(t *testing.T) {
	orig := readFixture(t, "gradients-rgb8.png") // sRGB, smooth gradients
	ref := decodeToImage(t, orig)

	t.Run("lossless EnsureSRGB is faithful", func(t *testing.T) {
		out := enc(t, sharp.FromBytes(orig).EnsureSRGB().PNG(format.PNGOptions{Compression: 6}))
		bias, rmse := toneStats(t, ref, decodeToImage(t, out))
		for i, b := range bias {
			if math.Abs(b) > 0.5 {
				t.Errorf("channel %d tone bias %.3f exceeds 0.5 (lossless should be faithful)", i, b)
			}
		}
		if rmse > 1.0 {
			t.Errorf("lossless RMSE %.3f too high", rmse)
		}
		t.Logf("lossless: bias=%.3f rmse=%.3f", bias, rmse)
	})

	for _, tc := range []struct {
		name string
		mk   *sharp.Image
	}{
		{"webp q90", sharp.FromBytes(orig).EnsureSRGB().WebP(format.WebPOptions{Quality: 90})},
		{"webp q75 sharpYUV", sharp.FromBytes(orig).EnsureSRGB().WebP(format.WebPOptions{Quality: 75, UseSharpYUV: true})},
		{"avif q50 e2", sharp.FromBytes(orig).EnsureSRGB().AVIF(format.AVIFOptions{Quality: 50, Effort: 2})},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := enc(t, tc.mk)
			bias, rmse := toneStats(t, ref, decodeToImage(t, out))
			for i, b := range bias {
				if math.Abs(b) > 2.0 {
					t.Errorf("channel %d tone bias %.3f exceeds 2.0 — tone shift", i, b)
				}
			}
			t.Logf("%s: bias=%.3f rmse=%.3f", tc.name, bias, rmse)
		})
	}
}
