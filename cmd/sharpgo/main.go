// sharpgo is a CLI front-end to sharp-go for batch image processing from the
// shell.
//
// Subcommands:
//   resize     scale + crop input to target dimensions
//   convert    re-encode input to a different format
//   metadata   print Metadata as JSON
//   composite  overlay one image onto another
//   info       libvips environment summary (alias for sharpgo-doctor)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	cmd := os.Args[1]
	args := os.Args[2:]
	var err error
	switch cmd {
	case "resize":
		err = cmdResize(args)
	case "convert":
		err = cmdConvert(args)
	case "metadata":
		err = cmdMetadata(args)
	case "composite":
		err = cmdComposite(args)
	case "info":
		err = cmdInfo(args)
	case "-h", "--help", "help":
		usage()
		return
	default:
		fmt.Fprintf(os.Stderr, "sharpgo: unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "sharpgo:", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `sharpgo — image processing via libvips

Usage:
  sharpgo resize    [-w N] [-h N] [--fit cover|contain|fill|inside|outside] [--position centre|north|...] [-q N] INPUT OUTPUT
  sharpgo convert   [--format jpeg|png|webp|avif|...] [-q N] [--lossless] INPUT OUTPUT
  sharpgo metadata  INPUT
  sharpgo composite --overlay PATH [--gravity centre|north|...] [--blend over|multiply|...] INPUT OUTPUT
  sharpgo info

Run 'sharpgo <cmd> -h' for subcommand-specific flags.
`)
}

// ---------- resize ----------

func cmdResize(args []string) error {
	fs := flag.NewFlagSet("resize", flag.ContinueOnError)
	w := fs.Int("w", 0, "target width (0 = preserve aspect)")
	h := fs.Int("h", 0, "target height (0 = preserve aspect)")
	fit := fs.String("fit", "cover", "cover|contain|fill|inside|outside")
	pos := fs.String("position", "centre", "centre|entropy|attention|north|northeast|east|southeast|south|southwest|west|northwest")
	q := fs.Int("q", 80, "JPEG/WebP/AVIF quality 1-100")
	keep := fs.Bool("keep-metadata", false, "preserve EXIF/ICC/XMP")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("resize: expected INPUT OUTPUT, got %d positional args", fs.NArg())
	}
	in, out := fs.Arg(0), fs.Arg(1)

	pipe := sharp.FromFile(in).
		Resize(sharp.ResizeOptions{
			Width:    *w,
			Height:   *h,
			Fit:      parseFit(*fit),
			Position: parsePosition(*pos),
		})
	if *keep {
		pipe = pipe.KeepMetadata()
	}
	pipe = applyFormatFromExt(pipe, out, *q, false)

	info, err := pipe.ToFile(context.Background(), out)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %dx%d %s (%d bytes)\n", out, info.Width, info.Height, info.Format, info.Size)
	return nil
}

// ---------- convert ----------

func cmdConvert(args []string) error {
	fs := flag.NewFlagSet("convert", flag.ContinueOnError)
	fmtStr := fs.String("format", "", "output format (default: infer from output extension)")
	q := fs.Int("q", 80, "quality 1-100")
	lossless := fs.Bool("lossless", false, "lossless mode (WebP/JXL/HEIF)")
	keep := fs.Bool("keep-metadata", false, "preserve EXIF/ICC/XMP")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("convert: expected INPUT OUTPUT, got %d positional args", fs.NArg())
	}
	in, out := fs.Arg(0), fs.Arg(1)

	pipe := sharp.FromFile(in)
	if *keep {
		pipe = pipe.KeepMetadata()
	}

	if *fmtStr != "" {
		pipe = applyFormatByName(pipe, *fmtStr, *q, *lossless)
	} else {
		pipe = applyFormatFromExt(pipe, out, *q, *lossless)
	}

	info, err := pipe.ToFile(context.Background(), out)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %s %d bytes\n", out, info.Format, info.Size)
	return nil
}

// ---------- metadata ----------

func cmdMetadata(args []string) error {
	fs := flag.NewFlagSet("metadata", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("metadata: expected INPUT, got %d positional args", fs.NArg())
	}
	md, err := sharp.FromFile(fs.Arg(0)).Metadata(context.Background())
	if err != nil {
		return err
	}
	// Suppress raw blobs — just expose their sizes.
	out := struct {
		Format        string  `json:"format"`
		Width         int     `json:"width"`
		Height        int     `json:"height"`
		Space         string  `json:"space"`
		Channels      int     `json:"channels"`
		Depth         string  `json:"depth"`
		Density       float64 `json:"density"`
		HasAlpha      bool    `json:"hasAlpha"`
		HasProfile    bool    `json:"hasProfile"`
		Orientation   int     `json:"orientation"`
		Pages         int     `json:"pages"`
		IsProgressive bool    `json:"isProgressive"`
		ExifLen       int     `json:"exifLen"`
		IccLen        int     `json:"iccLen"`
		XmpLen        int     `json:"xmpLen"`
		IptcLen       int     `json:"iptcLen"`
	}{
		Format: md.Format, Width: md.Width, Height: md.Height, Space: md.Space,
		Channels: md.Channels, Depth: md.Depth, Density: md.Density,
		HasAlpha: md.HasAlpha, HasProfile: md.HasProfile,
		Orientation: md.Orientation, Pages: md.Pages, IsProgressive: md.IsProgressive,
		ExifLen: len(md.Exif), IccLen: len(md.ICC),
		XmpLen: len(md.XMP), IptcLen: len(md.IPTC),
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// ---------- composite ----------

func cmdComposite(args []string) error {
	fs := flag.NewFlagSet("composite", flag.ContinueOnError)
	overlay := fs.String("overlay", "", "overlay image path (required)")
	gravity := fs.String("gravity", "southeast", "centre|north|northeast|east|southeast|south|southwest|west|northwest")
	blend := fs.String("blend", "over", "over|multiply|screen|overlay|...")
	q := fs.Int("q", 85, "JPEG quality for the output")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 2 {
		return fmt.Errorf("composite: expected INPUT OUTPUT, got %d positional args", fs.NArg())
	}
	if *overlay == "" {
		return fmt.Errorf("composite: --overlay is required")
	}
	overlayBytes, err := os.ReadFile(*overlay)
	if err != nil {
		return fmt.Errorf("read overlay: %w", err)
	}
	in, out := fs.Arg(0), fs.Arg(1)

	pipe := sharp.FromFile(in).
		Composite([]sharp.CompositeLayer{{
			Input:   overlayBytes,
			Gravity: parseGravity(*gravity),
			Blend:   parseBlend(*blend),
		}})
	pipe = applyFormatFromExt(pipe, out, *q, false)

	info, err := pipe.ToFile(context.Background(), out)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %dx%d %s (%d bytes)\n", out, info.Width, info.Height, info.Format, info.Size)
	return nil
}

// ---------- info ----------

func cmdInfo(_ []string) error {
	v := sharp.V()
	fmt.Printf("sharp-go (libvips %d.%d.%d, concurrency %d)\n", v.Major, v.Minor, v.Micro, sharp.Concurrency())
	fmt.Println()
	fmt.Println("Format    Load   Save")
	fmt.Println("-------   ----   ----")
	for _, name := range []string{"jpeg", "png", "webp", "avif", "gif", "tiff", "heif", "jxl", "jp2", "svg", "pdf", "raw"} {
		s := sharp.SupportedFormats()[name]
		fmt.Printf("%-9s %-6s %-6s\n", name, yesno(s.Load), yesno(s.Save))
	}
	return nil
}

func yesno(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// ---------- format-by-name and -by-extension ----------

func applyFormatFromExt(pipe *sharp.Image, path string, q int, lossless bool) *sharp.Image {
	ext := strings.ToLower(path)
	switch {
	case strings.HasSuffix(ext, ".jpg") || strings.HasSuffix(ext, ".jpeg"):
		return pipe.JPEG(format.JPEGOptions{Quality: q})
	case strings.HasSuffix(ext, ".png"):
		return pipe.PNG(format.PNGOptions{})
	case strings.HasSuffix(ext, ".webp"):
		return pipe.WebP(format.WebPOptions{Quality: q, Lossless: lossless})
	case strings.HasSuffix(ext, ".avif"):
		return pipe.AVIF(format.AVIFOptions{Quality: q, Lossless: lossless})
	case strings.HasSuffix(ext, ".gif"):
		return pipe.GIF(format.GIFOptions{})
	case strings.HasSuffix(ext, ".tiff") || strings.HasSuffix(ext, ".tif"):
		return pipe.TIFF(format.TIFFOptions{Quality: q})
	case strings.HasSuffix(ext, ".heic") || strings.HasSuffix(ext, ".heif"):
		return pipe.HEIF(format.HEIFOptions{Quality: q, Lossless: lossless})
	case strings.HasSuffix(ext, ".jxl"):
		return pipe.JXL(format.JXLOptions{Quality: q, Lossless: lossless})
	case strings.HasSuffix(ext, ".jp2"):
		return pipe.JP2(format.JP2Options{Quality: q, Lossless: lossless})
	}
	// Default: JPEG.
	return pipe.JPEG(format.JPEGOptions{Quality: q})
}

func applyFormatByName(pipe *sharp.Image, name string, q int, lossless bool) *sharp.Image {
	switch strings.ToLower(name) {
	case "jpeg", "jpg":
		return pipe.JPEG(format.JPEGOptions{Quality: q})
	case "png":
		return pipe.PNG(format.PNGOptions{})
	case "webp":
		return pipe.WebP(format.WebPOptions{Quality: q, Lossless: lossless})
	case "avif":
		return pipe.AVIF(format.AVIFOptions{Quality: q, Lossless: lossless})
	case "gif":
		return pipe.GIF(format.GIFOptions{})
	case "tiff":
		return pipe.TIFF(format.TIFFOptions{Quality: q})
	case "heif", "heic":
		return pipe.HEIF(format.HEIFOptions{Quality: q, Lossless: lossless})
	case "jxl":
		return pipe.JXL(format.JXLOptions{Quality: q, Lossless: lossless})
	case "jp2":
		return pipe.JP2(format.JP2Options{Quality: q, Lossless: lossless})
	}
	return pipe.JPEG(format.JPEGOptions{Quality: q})
}

// ---------- enum parsers ----------

func parseFit(s string) sharp.Fit {
	switch strings.ToLower(s) {
	case "contain":
		return sharp.FitContain
	case "fill":
		return sharp.FitFill
	case "inside":
		return sharp.FitInside
	case "outside":
		return sharp.FitOutside
	}
	return sharp.FitCover
}

func parsePosition(s string) sharp.Position {
	switch strings.ToLower(s) {
	case "entropy":
		return sharp.PositionEntropy
	case "attention":
		return sharp.PositionAttention
	case "north", "top":
		return sharp.PositionNorth
	case "northeast", "topright":
		return sharp.PositionNorthEast
	case "east", "right":
		return sharp.PositionEast
	case "southeast", "bottomright":
		return sharp.PositionSouthEast
	case "south", "bottom":
		return sharp.PositionSouth
	case "southwest", "bottomleft":
		return sharp.PositionSouthWest
	case "west", "left":
		return sharp.PositionWest
	case "northwest", "topleft":
		return sharp.PositionNorthWest
	}
	return sharp.PositionCentre
}

func parseGravity(s string) sharp.Gravity {
	switch strings.ToLower(s) {
	case "north", "top":
		return sharp.GravityNorth
	case "northeast", "topright":
		return sharp.GravityNorthEast
	case "east", "right":
		return sharp.GravityEast
	case "southeast", "bottomright":
		return sharp.GravitySouthEast
	case "south", "bottom":
		return sharp.GravitySouth
	case "southwest", "bottomleft":
		return sharp.GravitySouthWest
	case "west", "left":
		return sharp.GravityWest
	case "northwest", "topleft":
		return sharp.GravityNorthWest
	}
	return sharp.GravityCentre
}

func parseBlend(s string) sharp.BlendMode {
	switch strings.ToLower(s) {
	case "clear":
		return sharp.BlendClear
	case "source":
		return sharp.BlendSource
	case "multiply":
		return sharp.BlendMultiply
	case "screen":
		return sharp.BlendScreen
	case "overlay":
		return sharp.BlendOverlay
	case "darken":
		return sharp.BlendDarken
	case "lighten":
		return sharp.BlendLighten
	case "difference":
		return sharp.BlendDifference
	case "exclusion":
		return sharp.BlendExclusion
	case "add":
		return sharp.BlendAdd
	case "xor":
		return sharp.BlendXor
	}
	return sharp.BlendOver
}
