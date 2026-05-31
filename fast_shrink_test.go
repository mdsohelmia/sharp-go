package sharp_test

import (
	"bytes"
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

// jpegBuffer renders a solid JPEG so the buffer (fusion) path is exercised
// without depending on test fixtures.
func jpegBuffer(t *testing.T, w, h int) []byte {
	t.Helper()
	buf, _, err := sharp.FromCreate(sharp.CreateOptions{
		Width: w, Height: h, Channels: 3,
		Background: sharp.Color{R: 120, G: 90, B: 60},
	}).JPEG(format.JPEGOptions{Quality: 90}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("jpegBuffer: %v", err)
	}
	return buf
}

func TestFastShrinkOnLoadNilEqualsTrue(t *testing.T) {
	ctx := context.Background()
	src := jpegBuffer(t, 400, 400)
	tr := true

	a, _, err := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 100}).ToBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 100, FastShrinkOnLoad: &tr}).ToBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("nil and explicit-true FastShrinkOnLoad must produce identical output")
	}
}

func TestFastShrinkOnLoadOffProducesValidOutput(t *testing.T) {
	ctx := context.Background()
	src := jpegBuffer(t, 400, 400)
	off := false

	_, info, err := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 100, FastShrinkOnLoad: &off}).ToBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 100 {
		t.Fatalf("width = %d, want 100", info.Width)
	}
}

func TestFastShrinkOnLoadQualitySmoke(t *testing.T) {
	ctx := context.Background()
	src := jpegBuffer(t, 800, 800)
	off := false
	on := true

	ref := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 200, FastShrinkOnLoad: &off})
	cand := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 200, FastShrinkOnLoad: &on})

	res, err := sharp.Compare(ctx, ref, cand, sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 200 || res.Height != 200 {
		t.Fatalf("dims = %dx%d, want 200x200", res.Width, res.Height)
	}
	if res.RMSE < 0 || (res.RMSE > 0 && res.PSNR <= 0) {
		t.Fatalf("nonsensical metrics: RMSE=%v PSNR=%v", res.RMSE, res.PSNR)
	}
	t.Logf("fastShrink off vs on: RMSE=%.4f PSNR=%.2f dE00mean=%.5f", res.RMSE, res.PSNR, res.DeltaE.Mean)
}
