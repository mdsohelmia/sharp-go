package sharp

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/sohelmia/sharp-go/format"
	"github.com/sohelmia/sharp-go/internal/vips"
)

// ToFile executes the pipeline and writes the result to path. If no output
// format has been set explicitly via JPEG/PNG/etc., the format is inferred
// from the file extension.
func (im *Image) ToFile(ctx context.Context, path string) (Info, error) {
	if im.err != nil {
		return Info{}, im.err
	}
	if path == "" {
		return Info{}, errors.New("sharp: empty output path")
	}
	if im.opts.formatOut == formatUnknown {
		im.opts.formatOut = formatFromExt(path)
		if im.opts.formatOut == formatUnknown {
			return Info{}, errors.New("sharp: cannot infer output format from extension")
		}
	}
	data, info, err := im.ToBytes(ctx)
	if err != nil {
		return Info{}, err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return Info{}, err
	}
	return info, nil
}

// ToWriter executes the pipeline and writes the result to w. For JPEG and
// PNG output, bytes stream directly into w via libvips' VipsTarget — no
// intermediate buffer. Other formats fall back to ToBytes + io.Copy.
func (im *Image) ToWriter(ctx context.Context, w io.Writer) (Info, error) {
	if im.err != nil {
		return Info{}, im.err
	}
	switch im.opts.formatOut {
	case formatJPEG, formatPNG:
		return im.streamTo(ctx, w)
	}
	data, info, err := im.ToBytes(ctx)
	if err != nil {
		return Info{}, err
	}
	if _, err := w.Write(data); err != nil {
		return Info{}, err
	}
	info.Size = len(data)
	return info, nil
}

// streamTo runs the pipeline up to the encode step, then routes encoded
// bytes through a VipsTarget into w.
func (im *Image) streamTo(ctx context.Context, w io.Writer) (Info, error) {
	// Pin to one OS thread for the whole pipeline: libvips' error buffer is
	// thread-local, so the op that fails and the lastError() that reads it
	// must run on the same thread, else a goroutine reschedule between cgo
	// calls would surface a stale or empty message.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if im.opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, im.opts.timeout)
		defer cancel()
	}
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	vimg, stop, err := buildPipelineImage(ctx, im)
	if err != nil {
		return Info{}, err
	}
	defer stop()
	if im.opts.withMetadata != nil || im.opts.withExif != nil ||
		im.opts.withICCProfile != nil || im.opts.withXmp != nil {
		vimg, err = applyWithMetadata(vimg, &im.opts)
		if err != nil {
			return Info{}, err
		}
	}
	vips.ApplyKeep(vimg, vips.KeepFlags(im.opts.keepFlags))

	t, err := vips.NewTarget(w)
	if err != nil {
		return Info{}, err
	}
	defer t.Close()

	switch im.opts.formatOut {
	case formatJPEG:
		if err := vips.SaveJPEGTarget(vimg, t, jpegParamsFrom(im.opts.jpeg)); err != nil {
			return Info{}, err
		}
		return Info{
			Format: "jpeg", Width: vimg.Width(), Height: vimg.Height(),
			Channels: vimg.Bands(),
		}, nil
	case formatPNG:
		if err := vips.SavePNGTarget(vimg, t, pngParamsFrom(im.opts.png)); err != nil {
			return Info{}, err
		}
		return Info{
			Format: "png", Width: vimg.Width(), Height: vimg.Height(),
			Channels: vimg.Bands(),
		}, nil
	}
	return Info{}, errors.New("sharp: streamTo unreachable")
}

func formatFromExt(path string) formatID {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return formatJPEG
	case ".png":
		return formatPNG
	case ".webp":
		return formatWebP
	case ".gif":
		return formatGIF
	case ".tif", ".tiff":
		return formatTIFF
	case ".heic", ".heif":
		return formatHEIF
	case ".avif":
		return formatAVIF
	case ".jxl":
		return formatJXL
	case ".jp2", ".j2k", ".jpx", ".jpf":
		return formatJP2
	default:
		return formatUnknown
	}
}

