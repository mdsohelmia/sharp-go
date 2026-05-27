package sharp_test

import (
	"context"
	"math"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
)

func solid(w, h, ch int, r, g, b float64) *sharp.Image {
	return sharp.FromCreate(sharp.CreateOptions{
		Width: w, Height: h, Channels: ch,
		Background: sharp.Color{R: r, G: g, B: b, A: 255},
	})
}

func TestCompareIdentical(t *testing.T) {
	ctx := context.Background()
	res, err := sharp.Compare(ctx, solid(64, 48, 3, 100, 100, 100), solid(64, 48, 3, 100, 100, 100), sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.RMSE != 0 {
		t.Fatalf("RMSE = %v, want 0", res.RMSE)
	}
	if !math.IsInf(res.PSNR, 1) {
		t.Fatalf("PSNR = %v, want +Inf", res.PSNR)
	}
	if res.DeltaE.Mean != 0 || res.DeltaE.Max != 0 {
		t.Fatalf("deltaE = %+v, want zero", res.DeltaE)
	}
	if res.Width != 64 || res.Height != 48 {
		t.Fatalf("dims = %dx%d, want 64x48", res.Width, res.Height)
	}
}

func TestCompareKnownOffset(t *testing.T) {
	ctx := context.Background()
	// One channel differs by 10; RMSE over 3 bands = sqrt((10^2)/3) = 5.7735.
	res, err := sharp.Compare(ctx, solid(32, 32, 3, 100, 100, 100), solid(32, 32, 3, 110, 100, 100), sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := math.Sqrt(100.0 / 3.0)
	if math.Abs(res.RMSE-want) > 0.05 {
		t.Fatalf("RMSE = %v, want ~%v", res.RMSE, want)
	}
	if res.DeltaE.Mean <= 0 {
		t.Fatalf("deltaE mean = %v, want > 0", res.DeltaE.Mean)
	}
}

func TestCompareDeltaEMethods(t *testing.T) {
	ctx := context.Background()
	mk := func() (*sharp.Image, *sharp.Image) {
		return solid(32, 32, 3, 100, 120, 140), solid(32, 32, 3, 130, 110, 150)
	}
	var means []float64
	for _, m := range []sharp.DeltaEMethod{sharp.DeltaE2000, sharp.DeltaE76, sharp.DeltaECMC} {
		a, b := mk()
		res, err := sharp.Compare(ctx, a, b, sharp.CompareOptions{DeltaEMethod: m})
		if err != nil {
			t.Fatalf("method %d: %v", m, err)
		}
		if res.DeltaE.Mean <= 0 {
			t.Fatalf("method %d: deltaE mean = %v, want > 0", m, res.DeltaE.Mean)
		}
		means = append(means, res.DeltaE.Mean)
	}
	if means[0] == means[1] && means[1] == means[2] {
		t.Fatalf("dE2000/dE76/dECMC all equal (%v); expected the formulae to differ", means)
	}
}

func TestCompareAutoResize(t *testing.T) {
	ctx := context.Background()
	res, err := sharp.Compare(ctx, solid(100, 100, 3, 50, 60, 70), solid(40, 40, 3, 50, 60, 70), sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 100 || res.Height != 100 {
		t.Fatalf("dims = %dx%d, want 100x100 (ref size)", res.Width, res.Height)
	}
	if res.RMSE > 0.01 {
		t.Fatalf("RMSE = %v, want ~0 for identical-colour resize", res.RMSE)
	}
}

func TestCompareAlphaMismatch(t *testing.T) {
	ctx := context.Background()
	// ref RGB, cmp RGBA opaque white; band counts differ -> cmp flattened on white.
	res, err := sharp.Compare(ctx, solid(32, 32, 3, 255, 255, 255), solid(32, 32, 4, 255, 255, 255), sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.RMSE != 0 {
		t.Fatalf("RMSE = %v, want 0 (both white after flatten)", res.RMSE)
	}
}
