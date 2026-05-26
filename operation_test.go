package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func TestBlur(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Blur(sharp.BlurOptions{Sigma: 3.0}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Blur: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("dims = %dx%d, want 320x240", info.Width, info.Height)
	}
}

func TestSharpen(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Sharpen(sharp.SharpenOptions{}). // defaults
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Sharpen: %v", err)
	}
}

func TestGamma(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Gamma(sharp.GammaOptions{Exponent: 2.2}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Gamma: %v", err)
	}
}

func TestNegate(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Negate(sharp.NegateOptions{}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Negate: %v", err)
	}
}

func TestNegateKeepAlpha(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands-alpha.png")
	_, _, err := sharp.FromBytes(in).
		Negate(sharp.NegateOptions{KeepAlpha: true}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("NegateKeepAlpha: %v", err)
	}
}

func TestThreshold(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Threshold(sharp.ThresholdOptions{Value: 128, Grayscale: true}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Threshold: %v", err)
	}
}

func TestLinear(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Linear(sharp.LinearOptions{A: []float64{1.5}, B: []float64{-10}}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Linear: %v", err)
	}
}

func TestMedian(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Median(sharp.MedianOptions{Size: 5}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Median: %v", err)
	}
}

func TestChainedOps(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 200, Height: 150}).
		Gamma(sharp.GammaOptions{Exponent: 2.2}).
		Blur(sharp.BlurOptions{Sigma: 1.5}).
		Sharpen(sharp.SharpenOptions{}).
		WebP(format.WebPOptions{Quality: 70}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("chain: %v", err)
	}
	if info.Width != 200 || info.Height != 150 {
		t.Errorf("dims = %dx%d, want 200x150", info.Width, info.Height)
	}
	if info.Format != "webp" {
		t.Errorf("Format = %q, want webp", info.Format)
	}
}
