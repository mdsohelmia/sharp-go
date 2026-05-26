package sharp

import (
	"context"
	"errors"

	"github.com/sohelmia/sharp-go/format"
	"github.com/sohelmia/sharp-go/internal/vips"
)

// TileLayout names the directory layout for tile output.
type TileLayout int

const (
	TileLayoutDZ      TileLayout = iota // DeepZoom (default)
	TileLayoutZoomify
	TileLayoutGoogle
	TileLayoutIIIF
	TileLayoutIIIF3
)

// TileDepth names the pyramid depth.
type TileDepth int

const (
	TileDepthOnePixel TileDepth = iota
	TileDepthOneTile
	TileDepthOne
)

// TileContainer names the output container.
type TileContainer int

const (
	TileContainerFS  TileContainer = iota // filesystem directory tree
	TileContainerZIP                       // single .zip
	TileContainerSZI                       // .szi (DeepZoom zip)
)

// TileOptions configures ToTiles.
type TileOptions struct {
	Layout      TileLayout
	// Format selects the tile encoder. Only JPEG/PNG/WebP are supported by
	// libvips dzsave. Defaults to JPEG.
	Format      string // ".jpg" | ".png" | ".webp"
	Overlap     int    // default 1
	Size        int    // tile size in pixels; default 254
	Depth       TileDepth
	Container   TileContainer
	Compression int // zip mode; 0-9
	Quality     int // 1-100; default 75
}

// ToTiles writes a pyramid of tiles for the current image to outputPrefix.
// The exact files produced depend on Layout and Container — for DZ + FS it
// produces `<prefix>.dzi` plus `<prefix>_files/...`; for ZIP it produces a
// single `<prefix>.zip`.
func (im *Image) ToTiles(ctx context.Context, outputPrefix string, opts TileOptions) (Info, error) {
	if im.err != nil {
		return Info{}, im.err
	}
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	if outputPrefix == "" {
		return Info{}, errors.New("sharp: empty tile output prefix")
	}

	// We need to materialise the pipeline image without invoking a normal
	// format encoder. Cheapest path: render to PNG bytes, then re-decode and
	// hand the VipsImage to DzSave. Avoids re-doing the op pipeline.
	pngBytes, info, err := im.PNG(format.PNGOptions{}).ToBytes(ctx)
	if err != nil {
		return Info{}, err
	}
	vimg, err := vips.LoadBuffer(pngBytes)
	if err != nil {
		return Info{}, err
	}

	suffix := opts.Format
	if suffix == "" {
		suffix = ".jpg"
	}
	size := opts.Size
	if size == 0 {
		size = 254
	}
	overlap := opts.Overlap
	if overlap == 0 {
		overlap = 1
	}
	quality := opts.Quality
	if quality == 0 {
		quality = 75
	}

	err = vips.DzSave(vimg, vips.DzParams{
		Filename:    outputPrefix,
		Layout:      mapTileLayout(opts.Layout),
		Suffix:      suffix,
		Overlap:     overlap,
		TileSize:    size,
		Depth:       mapTileDepth(opts.Depth),
		Container:   mapTileContainer(opts.Container),
		Compression: opts.Compression,
		Quality:     quality,
	})
	if err != nil {
		return Info{}, err
	}
	info.Format = "dz"
	return info, nil
}

func mapTileLayout(l TileLayout) vips.DzLayout {
	switch l {
	case TileLayoutZoomify:
		return vips.DzLayoutZoomify
	case TileLayoutGoogle:
		return vips.DzLayoutGoogle
	case TileLayoutIIIF:
		return vips.DzLayoutIIIF
	case TileLayoutIIIF3:
		return vips.DzLayoutIIIF3
	case TileLayoutDZ:
		fallthrough
	default:
		return vips.DzLayoutDZ
	}
}

func mapTileDepth(d TileDepth) vips.DzDepth {
	switch d {
	case TileDepthOneTile:
		return vips.DzDepthOneTile
	case TileDepthOne:
		return vips.DzDepthOne
	case TileDepthOnePixel:
		fallthrough
	default:
		return vips.DzDepthOnePixel
	}
}

func mapTileContainer(c TileContainer) vips.DzContainer {
	switch c {
	case TileContainerZIP:
		return vips.DzContainerZIP
	case TileContainerSZI:
		return vips.DzContainerSZI
	case TileContainerFS:
		fallthrough
	default:
		return vips.DzContainerFS
	}
}
