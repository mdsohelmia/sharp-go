package sharp_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

// TestPerfSweep prints an encode timing matrix (width x format) at the default
// effort, plus a decode floor and a WebP effort ladder. Gated behind PERF=1.
//
//	PERF=1 go test -run TestPerfSweep -v .
func TestPerfSweep(t *testing.T) {
	if os.Getenv("PERF") == "" {
		t.Skip("set PERF=1 to run")
	}
	png := readFixture(t, "2569067123_aca715a2ee_o.png")
	ctx := context.Background()
	md, _ := sharp.FromBytes(png).Metadata(ctx)
	t.Logf("source %dx%d, %d bytes\n", md.Width, md.Height, len(png))

	bench := func(name string, mk func() *sharp.Image) {
		if _, _, err := mk().ToBytes(ctx); err != nil {
			t.Fatalf("%s warm: %v", name, err)
		}
		best := time.Hour
		var sz int
		for range 7 {
			s := time.Now()
			out, _, err := mk().ToBytes(ctx)
			d := time.Since(s)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if d < best {
				best = d
			}
			sz = len(out)
		}
		mark := ""
		if best > 300*time.Millisecond {
			mark = "  >300ms"
		}
		t.Logf("%-22s %7.1fms  %8d B%s", name, float64(best.Microseconds())/1000, sz, mark)
	}

	bench("DECODE floor w16", func() *sharp.Image {
		return sharp.FromBytes(png).Resize(sharp.ResizeOptions{Width: 16}).
			WebP(format.WebPOptions{Quality: 75, Effort: 0})
	})

	_ = fmt.Sprint
	resize := func(w int) *sharp.Image {
		im := sharp.FromBytes(png)
		if w > 0 {
			im = im.Resize(sharp.ResizeOptions{Width: w})
		}
		return im
	}

	// WebP libwebp-direct (sharpYUV) path — single-thread vs multithread.
	bench("webp1920 syuv e4 st", func() *sharp.Image {
		return resize(1920).WebP(format.WebPOptions{Quality: 75, Effort: 4, UseSharpYUV: true})
	})
	bench("webp1920 syuv e4 MT", func() *sharp.Image {
		return resize(1920).WebP(format.WebPOptions{Quality: 75, Effort: 4, UseSharpYUV: true, Multithread: true})
	})
	bench("webpFULL syuv e4 MT", func() *sharp.Image {
		return resize(0).WebP(format.WebPOptions{Quality: 75, Effort: 4, UseSharpYUV: true, Multithread: true})
	})

	// AVIF effort sweet spot.
	bench("avif1920 e2", func() *sharp.Image {
		return resize(1920).AVIF(format.AVIFOptions{Quality: 50, Effort: 2})
	})
	bench("avif1920 e3", func() *sharp.Image {
		return resize(1920).AVIF(format.AVIFOptions{Quality: 50, Effort: 3})
	})
	bench("avifFULL e2", func() *sharp.Image {
		return resize(0).AVIF(format.AVIFOptions{Quality: 50, Effort: 2})
	})
}