// buildPipelineImage runs the load + all op steps and returns the *vips.Image
// ready for encoding. Shared between execute() (buffer path) and streamTo().
//
// When the recorded pipeline is dominated by a resize operation, libvips'
// vips_thumbnail_{buffer,source} is invoked directly on the source bytes —
// fusing the decode and resize steps so shrink-on-load engages (JPEG DCT
// scale, PNG/WebP/HEIF native subsample). The Resize and (optionally) the
// sRGB transform are marked consumed before applyAllOps sees them.
func buildPipelineImage(ctx context.Context, im *Image) (*vips.Image, func(), error) {
	noStop := func() {}
	if err := vips.InitError(); err != nil {
		return nil, noStop, err
	}

	// Local copy: applyAllOps may mutate ops as the fused path consumes
	// resize / ensureSRGB. We must not touch im.opts since callers may share
	// it via Clone (which copies the struct but aliases pointer fields).
	opts := im.opts

	if im.in.closer != nil {
		defer im.in.closer.Close()
	}

	var (
		vimg *vips.Image
		err  error
	)
	switch {
	case im.in.synth != nil:
		vimg, err = renderSynth(im.in.synth)

	case im.in.raw != nil:
		buf, rerr := readInput(im.in)
		if rerr != nil {
			return nil, noStop, rerr
		}
		r := im.in.raw
		vimg, err = vips.LoadRawBuffer(buf, r.Width, r.Height, r.Channels, mapDepth(r.Depth))

	case im.in.reader != nil:
		vimg, err = loadFromReader(im.in.reader)

	case im.in.pages != 0 || im.in.page != 0:
		buf, rerr := readInput(im.in)
		if rerr != nil {
			return nil, noStop, rerr
		}
		pages := im.in.pages
		if pages == 0 {
			pages = 1
		}
		vimg, err = vips.LoadBufferPages(buf, pages, im.in.page)

	default:
		buf, rerr := readInput(im.in)
		if rerr != nil {
			return nil, noStop, rerr
		}
		if canFuseThumbnail(&opts) {
			vimg, err = loadFusedThumbnail(buf, &opts)
		} else {
			vimg, err = vips.LoadBuffer(buf)
		}
	}
	if err != nil {
		return nil, noStop, err
	}

	vimg, err = applyAllOps(vimg, &opts)
	if err != nil {
		return nil, noStop, err
	}

	// ctx watcher: on cancellation, flag the final image as killed so the
	// in-flight libvips computation aborts at the next checkpoint. Because
	// libvips is lazy, essentially all the pixel work happens in the encode
	// (the sink) performed by the CALLER after this function returns — so the
	// watcher must outlive buildPipelineImage. We return its stop func and let
	// the caller release it once encoding finishes. context.AfterFunc reuses
	// ctx's existing cancellation machinery: no goroutine, no channel, and
	// nothing registered for an uncancellable context.Background().
	stop := noStop
	if ctx.Done() != nil {
		final := vimg
		cancel := context.AfterFunc(ctx, func() { final.Kill() })
		stop = func() { cancel() }
	}
	return vimg, stop, nil
}

// canFuseThumbnail reports whether the pipeline can route through
// vips_thumbnail_{buffer,source}. The fused path requires the resize step
// to operate on the original source pixels (no pre-resize crop / affine /
// trim) and excludes paths that the post-decode applyResize handles
// specially.
func canFuseThumbnail(o *pipelineOpts) bool {
	if o.resize == nil {
		return false
	}
	if o.trim != nil || o.extract != nil || o.affine != nil {
		return false
	}
	// autoOrient no longer blocks fusion: libvips' thumbnail applies the EXIF
	// orientation tag (and strips it) before resizing when no_rotate=false,
	// which is exactly sharp's autoOrient semantics. The fused path sets
	// NoRotate=false and clears opts.autoOrient (see loadFusedThumbnail).
	if o.resize.Fit == FitCover && isEdgeGravity(o.resize.Position) {
		return false
	}
	return true
}

