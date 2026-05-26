# Changelog

## Unreleased

### v0.8 — batch concurrency

- **Batch terminal calls**: `ToBytesAll(ctx, images, opts)`,
  `ToFilesAll(ctx, images, paths, opts)`, `MetadataAll(ctx, images, opts)`.
  Worker pool sized by `BatchOptions.Concurrency` (default `runtime.NumCPU()`),
  results returned in input order, `StopOnFirstError` to abort on first
  failure, `PerJobTimeout` for individual deadlines.
- Total CPU pressure is roughly `BatchOptions.Concurrency × Concurrency()`;
  benchmarking shows libvips=1 + Go pool=NumCPU beats libvips=NumCPU +
  sequential by ~28% on small-image resize/encode workloads.

### v0.7 — CLI + streaming output

- **sharpgo CLI** (`cmd/sharpgo`): `resize` / `convert` / `metadata` /
  `composite` / `info` subcommands.
- **True streaming output** for JPEG and PNG via `VipsTargetCustom` +
  cgo `//export` write trampoline. `ToWriter` no longer buffers JPEG/PNG
  in memory — bytes stream directly into the supplied `io.Writer`.
- **Refactor**: `execute()` split into `buildPipelineImage` + `applyAllOps`
  + `encodePipeline`; shared between buffer + streaming paths.
- **Sharp parity tests** (`parity_test.go`): 11 cases covering aspect-
  preserving resize, FitInside/Outside, 90° rotate, AutoOrient on all 8
  EXIF orientations, HasAlpha + Channels detection across JPEG/PNG/CMYK,
  composite preserves base dims, JPEG/PNG quality monotonicity.
- **Regression fix**: width-only / height-only resize now correctly
  preserves aspect ratio (was filling the missing dim with source value).

### v0.6 — production prep

- **WithMetadata / WithExif / WithICCProfile / WithXmp** setters mirror
  sharp's writer-side metadata mutation. WithMetadata also implicitly enables
  KeepMetadata at encode.
- **Edge gravity positions** for FitCover: `PositionNorth`/`NorthEast`/
  `East`/`SouthEast`/`South`/`SouthWest`/`West`/`NorthWest`. Implemented as
  thumbnail(size=Force) + extract_area at gravity offset.
- **Metadata blobs** exposed: `Metadata.Exif`/`ICC`/`XMP`/`IPTC` are now
  populated with the raw bytes when present.
- **ToFormat dispatcher**: `ToFormat(FormatJPEG, opts)` (or `"jpeg"` string)
  routes to the per-format method. Type-mismatch returns a sticky error.
- **Clone** semantics verified: produces independent pipelines suitable for
  parallel terminal calls.
- **Fuzz tests** (`FuzzMetadataDecode`, `FuzzJPEGEncode`) on input decoders.
- **Leak test** (`TestLeakResize`): 200-iteration stress with libvips tracked
  memory / alloc-count assertions.
- **Benchmarks**: `ResizeJPEGSmall/Large`, `MetadataOnly`, `ResizeChainOps`,
  `ParallelResize`, `CompositeOverlay`.

### v0.5 — hardening

- `context.Context` cancellation: watcher goroutine calls
  `vips_image_set_kill` on source image; kill propagates through op chain.
- `.Timeout(d)` fluent method wraps the user ctx with WithTimeout.
- Utilities: `V()`, `SupportedFormats()`, `Block`/`Unblock`,
  `TrackedMem`/`TrackedAllocs`/`TrackedFiles`.
- slog hook via `SetLogSink` / `SetSlogSink` (cgo //export trampoline +
  GLib default handler).
- Animated multi-page input: `.Animated()`, `.Pages(n)`, `.Page(idx)`.

### v0.4 — metadata + remaining formats

- JXL, JP2 encoders.
- Keep* metadata: `KeepMetadata`, `KeepExif`, `KeepXmp`, `KeepIptc`,
  `KeepIccProfile`. Implemented via in-place `vips_image_remove` of EXIF
  blob + all synthesised `exif-*` fields + orientation tag, XMP/IPTC/ICC
  blobs before each save.
- Constructors: `FromCreate`, `FromText`, `Join`. Synth-backed inputs
  materialise into VipsImage at terminal-call time.

### v0.3 — channels + remaining formats

- Channel ops: `ExtractChannel`, `JoinChannel`, `Bandbool`,
  `PipelineColourspace`.
- TIFF (full compression suite + tile/pyramid), AVIF, HEIF.
- Raw I/O: `FromRawBytes`, `.Raw(...)`.
- Tile output: `ToTiles` (DeepZoom/Zoomify/Google/IIIF/IIIF3, FS/ZIP/SZI).

### v0.2 — all ops + WebP + GIF

- 16 image operations (Blur, Sharpen, Median, Gamma, Negate, Threshold,
  Linear, Modulate, Normalise, Clahe, Convolve, Boolean, Recomb, Dilate,
  Erode, Flatten).
- Colour: `Tint`, `Greyscale`, `ToColourspace`.
- Channel: `RemoveAlpha`, `EnsureAlpha`.
- Layout: `Extend`, `Trim`, `Affine`.
- `Composite` (25 blend modes, 9 gravity anchors, Tile, Premultiplied).
- WebP, GIF.

### v0.1 — foundation

- `FromBytes` / `FromFile` / `FromReader`, `ToBytes` / `ToFile` / `ToWriter`.
- `Resize` (Cover/Contain/Fill/Inside/Outside; Centre/Entropy/Attention
  positions; Lanczos3/Cubic/Linear/Mitchell/Nearest kernels).
- `Rotate` (lossless 90/180/270 + arbitrary angles with bg fill),
  `AutoOrient`, `Flip`, `Flop`, `Extract`.
- JPEG, PNG encoders.
- `Metadata()`, `Stats()`.
- Pipeline order: load → trim → extract → autoOrient → rotate → affine →
  resize → extend → composite → colour ops → image ops → channel ops →
  colourspace conversion → flip → flop → metadata strip → encode.
