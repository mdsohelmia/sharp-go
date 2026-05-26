// Watermark example: composite a PNG overlay onto a JPEG base in the
// bottom-right corner with 50% opacity.
//
// Usage:
//   go run ./examples/watermark base.jpg overlay.png out.jpg
package main

import (
	"context"
	"fmt"
	"os"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: watermark <base> <overlay> <output>")
		os.Exit(2)
	}
	base, overlay, out := os.Args[1], os.Args[2], os.Args[3]

	overlayBytes, err := os.ReadFile(overlay)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read overlay:", err)
		os.Exit(1)
	}

	// Pre-multiply the overlay alpha to 50% via Linear on the alpha band.
	overlayHalf, _, err := sharp.FromBytes(overlayBytes).
		EnsureAlpha(sharp.EnsureAlphaOptions{Alpha: 1}).
		Linear(sharp.LinearOptions{
			A: []float64{1, 1, 1, 0.5},
			B: []float64{0, 0, 0, 0},
		}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "prep overlay:", err)
		os.Exit(1)
	}

	info, err := sharp.FromFile(base).
		Composite([]sharp.CompositeLayer{
			{Input: overlayHalf, Gravity: sharp.GravitySouthEast, Blend: sharp.BlendOver},
		}).
		JPEG(format.JPEGOptions{Quality: 85}).
		ToFile(context.Background(), out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "watermark:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s: %dx%d (%d bytes)\n", out, info.Width, info.Height, info.Size)
}