// loadFromReader handles streaming input. The source is fully decoded into
// libvips-managed memory within this call (vips_image_copy_memory), so the
// reader — and any closer the caller drops when buildPipelineImage returns —
// is never read lazily during the later encode step. Resize runs post-decode
// via applyAllOps.
//
// Shrink-on-load fusion is intentionally NOT used for reader inputs: a fused
// thumbnail is a lazy pipeline that would read from the stream at encode time,
// after the caller's closer (e.g. an HTTP response body) has already been
// closed. Buffer and file inputs keep fusion (see loadFusedThumbnail) because
// their backing bytes have no closer and their lifetime is bound to the image.
func loadFromReader(r io.Reader) (*vips.Image, error) {
	src, err := vips.NewSource(r)
	if err != nil {
		return nil, err
	}
	defer src.Close()
	return vips.LoadSource(src)
}

// loadFusedThumbnail fuses load + resize over buf via a streaming Source
// (vips_thumbnail_source) so shrink-on-load engages while buf's lifetime is
// bound to the resulting image. On success, opts.resize is cleared so
// applyAllOps does not re-run the resize. If opts.ensureSRGB is set, the
// export profile is hoisted into the thumbnail call and ensureSRGB cleared.
func loadFusedThumbnail(buf []byte, opts *pipelineOpts) (*vips.Image, error) {
	r := opts.resize
	// The fused thumbnail needs a known width. When only height is supplied
	// (or neither), read the header lazily to compute the missing dimension —
	// far cheaper than a full decode.
	width, height := r.Width, r.Height
	if width <= 0 || height <= 0 {
		hdr, err := vips.LoadBufferLazy(buf)
		if err != nil {
			return nil, err
		}
		sw, sh := hdr.Width(), hdr.Height()
		// When autoOrient is fused into the thumbnail, the resize targets
		// apply to the *oriented* image. The header reports pre-orientation
		// dims, so swap them for quarter-turn orientations (5-8) before
		// deriving the missing dimension.
		if opts.autoOrient {
			switch hdr.Orientation() {
			case 5, 6, 7, 8:
				sw, sh = sh, sw
			}
		}
		width, height = resizeDimensions(r, sw, sh)
	}

	p := resizeThumbnailParams(r, width, height)
	if opts.ensureSRGB {
		p.ExportProfile = "srgb"
	}
	if opts.autoOrient {
		// Let thumbnail apply EXIF orientation before resizing (sharp's
		// autoOrient semantics) so shrink-on-load stays engaged.
		p.NoRotate = false
	}
	out, err := vips.ThumbnailBuffer(buf, p)
	if err != nil {
		return nil, err
	}

	// Mark the fused steps consumed so applyAllOps skips them.
	opts.resize = nil
	if opts.ensureSRGB {
		opts.ensureSRGB = false
	}
	if opts.autoOrient {
		opts.autoOrient = false
	}

	// FitContain padding still needs to run post-thumbnail; do it here so
	// applyAllOps stays a pure op-chain runner.
	return applyResizeContainPadding(out, r)
}

// execute is the single pipeline entry — terminal methods funnel through it.
// Applies all recorded ops in sharp's pipeline order and encodes to the
// requested format.
func execute(ctx context.Context, im *Image) ([]byte, Info, error) {
	// See streamTo: pin to one OS thread so libvips' thread-local error buffer
	// is read on the thread that wrote it.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	vimg, stop, err := buildPipelineImage(ctx, im)
	if err != nil {
		return nil, Info{}, err
	}
	defer stop()

	if im.opts.withMetadata != nil || im.opts.withExif != nil ||
		im.opts.withICCProfile != nil || im.opts.withXmp != nil {
		vimg, err = applyWithMetadata(vimg, &im.opts)
		if err != nil {
			return nil, Info{}, err
		}
	}
	vips.ApplyKeep(vimg, vips.KeepFlags(im.opts.keepFlags))

	return encodePipeline(vimg, im)
}

