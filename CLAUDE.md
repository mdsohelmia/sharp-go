# sharp-go

A Go port of the [sharp](https://github.com/lovell/sharp) Node.js image library,
built on **libvips via cgo**. Module path: `github.com/mdsohelmia/sharp-go`, Go ≥ 1.26.2.

## Goal

Idiomatic Go library with full sharp API parity and native-sharp performance,
usable as `go get github.com/mdsohelmia/sharp-go`. The `examples/proxy` is the
flagship consumer: an HTTP image-optimization proxy whose mission is to beat
**Fastly Image Optimizer** on both file size and perceptual quality (the proxy
dogfoods this library — improvements land here first).

## Hard rules

- **C only, no C++ — ever.** We bind the libvips **C** API (`vips/vips.h`,
  `vips_*` functions, GObject refcounting). The C++ wrapper (`vips-cpp`,
  `vips::VImage`) is deliberately avoided: smaller binary, no libstdc++ link,
  easier musl/cross-compile. Any helper that doesn't fit a one-line cgo call
  lives in plain C in `internal/vips/bridge.c` — never `.cc`/`.cpp`.
- `pkg-config: vips libwebp` (NOT `vips-cpp`). See `internal/vips/cgo.go`.
- The public packages (root, `format/`) must never `import "C"`. All cgo is
  confined to `internal/vips`, guarded by `//go:build cgo` (with a `!cgo` stub
  that returns a clear init error).
- Minimum libvips **8.16** — checked at init in `cgo.go`, fails fast. (8.15
  lacks the `smart-deblock`/`keep-duplicate-frames` save options and an AVIF
  encoder this code relies on.)

## Architecture

Deferred recorder + single execute, mirroring sharp's pipeline model:

- `*Image` (`image.go`) records options into `pipelineOpts`; chainable methods
  (`Resize`, `Sharpen`, `WebP`, …) mutate the receiver and `return im`.
- Only **terminal** methods touch libvips: `ToBytes`, `ToFile` (`output.go:18`),
  `ToWriter` (`output.go:44`), `Metadata` (`metadata.go:33`), `Stats`
  (`stats.go:25`). They funnel through `execute` → `buildPipelineImage` →
  `applyAllOps` in `output.go`, which applies ops in sharp's exact pipeline
  order (decode → autoOrient → trim → extract → resize → composite →
  colourspace → encode).
- `Clone()` deep-copies opts for parallel variants. A single `*Image` is **not**
  safe for concurrent option mutation; parallel terminal calls on *distinct*
  `*Image` values are safe.
- All terminal methods take `context.Context`; cancellation kills the in-flight
  libvips op (`context.AfterFunc` → `vimg.Kill()`).

### Threading model (matches govips, perf-critical)

libvips per-op concurrency defaults to **1** (`cgo.go`), not `NumCPU`. Sharp
uses `NumCPU` safely only because Node's libuv threadpool caps concurrent ops;
Go has no such cap, so `NumCPU` per op × unbounded request goroutines =
oversubscription thrash at 100% CPU. With concurrency 1 each pipeline runs
single-threaded and parallelism comes from running many pipelines (one goroutine
per request), spread across cores by the scheduler. Tune with `SetConcurrency`.

Terminal methods do **not** `runtime.LockOSThread` per call. Each op reads the
thread-local error buffer immediately after its own cgo call (`error.go`), so
the pipeline needn't stay on one OS thread. Pinning per op would force a fresh
per-thread libvips buffer cache onto every distinct M the scheduler touches →
unbounded RSS under load. `vips_thread_shutdown` is therefore never auto-called;
it's exposed as `sharp.ShutdownThread()` for code that drives libvips from its
own long-lived, self-pinned thread. Servers should bound in-flight pipelines
with a semaphore (see `examples/proxy`, `MAX_INFLIGHT_ENCODES`).

### Shrink-on-load fusion (perf-critical)

`canFuseThumbnail` (`output.go`) routes resize-dominated pipelines through
`vips_thumbnail_{buffer,source}` so the decoder shrinks on load (JPEG DCT scale,
etc.) — large-source/small-target sees big RSS reduction. `ensureSRGB` and
`autoOrient` are hoisted into the thumbnail call (export profile + `no_rotate`)
and cleared so `applyAllOps` skips them. Fusion is disabled by trim/extract/
affine and edge-gravity cover crops.

### Lazy sequential decode (non-fused paths)

When fusion is off, the load goes through `LoadBufferSeq`
(`vips_image_new_from_source` + `VIPS_ACCESS_SEQUENTIAL`, **no**
`vips_image_copy_memory`) — a single-pass streaming decode, matching sharp's
default. The buffer is wrapped in a streaming `Source` whose `cgo.Handle` keeps
it alive for the image's lazy lifetime (never hand libvips a raw Go pointer it
retains). Before an op that needs random access — trim, normalise, EXIF
auto-rotate (orientation ≥ 3), rotate, affine, vertical flip — `applyAllOps`
calls `vips.StaySequential`, a port of sharp's `StaySequential` that
`copy_memory`s the (already-shrunk) image only when actually needed. Streaming
encoders (JPEG/PNG) get a real peak-RSS win; WebP/AVIF buffer the whole frame so
the win is smaller. `Stats` deliberately loads random (`LoadBuffer`), as in sharp.

