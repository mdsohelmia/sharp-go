package sharp

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/mdsohelmia/sharp-go/format"
)

func tallPNGInternal(t *testing.T, w, h int) []byte {
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

func TestIsOutOfOrderErr(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"real vipspng", errors.New("vipspng: out of order read at line 757"), true},
		{"mixed case", errors.New("VipsForeignLoad: Out Of Order read"), true},
		{"unrelated", errors.New("vips: unable to load from buffer"), false},
		{"truncated", errors.New("read error"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := isOutOfOrderErr(c.err); got != c.want {
				t.Errorf("isOutOfOrderErr(%v) = %v, want %v", c.err, got, c.want)
			}
		})
	}
}

func TestCanRetryRandom(t *testing.T) {
	if im := FromBytes([]byte{1, 2, 3}); !im.canRetryRandom() {
		t.Errorf("byte input should be retryable")
	}
	if im := FromFile("x.png"); !im.canRetryRandom() {
		t.Errorf("file input should be retryable")
	}
	// A streaming reader is consumed on first decode — not retryable.
	if im := FromReader(bytes.NewReader([]byte{1, 2, 3})); im.canRetryRandom() {
		t.Errorf("reader input must NOT be retryable")
	}
}

// TestExecuteOnceForceRandom confirms the recovery decode path (the one the
// retry invokes) encodes a correct image end-to-end.
func TestExecuteOnceForceRandom(t *testing.T) {
	in := tallPNGInternal(t, 300, 900)
	im := FromBytes(in).WebP(format.WebPOptions{Quality: 80})

	data, info, err := executeOnce(context.Background(), im, true /* forceRandom */)
	if err != nil {
		t.Fatalf("executeOnce(forceRandom): %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("empty output")
	}
	if info.Width != 300 || info.Height != 900 {
		t.Errorf("dims = %dx%d, want 300x900", info.Width, info.Height)
	}
}