// applyAllOps runs every recorded operation in sharp's pipeline order against
// vimg, returning the final post-ops image. Operations consumed by the fused
// load path (resize / ensureSRGB) are skipped when the caller cleared them.
func applyAllOps(vimg *vips.Image, o *pipelineOpts) (*vips.Image, error) {
	var err error
	// ICC transform to sRGB happens first so all subsequent ops (trim,
	// resize, composite, …) run on universal sRGB pixels rather than
	// wide-gamut source colours.
	if o.ensureSRGB {
		vimg, err = vips.ICCTransform(vimg, "srgb", "srgb")
		if err != nil {
			return nil, err
		}
	}
	if o.trim != nil {
		vimg, err = applyTrim(vimg, o.trim)
		if err != nil {
			return nil, err
		}
	}
	if o.extract != nil {
		r := o.extract
		vimg, err = vips.ExtractArea(vimg, r.Left, r.Top, r.Width, r.Height)
		if err != nil {
			return nil, err
		}
	}
	if o.autoOrient {
		vimg, err = vips.Autorot(vimg)
		if err != nil {
			return nil, err
		}
	}
	if o.rotate != nil {
		vimg, err = applyRotate(vimg, o.rotate)
		if err != nil {
			return nil, err
		}
	}
	if o.affine != nil {
		vimg, err = applyAffine(vimg, o.affine)
		if err != nil {
			return nil, err
		}
	}
	if o.resize != nil {
		vimg, err = applyResize(vimg, o.resize)
		if err != nil {
			return nil, err
		}
	}
	if o.extend != nil {
		vimg, err = applyExtend(vimg, o.extend)
		if err != nil {
			return nil, err
		}
	}
	if len(o.composite) > 0 {
		vimg, err = applyComposite(vimg, o.composite)
		if err != nil {
			return nil, err
		}
	}
	if o.pipelineColourspace != nil {
		vimg, err = vips.Colourspace(vimg, vips.Interpretation(*o.pipelineColourspace))
		if err != nil {
			return nil, err
		}
	}
	if o.flatten != nil {
		bg := o.flatten.Background
		vimg, err = vips.Flatten(vimg, bg.R, bg.G, bg.B)
		if err != nil {
			return nil, err
		}
	}
	if o.normalise != nil {
		n := o.normalise
		vimg, err = vips.Normalise(vimg, n.Lower, n.Upper)
		if err != nil {
			return nil, err
		}
	}
	if o.clahe != nil {
		c := o.clahe
		vimg, err = vips.Clahe(vimg, c.Width, c.Height, c.MaxSlope)
		if err != nil {
			return nil, err
		}
	}
	if o.modulate != nil {
		m := o.modulate
		vimg, err = vips.Modulate(vimg, m.Brightness, m.Saturation, m.Hue, m.Lightness)
		if err != nil {
			return nil, err
		}
	}
	if o.tint != nil {
		c := o.tint.Colour
		vimg, err = vips.Tint(vimg, c.R, c.G, c.B)
		if err != nil {
			return nil, err
		}
	}
	if o.greyscale {
		vimg, err = vips.Greyscale(vimg)
		if err != nil {
			return nil, err
		}
	}
	if o.recomb != nil {
		r := o.recomb
		vimg, err = vips.Recomb(vimg, r.Matrix, r.N)
		if err != nil {
			return nil, err
		}
	}
	if o.gamma != nil {
		g := o.gamma
		vimg, err = vips.Gamma(vimg, g.Exponent, g.ExponentOut)
		if err != nil {
			return nil, err
		}
	}
	if o.median != nil {
		vimg, err = vips.Median(vimg, o.median.Size)
		if err != nil {
			return nil, err
		}
	}
	if o.blur != nil {
		vimg, err = vips.Gaussblur(vimg, o.blur.Sigma)
		if err != nil {
			return nil, err
		}
	}
	if o.sharpen != nil {
		s := o.sharpen
		vimg, err = vips.Sharpen(vimg, vips.SharpenParams{
			Sigma: s.Sigma, M1: s.M1, M2: s.M2,
			X1: s.X1, Y2: s.Y2, Y3: s.Y3,
		})
		if err != nil {
			return nil, err
		}
	}
	if o.threshold != nil {
		t := o.threshold
		vimg, err = vips.Threshold(vimg, t.Value, t.Grayscale)
		if err != nil {
			return nil, err
		}
	}
	if o.boolean != nil {
		b := o.boolean
		vimg, err = vips.BooleanConst(vimg, mapBooleanOp(b.Op), b.Constant)
		if err != nil {
			return nil, err
		}
	}
	if o.linear != nil {
		l := o.linear
		vimg, err = vips.Linear(vimg, l.A, l.B)
		if err != nil {
			return nil, err
		}
	}
	if o.convolve != nil {
		c := o.convolve
		vimg, err = vips.Convolve(vimg, vips.ConvolveParams{
			Kernel: c.Kernel, Width: c.Width, Height: c.Height,
			Scale: c.Scale, Offset: c.Offset,
		})
		if err != nil {
			return nil, err
		}
	}
	if o.dilate != nil {
		vimg, err = vips.Morph(vimg, o.dilate.Size, vips.MorphDilate)
		if err != nil {
			return nil, err
		}
	}
	if o.erode != nil {
		vimg, err = vips.Morph(vimg, o.erode.Size, vips.MorphErode)
		if err != nil {
			return nil, err
		}
	}
	if o.negate != nil {
		vimg, err = vips.Negate(vimg, o.negate.KeepAlpha)
		if err != nil {
			return nil, err
		}
	}
	if o.removeAlpha {
		vimg, err = vips.RemoveAlpha(vimg)
		if err != nil {
			return nil, err
		}
	}
	if o.ensureAlpha != nil {
		vimg, err = vips.EnsureAlpha(vimg, o.ensureAlpha.Alpha)
		if err != nil {
			return nil, err
		}
	}
	if o.extractChannel != nil {
		vimg, err = vips.ExtractBand(vimg, *o.extractChannel)
		if err != nil {
			return nil, err
		}
	}
	if o.joinChannel != nil {
		vimg, err = applyJoinChannel(vimg, o.joinChannel)
		if err != nil {
			return nil, err
		}
	}
	if o.bandbool != nil {
		vimg, err = vips.Bandbool(vimg, mapBooleanOp(o.bandbool.Op))
		if err != nil {
			return nil, err
		}
	}
	if o.toColourspace != nil {
		vimg, err = vips.Colourspace(vimg, vips.Interpretation(*o.toColourspace))
		if err != nil {
			return nil, err
		}
	}
	if o.flip {
		vimg, err = vips.Flip(vimg, vips.DirectionVertical)
		if err != nil {
			return nil, err
		}
	}
	if o.flop {
		vimg, err = vips.Flip(vimg, vips.DirectionHorizontal)
		if err != nil {
			return nil, err
		}
	}

	return vimg, nil
}

