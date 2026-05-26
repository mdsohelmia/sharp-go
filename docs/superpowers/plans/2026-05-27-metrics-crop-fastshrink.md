# Similarity Metrics, Low/High/All Crop, fastShrinkOnLoad — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a native `sharp.Compare` similarity-metrics API (RMSE/PSNR/deltaE), expose the libvips Low/High/All crop strategies through `Position`, and add a `fastShrinkOnLoad` resize toggle.

**Architecture:** Three independent additions. Crop modes and fastShrinkOnLoad are pure-Go option wiring (no C). Metrics adds two small NULL-terminated-varargs wrappers in `bridge.c` (`vips_subtract` on float-cast inputs, and `vips_dE00/76/CMC`), thin `internal/vips` Go wrappers, and a public `Compare` that realizes both pipelines via the existing `buildPipelineImage` path and derives metrics from `vips_stats`.

**Tech Stack:** Go ≥ 1.26.2, cgo, libvips ≥ 8.16 (C API only — no C++).

Spec: `docs/superpowers/specs/2026-05-26-metrics-crop-fastshrink-design.md`

**Implementation order:** Part A (crop modes) → Part B (metrics) → Part C (fastShrinkOnLoad). Part C's quality cross-check dogfoods Part B, so metrics land first.

**Testing note:** Tests live in `package sharp_test` (external) and use the public API. They synthesize inputs with `sharp.FromCreate` (and JPEG buffers derived from it) so they do **not** depend on the `test/fixtures` directory, which is currently absent and causes `fixturePath` to skip.

---

## Part A — Expose Low/High/All crop

### Task 1: Add PositionLow/High/All and wire mapPosition

**Files:**
- Modify: `options.go` (Position const block, ends ~line 44)
- Modify: `resize.go:162-173` (`mapPosition`)
- Test: `crop_modes_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `crop_modes_test.go`:

```go
package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
)

