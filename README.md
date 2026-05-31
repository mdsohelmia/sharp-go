# sharp-go

A Go port of [sharp](https://github.com/lovell/sharp), the Node.js
image-processing library. Built on libvips via cgo (C API only — no C++).

```go
import sharp "github.com/mdsohelmia/sharp-go"
import "github.com/mdsohelmia/sharp-go/format"

data, info, err := sharp.FromFile("input.jpg").
    Resize(sharp.ResizeOptions{Width: 800, Height: 600, Fit: sharp.FitCover}).
    Sharpen(sharp.SharpenOptions{}).
    WebP(format.WebPOptions{Quality: 80}).
    ToBytes(ctx)
```

## Status

v1.1.0 — stable public API under semver. ~99% sharp API parity. Test suite
passes on macOS arm64 + libvips 8.18.2 (fixture-based tests require the upstream
sharp fixtures — see [Test fixtures](#test-fixtures)).

## Design

- **Go + C only.** Sharp's C++ NAPI binding is reference-only. All libvips
  access is via the C API (`vips_*` functions). No `vips-cpp`, no
  libstdc++ link, smaller binary.
- **Idiomatic Go.** Option structs, `error` returns, `context.Context` for
  cancellation. `*Image` methods chain ergonomically but don't promise
  immutability — `Clone()` first if you need parallel variants.
- **Deferred pipeline.** Operations record options on `*Image`; one ordered
  execution happens at terminal calls (`ToBytes`, `ToFile`, `ToWriter`,
  `Metadata`, `Stats`).
- **Goroutine-safe.** Parallel terminal calls on distinct `*Image` values
  are safe; libvips handles inter-op thread scheduling. `*Image` itself is
  not safe for concurrent option mutation.

## Prerequisites

- Go ≥ 1.26.2
- libvips ≥ 8.16 with development headers (AVIF + recent WebP/GIF options)
- `pkg-config`

```sh
# macOS
brew install vips

# Debian / Ubuntu
sudo apt install libvips-dev pkg-config

# Alpine
apk add vips-dev pkgconf

# Windows
vcpkg install vips
```

## Verify install

```sh
go run github.com/mdsohelmia/sharp-go/cmd/sharpgo-doctor
```

Prints libvips version + per-format load/save support + SIMD status.

## Usage

### Input

```go
sharp.FromFile("input.jpg")
sharp.FromBytes(buf)
sharp.FromReader(r)          // buffers to memory
sharp.FromRawBytes(buf, format.RawInput{Width: 1920, Height: 1080, Channels: 3})

// Programmatic input
sharp.FromCreate(sharp.CreateOptions{Width: 200, Height: 200, Background: sharp.Color{R: 255, A: 255}})
sharp.FromText(sharp.TextOptions{Text: "hello", Font: "sans 24", RGBA: true})

// Compose multiple inputs
sharp.Join([]*sharp.Image{a, b, c}, sharp.JoinOptions{Across: 3})

// Multi-page / animated input
sharp.FromBytes(gif).Animated()        // load all pages
sharp.FromBytes(gif).Pages(-1)         // same
sharp.FromBytes(pdf).Page(2)           // load specific page
```

### Resize / crop / rotate

```go
.Resize(sharp.ResizeOptions{
    Width: 800, Height: 600,
    Fit:      sharp.FitCover,           // Cover, Contain, Fill, Inside, Outside
    Position: sharp.PositionAttention,  // Centre, Entropy, Attention, Low, High, All,
                                        //   North, NorthEast, East, ... (edge gravities)
    Kernel:   sharp.KernelLanczos3,
    Background: sharp.Color{R:255,G:255,B:255,A:255}, // for FitContain
    // FastShrinkOnLoad: &off,          // *bool; nil/true = shrink on load (fast,
                                        //   default). false = full decode then resize
                                        //   (higher quality, more memory).
})

.Extract(sharp.ExtractRegion{Left: 100, Top: 50, Width: 400, Height: 300})
.Extend(sharp.ExtendOptions{Top: 20, Left: 30, Background: sharp.Color{}})
.Trim(sharp.TrimOptions{Threshold: 10})
.Affine(sharp.AffineOptions{Matrix: [4]float64{0.5, 0, 0, 0.5}})

.Rotate(sharp.RotateOptions{Angle: 90})  // 0/90/180/270 lossless; other angles use bg fill
.AutoOrient()                            // apply EXIF orientation
.Flip()                                  // up-down mirror
.Flop()                                  // left-right mirror
```

### Image operations

```go
.Blur(sharp.BlurOptions{Sigma: 1.5})
.Sharpen(sharp.SharpenOptions{Sigma: 1, M1: 1, M2: 2, X1: 2, Y2: 10, Y3: 20})
.Median(sharp.MedianOptions{Size: 3})
.Gamma(sharp.GammaOptions{Exponent: 2.2})
.Negate(sharp.NegateOptions{KeepAlpha: true})
.Threshold(sharp.ThresholdOptions{Value: 128, Grayscale: true})
.Linear(sharp.LinearOptions{A: []float64{1.1, 1.1, 1.1}, B: []float64{0, 0, 0}})

.Modulate(sharp.ModulateOptions{Brightness: 1.2, Saturation: 0.8, Hue: 30})
.Normalise(sharp.NormaliseOptions{})
.Clahe(sharp.ClaheOptions{Width: 8, Height: 8, MaxSlope: 3})
.Convolve(sharp.ConvolveOptions{Kernel: kernel, Width: 3, Height: 3, Scale: 9})
.Boolean(sharp.BooleanOptions{Op: sharp.BooleanAnd, Constant: 0xF0})
.Recomb(sharp.RecombOptions{Matrix: m, N: 3})
.Dilate(sharp.MorphOptions{Size: 1})
.Erode(sharp.MorphOptions{Size: 1})
.Flatten(sharp.FlattenOptions{Background: sharp.Color{R:255,G:255,B:255}})
```

### Colour and channels

```go
.Tint(sharp.TintOptions{Colour: sharp.Color{R: 255, G: 100, B: 50}})
.Greyscale()
.PipelineColourspace(sharp.ColourspaceLAB)
.ToColourspace(sharp.ColourspaceSRGB)
.RemoveAlpha()
.EnsureAlpha(sharp.EnsureAlphaOptions{Alpha: 1})
.ExtractChannel(1)                       // single band
.JoinChannel(sharp.JoinChannelOptions{Inputs: [][]byte{maskBytes}})
.Bandbool(sharp.BandboolOptions{Op: sharp.BooleanOr})
```

### Composite

```go
.Composite([]sharp.CompositeLayer{
    // Source: exactly one of Input ([]byte), InputPath (string), or Prepared.
    {Input: logoBytes, Gravity: sharp.GravitySouthEast, Blend: sharp.BlendOver},
    {InputPath: "pattern.png", Tile: true,              Blend: sharp.BlendMultiply},
    // Explicit offset instead of Gravity:
    {Input: badge, Top: 12, Left: 12, HasOffset: true},
})
```

25 blend modes (Over/In/Out/Atop/Dest*/Xor/Add/Multiply/Screen/Overlay/Darken/
Lighten/Colour-Dodge/Colour-Burn/Hard-Light/Soft-Light/Difference/Exclusion/...),
9 gravity anchors + Tile + explicit Top/Left offset.

For a watermark reused across many images, decode it once with
`PrepareOverlay` and pass it via `Prepared` — this skips re-decoding (and the
alpha upgrade) on every composite:

```go
wm, err := sharp.PrepareOverlay(logoBytes)
defer wm.Close()
// ... per image:
img.Composite([]sharp.CompositeLayer{
    {Prepared: wm, Gravity: sharp.GravitySouthEast},
})
```

### Metadata

```go
md, err := sharp.FromFile("photo.jpg").Metadata(ctx)
// md.Format, Width, Height, Channels, Space, Depth, Density, HasAlpha,
//    HasProfile, Orientation, Pages, IsProgressive
// md.Exif, ICC, XMP, IPTC ([]byte)

// Write metadata
.WithMetadata(sharp.WithMetadataOptions{Orientation: 6, Density: 300})
.WithExif(sharp.WithExifOptions{Tags: map[string]string{"exif-ifd0-Make": "Canon"}})
.WithICCProfile(sharp.WithICCProfileOptions{Profile: "srgb"})
.WithXmp(sharp.WithXmpOptions{XmpPacket: xmpBytes})

// Or just preserve what's present in the input
.KeepMetadata()      // all
.KeepExif()
.KeepIccProfile()
.KeepXmp()
.KeepIptc()
```

Without any Keep/With call, all metadata is stripped at encode (sharp default).

### Output

```go
.JPEG(format.JPEGOptions{Quality: 80, MozJPEG: true})  // MozJPEG = trellis + progressive + optimised scans
.PNG(format.PNGOptions{Compression: 9, Palette: true})
.WebP(format.WebPOptions{Quality: 80, Effort: 4})      // Effort 0 (fast) – 6 (small); default 4
.AVIF(format.AVIFOptions{Quality: 50, Effort: 4})      // Effort 0 (fast) – 9 (small); default 4
.HEIF(format.HEIFOptions{Compression: format.HEIFCompressionHEVC})
.GIF(format.GIFOptions{})
.TIFF(format.TIFFOptions{Compression: format.TIFFCompressionLZW, Tile: true})
.JXL(format.JXLOptions{Quality: 90})
.JP2(format.JP2Options{Quality: 50})
.Raw(format.RawOptions{Depth: format.RawDepthUchar})   // uncompressed packed pixels

// Dispatcher for dynamic format choice
.ToFormat(sharp.FormatWebP, format.WebPOptions{Quality: 80})
.ToFormat("avif", nil)  // string also works; nil uses defaults

// Terminal calls
data, info, err := pipeline.ToBytes(ctx)
info, err            := pipeline.ToFile(ctx, "out.jpg")    // infers format from extension
info, err            := pipeline.ToWriter(ctx, w)
info, err            := pipeline.ToTiles(ctx, "pyramid", sharp.TileOptions{Layout: sharp.TileLayoutDZ})
```

### Advanced WebP (libwebp-direct)

Setting `UseSharpYUV` routes encoding through a libwebp-direct path that exposes
knobs `vips_webpsave` hides — sharper chroma and tighter byte budgets at the
same quality. On photographic content this measures ~0.10 butteraugli / +2
ssimulacra2 better than the default WebP saver at equal size.

```go
.WebP(format.WebPOptions{
    Quality:     75,
    Effort:      4,      // 0 (fast) – 6 (small)
    UseSharpYUV: true,   // sharper RGB→YUV; enables the knobs below
    Multithread: true,   // parallel token-partition encode (~17% faster, sub-0.1% size)
    Preset:      "photo", // "" | picture | photo | drawing | icon | text
    AutoFilter:  true,    // auto-tune the deblocking filter
    SNSStrength: 50,      // spatial-noise-shaping 0–100 (0 = libwebp default)
    TargetSize:  0,       // bytes; >0 makes libwebp bisect Q to hit a budget
    SmartSubsample: true, // higher-quality chroma subsampling
})
```

Without `UseSharpYUV`, `.WebP` uses libvips' standard `webpsave` and the
sharp-only fields above are ignored.

### Stats

```go
st, err := sharp.FromFile("photo.jpg").Stats(ctx)
// st.Channels[i].Min/Max/Mean/Deviation/Sum/SumSquare
```

### Compare (image similarity)

Native, dependency-free similarity metrics between two pipelines — useful for
regression-testing an optimiser or comparing encoder settings. Both inputs are
realised to pixels, normalised to sRGB, and `cmp` is auto-resized to `ref`'s
dimensions (Lanczos3) when they differ.

```go
res, err := sharp.Compare(ctx,
    sharp.FromFile("original.png"),
    sharp.FromFile("original.png").Resize(sharp.ResizeOptions{Width: 800}).WebP(format.WebPOptions{Quality: 80}),
    sharp.CompareOptions{DeltaEMethod: sharp.DeltaE2000}, // DeltaE2000 (default) | DeltaE76 | DeltaECMC
)
// res.RMSE   sRGB 8-bit units; 0 = identical
// res.PSNR   dB; +Inf when identical
// res.DeltaE.Mean / .Max   CIE colour difference (computed in LAB)
// res.Width / res.Height   dimensions metrics were computed at (ref's)
```

## Concurrency, cancellation, and limits

```go
sharp.SetConcurrency(8)                                    // libvips thread count
sharp.SetCache(maxOps, maxFiles int, maxMem uint64)        // operation cache

// Batch terminal calls with bounded worker pool. Each *Image is independent;
// distinct *Image values are safe to evaluate concurrently. Results are
// returned in input order.
images := []*sharp.Image{
    sharp.FromFile("a.jpg").Resize(sharp.ResizeOptions{Width: 800}).WebP(format.WebPOptions{}),
    sharp.FromFile("b.jpg").Resize(sharp.ResizeOptions{Width: 800}).WebP(format.WebPOptions{}),
    // ...
}
results := sharp.ToBytesAll(ctx, images, sharp.BatchOptions{
    Concurrency:      8,
    StopOnFirstError: true,
    PerJobTimeout:    10 * time.Second,
})
// Companion APIs:
//   sharp.ToFilesAll(ctx, images, paths, opts)
//   sharp.MetadataAll(ctx, images, opts)

// Tuning note: total CPU pressure is roughly
//   BatchOptions.Concurrency × sharp.Concurrency()
// For CPU-bound resize/encode workloads, setting libvips to 1
// (sharp.SetConcurrency(1)) and using a Go-side pool sized to
// runtime.NumCPU() often beats the inverse — try both.

// Per-call ctx cancellation aborts at the next libvips checkpoint.
ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
data, info, err := pipeline.ToBytes(ctx)

// Or use the fluent Timeout:
pipeline.Timeout(5 * time.Second).ToBytes(ctx)

// Disable a risky loader (e.g. PDF) in sandboxed environments.
sharp.Block("VipsForeignLoadPdf")
defer sharp.Unblock("VipsForeignLoadPdf")
```

## Logging

```go
sharp.SetSlogSink(slog.Default())  // route libvips warnings to slog
// Or a custom sink:
sharp.SetLogSink(func(domain string, level sharp.LogLevel, message string) {
    log.Printf("[%s/%v] %s", domain, level, message)
})
```

## Memory / leak monitoring

```go
sharp.TrackedMem()    // libvips-tracked bytes
sharp.TrackedAllocs() // libvips-tracked alloc count
sharp.TrackedFiles()  // libvips-tracked file descriptors
```

## Layout

```
sharp-go/
  sharp.go              version, concurrency
  image.go              *Image type, pipelineOpts
  input.go              FromBytes/File/Reader, Animated/Pages/Page
  output.go             ToBytes/File/Writer, execute()
  create.go             FromCreate/FromText/Join
  resize.go             Resize, edge-gravity crop
  rotate.go             Rotate/AutoOrient/Flip/Flop, Extract
  layout.go             Extend/Trim/Affine
  operation.go          Blur/Sharpen/Median/Gamma/Negate/Threshold/Linear/
                        Modulate/Normalise/Clahe/Convolve/Boolean/Recomb/
                        Dilate/Erode/Flatten
  colour.go             Tint/Greyscale/PipelineColourspace/ToColourspace
  channel.go            RemoveAlpha/EnsureAlpha/ExtractChannel/JoinChannel/Bandbool
  composite.go          Composite + blend modes + gravity
  keep.go               KeepMetadata/Exif/Xmp/Iptc/IccProfile
  with_metadata.go      WithMetadata/Exif/ICCProfile/Xmp
  metadata.go           Metadata()
  stats.go              Stats()
  compare.go            Compare() — RMSE/PSNR/deltaE similarity metrics
  tile.go               ToTiles (DZI/Zoomify/IIIF)
  toformat.go           ToFormat dispatcher
  utility.go            V/SupportedFormats/Block/Unblock/Tracked*
  log.go                SetLogSink/SetSlogSink
  options.go            shared types (Color, Fit, Position, Kernel)
  errors.go             typed errors

  format/               per-encoder options structs
    jpeg.go png.go webp.go gif.go tiff.go heif.go avif.go jxl.go jp2.go raw.go

  internal/vips/        cgo binding (C only, no C++)
    cgo.go              init, version, concurrency, cache, blocking, tracked
    bridge.{c,h}        plain-C helpers
    image.go            *VipsImage wrapper, runtime.AddCleanup
    load.go save.go     foreign loaders/savers
    op_*.go             libvips op wrappers per category
    metadata.go         header accessors
    log.go              g_log handler + slog routing
    error.go            vips_error_buffer pump

  cmd/sharpgo/          CLI (resize/convert/metadata/composite/info)
  cmd/sharpgo-doctor/   env probe (libvips version + format support)
  examples/             proxy (flagship), thumbnail, format-convert, watermark
```

## Command-line tools

```sh
go install github.com/mdsohelmia/sharp-go/cmd/sharpgo@latest
go install github.com/mdsohelmia/sharp-go/cmd/sharpgo-doctor@latest
```

`sharpgo` is a thin CLI over the library:

```sh
sharpgo resize    -w 800 -h 600 -fit cover in.jpg out.webp
sharpgo convert   -q 80 in.png out.avif
sharpgo metadata  in.jpg                      # JSON
sharpgo composite -overlay logo.png -gravity southeast in.jpg out.jpg
sharpgo info                                  # libvips capabilities
sharpgo help
```

`sharpgo-doctor` prints the detected libvips version and per-format load/save
support — run it first when diagnosing a build.

## Examples

Runnable programs under `examples/`:

| dir | what it shows |
|-----|---------------|
| `proxy` | flagship HTTP image-optimization proxy (resize + AVIF/WebP, origin + output disk cache) |
| `thumbnail` | shrink-on-load thumbnailing |
| `format-convert` | batch format conversion |
| `watermark` | compositing a logo overlay |

```sh
make proxy                       # run the proxy on :3003 (PROXY_PORT to override)
go run ./examples/thumbnail
```

## Building & development

A `Makefile` wraps the common workflows (run `make help` for the full list):

```sh
make build        # go build ./...
make test         # full suite (fixture tests skip if test/fixtures is absent)
make test-race    # race detector
make bench        # benchmarks
make cover        # coverage profile + summary
make vet          # go vet
make fmt          # gofmt -w .
make check        # vet + race tests
make doctor       # libvips capability probe
make install      # install the sharpgo + sharpgo-doctor CLIs
make deps-help    # libvips install command per platform
```

Override the Go toolchain with `make GO=go1.24 test`.

### Test fixtures

The fixture-based tests read images from `test/fixtures/` (sourced from the
upstream sharp repo). The helper **skips** any test whose fixture is missing,
so `go test ./...` passes on a fresh clone without them — but to run the full
suite, populate `test/fixtures/` with the upstream sharp fixtures.

## License

Apache-2.0 (matches upstream sharp) — see [LICENSE](LICENSE). Portions of
`internal/vips` are adapted from [imgproxy](https://github.com/imgproxy/imgproxy)
(also Apache-2.0); attribution is in [NOTICE](NOTICE).
