package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestResizeCover(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	out, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 320, Height: 240, Fit: sharp.FitCover}).
		JPEG(format.JPEGOptions{Quality: 80}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("dimensions = %dx%d, want 320x240", info.Width, info.Height)
	}
	if len(out) == 0 {
		t.Errorf("empty output")
	}
}

func TestResizeInside(t *testing.T) {
	// Source is 2725x2225 — inside box of 200x200 should produce 200x163-ish.
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 200, Height: 200, Fit: sharp.FitInside}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width > 200 || info.Height > 200 {
		t.Errorf("FitInside should not exceed box: got %dx%d", info.Width, info.Height)
	}
	if info.Width != 200 && info.Height != 200 {
		t.Errorf("FitInside should hit exactly one dimension: got %dx%d", info.Width, info.Height)
	}
}

func TestResizeContain(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{
			Width: 400, Height: 400, Fit: sharp.FitContain,
			Background: sharp.Color{R: 255, G: 0, B: 0, A: 255},
		}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 400 || info.Height != 400 {
		t.Errorf("FitContain should pad to box: got %dx%d, want 400x400", info.Width, info.Height)
	}
}

func TestResizeFill(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 150, Height: 300, Fit: sharp.FitFill}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 150 || info.Height != 300 {
		t.Errorf("FitFill should match box exactly: got %dx%d", info.Width, info.Height)
	}
}

func TestResizeWidthOnly(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 500}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 500 {
		t.Errorf("Width = %d, want 500", info.Width)
	}
}

func TestResizeNoDimensionsErrors(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	_, _, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err == nil {
		t.Errorf("expected error for empty ResizeOptions")
	}
}

func TestResizeAttentionCrop(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{
			Width: 200, Height: 200,
			Fit:      sharp.FitCover,
			Position: sharp.PositionAttention,
		}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Width != 200 || info.Height != 200 {
		t.Errorf("dimensions = %dx%d, want 200x200", info.Width, info.Height)
	}
}
