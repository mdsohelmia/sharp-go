package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestWebPEncode(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	out, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 400, Height: 300}).
		WebP(format.WebPOptions{Quality: 80}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Format != "webp" {
		t.Errorf("Format = %q, want webp", info.Format)
	}
	// WebP magic: "RIFF" .... "WEBP"
	if string(out[0:4]) != "RIFF" || string(out[8:12]) != "WEBP" {
		t.Errorf("output not WebP")
	}
}

func TestWebPLossless(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands.png")
	out, info, err := sharp.FromBytes(in).
		WebP(format.WebPOptions{Lossless: true}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Format != "webp" {
		t.Errorf("Format = %q, want webp", info.Format)
	}
	if len(out) == 0 {
		t.Errorf("empty output")
	}
}

func TestGIFEncode(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands.png")
	out, info, err := sharp.FromBytes(in).
		GIF(format.GIFOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if info.Format != "gif" {
		t.Errorf("Format = %q, want gif", info.Format)
	}
	if string(out[0:3]) != "GIF" {
		t.Errorf("output not GIF")
	}
}

func TestWebPExtensionInference(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	dir := t.TempDir()
	path := dir + "/out.webp"
	info, err := sharp.FromBytes(in).ToFile(context.Background(), path)
	if err != nil {
		t.Fatalf("ToFile: %v", err)
	}
	if info.Format != "webp" {
		t.Errorf("Format = %q, want webp", info.Format)
	}
}
