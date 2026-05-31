# Design: similarity metrics API, Low/High/All crop, fastShrinkOnLoad toggle

Date: 2026-05-26
Status: Approved (pending written-spec review)

## Overview

Three independent additions to sharp-go, specced together at the user's request:

1. A native image-similarity API (`sharp.Compare`) exposing RMSE, PSNR, and
   CIE deltaE.
2. Exposure of the libvips `Interesting` crop modes Low/High/All through the
   public `Position` enum.
3. A `fastShrinkOnLoad` toggle on `ResizeOptions`.

They are unrelated in function and could ship independently; they share this
document only for convenience. Implementation may split them into separate
commits/PRs.

## Non-goals

- No SSIM. libvips has no SSIM op; building one is out of scope this round.
- No external metric tools (butteraugli/ssimulacra2/dssim). Those stay in
  `examples/proxy`. `Compare` is native-only (libvips/cgo, zero runtime deps).
- No new C in `bridge.c` — Part 1 composes existing libvips ops from Go.

---

## Part 1 — Similarity metrics API

### Public surface (new file `compare.go`, root package)

```go
// Compare realizes ref and cmp to pixels and computes similarity metrics.
// cmp is resized to ref's dimensions (Lanczos3) when they differ.
func Compare(ctx context.Context, ref, cmp *Image, opts CompareOptions) (CompareResult, error)

type DeltaEMethod int

const (
    DeltaE2000 DeltaEMethod = iota // default (vips_dE00)
    DeltaE76                       // vips_dE76
    DeltaECMC                      // vips_dECMC
)

type CompareOptions struct {
    DeltaEMethod DeltaEMethod // zero value = DeltaE2000
}

type CompareResult struct {
    RMSE   float64      // sRGB 8-bit units; 0 = identical
    PSNR   float64      // dB; math.Inf(+1) when identical (MSE == 0)
    DeltaE DeltaEResult
    Width  int          // dimensions metrics were computed at (== ref's)
    Height int
}

type DeltaEResult struct {
    Mean float64
    Max  float64
}
```

### Why `*Image` inputs

Taking `*Image` (not raw bytes) lets callers compare two *pipelines*, e.g.:

```go
res, err := sharp.Compare(ctx,
    sharp.FromFile("orig.png"),
    sharp.FromFile("orig.png").Resize(sharp.ResizeOptions{Width: 800}).WebP(format.WebPOptions{Quality: 80}),
    sharp.CompareOptions{})
```

`Compare` realizes each input to **pixels** using the same internal path that
`Stats` uses (run geometry/colour ops; skip the encode step). Recorded
encode/format options on either `*Image` are irrelevant to a pixel comparison
and are ignored.

### Computation (composed libvips ops, no new C)

1. Realize ref and cmp to in-memory pixel images.
2. Convert both to **sRGB 8-bit** (reuse existing `ensureSRGB`).
3. **Alpha handling:** if band counts differ (one has alpha, one does not),
   flatten alpha onto solid **white** so both become 3-band RGB. This is
   deterministic and keeps the metric well-defined. (If both have alpha, the
   alpha band participates in the comparison.)
4. If dimensions differ, resize **cmp → ref's WxH** with Lanczos3.
5. **RMSE:** cast both to float (`vips_cast` to float, so differences aren't
   clamped to unsigned) → `vips_subtract(ref, cmp)` → square (`vips_multiply`
   by self) → `vips_avg` (mean over all bands/pixels = MSE) → `sqrt` in Go.
6. **PSNR:** derived in Go from MSE (`MSE = RMSE²`):
   `PSNR = 10*log10(255² / MSE)`; `math.Inf(+1)` when `MSE == 0`.
7. **deltaE:** convert both sRGB→LAB (`vips_colourspace` to
   `VIPS_INTERPRETATION_LAB`), run the selected `vips_dE00`/`dE76`/`dECMC` →
   1-band per-pixel deltaE image → `vips_avg` (Mean) and `vips_max` (Max).

### Internal wiring

- New `internal/vips/op_compare.go`:
  - `RMSE(a, b *Image) (float64, error)` — subtract/square/avg.
  - `DeltaE(a, b *Image, method DeltaEMethod) (mean, max float64, error)`,
    where `DeltaEMethod` is the internal counterpart to the public enum
    (DeltaE2000/76/CMC).
  - `ToLAB(im *Image) error` colourspace helper alongside the existing
    `ensureSRGB`.
- All ops go through the existing GObject refcount/error conventions in
  `internal/vips`. On any libvips failure, return a non-nil typed error (never
  a nil error on failure — matches the existing invariant).
