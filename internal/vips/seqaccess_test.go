package vips

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

func tallPNGBytes(t *testing.T, w, h int) []byte {
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

// TestSequentialBackwardReadMechanism proves the failure mechanism behind the
// production `vipspng: out of order read` metric, and the premise the recovery
// relies on: a SEQUENTIAL-access decode rejects a backward-reading op (vertical
// flip), whereas a RANDOM-access decode of the same bytes succeeds.
func TestSequentialBackwardReadMechanism(t *testing.T) {
	buf := tallPNGBytes(t, 200, 1000)

	// Sequential decode + vertical flip: rows are pulled bottom-to-top, which
	// the streaming loader cannot serve -> "out of order read".
	seq, err := LoadBufferSeq(buf, AccessSequential)
	if err != nil {
		t.Fatalf("sequential load: %v", err)
	}
	flipped, err := Flip(seq, DirectionVertical)
	if err != nil {
		t.Fatalf("flip: %v", err)
	}
	if _, err := SavePNG(flipped, PNGParams{Compression: 6, Effort: 1}); err == nil {
		t.Fatalf("expected out-of-order error on sequential backward read, got nil")
	} else if !strings.Contains(strings.ToLower(err.Error()), "out of order") {
		t.Fatalf("expected an 'out of order' error, got: %v", err)
	}

	// Random decode of the same bytes: the recovery path must succeed.
	rnd, err := LoadBuffer(buf)
	if err != nil {
		t.Fatalf("random load: %v", err)
	}
	flipped2, err := Flip(rnd, DirectionVertical)
	if err != nil {
		t.Fatalf("flip (random): %v", err)
	}
	out, err := SavePNG(flipped2, PNGParams{Compression: 6, Effort: 1})
	if err != nil {
		t.Fatalf("random save: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("empty output on random path")
	}
}
