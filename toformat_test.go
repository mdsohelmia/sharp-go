package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestToFormatJPEG(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		ToFormat(sharp.FormatJPEG, format.JPEGOptions{Quality: 70}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToFormat: %v", err)
	}
	if info.Format != "jpeg" {
		t.Errorf("Format = %q, want jpeg", info.Format)
	}
}

func TestToFormatPNGNoOpts(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		ToFormat(sharp.FormatPNG, nil).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToFormat: %v", err)
	}
	if info.Format != "png" {
		t.Errorf("Format = %q, want png", info.Format)
	}
}

func TestToFormatString(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		ToFormat("webp", nil).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToFormat: %v", err)
	}
	if info.Format != "webp" {
		t.Errorf("Format = %q, want webp", info.Format)
	}
}

func TestToFormatUnknown(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		ToFormat("bogus", nil).
		ToBytes(context.Background())
	if err == nil {
		t.Errorf("expected error for unknown format")
	}
}

func TestToFormatWrongOptsType(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	// Passing PNG opts to a JPEG dispatch should fail.
	_, _, err := sharp.FromBytes(in).
		ToFormat(sharp.FormatJPEG, format.PNGOptions{Compression: 6}).
		ToBytes(context.Background())
	if err == nil {
		t.Errorf("expected error for wrong opts type")
	}
}

func TestCloneIndependent(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	base := sharp.FromBytes(in).Resize(sharp.ResizeOptions{Width: 160, Height: 120})
	// Branch into two formats from same recorder.
	jpegPipe := base.Clone().JPEG(format.JPEGOptions{Quality: 60})
	pngPipe := base.Clone().PNG(format.PNGOptions{})

	_, j, err := jpegPipe.ToBytes(context.Background())
	if err != nil {
		t.Fatalf("jpeg: %v", err)
	}
	_, p, err := pngPipe.ToBytes(context.Background())
	if err != nil {
		t.Fatalf("png: %v", err)
	}
	if j.Format != "jpeg" || p.Format != "png" {
		t.Errorf("formats = %q / %q, want jpeg / png", j.Format, p.Format)
	}
	if j.Width != 160 || p.Width != 160 {
		t.Errorf("dims = %d / %d, both want 160", j.Width, p.Width)
	}
}
