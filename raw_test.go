package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestRawRoundtrip(t *testing.T) {
	// Decode a JPEG, emit raw pixels, then re-import via FromRawBytes and
	// confirm dims survive.
	in := readFixture(t, "320x240.jpg")
	pixels, info, err := sharp.FromBytes(in).
		Raw(format.RawOptions{Depth: format.RawDepthUchar}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Raw out: %v", err)
	}
	expectedBytes := info.Width * info.Height * info.Channels
	if len(pixels) != expectedBytes {
		t.Errorf("raw pixel length = %d, want %d", len(pixels), expectedBytes)
	}

	// Round-trip through FromRawBytes.
	_, info2, err := sharp.FromRawBytes(pixels, format.RawInput{
		Width:    info.Width,
		Height:   info.Height,
		Channels: info.Channels,
		Depth:    format.RawDepthUchar,
	}).PNG(format.PNGOptions{}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Raw in: %v", err)
	}
	if info2.Width != info.Width || info2.Height != info.Height {
		t.Errorf("after raw roundtrip dims = %dx%d, want %dx%d",
			info2.Width, info2.Height, info.Width, info.Height)
	}
}
