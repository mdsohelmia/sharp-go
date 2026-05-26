package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
)

func TestMetadataJPEG(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	md, err := sharp.FromBytes(in).Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Format != "jpeg" {
		t.Errorf("Format = %q, want jpeg", md.Format)
	}
	if md.Width != 320 || md.Height != 240 {
		t.Errorf("dims = %dx%d, want 320x240", md.Width, md.Height)
	}
	if md.Channels != 3 {
		t.Errorf("Channels = %d, want 3", md.Channels)
	}
	if md.Space != "srgb" {
		t.Errorf("Space = %q, want srgb", md.Space)
	}
	if md.Depth != "uchar" {
		t.Errorf("Depth = %q, want uchar", md.Depth)
	}
	if md.Pages != 1 {
		t.Errorf("Pages = %d, want 1", md.Pages)
	}
}

func TestMetadataPNG(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands.png")
	md, err := sharp.FromBytes(in).Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Format != "png" {
		t.Errorf("Format = %q, want png", md.Format)
	}
	if md.Width <= 0 || md.Height <= 0 {
		t.Errorf("dims = %dx%d, want positive", md.Width, md.Height)
	}
}

func TestMetadataNoDecode(t *testing.T) {
	// Metadata on a huge file should be fast — it reads only the header.
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	md, err := sharp.FromBytes(in).Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Format != "jpeg" {
		t.Errorf("Format = %q, want jpeg", md.Format)
	}
	if md.Width != 2725 || md.Height != 2225 {
		t.Errorf("dims = %dx%d, want 2725x2225", md.Width, md.Height)
	}
}

func TestStats(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	st, err := sharp.FromBytes(in).Stats(context.Background())
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if len(st.Channels) != 3 {
		t.Fatalf("Channels = %d, want 3", len(st.Channels))
	}
	for i, c := range st.Channels {
		if c.Min < 0 || c.Max > 255 {
			t.Errorf("channel %d out of range: min=%v max=%v", i, c.Min, c.Max)
		}
		if c.Mean < 0 || c.Mean > 255 {
			t.Errorf("channel %d mean out of range: %v", i, c.Mean)
		}
		if c.Deviation < 0 {
			t.Errorf("channel %d negative deviation: %v", i, c.Deviation)
		}
	}
}