// encodePipeline performs the final encode step.
func encodePipeline(vimg *vips.Image, im *Image) ([]byte, Info, error) {
	switch im.opts.formatOut {
	case formatJPEG, formatUnknown:
		// JPEG is the fallback format when none specified, matching sharp's
		// toBuffer() behaviour with no explicit format call on a JPEG input.
		out, err := vips.SaveJPEG(vimg, jpegParamsFrom(im.opts.jpeg))
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "jpeg",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	case formatPNG:
		out, err := vips.SavePNG(vimg, pngParamsFrom(im.opts.png))
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "png",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	case formatWebP:
		var out []byte
		var err error
		if im.opts.webp.UseSharpYUV {
			// libwebp-direct path exposes use_sharp_yuv + autofilter +
			// sns_strength + target_size that vips_webpsave_buffer hides.
			out, err = vips.SaveWebPSharpYUV(vimg, webpSharpYUVParamsFrom(im.opts.webp))
		} else {
			out, err = vips.SaveWebP(vimg, webpParamsFrom(im.opts.webp))
		}
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "webp",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	case formatGIF:
		out, err := vips.SaveGIF(vimg, gifParamsFrom(im.opts.gif))
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "gif",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	case formatTIFF:
		out, err := vips.SaveTIFF(vimg, tiffParamsFrom(im.opts.tiff))
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "tiff",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	case formatHEIF:
		out, err := vips.SaveHEIF(vimg, heifParamsFrom(im.opts.heif))
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "heif",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	case formatAVIF:
		out, err := vips.SaveHEIF(vimg, avifParamsFrom(im.opts.avif))
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "avif",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	case formatRaw:
		out, err := vips.SaveRaw(vimg, mapDepth(im.opts.raw.Depth))
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "raw",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	case formatJXL:
		out, err := vips.SaveJXL(vimg, jxlParamsFrom(im.opts.jxl))
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "jxl",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	case formatJP2:
		out, err := vips.SaveJP2(vimg, jp2ParamsFrom(im.opts.jp2))
		if err != nil {
			return nil, Info{}, err
		}
		return out, Info{
			Format:   "jp2",
			Width:    vimg.Width(),
			Height:   vimg.Height(),
			Channels: vimg.Bands(),
			Size:     len(out),
		}, nil

	default:
		return nil, Info{}, errors.New("sharp: unsupported output format")
	}
}

