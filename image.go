package sharp

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mdsohelmia/sharp-go/format"
)

// imagePool recycles *Image handles across pipeline calls. The pool is fed
// from terminal methods (ToBytes / ToFile / ToWriter / Metadata / Stats /
// Release of caller) after the underlying libvips work completes.
//
// Pooled handles are *opt-in*: a fresh *Image is allocated when the pool is
// empty, and stale handles that escape Release are reclaimed by GC. Callers
// must not retain a pipeline handle after the terminal call — a sticky
// "released" flag guards against use-after-recycle.
var imagePool = sync.Pool{
	New: func() any { return &Image{} },
}

// acquireImage draws an *Image from the pool with all fields zeroed.
func acquireImage() *Image {
	im := imagePool.Get().(*Image)
	im.reset()
	return im
}

// recycle returns im to the pool after marking it as released. Calling
// recycle is safe even when the caller still holds a reference — subsequent
// chain methods will return a sticky ErrImageReleased instead of mutating
// pool-managed state.
func (im *Image) recycle() {
	if im == nil {
		return
	}
	atomic.StoreUint32(&im.released, 1)
	im.reset()
	imagePool.Put(im)
}

// reset zeroes all fields so the handle can be reused. Equivalent to
// `*im = Image{}` but inlined for clarity.
func (im *Image) reset() {
	if im.opts.composite != nil {
		im.opts.composite = im.opts.composite[:0] // keep backing array
	}
	composite := im.opts.composite
	*im = Image{}
	im.opts.composite = composite
}


// Image records a sequence of operations to apply to a single input image.
// Operations are deferred; libvips is only invoked by terminal methods
// (ToBytes, ToFile, Metadata, Stats).
//
// *Image is not safe for concurrent option mutation. Clone first for
// parallel variants. Concurrent terminal calls on distinct *Image values
// are safe.
type Image struct {
	in       inputSource
	opts     pipelineOpts
	err      error
	released uint32 // sticky: 1 once recycled into imagePool
}

// inputSource is the recorded input. Exactly one of bytes/path/synth/reader
// is set; if raw is non-nil, bytes is interpreted as raw pixel data with the
// given layout rather than a compressed image.
type inputSource struct {
	bytes  []byte
	path   string
	raw    *format.RawInput
	synth  *synthSpec
	reader io.Reader // streaming input; nil unless FromReader/FromURL used
	closer io.Closer // optional: e.g., http.Response.Body — closed after pipeline

	// pages: 1 (default) loads only the first page of multi-page images.
	// -1 loads all pages (animated GIF/WebP/HEIF/TIFF). >0 loads N pages.
	pages int
	// page: starting page index (0-based) for multi-page input.
	page int
}

// synthSpec drives FromCreate/FromText/Join — the image is built directly in
// libvips at terminal-call time rather than decoded from bytes.
type synthSpec struct {
	kind   synthKind
	create CreateOptions
	text   TextOptions
	join   []*Image
	join2  JoinOptions
}

type synthKind int

const (
	synthCreate synthKind = iota
	synthText
	synthJoin
)

// pipelineOpts is the recorded option set. As more operations land, fields
// are added here in the same shape as sharp's PipelineBaton (src/pipeline.h).
type pipelineOpts struct {
	formatOut formatID

	extract       *ExtractRegion
	trim          *TrimOptions
	autoOrient    bool
	rotate        *RotateOptions
	affine        *AffineOptions
	resize        *ResizeOptions
	extend        *ExtendOptions
	composite     []CompositeLayer
	flatten       *FlattenOptions
	tint          *TintOptions
	greyscale     bool
	modulate      *ModulateOptions
	normalise     *NormaliseOptions
	clahe         *ClaheOptions
	gamma         *GammaOptions
	median        *MedianOptions
	blur          *BlurOptions
	sharpen       *SharpenOptions
	convolve      *ConvolveOptions
	threshold     *ThresholdOptions
	boolean       *BooleanOptions
	recomb        *RecombOptions
	linear        *LinearOptions
	dilate        *MorphOptions
	erode         *MorphOptions
	negate        *NegateOptions
	removeAlpha         bool
	ensureAlpha         *EnsureAlphaOptions
	extractChannel      *int
	joinChannel         *JoinChannelOptions
	bandbool            *BandboolOptions
	pipelineColourspace *ColourspaceID
	toColourspace       *ColourspaceID
	ensureSRGB          bool
	keepFlags           int
	withMetadata        *WithMetadataOptions
	withExif            *WithExifOptions
	withICCProfile      *WithICCProfileOptions
	withXmp             *WithXmpOptions
	timeout             time.Duration
	flip                bool
	flop                bool

	jpeg format.JPEGOptions
	png  format.PNGOptions
	webp format.WebPOptions
	gif  format.GIFOptions
	tiff format.TIFFOptions
	heif format.HEIFOptions
	avif format.AVIFOptions
	raw  format.RawOptions
	jxl  format.JXLOptions
	jp2  format.JP2Options
}

