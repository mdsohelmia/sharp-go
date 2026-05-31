package sharp

import (
	"context"
	"errors"
	"math"
	"runtime"

	"github.com/mdsohelmia/sharp-go/internal/vips"
)

// DeltaEMethod selects the CIE colour-difference formula used by Compare.
type DeltaEMethod int

const (
	DeltaE2000 DeltaEMethod = iota // CIE dE 2000 (default)
	DeltaE76                       // CIE dE 1976
	DeltaECMC                      // CMC l:c
)

// CompareOptions configures Compare.
type CompareOptions struct {
	// DeltaEMethod selects the colour-difference formula. Zero value = dE2000.
	DeltaEMethod DeltaEMethod
}

// DeltaEResult summarises the per-pixel CIE colour difference.
type DeltaEResult struct {
	Mean float64
	Max  float64
}

// CompareResult holds similarity metrics between two images, computed at the
// reference image's dimensions.
type CompareResult struct {
	RMSE   float64 // sRGB 8-bit units; 0 = identical
	PSNR   float64 // dB; +Inf when identical
	DeltaE DeltaEResult
	Width  int
	Height int
}

// Compare realizes ref and cmp to pixels and computes similarity metrics.
// Both pipelines are executed (geometry/colour ops applied; encode skipped).
// cmp is resized to ref's dimensions (Lanczos3) when they differ, and alpha is
// flattened onto white when only one input carries an alpha channel.
func Compare(ctx context.Context, ref, cmp *Image, opts CompareOptions) (CompareResult, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if ref == nil || cmp == nil {
		return CompareResult{}, errors.New("sharp: Compare requires non-nil images")
	}
	if ref.err != nil {
		return CompareResult{}, ref.err
	}
	if cmp.err != nil {
		return CompareResult{}, cmp.err
	}
	if err := ctx.Err(); err != nil {
		return CompareResult{}, err
	}

	rImg, rStop, err := buildPipelineImage(ctx, ref)
	if err != nil {
		return CompareResult{}, err
	}
	defer rStop()
	cImg, cStop, err := buildPipelineImage(ctx, cmp)
	if err != nil {
		return CompareResult{}, err
	}
	defer cStop()

	// Normalise colour to sRGB for RMSE/PSNR.
	if rImg, err = vips.Colourspace(rImg, vips.InterpretationSRGB); err != nil {
		return CompareResult{}, err
	}
	if cImg, err = vips.Colourspace(cImg, vips.InterpretationSRGB); err != nil {
		return CompareResult{}, err
	}

	rw, rh := rImg.Width(), rImg.Height()
	if rw == 0 || rh == 0 {
		return CompareResult{}, errors.New("sharp: Compare reference has zero area")
	}
	if cImg.Width() != rw || cImg.Height() != rh {
		cImg, err = vips.ThumbnailImage(cImg, vips.ThumbnailParams{
			Width: rw, Height: rh,
			Kernel: vips.KernelLanczos3, Size: vips.SizeForce,
			Crop: vips.InterestingNone, NoRotate: true,
		})
		if err != nil {
			return CompareResult{}, err
		}
	}

	// Band normalisation: flatten alpha onto white only when counts differ.
	if rImg.Bands() != cImg.Bands() {
		if rImg.Bands() == 4 {
			if rImg, err = vips.Flatten(rImg, 255, 255, 255); err != nil {
				return CompareResult{}, err
			}
		}
		if cImg.Bands() == 4 {
			if cImg, err = vips.Flatten(cImg, 255, 255, 255); err != nil {
				return CompareResult{}, err
			}
		}
	}
	if rImg.Bands() != cImg.Bands() {
		return CompareResult{}, errors.New("sharp: Compare band-count mismatch")
	}

	// RMSE / PSNR from the squared error of the float difference.
	diff, err := vips.Subtract(rImg, cImg)
	if err != nil {
		return CompareResult{}, err
	}
	bands, err := vips.Stats(diff)
	if err != nil {
		return CompareResult{}, err
	}
	var sumSq float64
	for _, b := range bands {
		sumSq += b.SumSquare
	}
	n := float64(diff.Width()) * float64(diff.Height()) * float64(diff.Bands())
	mse := sumSq / n
	rmse := math.Sqrt(mse)
	psnr := math.Inf(1)
	if mse > 0 {
		psnr = 10 * math.Log10(255*255/mse)
	}

	// deltaE in LAB.
	rLab, err := vips.Colourspace(rImg, vips.InterpretationLAB)
	if err != nil {
		return CompareResult{}, err
	}
	cLab, err := vips.Colourspace(cImg, vips.InterpretationLAB)
	if err != nil {
		return CompareResult{}, err
	}
	// The public DeltaEMethod constants mirror vips.DeltaE* in iota order, so
	// the numeric cast is safe; keep the two enums in sync if either changes.
	de, err := vips.DeltaE(rLab, cLab, vips.DeltaEMethod(opts.DeltaEMethod))
	if err != nil {
		return CompareResult{}, err
	}
	deStats, err := vips.Stats(de)
	if err != nil {
		return CompareResult{}, err
	}

	return CompareResult{
		RMSE:   rmse,
		PSNR:   psnr,
		DeltaE: DeltaEResult{Mean: deStats[0].Mean, Max: deStats[0].Max},
		Width:  rw,
		Height: rh,
	}, nil
}
