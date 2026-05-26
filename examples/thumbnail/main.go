// Thumbnail generator: resize an input file to 320x240 (cover) and write
// the result as a quality-80 JPEG.
//
// Usage:
//   go run ./examples/thumbnail input.jpg output.jpg
package main

import (
	"context"
	"fmt"
	"os"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: thumbnail <input> <output>")
		os.Exit(2)
	}
	in, out := os.Args[1], os.Args[2]

	info, err := sharp.FromFile(in).
		Resize(sharp.ResizeOptions{Width: 320, Height: 240, Fit: sharp.FitCover}).
		JPEG(format.JPEGOptions{Quality: 80, MozJPEG: true}).
		ToFile(context.Background(), out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "thumbnail:", err)
		os.Exit(1)
	}
	fmt.Printf("wrote %s: %dx%d %s (%d bytes)\n",
		out, info.Width, info.Height, info.Format, info.Size)
}
