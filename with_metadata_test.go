package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestWithMetadataOrientation(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	out, _, err := sharp.FromBytes(in).
		WithMetadata(sharp.WithMetadataOptions{Orientation: 6}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("WithMetadata: %v", err)
	}
	md, err := sharp.FromBytes(out).Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Orientation != 6 {
		t.Errorf("Orientation = %d, want 6", md.Orientation)
	}
}

func TestWithMetadataDensity(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	out, _, err := sharp.FromBytes(in).
		WithMetadata(sharp.WithMetadataOptions{Density: 300}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("WithMetadata: %v", err)
	}
	md, err := sharp.FromBytes(out).Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Density < 290 || md.Density > 310 {
		t.Errorf("Density = %v, want ~300", md.Density)
	}
}

func TestWithExif(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	out, _, err := sharp.FromBytes(in).
		WithExif(sharp.WithExifOptions{
			Tags: map[string]string{
				"exif-ifd0-Make":     "sharp-go",
				"exif-ifd0-Software": "test",
			},
		}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("WithExif: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("empty output")
	}
}

func TestWithXmp(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	xmp := []byte(`<?xpacket begin="\xef\xbb\xbf" id="W5M0MpCehiHzreSzNTczkc9d"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/">
      <dc:creator><rdf:Seq><rdf:li>sharp-go test</rdf:li></rdf:Seq></dc:creator>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>
<?xpacket end="w"?>`)
	out, _, err := sharp.FromBytes(in).
		WithXmp(sharp.WithXmpOptions{XmpPacket: xmp}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("WithXmp: %v", err)
	}
	if len(out) == 0 {
		t.Errorf("empty output")
	}
}

func TestWithICCProfileSRGB(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		WithICCProfile(sharp.WithICCProfileOptions{Profile: "srgb"}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Skipf("WithICCProfile srgb: %v (libvips may lack icc support)", err)
	}
}