func webpParamsFrom(o format.WebPOptions) vips.WebPParams {
	q := o.Quality
	if q == 0 {
		q = 80
	}
	aq := o.AlphaQuality
	if aq == 0 {
		aq = 100
	}
	e := o.Effort
	if e == 0 {
		e = 4
	}
	return vips.WebPParams{
		Quality:        q,
		AlphaQuality:   aq,
		Lossless:       o.Lossless,
		NearLossless:   o.NearLossless,
		SmartSubsample: o.SmartSubsample,
		SmartDeblock:   o.SmartDeblock,
		Passes:         o.Passes,
		Preset:         mapWebPPreset(o.Preset),
		Effort:         e,
		Loop:           o.Loop,
		MinSize:        o.MinSize,
		Mixed:          o.Mixed,
	}
}

func webpSharpYUVParamsFrom(o format.WebPOptions) vips.WebPSharpYUVParams {
	q := o.Quality
	if q == 0 {
		q = 80
	}
	e := o.Effort
	if e == 0 {
		e = 4
	}
	return vips.WebPSharpYUVParams{
		Quality:     q,
		Effort:      e,
		UseSharpYUV: true,
		AutoFilter:  o.AutoFilter,
		SNSStrength: o.SNSStrength,
		TargetSize:  o.TargetSize,
		Passes:      o.Passes,
		Preset:      mapWebPPreset(o.Preset),
	}
}

func mapWebPPreset(p string) vips.WebPPreset {
	switch p {
	case "picture":
		return vips.WebPPresetPicture
	case "photo":
		return vips.WebPPresetPhoto
	case "drawing":
		return vips.WebPPresetDrawing
	case "icon":
		return vips.WebPPresetIcon
	case "text":
		return vips.WebPPresetText
	}
	return vips.WebPPresetDefault
}

func tiffParamsFrom(o format.TIFFOptions) vips.TIFFParams {
	q := o.Quality
	if q == 0 {
		q = 80
	}
	tw := o.TileWidth
	if tw == 0 {
		tw = 256
	}
	th := o.TileHeight
	if th == 0 {
		th = 256
	}
	return vips.TIFFParams{
		Compression: int(o.Compression),
		Quality:     q,
		Predictor:   int(o.Predictor),
		Tile:        o.Tile,
		TileWidth:   tw,
		TileHeight:  th,
		Pyramid:     o.Pyramid,
		Bitdepth:    o.Bitdepth,
		BigTIFF:     o.BigTIFF,
	}
}

func heifParamsFrom(o format.HEIFOptions) vips.HEIFParams {
	q := o.Quality
	if q == 0 {
		q = 50
	}
	e := o.Effort
	if e == 0 {
		e = 4
	}
	bd := o.Bitdepth
	if bd == 0 {
		bd = 12
	}
	comp := int(o.Compression)
	if comp == 0 {
		comp = vips.HEIFCompressionHEVC
	}
	return vips.HEIFParams{
		Compression:        comp,
		Quality:            q,
		Lossless:           o.Lossless,
		Effort:             e,
		Bitdepth:           bd,
		ChromaSubsample444: o.ChromaSubsampling == "4:4:4",
	}
}

func avifParamsFrom(o format.AVIFOptions) vips.HEIFParams {
	q := o.Quality
	if q == 0 {
		q = 50
	}
	e := o.Effort
	if e == 0 {
		e = 4
	}
	bd := o.Bitdepth
	if bd == 0 {
		bd = 8
	}
	return vips.HEIFParams{
		Compression:        vips.HEIFCompressionAV1,
		Quality:            q,
		Lossless:           o.Lossless,
		Effort:             e,
		Bitdepth:           bd,
		ChromaSubsample444: o.ChromaSubsampling == "4:4:4",
	}
}