func TestResizeCropModesLowHighAll(t *testing.T) {
	ctx := context.Background()
	for _, pos := range []sharp.Position{sharp.PositionLow, sharp.PositionHigh, sharp.PositionAll} {
		src := sharp.FromCreate(sharp.CreateOptions{
			Width: 100, Height: 80, Channels: 3,
			Background: sharp.Color{R: 10, G: 20, B: 30},
		})
		_, info, err := src.Resize(sharp.ResizeOptions{
			Width: 40, Height: 40, Fit: sharp.FitCover, Position: pos,
		}).ToBytes(ctx)
		if err != nil {
			t.Fatalf("position %d: %v", pos, err)
		}
		if info.Width != 40 || info.Height != 40 {
			t.Fatalf("position %d: got %dx%d, want 40x40", pos, info.Width, info.Height)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestResizeCropModesLowHighAll -v`
Expected: BUILD FAIL — `undefined: sharp.PositionLow` (and High/All).

- [ ] **Step 3: Add the enum values**

In `options.go`, append to the `Position` const block, immediately after `PositionNorthWest` (keep existing values' positions stable):

```go
	PositionNorthWest
	// PositionLow biases the smart crop toward low-value (dark) regions.
	// sharp-go extension: sharp's `position` exposes only entropy/attention.
	PositionLow
	// PositionHigh biases the smart crop toward high-value (bright) regions.
	PositionHigh
	// PositionAll treats the whole frame as interesting (near-centre crop).
	PositionAll
)
```

- [ ] **Step 4: Wire mapPosition**

In `resize.go`, add cases to `mapPosition` before the `PositionCentre`/default arm:

```go
func mapPosition(p Position) vips.Interesting {
	switch p {
	case PositionEntropy:
		return vips.InterestingEntropy
	case PositionAttention:
		return vips.InterestingAttention
	case PositionLow:
		return vips.InterestingLow
	case PositionHigh:
		return vips.InterestingHigh
	case PositionAll:
		return vips.InterestingAll
	case PositionCentre:
		fallthrough
	default:
		return vips.InterestingCentre
	}
}
```

(No change to `isEdgeGravity` — these route through the existing thumbnail smartcrop path.)

- [ ] **Step 5: Run test to verify it passes**

Run: `go test . -run TestResizeCropModesLowHighAll -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add options.go resize.go crop_modes_test.go
git commit -m "feat: expose Low/High/All smartcrop strategies via Position"
```

### Task 2: Expose low/high/all in the CLI

**Files:**
- Modify: `cmd/sharpgo/main.go:78` (flag help) and `cmd/sharpgo/main.go:318-341` (`parsePosition`)

- [ ] **Step 1: Update the flag help string**

In `cmd/sharpgo/main.go` line ~78, extend the `-position` help to include the new values:

```go
	pos := fs.String("position", "centre", "centre|entropy|attention|low|high|all|north|northeast|east|southeast|south|southwest|west|northwest")
```

- [ ] **Step 2: Add parse cases**

In `parsePosition`, add cases alongside `entropy`/`attention`:

```go
	case "low":
		return sharp.PositionLow
	case "high":
		return sharp.PositionHigh
	case "all":
		return sharp.PositionAll
```

- [ ] **Step 3: Verify build**

Run: `go build ./...`
Expected: success.

- [ ] **Step 4: Smoke-test the CLI parse path**

Run: `go vet ./cmd/sharpgo`
Expected: no issues.

- [ ] **Step 5: Commit**

```bash
git add cmd/sharpgo/main.go
git commit -m "feat(cli): accept low/high/all crop positions"
```

---

## Part B — Similarity metrics API

### Task 3: Add C bridge wrappers (subtract, delta_e)

**Files:**
- Modify: `internal/vips/bridge.h` (add declarations after `sharpgo_colourspace`, ~line 234)
- Modify: `internal/vips/bridge.c` (add definitions after `sharpgo_colourspace`, ~line 535)

- [ ] **Step 1: Declare the bridge functions in bridge.h**

Add after the `sharpgo_colourspace` declaration:

```c
// Subtract right from left, casting both to float first so the difference is
// signed and unclipped. Output is float. Used by the similarity-metrics API.
int sharpgo_subtract(VipsImage *left, VipsImage *right, VipsImage **out);

// CIE colour-difference between two images, producing a 1-band result.
// method: 0=dE2000 (default), 1=dE76, 2=dECMC. libvips converts the inputs
// to LAB internally, so any input colourspace is accepted.
int sharpgo_delta_e(VipsImage *left, VipsImage *right, VipsImage **out, int method);
```

- [ ] **Step 2: Define the bridge functions in bridge.c**

Add after `sharpgo_colourspace`:

```c
int sharpgo_subtract(VipsImage *left, VipsImage *right, VipsImage **out) {
	VipsImage *lf = NULL, *rf = NULL;
	if (vips_cast(left, &lf, VIPS_FORMAT_FLOAT, NULL)) return -1;
	if (vips_cast(right, &rf, VIPS_FORMAT_FLOAT, NULL)) {
		g_object_unref(lf);
		return -1;
	}
	int rc = vips_subtract(lf, rf, out, NULL);
	g_object_unref(lf);
	g_object_unref(rf);
	return rc;
}

int sharpgo_delta_e(VipsImage *left, VipsImage *right, VipsImage **out, int method) {
	switch (method) {
	case 1:
		return vips_dE76(left, right, out, NULL);
	case 2:
		return vips_dECMC(left, right, out, NULL);
	default:
		return vips_dE00(left, right, out, NULL);
	}
}
```

- [ ] **Step 3: Verify cgo compiles**

Run: `go build ./internal/vips`
Expected: success (the C compiles against the included `vips/vips.h`).

- [ ] **Step 4: Commit**

```bash
git add internal/vips/bridge.h internal/vips/bridge.c
git commit -m "feat(vips): add subtract (float) and delta_e C bridges"
```

### Task 4: Add internal/vips Go wrappers

**Files:**
- Create: `internal/vips/op_compare.go`

- [ ] **Step 1: Write the Go wrappers**

Create `internal/vips/op_compare.go`:

```go
//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

// DeltaEMethod selects the CIE colour-difference formula.
type DeltaEMethod int

const (
	DeltaE2000 DeltaEMethod = iota // vips_dE00
	DeltaE76                       // vips_dE76
	DeltaECMC                      // vips_dECMC
)

// Subtract returns a float image of (a - b). a and b must share dimensions
// and band count.
func Subtract(a, b *Image) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_subtract(a.ptr, b.ptr, &out); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}

// DeltaE returns a 1-band per-pixel CIE colour-difference image between a and
// b using the given formula. Inputs are converted to LAB by libvips.
func DeltaE(a, b *Image, method DeltaEMethod) (*Image, error) {
	var out *C.VipsImage
	if rc := C.sharpgo_delta_e(a.ptr, b.ptr, &out, C.int(method)); rc != 0 {
		return nil, loadError()
	}
	return wrap(out), nil
}
```

- [ ] **Step 2: Verify build**

Run: `go build ./internal/vips`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/vips/op_compare.go
git commit -m "feat(vips): Subtract and DeltaE Go wrappers"
```

### Task 5: Public Compare API + tests

**Files:**
- Create: `compare.go`
- Test: `compare_test.go` (create)

- [ ] **Step 1: Write the failing tests**

Create `compare_test.go`:

```go
package sharp_test

import (
	"context"
	"math"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
)

func solid(w, h, ch int, r, g, b float64) *sharp.Image {
	return sharp.FromCreate(sharp.CreateOptions{
		Width: w, Height: h, Channels: ch,
		Background: sharp.Color{R: r, G: g, B: b, A: 255},
	})
}

func TestCompareIdentical(t *testing.T) {
	ctx := context.Background()
	res, err := sharp.Compare(ctx, solid(64, 48, 3, 100, 100, 100), solid(64, 48, 3, 100, 100, 100), sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.RMSE != 0 {
		t.Fatalf("RMSE = %v, want 0", res.RMSE)
	}
	if !math.IsInf(res.PSNR, 1) {
		t.Fatalf("PSNR = %v, want +Inf", res.PSNR)
	}
	if res.DeltaE.Mean != 0 || res.DeltaE.Max != 0 {
		t.Fatalf("deltaE = %+v, want zero", res.DeltaE)
	}
	if res.Width != 64 || res.Height != 48 {
		t.Fatalf("dims = %dx%d, want 64x48", res.Width, res.Height)
	}
}

func TestCompareKnownOffset(t *testing.T) {
	ctx := context.Background()
	// One channel differs by 10; RMSE over 3 bands = sqrt((10^2)/3) = 5.7735.
	res, err := sharp.Compare(ctx, solid(32, 32, 3, 100, 100, 100), solid(32, 32, 3, 110, 100, 100), sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	want := math.Sqrt(100.0 / 3.0)
	if math.Abs(res.RMSE-want) > 0.05 {
		t.Fatalf("RMSE = %v, want ~%v", res.RMSE, want)
	}
	if res.DeltaE.Mean <= 0 {
		t.Fatalf("deltaE mean = %v, want > 0", res.DeltaE.Mean)
	}
}

func TestCompareDeltaEMethods(t *testing.T) {
	ctx := context.Background()
	mk := func() (*sharp.Image, *sharp.Image) {
		return solid(32, 32, 3, 100, 120, 140), solid(32, 32, 3, 130, 110, 150)
	}
	var means []float64
	for _, m := range []sharp.DeltaEMethod{sharp.DeltaE2000, sharp.DeltaE76, sharp.DeltaECMC} {
		a, b := mk()
		res, err := sharp.Compare(ctx, a, b, sharp.CompareOptions{DeltaEMethod: m})
		if err != nil {
			t.Fatalf("method %d: %v", m, err)
		}
		if res.DeltaE.Mean <= 0 {
			t.Fatalf("method %d: deltaE mean = %v, want > 0", m, res.DeltaE.Mean)
		}
		means = append(means, res.DeltaE.Mean)
	}
	// The three formulae should not all collapse to the same value.
	if means[0] == means[1] && means[1] == means[2] {
		t.Fatalf("dE2000/dE76/dECMC all equal (%v); expected the formulae to differ", means)
	}
}

func TestCompareAutoResize(t *testing.T) {
	ctx := context.Background()
	res, err := sharp.Compare(ctx, solid(100, 100, 3, 50, 60, 70), solid(40, 40, 3, 50, 60, 70), sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 100 || res.Height != 100 {
		t.Fatalf("dims = %dx%d, want 100x100 (ref size)", res.Width, res.Height)
	}
	if res.RMSE > 0.01 {
		t.Fatalf("RMSE = %v, want ~0 for identical-colour resize", res.RMSE)
	}
}

func TestCompareAlphaMismatch(t *testing.T) {
	ctx := context.Background()
	// ref RGB, cmp RGBA opaque white; band counts differ -> cmp flattened on white.
	res, err := sharp.Compare(ctx, solid(32, 32, 3, 255, 255, 255), solid(32, 32, 4, 255, 255, 255), sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if math.IsNaN(res.RMSE) || math.IsInf(res.RMSE, 0) && !math.IsInf(res.PSNR, 1) {
		t.Fatalf("unexpected RMSE %v", res.RMSE)
	}
	if res.RMSE != 0 {
		t.Fatalf("RMSE = %v, want 0 (both white after flatten)", res.RMSE)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test . -run TestCompare -v`
Expected: BUILD FAIL — `undefined: sharp.Compare` / `sharp.CompareOptions` / `sharp.CompareResult`.

- [ ] **Step 3: Write compare.go**

Create `compare.go`:

```go
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
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test . -run TestCompare -v`
Expected: PASS (TestCompareIdentical, TestCompareKnownOffset, TestCompareAutoResize, TestCompareAlphaMismatch).

- [ ] **Step 5: Run vet and the full root package tests**

Run: `go vet . && go test . -v`
Expected: no vet issues; all tests pass.

- [ ] **Step 6: Commit**

```bash
git add compare.go compare_test.go
git commit -m "feat: native sharp.Compare similarity metrics (RMSE/PSNR/deltaE)"
```

---

## Part C — fastShrinkOnLoad toggle

### Task 6: Add FastShrinkOnLoad option and gate fusion

**Files:**
- Modify: `resize.go:11-25` (`ResizeOptions`)
- Modify: `output.go:242-257` (`canFuseThumbnail`)
- Test: `fast_shrink_test.go` (create)

- [ ] **Step 1: Write the failing test**

Create `fast_shrink_test.go`:

```go
package sharp_test

import (
	"bytes"
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

// jpegBuffer renders a solid JPEG so the buffer (fusion) path is exercised
// without depending on test fixtures.
func jpegBuffer(t *testing.T, w, h int) []byte {
	t.Helper()
	buf, _, err := sharp.FromCreate(sharp.CreateOptions{
		Width: w, Height: h, Channels: 3,
		Background: sharp.Color{R: 120, G: 90, B: 60},
	}).JPEG(format.JPEGOptions{Quality: 90}).ToBytes(context.Background())
	if err != nil {
		t.Fatalf("jpegBuffer: %v", err)
	}
	return buf
}

func TestFastShrinkOnLoadNilEqualsTrue(t *testing.T) {
	ctx := context.Background()
	src := jpegBuffer(t, 400, 400)
	tr := true

	a, _, err := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 100}).ToBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 100, FastShrinkOnLoad: &tr}).ToBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("nil and explicit-true FastShrinkOnLoad must produce identical output")
	}
}

func TestFastShrinkOnLoadOffProducesValidOutput(t *testing.T) {
	ctx := context.Background()
	src := jpegBuffer(t, 400, 400)
	off := false

	_, info, err := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 100, FastShrinkOnLoad: &off}).ToBytes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 100 {
		t.Fatalf("width = %d, want 100", info.Width)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test . -run TestFastShrinkOnLoad -v`
Expected: BUILD FAIL — `unknown field FastShrinkOnLoad in struct literal`.

- [ ] **Step 3: Add the field**

In `resize.go`, add to `ResizeOptions` (after `WithoutReduction`):

```go
	// WithoutReduction disables downscaling.
	WithoutReduction bool

	// FastShrinkOnLoad controls decoder shrink-on-load fusion. nil/true (the
	// default) lets the decoder shrink on load (JPEG DCT scale, etc.) for
	// speed. false forces a full decode followed by a post-decode resize,
	// avoiding shrink-on-load aliasing at the cost of speed and peak memory.
	FastShrinkOnLoad *bool
```

- [ ] **Step 4: Gate fusion in canFuseThumbnail**

In `output.go`, add to `canFuseThumbnail` immediately after the `o.resize == nil` guard:

```go
func canFuseThumbnail(o *pipelineOpts) bool {
	if o.resize == nil {
		return false
	}
	if o.resize.FastShrinkOnLoad != nil && !*o.resize.FastShrinkOnLoad {
		return false
	}
	if o.trim != nil || o.extract != nil || o.affine != nil {
		return false
	}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `go test . -run TestFastShrinkOnLoad -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add resize.go output.go fast_shrink_test.go
git commit -m "feat: add FastShrinkOnLoad resize toggle"
```

### Task 7: Quality cross-check dogfooding Compare

**Files:**
- Modify: `fast_shrink_test.go` (add a test)

- [ ] **Step 1: Add the cross-check test**

Append to `fast_shrink_test.go`:

```go
func TestFastShrinkOnLoadQualitySmoke(t *testing.T) {
	ctx := context.Background()
	// A non-fixture, detailed-ish source: a JPEG of a created image. Solid
	// content makes both paths near-identical; the value here is exercising
	// Compare against the two resize paths end to end.
	src := jpegBuffer(t, 800, 800)
	off := false
	on := true

	ref := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 200, FastShrinkOnLoad: &off})
	cand := sharp.FromBytes(src).Resize(sharp.ResizeOptions{Width: 200, FastShrinkOnLoad: &on})

	res, err := sharp.Compare(ctx, ref, cand, sharp.CompareOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Width != 200 || res.Height != 200 {
		t.Fatalf("dims = %dx%d, want 200x200", res.Width, res.Height)
	}
	if res.RMSE < 0 || (res.RMSE > 0 && res.PSNR <= 0) {
		t.Fatalf("nonsensical metrics: RMSE=%v PSNR=%v", res.RMSE, res.PSNR)
	}
	t.Logf("fastShrink off vs on: RMSE=%.4f PSNR=%.2f dE00mean=%.5f", res.RMSE, res.PSNR, res.DeltaE.Mean)
}
```

- [ ] **Step 2: Run test to verify it passes**

Run: `go test . -run TestFastShrinkOnLoadQualitySmoke -v`
Expected: PASS (with a logged metrics line).

- [ ] **Step 3: Commit**

```bash
git add fast_shrink_test.go
git commit -m "test: cross-check FastShrinkOnLoad paths via Compare"
```

---

## Final verification

### Task 8: Full build, vet, and test sweep

- [ ] **Step 1: Build everything**

Run: `go build ./...`
Expected: success.

- [ ] **Step 2: Vet**

Run: `go vet ./...`
Expected: no issues.

- [ ] **Step 3: Full test suite**

Run: `go test ./...`
Expected: all packages pass (fixture-dependent tests may skip, which is expected here).

- [ ] **Step 4: Animated-input regression check for crop modes**

This honors the CLAUDE.md rule that any resize/crop change includes an animated-input variant. It skips automatically when fixtures are absent.

Add to `crop_modes_test.go`:

```go
func TestResizeCropModesAnimated(t *testing.T) {
	ctx := context.Background()
	buf := readFixture(t, "rgb-with-alpha.webp") // skips if fixtures absent
	_, info, err := sharp.FromBytes(buf).Animated().
		Resize(sharp.ResizeOptions{Width: 32, Height: 32, Fit: sharp.FitCover, Position: sharp.PositionHigh}).
		WebP(format.WebPOptions{}).ToBytes(ctx)
	if err != nil {
		t.Fatalf("animated crop: %v", err)
	}
	if info.Width != 32 {
		t.Fatalf("width = %d, want 32", info.Width)
	}
}
```

Add the imports `sharp "github.com/mdsohelmia/sharp-go"` (already present) and `"github.com/mdsohelmia/sharp-go/format"` to `crop_modes_test.go`. `readFixture` is defined in `roundtrip_test.go` (same `sharp_test` package).

Run: `go test . -run TestResizeCropModesAnimated -v`
Expected: PASS or SKIP (skip when the fixture is missing).

- [ ] **Step 5: Final commit**

```bash
git add crop_modes_test.go
git commit -m "test: animated-input crop-mode regression guard"
```

---

## Self-review notes (addressed)

- **subtract format clipping:** handled by casting both inputs to `VIPS_FORMAT_FLOAT` inside `sharpgo_subtract` before subtracting — negatives are preserved regardless of libvips' promotion table.
- **deltaE colourspace:** Compare converts both images to `InterpretationLAB` before `vips.DeltaE`, so the metric is never computed in sRGB space. (`vips_dE00` also converts internally, making this belt-and-suspenders.)
- **Type consistency:** `DeltaEMethod` constants share iota order across the public (`sharp`) and internal (`vips`) packages and the C `switch` (0=dE2000, 1=dE76, 2=dECMC); `vips.DeltaEMethod(opts.DeltaEMethod)` is a safe numeric conversion.
- **Band normalisation:** matches the spec — flatten-on-white only when band counts differ; equal counts (both 3 or both 4) compare as-is.
- **fastShrinkOnLoad mechanism:** implemented as "disable shrink-on-load fusion" (full decode + post-decode resize when false), which needs no libvips signature changes and is fully testable.
```
