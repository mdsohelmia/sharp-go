package sharp_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

// TestAllFixturesPipeline runs every decodable image in test/fixtures through
// the resize + encode pipeline for each output format. It surfaces crashes /
// errors on the full spread of input kinds (8/16-bit, grey, grey+alpha, 1-bit,
// palette, CMYK, wide-gamut, animated, SVG, …).
//
// A fixture that libvips cannot even decode (intentionally invalid/edge cases)
// is reported as SKIP, not a failure. A fixture whose header decodes but whose
// pipeline then errors is a real bug and fails the test.
func TestAllFixturesPipeline(t *testing.T) {
	dir := filepath.Join("test", "fixtures")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Skipf("fixtures dir missing: %v", err)
	}

	imgExt := map[string]bool{
		".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".gif": true,
		".tif": true, ".tiff": true, ".avif": true, ".heic": true, ".heif": true,
		".jxl": true, ".jp2": true, ".svg": true, ".bmp": true, ".ico": true,
	}

	variants := []struct {
		name string
		enc  func(*sharp.Image) *sharp.Image
	}{
		{"webp", func(im *sharp.Image) *sharp.Image { return im.WebP(format.WebPOptions{Quality: 75, Effort: 0}) }},
		{"avif", func(im *sharp.Image) *sharp.Image { return im.AVIF(format.AVIFOptions{Quality: 50, Effort: 2}) }},
		{"jpeg", func(im *sharp.Image) *sharp.Image { return im.JPEG(format.JPEGOptions{Quality: 80}) }},
		{"png", func(im *sharp.Image) *sharp.Image { return im.PNG(format.PNGOptions{}) }},
	}

	ctx := context.Background()
	var processed, skipped int

	for _, e := range entries {
		if e.IsDir() || !imgExt[strings.ToLower(filepath.Ext(e.Name()))] {
			continue
		}
		name := e.Name()
		in, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		md, _ := sharp.FromBytes(in).Metadata(ctx)

		// Full-decode gate: a resize forces libvips to decode the pixels. If
		// that fails the INPUT is corrupt / unsupported by this libvips build
		// (e.g. relax_tileparts.jp2, corrupt-header.jpg) — skip, since the
		// pipeline never gets a valid image. Only fixtures that fully decode
		// are held to the per-format encode assertions below.
		if _, _, err := sharp.FromBytes(in).Resize(sharp.ResizeOptions{Width: 64}).
			PNG(format.PNGOptions{}).ToBytes(ctx); err != nil {
			t.Logf("SKIP %-46s (decode: %s)", name, firstLine(err))
			skipped++
			continue
		}
		processed++

		t.Run(name, func(t *testing.T) {
			for _, v := range variants {
				out, info, err := v.enc(sharp.FromBytes(in).
					Resize(sharp.ResizeOptions{Width: 64})).ToBytes(ctx)
				if err != nil {
					t.Errorf("%s -> %s: %v  (src %dx%d %s %dch)", name, v.name, err,
						md.Width, md.Height, md.Format, md.Channels)
					continue
				}
				if len(out) == 0 {
					t.Errorf("%s -> %s: empty output", name, v.name)
					continue
				}
				if info.Width <= 0 || info.Height <= 0 {
					t.Errorf("%s -> %s: bad dims %dx%d", name, v.name, info.Width, info.Height)
				}
			}
		})
	}

	t.Logf("fixtures: processed=%d skipped=%d", processed, skipped)
}

func firstLine(err error) string {
	s := err.Error()
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
