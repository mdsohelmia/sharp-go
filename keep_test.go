package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func TestKeepEXIFStripsByDefault(t *testing.T) {
	in := readFixture(t, "Landscape_2.jpg") // has EXIF orientation tag
	out, _, err := sharp.FromBytes(in).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	// Re-read metadata; EXIF should be stripped.
	md, err := sharp.FromBytes(out).Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	// Stripped EXIF means no orientation tag or the libvips default of 1.
	// The point is: the original orientation (2) was not preserved.
	if md.Orientation != 0 && md.Orientation != 1 {
		t.Errorf("default strip: Orientation = %d, want 0 or 1 (stripped)", md.Orientation)
	}
}

func TestKeepEXIFPreserves(t *testing.T) {
	in := readFixture(t, "Landscape_2.jpg")
	// First read original orientation.
	srcMd, err := sharp.FromBytes(in).Metadata(context.Background())
	if err != nil {
		t.Fatalf("src Metadata: %v", err)
	}
	if srcMd.Orientation == 0 {
		t.Skip("fixture lacks EXIF orientation; skipping")
	}

	out, _, err := sharp.FromBytes(in).
		KeepExif().
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	md, err := sharp.FromBytes(out).Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Orientation != srcMd.Orientation {
		t.Errorf("KeepExif: Orientation = %d, want %d", md.Orientation, srcMd.Orientation)
	}
}

func TestKeepMetadata(t *testing.T) {
	in := readFixture(t, "Landscape_2.jpg")
	srcMd, err := sharp.FromBytes(in).Metadata(context.Background())
	if err != nil {
		t.Fatalf("src Metadata: %v", err)
	}
	if srcMd.Orientation == 0 {
		t.Skip("fixture lacks EXIF orientation; skipping")
	}

	out, _, err := sharp.FromBytes(in).
		KeepMetadata().
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	md, err := sharp.FromBytes(out).Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Orientation != srcMd.Orientation {
		t.Errorf("KeepMetadata: Orientation = %d, want %d", md.Orientation, srcMd.Orientation)
	}
}
