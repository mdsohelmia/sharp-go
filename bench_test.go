package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

// readFixtureB is a benchmark-friendly fixture loader.
func readFixtureB(b *testing.B, name string) []byte {
	b.Helper()
	data, err := readFixtureBytes(name)
	if err != nil {
		b.Skipf("fixture %s missing: %v", name, err)
	}
	return data
}

func BenchmarkResizeJPEGSmall(b *testing.B) {
	in := readFixtureB(b, "320x240.jpg")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 160, Height: 120}).
			JPEG(format.JPEGOptions{Quality: 80}).
			ToBytes(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResizeJPEGLarge(b *testing.B) {
	in := readFixtureB(b, "2569067123_aca715a2ee_o.jpg")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 800, Height: 600}).
			JPEG(format.JPEGOptions{Quality: 80}).
			ToBytes(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMetadataOnly(b *testing.B) {
	in := readFixtureB(b, "2569067123_aca715a2ee_o.jpg")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := sharp.FromBytes(in).Metadata(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkResizeChainOps(b *testing.B) {
	in := readFixtureB(b, "320x240.jpg")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 200, Height: 150}).
			Gamma(sharp.GammaOptions{Exponent: 2.2}).
			Blur(sharp.BlurOptions{Sigma: 1.0}).
			Sharpen(sharp.SharpenOptions{}).
			WebP(format.WebPOptions{Quality: 80}).
			ToBytes(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParallelResize(b *testing.B) {
	in := readFixtureB(b, "320x240.jpg")
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		ctx := context.Background()
		for pb.Next() {
			_, _, err := sharp.FromBytes(in).
				Resize(sharp.ResizeOptions{Width: 160, Height: 120}).
				JPEG(format.JPEGOptions{Quality: 80}).
				ToBytes(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkResizeJPEGPooled mirrors BenchmarkResizeJPEGSmall but recycles the
// encoded output via sharp.Release — confirms zero per-op encode allocation
// on the hot path when callers opt in.
func BenchmarkResizeJPEGPooled(b *testing.B) {
	in := readFixtureB(b, "320x240.jpg")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, _, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 160, Height: 120}).
			JPEG(format.JPEGOptions{Quality: 80}).
			ToBytes(ctx)
		if err != nil {
			b.Fatal(err)
		}
		sharp.Release(out)
	}
}

// BenchmarkResizeJPEGFullyPooled combines sharp.Release with explicit handle
// recycling — the recommended hot-path pattern for max-throughput servers.
func BenchmarkResizeJPEGFullyPooled(b *testing.B) {
	in := readFixtureB(b, "320x240.jpg")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		im := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 160, Height: 120}).
			JPEG(format.JPEGOptions{Quality: 80})
		out, _, err := im.ToBytes(ctx)
		if err != nil {
			b.Fatal(err)
		}
		sharp.Release(out)
		im.Recycle()
	}
}

func BenchmarkCompositeOverlay(b *testing.B) {
	base := readFixtureB(b, "320x240.jpg")
	overlay := readFixtureB(b, "Flag_of_the_Netherlands.png")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := sharp.FromBytes(base).
			Composite([]sharp.CompositeLayer{
				{Input: overlay, Gravity: sharp.GravityCentre, Blend: sharp.BlendOver},
			}).
			JPEG(format.JPEGOptions{}).
			ToBytes(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPreparedOverlay reuses a single pre-decoded overlay across all
// composite operations — the canonical hot-path pattern for watermark
// workloads. Compare against BenchmarkCompositeOverlay above to see the
// per-call decode cost the prepared path avoids.
func BenchmarkPreparedOverlay(b *testing.B) {
	base := readFixtureB(b, "320x240.jpg")
	overlay := readFixtureB(b, "Flag_of_the_Netherlands.png")
	prep, err := sharp.PrepareOverlay(overlay)
	if err != nil {
		b.Fatalf("PrepareOverlay: %v", err)
	}
	defer prep.Close()
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		out, _, err := sharp.FromBytes(base).
			Composite([]sharp.CompositeLayer{
				{Prepared: prep, Gravity: sharp.GravityCentre, Blend: sharp.BlendOver},
			}).
			JPEG(format.JPEGOptions{}).
			ToBytes(ctx)
		if err != nil {
			b.Fatal(err)
		}
		sharp.Release(out)
	}
}
