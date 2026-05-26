// Format converter: read any supported input, encode to all popular output
// formats, print each output's size.
//
// Usage:
//   go run ./examples/format-convert input.jpg
package main

import (
	"context"
	"fmt"
	"os"

	sharp "github.com/mdsohelmia/sharp-go"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: format-convert <input>")
		os.Exit(2)
	}
	in := os.Args[1]
	buf, err := os.ReadFile(in)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}

	targets := []sharp.FormatID{
		sharp.FormatJPEG, sharp.FormatPNG, sharp.FormatWebP,
		sharp.FormatAVIF, sharp.FormatTIFF, sharp.FormatJXL,
	}

	ctx := context.Background()
	for _, f := range targets {
		out, info, err := sharp.FromBytes(buf).
			Resize(sharp.ResizeOptions{Width: 800}).
			ToFormat(f, nil).
			ToBytes(ctx)
		if err != nil {
			fmt.Printf("  %-6s: skip (%v)\n", f, err)
			continue
		}
		fmt.Printf("  %-6s: %dx%d %d bytes\n", info.Format, info.Width, info.Height, len(out))
	}
}
