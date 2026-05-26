package sharp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func loadAnimatedFixture(t *testing.T) []byte {
	t.Helper()
	candidates := []string{
		"animated-loop-3.gif",
		"Crash_test.gif",
	}
	for _, name := range candidates {
		p := filepath.Join("test", "fixtures", name)
		if _, err := os.Stat(p); err == nil {
			b, _ := os.ReadFile(p)
			return b
		}
	}
	t.Skip("no animated GIF fixture available")
	return nil
}

func TestAnimatedLoadAllPages(t *testing.T) {
	in := loadAnimatedFixture(t)
	md, err := sharp.FromBytes(in).
		Animated().
		Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Pages < 2 {
		t.Errorf("animated Pages = %d, want >= 2", md.Pages)
	}
}

func TestAnimatedRoundtripGIF(t *testing.T) {
	in := loadAnimatedFixture(t)
	out, info, err := sharp.FromBytes(in).
		Animated().
		GIF(format.GIFOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("animated GIF roundtrip: %v", err)
	}
	if info.Format != "gif" {
		t.Errorf("Format = %q, want gif", info.Format)
	}

	// Verify output is still animated (multiple pages).
	md, err := sharp.FromBytes(out).Animated().Metadata(context.Background())
	if err != nil {
		t.Fatalf("output Metadata: %v", err)
	}
	if md.Pages < 2 {
		t.Errorf("output Pages = %d, want >= 2 (lost frames)", md.Pages)
	}
}

func TestFirstPageOnly(t *testing.T) {
	in := loadAnimatedFixture(t)
	// Default behaviour: load only first frame.
	md, err := sharp.FromBytes(in).Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata: %v", err)
	}
	if md.Pages < 1 {
		t.Errorf("Pages = %d, want >= 1", md.Pages)
	}
}

func TestPageIndex(t *testing.T) {
	in := loadAnimatedFixture(t)
	// Load second page only.
	_, info, err := sharp.FromBytes(in).
		Page(1).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Page(1): %v", err)
	}
	if info.Width <= 0 || info.Height <= 0 {
		t.Errorf("Page(1) dims = %dx%d", info.Width, info.Height)
	}
}