// formatID enumerates the output formats sharp supports.
type formatID int

const (
	formatUnknown formatID = iota
	formatJPEG
	formatPNG
	formatWebP
	formatGIF
	formatTIFF
	formatHEIF
	formatAVIF
	formatRaw
	formatJXL
	formatJP2
)

// Info is the post-execution descriptor returned alongside output bytes.
type Info struct {
	Format   string
	Width    int
	Height   int
	Channels int
	Size     int
}

// Clone returns a deep copy of opts. The underlying input bytes are shared
// (immutable from the caller's perspective). The clone draws from the
// internal handle pool to amortise allocation under high throughput.
func (im *Image) Clone() *Image {
	if im == nil {
		return nil
	}
	c := acquireImage()
	c.in = im.in
	c.opts = im.opts
	c.err = im.err
	// released stays 0 from acquireImage's reset().
	return c
}

// Err returns the first sticky error recorded, if any. Terminal methods
// return this error before invoking libvips.
func (im *Image) Err() error { return im.err }

// Recycle returns the handle to the internal pool for reuse. Use this in
// throughput-sensitive code paths that finish with the *Image after a
// terminal call (ToBytes / ToFile / Metadata / Stats). Calling Recycle is
// optional — handles are also reclaimed by GC. After Recycle, the handle
// must not be used; subsequent chain methods record ErrImageReleased.
func (im *Image) Recycle() {
	im.recycle()
}

// stickyErr records err as the first sticky error if none is set.
func (im *Image) stickyErr(err error) {
	if im.err == nil && err != nil {
		im.err = err
	}
}

// JPEG sets the output format to JPEG and records the encoder options.
func (im *Image) JPEG(opts format.JPEGOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatJPEG
	im.opts.jpeg = opts
	return im
}

// PNG sets the output format to PNG and records the encoder options.
func (im *Image) PNG(opts format.PNGOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatPNG
	im.opts.png = opts
	return im
}

// WebP sets the output format to WebP and records the encoder options.
func (im *Image) WebP(opts format.WebPOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatWebP
	im.opts.webp = opts
	return im
}

// GIF sets the output format to GIF and records the encoder options.
func (im *Image) GIF(opts format.GIFOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatGIF
	im.opts.gif = opts
	return im
}

// TIFF sets the output format to TIFF and records the encoder options.
func (im *Image) TIFF(opts format.TIFFOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatTIFF
	im.opts.tiff = opts
	return im
}

// HEIF sets the output format to HEIF (HEVC/HEIC by default).
func (im *Image) HEIF(opts format.HEIFOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatHEIF
	im.opts.heif = opts
	return im
}

// AVIF sets the output format to AVIF (HEIF container, AV1 codec).
func (im *Image) AVIF(opts format.AVIFOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatAVIF
	im.opts.avif = opts
	return im
}

// Raw sets the output format to raw pixel bytes (no container).
func (im *Image) Raw(opts format.RawOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatRaw
	im.opts.raw = opts
	return im
}

// JXL sets the output format to JPEG XL.
func (im *Image) JXL(opts format.JXLOptions) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatJXL
	im.opts.jxl = opts
	return im
}

// JP2 sets the output format to JPEG 2000.
func (im *Image) JP2(opts format.JP2Options) *Image {
	if im.err != nil {
		return im
	}
	im.opts.formatOut = formatJP2
	im.opts.jp2 = opts
	return im
}

// ToBytes executes the recorded pipeline and returns the encoded bytes. ctx
// cancellation aborts in-flight libvips ops at the next checkpoint.
func (im *Image) ToBytes(ctx context.Context) ([]byte, Info, error) {
	if im.err != nil {
		return nil, Info{}, im.err
	}
	if im.opts.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, im.opts.timeout)
		defer cancel()
	}
	if err := ctx.Err(); err != nil {
		return nil, Info{}, err
	}
	return execute(ctx, im)
}

// Timeout records an upper bound for the pipeline execution. Mirrors sharp's
// .timeout({seconds:N}). Zero (default) means no timeout.
func (im *Image) Timeout(d time.Duration) *Image {
	if im.err != nil {
		return im
	}
	im.opts.timeout = d
	return im
}