func jxlParamsFrom(o format.JXLOptions) vips.JXLParams {
	q := o.Quality
	if q == 0 {
		q = 75
	}
	e := o.Effort
	if e == 0 {
		e = 7
	}
	bd := o.Bitdepth
	if bd == 0 {
		bd = 8
	}
	d := o.Distance
	if d == 0 {
		d = 1
	}
	return vips.JXLParams{
		Quality:  q,
		Tier:     o.Tier,
		Distance: d,
		Effort:   e,
		Lossless: o.Lossless,
		Bitdepth: bd,
	}
}

func jp2ParamsFrom(o format.JP2Options) vips.JP2Params {
	q := o.Quality
	if q == 0 {
		q = 48
	}
	tw := o.TileWidth
	if tw == 0 {
		tw = 512
	}
	th := o.TileHeight
	if th == 0 {
		th = 512
	}
	return vips.JP2Params{
		Quality:            q,
		Lossless:           o.Lossless,
		TileWidth:          tw,
		TileHeight:         th,
		ChromaSubsample444: o.ChromaSubsampling == "4:4:4",
	}
}

func mapDepth(d format.RawDepth) vips.BandFormat {
	switch d {
	case format.RawDepthChar:
		return vips.BandFormatChar
	case format.RawDepthUshort:
		return vips.BandFormatUshort
	case format.RawDepthShort:
		return vips.BandFormatShort
	case format.RawDepthUint:
		return vips.BandFormatUint
	case format.RawDepthInt:
		return vips.BandFormatInt
	case format.RawDepthFloat:
		return vips.BandFormatFloat
	case format.RawDepthDouble:
		return vips.BandFormatDouble
	case format.RawDepthUchar:
		fallthrough
	default:
		return vips.BandFormatUchar
	}
}

func mapBooleanOp(op BooleanOp) vips.BooleanOp {
	switch op {
	case BooleanOr:
		return vips.BooleanOr
	case BooleanEor:
		return vips.BooleanEor
	case BooleanAnd:
		fallthrough
	default:
		return vips.BooleanAnd
	}
}

func gifParamsFrom(o format.GIFOptions) vips.GIFParams {
	e := o.Effort
	if e == 0 {
		e = 7
	}
	bd := o.Bitdepth
	if bd == 0 {
		bd = 8
	}
	ipme := o.InterpaletteMaxError
	if ipme == 0 {
		ipme = 3
	}
	return vips.GIFParams{
		Dither:               o.Dither,
		Effort:               e,
		Bitdepth:             bd,
		InterframeMaxError:   o.InterframeMaxError,
		InterpaletteMaxError: ipme,
		Interlace:            o.Interlace,
		Reuse:                !o.ForceNoReuse,
		KeepDuplicateFrames:  o.KeepDuplicateFrames,
	}
}

func jpegParamsFrom(o format.JPEGOptions) vips.JPEGParams {
	q := o.Quality
	if q == 0 {
		q = 80
	}
	optimiseCoding := true
	if o.OptimiseCoding != nil {
		optimiseCoding = *o.OptimiseCoding
	}
	progressive := o.Progressive
	trellis := o.TrellisQuantisation
	overshoot := o.OvershootDeringing
	optimiseScans := o.OptimiseScans
	if o.MozJPEG {
		progressive = true
		trellis = true
		overshoot = true
		optimiseScans = true
	}
	return vips.JPEGParams{
		Quality:              q,
		Progressive:          progressive,
		OptimiseCoding:       optimiseCoding,
		TrellisQuantisation:  trellis,
		OvershootDeringing:   overshoot,
		OptimiseScans:        optimiseScans,
		QuantisationTable:    o.QuantisationTable,
		ChromaSubsampling444: o.ChromaSubsampling == "4:4:4",
	}
}

func pngParamsFrom(o format.PNGOptions) vips.PNGParams {
	c := o.Compression
	if c == 0 {
		c = 6
	}
	q := o.Quality
	if q == 0 {
		q = 100
	}
	e := o.Effort
	if e == 0 {
		e = 7
	}
	return vips.PNGParams{
		Compression: c,
		Progressive: o.Progressive,
		Palette:     o.Palette,
		Quality:     q,
		Effort:      e,
		Bitdepth:    o.Bitdepth,
	}
}
