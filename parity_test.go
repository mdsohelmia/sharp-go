// Sharp behavioural-parity tests.
//
// Each test case mirrors a specific assertion from upstream sharp's
// test/unit/* suite. The reference assertion (dimensions, channels, format,
// or a small tolerance on encoded size) is the bar these tests defend.

package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

// ParityResizePreservesAspectWidthOnly: ported from test/unit/resize.js
// "shrink width and height" — width=320, height absent → output keeps source
// aspect ratio.
func TestParityResizePreservesAspectWidthOnly(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg") // 2725 x 2225
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 320}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if info.Width != 320 {
		t.Errorf("Width = %d, want 320", info.Width)
	}
	// 2725/2225 ≈ 1.2247. 320/1.2247 ≈ 261.
	if info.Height < 258 || info.Height > 264 {
		t.Errorf("Height = %d, want 261±3 (aspect preserved)", info.Height)
	}
}

// ParityResizeHeightOnly: same as above but height is the only constraint.
func TestParityResizeHeightOnly(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Height: 320}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if info.Height != 320 {
		t.Errorf("Height = %d, want 320", info.Height)
	}
	// 2725/2225 * 320 ≈ 391.
	if info.Width < 388 || info.Width > 394 {
		t.Errorf("Width = %d, want 391±3", info.Width)
	}
}

// ParityResizeInsideNoEnlargement: ported from test/unit/resize.js
// FitInside with a box larger than the source should leave the image at
// source dimensions (or smaller).
func TestParityResizeInsideNoEnlargement(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 1000, Height: 1000, Fit: sharp.FitInside}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if info.Width > 320 || info.Height > 240 {
		t.Errorf("FitInside enlarged: %dx%d, source 320x240", info.Width, info.Height)
	}
}

// ParityResizeOutsideNoReduction: FitOutside with a smaller-than-source box
// should leave the image at source dimensions or larger.
func TestParityResizeOutsideNoReduction(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 100, Height: 100, Fit: sharp.FitOutside}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Resize: %v", err)
	}
	if info.Width < 320 || info.Height < 240 {
		t.Errorf("FitOutside reduced: %dx%d, source 320x240", info.Width, info.Height)
	}
}

// ParityRotateChangesDimensions: 90° rotation of a 320x240 image yields 240x320.
func TestParityRotateChangesDimensions(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Rotate(sharp.RotateOptions{Angle: 90}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if info.Width != 240 || info.Height != 320 {
		t.Errorf("90° rotate dims = %dx%d, want 240x320", info.Width, info.Height)
	}
}

// ParityAutoOrient applies EXIF orientation to all 8 Landscape_N fixtures
// and verifies the output is landscape (W > H).
func TestParityAutoOrientLandscapeSet(t *testing.T) {
	for n := 1; n <= 8; n++ {
		name := "Landscape_" + itoa(n) + ".jpg"
		t.Run(name, func(t *testing.T) {
			in := readFixture(t, name)
			_, info, err := sharp.FromBytes(in).
				AutoOrient().
				JPEG(format.JPEGOptions{}).
				ToBytes(context.Background())
			if err != nil {
				t.Fatalf("AutoOrient %s: %v", name, err)
			}
			if info.Width <= info.Height {
				t.Errorf("%s after AutoOrient: %dx%d, expected landscape", name, info.Width, info.Height)
			}
		})
	}
}

// itoa for parity_test.go locally (so test depends on no internal helpers).
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	const digits = "0123456789"
	var b [4]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = digits[i%10]
		i /= 10
	}
	return string(b[pos:])
}

