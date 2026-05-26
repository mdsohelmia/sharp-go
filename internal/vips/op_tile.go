//go:build cgo

package vips

/*
#include <stdlib.h>
#include "bridge.h"
*/
import "C"

import "unsafe"

// DzLayout enumerates pyramid file layouts (libvips VipsForeignDzLayout).
type DzLayout int

const (
	DzLayoutDZ      DzLayout = C.VIPS_FOREIGN_DZ_LAYOUT_DZ
	DzLayoutZoomify DzLayout = C.VIPS_FOREIGN_DZ_LAYOUT_ZOOMIFY
	DzLayoutGoogle  DzLayout = C.VIPS_FOREIGN_DZ_LAYOUT_GOOGLE
	DzLayoutIIIF    DzLayout = C.VIPS_FOREIGN_DZ_LAYOUT_IIIF
	DzLayoutIIIF3   DzLayout = C.VIPS_FOREIGN_DZ_LAYOUT_IIIF3
)

// DzDepth enumerates pyramid depths.
type DzDepth int

const (
	DzDepthOnePixel DzDepth = C.VIPS_FOREIGN_DZ_DEPTH_ONEPIXEL
	DzDepthOneTile  DzDepth = C.VIPS_FOREIGN_DZ_DEPTH_ONETILE
	DzDepthOne      DzDepth = C.VIPS_FOREIGN_DZ_DEPTH_ONE
)

// DzContainer enumerates output containers.
type DzContainer int

const (
	DzContainerFS  DzContainer = C.VIPS_FOREIGN_DZ_CONTAINER_FS
	DzContainerZIP DzContainer = C.VIPS_FOREIGN_DZ_CONTAINER_ZIP
	DzContainerSZI DzContainer = C.VIPS_FOREIGN_DZ_CONTAINER_SZI
)

// DzParams configures DzSave.
type DzParams struct {
	Filename    string
	Layout      DzLayout
	Suffix      string // ".jpg"|".webp"|".png"; default ".jpeg"
	Overlap     int
	TileSize    int
	Depth       DzDepth
	Container   DzContainer
	Compression int
	Quality     int
}

// DzSave writes a tiled pyramid (DeepZoom/Zoomify/IIIF) to disk.
func DzSave(im *Image, p DzParams) error {
	cf := C.CString(p.Filename)
	defer C.free(unsafe.Pointer(cf))
	cs := C.CString(p.Suffix)
	defer C.free(unsafe.Pointer(cs))
	rc := C.sharpgo_dzsave(
		im.ptr,
		cf,
		C.int(p.Layout),
		cs,
		C.int(p.Overlap), C.int(p.TileSize),
		C.int(p.Depth), C.int(p.Container),
		C.int(p.Compression), C.int(p.Quality),
	)
	if rc != 0 {
		return lastError()
	}
	return nil
}
