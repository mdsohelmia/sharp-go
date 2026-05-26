package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

// CPU-bound (large image, heavy ops). On a 10-core M4 with libvips
// concurrency=10 by default, batch parallelism oversubscribes for small
// images but wins for big ones because libvips spends more time per-op.
func BenchmarkToBytesAll8(b *testing.B)  { benchToBytesAll(b, 8) }
func BenchmarkToBytesAll16(b *testing.B) { benchToBytesAll(b, 16) }

// Batched processing with libvips concurrency dropped to 1 — turns
// parallelism control entirely over to the Go-side worker pool.
func BenchmarkToBytesAll8Single(b *testing.B) {
	orig := sharp.Concurrency()
	defer sharp.SetConcurrency(orig)
	sharp.SetConcurrency(1)
	benchToBytesAll(b, 8)
}

func benchToBytesAll(b *testing.B, concurrency int) {
	in := readFixtureB(b, "320x240.jpg")
	const batch = 32
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		images := make([]*sharp.Image, batch)
		for k := range images {
			images[k] = sharp.FromBytes(in).
				Resize(sharp.ResizeOptions{Width: 160, Height: 120}).
				JPEG(format.JPEGOptions{Quality: 80})
		}
		results := sharp.ToBytesAll(ctx, images, sharp.BatchOptions{Concurrency: concurrency})
		for _, r := range results {
			if r.Err != nil {
				b.Fatal(r.Err)
			}
		}
	}
}

func BenchmarkSequentialResize32(b *testing.B) {
	in := readFixtureB(b, "320x240.jpg")
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for k := 0; k < 32; k++ {
			_, _, err := sharp.FromBytes(in).
				Resize(sharp.ResizeOptions{Width: 160, Height: 120}).
				JPEG(format.JPEGOptions{Quality: 80}).
				ToBytes(ctx)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
