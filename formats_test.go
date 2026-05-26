package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestTIFFEncode(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	out, info, err := sharp.FromBytes(in).
		TIFF(format.TIFFOptions{
			Compression: format.TIFFCompressionDeflate,
			Quality:     80,
		}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("TIFF: %v", err)
	}
	if info.Format != "tiff" {
		t.Errorf("Format = %q, want tiff", info.Format)
	}
	// TIFF little-endian magic "II\x2a\x00" or big-endian "MM\x00\x2a"
	if !(out[0] == 'I' && out[1] == 'I' || out[0] == 'M' && out[1] == 'M') {
		t.Errorf("not a TIFF: %x", out[:4])
	}
}

func TestTIFFTiled(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	_, info, err := sharp.FromBytes(in).
		TIFF(format.TIFFOptions{
			Compression: format.TIFFCompressionLZW,
			Tile:        true,
			TileWidth:   256,
			TileHeight:  256,
		}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("TIFF tiled: %v", err)
	}
	if info.Format != "tiff" {
		t.Errorf("Format = %q, want tiff", info.Format)
	}
}

func TestAVIFEncode(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	out, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 160, Height: 120}).
		AVIF(format.AVIFOptions{Quality: 50}).
		ToBytes(context.Background())
	if err != nil {
		t.Skipf("AVIF: %v (libvips may lack aom/svt encoder)", err)
	}
	if info.Format != "avif" {
		t.Errorf("Format = %q, want avif", info.Format)
	}
	if len(out) < 100 {
		t.Errorf("AVIF output suspiciously small: %d", len(out))
	}
}

func TestHEIFEncode(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 160, Height: 120}).
		HEIF(format.HEIFOptions{
			Compression: format.HEIFCompressionHEVC,
			Quality:     50,
		}).
		ToBytes(context.Background())
	if err != nil {
		t.Skipf("HEIF: %v (libvips may lack x265)", err)
	}
	if info.Format != "heif" {
		t.Errorf("Format = %q, want heif", info.Format)
	}
}

func TestTIFFExtensionInference(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	dir := t.TempDir()
	info, err := sharp.FromBytes(in).ToFile(context.Background(), dir+"/out.tiff")
	if err != nil {
		t.Fatalf("ToFile: %v", err)
	}
	if info.Format != "tiff" {
		t.Errorf("Format = %q, want tiff", info.Format)
	}
}

func TestJXLEncode(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	out, info, err := sharp.FromBytes(in).
		JXL(format.JXLOptions{Quality: 80}).
		ToBytes(context.Background())
	if err != nil {
		t.Skipf("JXL: %v (libvips may lack libjxl)", err)
	}
	if info.Format != "jxl" {
		t.Errorf("Format = %q, want jxl", info.Format)
	}
	if len(out) < 100 {
		t.Errorf("JXL output suspiciously small: %d", len(out))
	}
}

func TestJP2Encode(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	out, info, err := sharp.FromBytes(in).
		JP2(format.JP2Options{Quality: 50}).
		ToBytes(context.Background())
	if err != nil {
		t.Skipf("JP2: %v (libvips may lack libopenjp2)", err)
	}
	if info.Format != "jp2" {
		t.Errorf("Format = %q, want jp2", info.Format)
	}
	if len(out) < 100 {
		t.Errorf("JP2 output suspiciously small: %d", len(out))
	}
}

func TestJXLLossless(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands.png")
	_, info, err := sharp.FromBytes(in).
		JXL(format.JXLOptions{Lossless: true, Effort: 3}).
		ToBytes(context.Background())
	if err != nil {
		t.Skipf("JXL lossless: %v", err)
	}
	if info.Format != "jxl" {
		t.Errorf("Format = %q, want jxl", info.Format)
	}
}

func TestAVIFExtensionInference(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	dir := t.TempDir()
	info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 100, Height: 80}).
		ToFile(context.Background(), dir+"/out.avif")
	if err != nil {
		t.Skipf("AVIF: %v", err)
	}
	if info.Format != "avif" {
		t.Errorf("Format = %q, want avif", info.Format)
	}
}