// ParityMetadataAlphaDetection: sharp's metadata reports hasAlpha=true for
// PNGs with an alpha channel.
func TestParityMetadataAlphaDetection(t *testing.T) {
	cases := []struct {
		fixture  string
		hasAlpha bool
	}{
		{"Flag_of_the_Netherlands.png", false},
		{"Flag_of_the_Netherlands-alpha.png", true},
	}
	for _, c := range cases {
		md, err := sharp.FromBytes(readFixture(t, c.fixture)).Metadata(context.Background())
		if err != nil {
			t.Errorf("%s: %v", c.fixture, err)
			continue
		}
		if md.HasAlpha != c.hasAlpha {
			t.Errorf("%s: HasAlpha = %v, want %v", c.fixture, md.HasAlpha, c.hasAlpha)
		}
	}
}

// ParityMetadataChannels for JPEG, PNG-rgb, PNG-rgba mirrors sharp's
// metadata().channels output.
func TestParityMetadataChannels(t *testing.T) {
	cases := []struct {
		fixture  string
		channels int
	}{
		{"320x240.jpg", 3},                              // RGB JPEG
		{"Flag_of_the_Netherlands.png", 3},              // RGB PNG
		{"Flag_of_the_Netherlands-alpha.png", 4},        // RGBA PNG
		{"Channel_digital_image_CMYK_color.jpg", 4},     // CMYK JPEG
	}
	for _, c := range cases {
		md, err := sharp.FromBytes(readFixture(t, c.fixture)).Metadata(context.Background())
		if err != nil {
			t.Errorf("%s: %v", c.fixture, err)
			continue
		}
		if md.Channels != c.channels {
			t.Errorf("%s: Channels = %d, want %d", c.fixture, md.Channels, c.channels)
		}
	}
}

// ParityFlipChainPreservesDims: flip + flop together preserve dimensions.
func TestParityFlipChainPreservesDims(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Flip().Flop().Flip().Flop().
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("flip/flop chain: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("4x flip/flop: %dx%d, want 320x240", info.Width, info.Height)
	}
}

// ParityExtractWithinBounds reproduces a sharp test/unit/extract.js case.
func TestParityExtractWithinBounds(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Extract(sharp.ExtractRegion{Left: 0, Top: 0, Width: 320, Height: 240}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Extract full: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("dims = %dx%d, want 320x240", info.Width, info.Height)
	}
}

// ParityCompositeKeepsBaseDims: overlaying never changes the base dimensions.
func TestParityCompositeKeepsBaseDims(t *testing.T) {
	base := readFixture(t, "320x240.jpg")
	overlay := readFixture(t, "Flag_of_the_Netherlands.png")
	_, info, err := sharp.FromBytes(base).
		Composite([]sharp.CompositeLayer{
			{Input: overlay, Gravity: sharp.GravityCentre},
		}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Composite: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("composite dims = %dx%d, want 320x240", info.Width, info.Height)
	}
}

// ParityJPEGQualityImpactsSize: higher quality produces larger files.
func TestParityJPEGQualityImpactsSize(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	q10, _, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 400, Height: 300}).
		JPEG(format.JPEGOptions{Quality: 10}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("q10: %v", err)
	}
	q90, _, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 400, Height: 300}).
		JPEG(format.JPEGOptions{Quality: 90}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("q90: %v", err)
	}
	if len(q90) <= len(q10) {
		t.Errorf("q90 (%d) should be larger than q10 (%d)", len(q90), len(q10))
	}
}

// ParityPNGCompressionImpactsSize: higher compression yields smaller files.
func TestParityPNGCompressionImpactsSize(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.png")
	c1, _, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 200, Height: 150}).
		PNG(format.PNGOptions{Compression: 1}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("c1: %v", err)
	}
	c9, _, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 200, Height: 150}).
		PNG(format.PNGOptions{Compression: 9}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("c9: %v", err)
	}
	if len(c9) >= len(c1) {
		t.Errorf("c9 (%d) should be smaller than c1 (%d)", len(c9), len(c1))
	}
}

// ParityGreyscaleSetsChannels: greyscale conversion produces a 1-band image.
func TestParityGreyscaleSetsChannels(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Greyscale().
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Greyscale: %v", err)
	}
	if info.Channels != 1 {
		t.Errorf("Channels after Greyscale = %d, want 1", info.Channels)
	}
}
