package sharp_test

import (
	"bytes"
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

// TestEnsureSRGBConvertsCMYK guards the sRGB-skip optimization: a CMYK source
// must still be converted (not skipped), which collapses 4 channels to 3.
func TestEnsureSRGBConvertsCMYK(t *testing.T) {
	for _, name := range []string{
		"Channel_digital_image_CMYK_color.jpg",            // CMYK + profile
		"Channel_digital_image_CMYK_color_no_profile.jpg", // CMYK, no profile
	} {
		in := readFixture(t, name)
		_, info, err := sharp.FromBytes(in).
			EnsureSRGB().
			JPEG(format.JPEGOptions{Quality: 80}).
			ToBytes(context.Background())
		if err != nil {
			t.Fatalf("%s: ToBytes: %v", name, err)
		}
		if info.Channels != 3 {
			t.Errorf("%s: channels = %d, want 3 (EnsureSRGB should convert CMYK to sRGB)", name, info.Channels)
		}
	}
}

// TestEnsureSRGBConvertsWideGamut guards that wide-gamut sources are still
// converted (the transform is not skipped): converting changes pixels, so the
// EnsureSRGB output differs from the non-converted encode.
func TestEnsureSRGBConvertsWideGamut(t *testing.T) {
	for _, name := range []string{"p3.png", "prophoto.png"} {
		in := readFixture(t, name)
		conv, _, err := sharp.FromBytes(in).EnsureSRGB().
			PNG(format.PNGOptions{}).ToBytes(context.Background())
		if err != nil {
			t.Fatalf("%s: EnsureSRGB ToBytes: %v", name, err)
		}
		raw, _, err := sharp.FromBytes(in).
			PNG(format.PNGOptions{}).ToBytes(context.Background())
		if err != nil {
			t.Fatalf("%s: raw ToBytes: %v", name, err)
		}
		if bytes.Equal(conv, raw) {
			t.Errorf("%s: EnsureSRGB produced identical bytes — wide-gamut conversion was skipped", name)
		}
	}
}

// TestEnsureSRGBAlreadySRGB is a smoke test: an already-sRGB image (the skip
// path) still encodes correctly.
func TestEnsureSRGBAlreadySRGB(t *testing.T) {
	in := readFixture(t, "gradients-rgb8.png")
	_, info, err := sharp.FromBytes(in).EnsureSRGB().
		WebP(format.WebPOptions{Quality: 75}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Channels < 3 {
		t.Errorf("channels = %d, want >=3", info.Channels)
	}
}