- `Compare` honors `context.Context` cancellation exactly as other terminal
  methods do (`context.AfterFunc` → kill in-flight op).

### Errors

- Realizing either pipeline can fail (decode/op error) → wrapped, returned.
- Zero-area image (after realize) → typed error.
- No "dimension mismatch" error path: mismatches are resolved by auto-resize.

---

## Part 2 — Expose Low/High/All crop

`internal/vips/op_resize.go` already defines `InterestingLow`, `InterestingHigh`,
`InterestingAll` mapped to the `VIPS_INTERESTING_*` enum. They are simply not
reachable from the public API.

### Changes

- `options.go`: add `PositionLow`, `PositionHigh`, `PositionAll` to the
  `Position` enum (after the existing entropy/attention entries; before or
  after edge gravities — placement is cosmetic, append to avoid renumbering
  surprises in any serialized form).
- `resize.go` `mapPosition`: add three cases →
  `vips.InterestingLow/High/All`.
- These are **not** edge gravities, so `isEdgeGravity` is unchanged and they
  route through the existing smartcrop/thumbnail fusion path automatically. No
  pipeline-order edits.
- `cmd/sharpgo/main.go` `parsePosition`: accept `low`, `high`, `all`; update
  the `-position` flag help string.

### Parity note

This is a **deliberate sharp-go extension**. sharp's `position` option exposes
only `entropy` and `attention` for smart cropping. Low/High/All bias the crop
toward low- or high-value pixels (or treat the whole frame as interesting) and
are libvips features sharp does not surface. Documented as such so the
"one JS method = one Go method" convention isn't misread as broken.
`VIPS_INTERESTING_ALL` keeps the full/centre region — documented so callers
aren't surprised it behaves like a near-centre crop.

---

## Part 3 — `fastShrinkOnLoad` toggle

### Change

- Add `FastShrinkOnLoad *bool` to `ResizeOptions`.
  - `nil` (default) and `true` → current behavior: decoder shrinks on load
    (JPEG DCT scale, etc.) via the fusion path.
  - `false` → back off the shrink-on-load factor so the resampler does more of
    the downscale work, avoiding shrink-on-load aliasing. Higher quality,
    slower, higher RSS.
- `*bool` (not `bool`) because the default is **on**; a zero-value `false`
  would otherwise silently disable it.

### Behavior vs. mechanism

This spec fixes the **behavior** (a default-on toggle matching sharp's
`fastShrinkOnLoad`). The exact libvips wiring — whether implemented via a
`vips_thumbnail` option, by adjusting the JPEG/WebP load `shrink` factor, or by
disabling fusion for this op — will be confirmed against current libvips docs
(via Context7) during the planning step, per the project rule to verify libvips
signatures rather than rely on memory.

---

## Testing

### Part 1 (metrics)

- **Identical:** `Compare(img, img)` → RMSE == 0, PSNR == +Inf, deltaE Mean/Max
  == 0.
- **Known offset:** add a constant N to one image → RMSE ≈ N (within rounding),
  PSNR matches the closed-form value.
- **Colourspace shift:** a tone shift produces non-zero deltaE; verify
  dE00 vs dE76 vs dECMC differ as expected.
- **Alpha mismatch:** RGB vs RGBA inputs → flatten-on-white path runs, result
  is finite and deterministic.
- **Auto-resize:** differently-sized inputs → metrics computed at ref's
  dimensions; `Width/Height` report ref's size.
- **Cancellation:** cancelled context → in-flight op killed, typed error.

### Part 2 (crop modes)

- Fixture asserting each of Low/High/All routes through smartcrop and produces
  the target dimensions without error.
- An **animated/multi-page** input variant (CLAUDE.md rule for any crop/resize
  touch) to confirm no per-page regression.

### Part 3 (fastShrinkOnLoad)

- On-vs-off run on a large-source/small-target JPEG; assert both succeed and
  that `false` yields equal-or-better quality, measured with the **new
  `sharp.Compare` API** (dogfooding Part 1).
- Confirm `nil` and `true` produce identical output.

## Risks

- **Pipeline-order**: Part 1 realizes pixels via the existing Stats path and
  Part 3 touches the fusion decision — both are resize-adjacent. The
  animated-input tests and the on/off-equivalence test guard regressions.
- **deltaE colourspace correctness**: deltaE must run in LAB; an accidental
  sRGB-space deltaE would silently produce wrong-but-plausible numbers. The
  known-offset/colourshift tests catch gross errors; LAB conversion is asserted
  explicitly in the op-level test.
