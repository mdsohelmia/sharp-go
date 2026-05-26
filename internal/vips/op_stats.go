//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

// BandStats holds the per-band statistics returned by vips_stats.
type BandStats struct {
	Min       float64
	Max       float64
	Sum       float64
	SumSquare float64
	Mean      float64
	Deviation float64
}

// Stats computes per-channel statistics. Returns one BandStats entry per
// channel; the combined-all-bands row is dropped (matches sharp's stats() shape).
func Stats(im *Image) ([]BandStats, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_stats(im.ptr, &out); rc != 0 {
		return nil, loadError()
	}
	defer C.g_object_unref(C.gpointer(out))

	// Result is a (bands+1) x 6 matrix; row 0 is combined, rows 1..n per band.
	bands := im.Bands()
	res := make([]BandStats, bands)
	for b := 0; b < bands; b++ {
		row := b + 1
		res[b] = BandStats{
			Min:       float64(C.sharpgo_matrix_get(out, 0, C.int(row))),
			Max:       float64(C.sharpgo_matrix_get(out, 1, C.int(row))),
			Sum:       float64(C.sharpgo_matrix_get(out, 2, C.int(row))),
			SumSquare: float64(C.sharpgo_matrix_get(out, 3, C.int(row))),
			Mean:      float64(C.sharpgo_matrix_get(out, 4, C.int(row))),
			Deviation: float64(C.sharpgo_matrix_get(out, 5, C.int(row))),
		}
	}
	return res, nil
}