### libwebp-direct encoder

libvips' `webpsave` hides several cwebp knobs. `internal/vips/bridge.c`
(`sharpgo_webpsave_sharp_yuv`) bypasses it and calls **libwebp directly** to
expose `use_sharp_yuv`, `autofilter`, `sns_strength`, `target_size`,
`segments`, `passes`. Routed via `format.WebPOptions{UseSharpYUV: true}` →
`vips.SaveWebPSharpYUV`. CRITICAL: `pic.use_argb = 1` must be set or
`use_sharp_yuv` is silently ignored.

## Layout

```
*.go                 public API (one file per sharp method category)
format/              per-format option structs (jpeg, png, webp, avif, …)
internal/vips/       cgo binding — the ONLY package that imports "C"
  cgo.go             init, version check, concurrency/cache config
  bridge.c/.h        plain-C helpers (varargs, libwebp-direct, callbacks)
  op_*.go            libvips C op wrappers per category
  save.go load.go    foreign savers/loaders
cmd/sharpgo          CLI front-end (resize/convert/metadata)
cmd/sharpgo-doctor   prints detected libvips env + available loaders/savers
examples/            proxy (flagship), thumbnail, format-convert, watermark
test/testutil/       RMSE / colour-distance comparison helpers
```

## API conventions

- One JS method = one Go method; camelCase → PascalCase (`autoOrient` →
  `AutoOrient`).
- Multi-positional JS args → single options struct (`Resize(ResizeOptions{…})`);
  single scalars stay scalar (`Gamma(2.2)`).
- JS enums → typed Go constants (`FitCover`, `KernelLanczos3`).
- Boolean knobs are optional struct fields; zero value = libvips default.

## Build / test

```bash
go build ./...
go test ./...            # full suite
go run ./cmd/sharpgo-doctor    # verify libvips links + capabilities
go run ./examples/proxy        # flagship CDN proxy
```

Requires libvips installed (`brew install vips` / `apt install libvips-dev` /
`apk add vips-dev pkgconf`).

### Test fixtures

Fixture-based tests read from `test/fixtures/` (module-relative,
`roundtrip_test.go`), vendored from the upstream sharp repo. `fixturePath`
**skips** (does not fail) a test whose fixture is missing, so `go test ./...`
passes on a fresh clone without them — populate `test/fixtures/` to run the full
suite. (The fixtures are now present in-repo; the earlier `../test/fixtures`
relative-path gotcha is resolved.)

## Memory / safety notes

- Each `*Image` wraps a `*C.VipsImage`; finalized via `runtime.AddCleanup`.
- Input buffers must outlive the **pipeline**, not just the call — the non-fused
  load is lazy. They're wrapped in a streaming `Source` whose `cgo.Handle` keeps
  them (and their backing `[]byte`) alive until the image is collected. Never
  hand libvips a raw Go pointer it retains (the cgo-pointer rule — why the
  earlier path used `copy_memory`).
- libvips per-thread state isn't auto-freed (no per-op `vips_thread_shutdown`).
  Code running pipelines from its own pinned OS thread should call
  `sharp.ShutdownThread()` on that thread before retiring it.
- Feature-detect optional codecs (HEIF/AVIF/JXL) via `vips_type_find`; return a
  typed error rather than a raw libvips message when unavailable.
- Encoded output slices can be recycled via `sharp.Release(b)` (optional pool).

## When working here

- Reference the upstream sharp sources for ordering/semantics, but reimplement
  against the C API — never copy C++.
- Pipeline-order changes are the highest-risk edits; they look fine on simple
  inputs and break on complex chains and animated/multi-page images. Add an
  animated-input variant when touching any op.
- Use Context7 MCP for current libvips/cwebp API details before relying on
  remembered signatures.
