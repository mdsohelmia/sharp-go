// bridge.h — plain-C helpers that cgo cannot express inline (libvips uses
// GLib varargs which cgo cannot call directly).
//
// No C++. Compiled by cgo alongside Go sources.

#ifndef SHARPGO_BRIDGE_H
#define SHARPGO_BRIDGE_H

#include <stdint.h>
#include <vips/vips.h>
#include <vips/connection.h>

// Load any supported image from an in-memory buffer with auto format detection.
// The pixel data is fully decoded and copied to libvips-managed memory so the
// caller's buffer may be freed immediately on return.
//
// Returns 0 on success, -1 on failure (call vips_error_buffer for details).
int sharpgo_load_buffer(const void *buf, size_t len, VipsImage **out);

// Encode an image to a JPEG-compressed in-memory buffer.
// On success, *buf is allocated with g_malloc and the caller must g_free it.
int sharpgo_jpegsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,                      // 1-100
    int progressive,                  // 0/1 -> "interlace"
    int optimise_coding,              // 0/1
    int trellis_quantisation,         // 0/1
    int overshoot_deringing,          // 0/1
    int optimise_scans,               // 0/1
    int quantisation_table,           // 0-8
    int chroma_subsampling_444);      // 0=auto (4:2:0), 1=4:4:4

// Encode an image to a PNG-compressed in-memory buffer.
int sharpgo_pngsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int compression,                  // 0-9
    int progressive,                  // 0/1
    int palette,                      // 0/1
    int quality,                      // 1-100, palette only
    int effort,                       // 1-10
    int bitdepth);                    // 0=auto, 1/2/4/8/16

// Encode an image to a WebP-compressed in-memory buffer.
int sharpgo_webpsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,                      // 1-100
    int alpha_quality,                // 0-100
    int lossless,                     // 0/1
    int near_lossless,                // 0/1
    int smart_subsample,              // 0/1
    int effort,                       // 0-6
    int loop,                         // animated loop count; 0=infinite
    int min_size,                     // 0/1
    int mixed,                        // 0/1
    int smart_deblock,                // 0/1 auto-adjust deblocking filter
    int passes,                       // 1-10 entropy-analysis passes; 0=default
    int preset);                      // VipsForeignWebpPreset; 0=default

// sharpgo_webpsave_sharp_yuv bypasses libvips's webpsave and calls libwebp
// directly so we can set WebPConfig.use_sharp_yuv + autofilter +
// sns_strength + target_PSNR + segments — libvips exposes none of these.
// On photographic content this beats vips_webpsave_buffer by ~0.10
// butteraugli / +2 ssimulacra2 at the same byte count.
int sharpgo_webpsave_sharp_yuv(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,
    int effort,
    int use_sharp_yuv,
    int autofilter,
    int sns_strength,
    int target_size,
    float target_psnr,
    int passes,
    int preset,
    int segments,
    int multithread);

// Encode an image to a GIF in-memory buffer.
int sharpgo_gifsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    double dither,                    // 0.0-1.0
    int effort,                       // 1-10
    int bitdepth,                     // 1-8
    int interframe_maxerror,          // 0-32
    int interpalette_maxerror,        // 0-256
    int interlace,                    // 0/1
    int reuse,                        // 0/1
    int keep_duplicate_frames);       // 0/1

// Thumbnail an existing image: produces a width x height result with libvips'
// optimised resize + optional crop. Use crop=VIPS_INTERESTING_NONE for "no
// crop, scale to fit one dimension".
//
// kernel: VipsKernel enum (e.g. VIPS_KERNEL_LANCZOS3)
// size:   VipsSize enum   (VIPS_SIZE_BOTH/UP/DOWN/FORCE)
// crop:   VipsInteresting (VIPS_INTERESTING_NONE/CENTRE/ENTROPY/ATTENTION/LOW/HIGH/ALL)
int sharpgo_thumbnail_image(
    VipsImage *in, VipsImage **out,
    int width, int height,
    int kernel, int size, int crop,
    int no_rotate);

// Embed (pad) an image into a new w x h canvas at offset (x, y), filling the
// background with the supplied RGBA bytes.
int sharpgo_embed(
    VipsImage *in, VipsImage **out,
    int x, int y, int width, int height,
    double bg_r, double bg_g, double bg_b, double bg_a);

// Rotate by a multiple of 90 degrees. angle_quarter: 1=90, 2=180, 3=270.
int sharpgo_rot(VipsImage *in, VipsImage **out, int angle_quarter);

// Rotate by an arbitrary angle (degrees). Pads with background colour where
// the rotated image overflows the bounding box.
int sharpgo_rotate(
    VipsImage *in, VipsImage **out,
    double angle,
    double bg_r, double bg_g, double bg_b, double bg_a);

// Flip across an axis. direction: 0=horizontal (flop), 1=vertical (flip).
int sharpgo_flip(VipsImage *in, VipsImage **out, int direction);

// Apply EXIF orientation and clear the orientation tag.
int sharpgo_autorot(VipsImage *in, VipsImage **out);

// Crop to a sub-region.
int sharpgo_extract_area(
    VipsImage *in, VipsImage **out,
    int left, int top, int width, int height);

// Detect the loader (e.g. "VipsForeignLoadJpegBuffer") for an in-memory image
// without decoding. Returns NULL if the format is unrecognised. The returned
// string is owned by libvips — do not free.
const char *sharpgo_find_load_buffer(const void *buf, size_t len);

// Load an image from a buffer in lazy mode (no pixel decode). Pair with
// header accessors that do not force a read. Caller is responsible for
// keeping buf alive until the image is unref'd.
int sharpgo_load_buffer_lazy(const void *buf, size_t len, VipsImage **out);

// Load an image, requesting `pages` frames (animated GIF/WebP/HEIF/TIFF).
// pages: 1 = first frame (default), -1 = all frames, >0 = specific count.
// page: starting page index (0-based).
int sharpgo_load_buffer_pages(
    const void *buf, size_t len,
    int pages, int page,
    VipsImage **out);

// Compute per-band statistics. The resulting image is a (bands+1) x 6 matrix
// where row 0 is the combined stats and rows 1..bands are per-band.
// Columns: min, max, sum, sum_sq, mean, deviation.
int sharpgo_stats(VipsImage *in, VipsImage **out);

// Read a double value from a 1-band DOUBLE image at (x, y).
double sharpgo_matrix_get(VipsImage *m, int x, int y);

// Header accessors. Each returns 0 on success, -1 if the metadata is absent.
int sharpgo_get_int(VipsImage *in, const char *name, int *out);
int sharpgo_get_double(VipsImage *in, const char *name, double *out);
int sharpgo_get_string(VipsImage *in, const char *name, const char **out);

// Returns 1 if the image has an alpha channel, 0 otherwise.
int sharpgo_has_alpha(VipsImage *in);

// Returns the libvips interpretation enum value.
int sharpgo_get_interpretation(VipsImage *in);

// Returns the libvips band format enum value (uchar/ushort/...).
int sharpgo_get_band_format(VipsImage *in);

// Returns horizontal/vertical resolution in pixels per millimetre.
double sharpgo_get_xres(VipsImage *in);
double sharpgo_get_yres(VipsImage *in);

// Read a blob metadata field (EXIF, ICC, XMP, IPTC bytes). Returns 0 on
// success and writes the blob pointer + length into *data / *len. The blob
// is owned by libvips — do not free; copy if you need to retain it past the
// image's lifetime.
int sharpgo_get_blob(VipsImage *in, const char *name,
    const void **data, size_t *len);

// Gaussian blur with the given sigma. Sigma <= 0.3 is treated as "off".
int sharpgo_gaussblur(VipsImage *in, VipsImage **out, double sigma);

// Sharpen via unsharp mask. Sigma controls the blur radius; m1/m2/x1/y2/y3
// shape the response curve. Pass 0 for any field to use libvips defaults.
int sharpgo_sharpen(
    VipsImage *in, VipsImage **out,
    double sigma, double m1, double m2,
    double x1, double y2, double y3);

// Gamma correction. exponent is the input gamma; if exponent_out > 0 it sets
// the output gamma separately (sharp's two-arg gamma).
int sharpgo_gamma(VipsImage *in, VipsImage **out, double exponent, double exponent_out);

// Negate pixel values. If keep_alpha, the alpha channel (if present) is left
// untouched.
int sharpgo_negate(VipsImage *in, VipsImage **out, int keep_alpha);

// Threshold: pixel > value -> 255, else 0. If grayscale, convert to b-w first.
int sharpgo_threshold(VipsImage *in, VipsImage **out, double value, int grayscale);

// Linear transform: out = in * a + b. Pass per-channel arrays; nA/nB are the
// element counts (must equal 1 or the image band count).
int sharpgo_linear(
    VipsImage *in, VipsImage **out,
    const double *a, int n_a,
    const double *b, int n_b);

// Median filter with a square window of width pixels.
int sharpgo_median(VipsImage *in, VipsImage **out, int size);

// Tint the image (preserve luminance, override chroma) toward the given
// colour. Uses Lab-space transform.
int sharpgo_tint(VipsImage *in, VipsImage **out,
    double r, double g, double b);

// Convert to a single-band b/w image.
int sharpgo_greyscale(VipsImage *in, VipsImage **out);

// Convert colourspace by name (libvips VipsInterpretation enum).
int sharpgo_colourspace(VipsImage *in, VipsImage **out, int interpretation);

// Remove the alpha channel if present.
int sharpgo_remove_alpha(VipsImage *in, VipsImage **out);

// Ensure an alpha channel exists, adding one with the given value if not.
int sharpgo_ensure_alpha(VipsImage *in, VipsImage **out, double alpha);

// Convolve with a (kernel_w x kernel_h) kernel of doubles. Scale/offset are
// applied per libvips conv() conventions; pass scale=0 to auto-derive.
int sharpgo_convolve(VipsImage *in, VipsImage **out,
    const double *kernel, int kernel_w, int kernel_h,
    double scale, double offset);

// Boolean op (AND/OR/EOR/LSHIFT/RSHIFT) against a single-value constant.
// operation: VipsOperationBoolean enum.
int sharpgo_boolean_const(VipsImage *in, VipsImage **out,
    int operation, double constant);

// Recomb: per-band linear recombination via an N x N matrix (N = band count).
int sharpgo_recomb(VipsImage *in, VipsImage **out,
    const double *matrix, int n);

// Morphological dilate/erode with a 3x3 square mask iterated `size` times.
// mode: 0=dilate, 1=erode.
int sharpgo_morph(VipsImage *in, VipsImage **out, int size, int mode);

// Flatten alpha onto a background colour.
int sharpgo_flatten(VipsImage *in, VipsImage **out,
    double bg_r, double bg_g, double bg_b);

// Histogram equalisation (CLAHE) over local tiles of size width x height,
// optionally clipped to a maximum slope.
int sharpgo_clahe(VipsImage *in, VipsImage **out,
    int width, int height, int max_slope);

// Normalise: stretch the dynamic range so the lowest band-min reaches 0 and
// the highest band-max reaches 255 (matches sharp's normalise() semantics).
int sharpgo_normalise(VipsImage *in, VipsImage **out, int lower_pct, int upper_pct);

// Modulate: scale brightness/saturation and rotate hue in LCh space. Add
// `lightness_add` (in L*) after the multiplicative steps.
int sharpgo_modulate(VipsImage *in, VipsImage **out,
    double brightness, double saturation, double hue_deg, double lightness_add);

// Find the bounding box of non-background pixels (auto-trim).
// Returns 0 on success; *left/*top/*width/*height set.
int sharpgo_find_trim(VipsImage *in,
    double threshold, int line_art,
    int *left, int *top, int *width, int *height);

// Affine transform with a 2x2 matrix. Pads with bg colour.
int sharpgo_affine(VipsImage *in, VipsImage **out,
    double a, double b, double c, double d,
    double bg_r, double bg_g, double bg_b, double bg_a);

// Replicate (tile) `in` to fill width x height.
int sharpgo_replicate(VipsImage *in, VipsImage **out, int width, int height);

// Composite overlay onto base at (x, y) with the given blend mode
// (VipsBlendMode enum). Both images must have alpha; the caller is
// responsible for EnsureAlpha if needed.
int sharpgo_composite2(VipsImage *base, VipsImage *overlay, VipsImage **out,
    int blend_mode, int x, int y, int premultiplied);

// Extract a single band by index (0-based).
int sharpgo_extract_band(VipsImage *in, VipsImage **out, int band);

// Join `n` images band-wise (all must share width/height).
int sharpgo_bandjoin(VipsImage **inputs, int n, VipsImage **out);

// Reduce all bands to a single channel via the bitwise op
// (VipsOperationBoolean: AND/OR/EOR).
int sharpgo_bandbool(VipsImage *in, VipsImage **out, int op);

// Encode an image to a TIFF in-memory buffer.
int sharpgo_tiffsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int compression,                  // VipsForeignTiffCompression enum
    int quality,                      // JPEG-in-TIFF Q
    int predictor,                    // VipsForeignTiffPredictor enum
    int tile,                         // 0/1
    int tile_width,
    int tile_height,
    int pyramid,                      // 0/1
    int bitdepth,                     // 0=auto
    int bigtiff);                     // 0/1

// Encode an image to a HEIF/AVIF in-memory buffer.
// compression: VipsForeignHeifCompression (HEVC/AVC/JPEG/AV1).
int sharpgo_heifsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int compression,
    int quality,
    int lossless,
    int effort,
    int bitdepth,
    int chroma_subsampling_444);      // 0=auto, 1=off (4:4:4)

// Load raw pixel data from an in-memory buffer with explicit dimensions and
// band format (VipsBandFormat enum).
int sharpgo_load_raw_buffer(
    const void *buf, size_t len,
    int width, int height, int bands, int format,
    VipsImage **out);

// Cast the image to band_format and write its pixels to an in-memory buffer.
int sharpgo_rawsave_buffer(
    VipsImage *in,
    int band_format,
    void **buf, size_t *len);

// Encode an image to a JXL in-memory buffer.
int sharpgo_jxlsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,
    int tier,
    double distance,
    int effort,
    int lossless,
    int bitdepth);

// Encode an image to a JPEG 2000 in-memory buffer.
int sharpgo_jp2ksave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,
    int lossless,
    int tile_width,
    int tile_height,
    int chroma_subsampling_444);

// Save as a pyramidal tile set (DeepZoom/Zoomify/Google/IIIF). filename is
// the destination prefix; the actual files / .dzi/.zip are derived from it.
// Build a solid-colour image of the given dimensions. bands must be 3 or 4.
// If alpha (4 bands) is requested, bg_a sets the alpha value.
int sharpgo_create_solid(VipsImage **out,
    int width, int height, int bands,
    double bg_r, double bg_g, double bg_b, double bg_a);

// Render text via libvips pango. Returns an RGBA image (4 bands).
// fontfile, font may be NULL/"" to use system defaults.
int sharpgo_create_text(VipsImage **out,
    const char *text,
    const char *font,
    const char *fontfile,
    int width, int height,
    int dpi, int spacing,
    int rgba);

// Join an array of images into a grid (across columns wide). All inputs must
// share band count.
int sharpgo_arrayjoin(VipsImage **inputs, int n, VipsImage **out,
    int across, int hspacing, int vspacing,
    double bg_r, double bg_g, double bg_b, double bg_a);

// Install the Go-side log handler as GLib's default handler. After this
// runs, all g_log()/g_message()/g_warning()/g_critical()/g_error() calls
// (including those from libvips internals) are routed to sharpgoLogTrampoline.
void sharpgo_install_log_handler(void);

// Restore the GLib default log handler. Calls revert to GLib's built-in
// stderr output.
void sharpgo_uninstall_log_handler(void);

// Set integer metadata on the image. orientation/n-pages/page-height etc.
void sharpgo_set_int(VipsImage *in, const char *name, int value);

// Set string metadata on the image (XMP, ICC name, etc.).
void sharpgo_set_string(VipsImage *in, const char *name, const char *value);

// Set blob metadata (XMP packet, EXIF blob, ICC profile bytes).
void sharpgo_set_blob(VipsImage *in, const char *name, const void *data, size_t len);

// Override the resolution metadata. xres/yres in pixels-per-millimetre.
void sharpgo_set_resolution(VipsImage *in, double xres, double yres);

// Apply an ICC profile transform: convert pixels from the embedded profile
// (or from "input_profile" if no embedded one) into the named output profile.
// output_profile may be a built-in name ("srgb"|"p3"|"cmyk") or a file path.
int sharpgo_icc_transform(VipsImage *in, VipsImage **out,
    const char *output_profile, const char *input_profile);

// Embed an ICC profile by attaching its bytes as image metadata; does not
// convert pixels.
void sharpgo_set_icc_profile_blob(VipsImage *in, const void *data, size_t len);

// Set the kill flag on an image. Any libvips operation running on it (or its
// downstream pipeline) will abort at the next checkpoint. Idempotent.
void sharpgo_image_kill(VipsImage *in);

// Bump the GObject refcount on the image. Used by PreparedOverlay to share
// a decoded watermark across many composite calls without redecoding.
void sharpgo_image_ref(VipsImage *in);

// Block/unblock an operation by class name (e.g. "VipsForeignLoadHeif").
void sharpgo_block_operation(const char *name, int blocked);

// Create a VipsTargetCustom whose "write" signal calls back into Go via
// sharpgoTargetWriteTrampoline, passing the given id.
VipsTargetCustom *sharpgo_target_new(long long id);

// Drop the target reference.
void sharpgo_target_unref(VipsTargetCustom *target);

// Create a VipsSource subclass backed by a Go io.Reader. The reader is
// referenced through a runtime/cgo.Handle (passed as `handle`) and released
// in the GObject dispose handler — so the source (and its Go reader) lives
// exactly as long as any VipsImage that references it, with no global table.
// The returned source has refcount 1 (the caller's creation reference).
VipsSource *sharpgo_source_new(uintptr_t handle);

// Drop the caller's creation reference. After a load/thumbnail call the
// resulting image holds its own reference, so the source survives until the
// image (and its lazy pipeline) is freed.
void sharpgo_source_unref(VipsSource *src);

// Load any supported image from a streaming source with auto format
// detection. The decoded pixel data is materialised into libvips-managed
// memory before return; the Go reader may be released afterwards.
int sharpgo_load_source(VipsSource *src, VipsImage **out);

// Fused load + thumbnail directly from a VipsSource. Activates shrink-on-load
// like sharpgo_thumbnail_buffer but consumes bytes from the streaming source.
// The result is a lazy pipeline that reads from src on demand, so src must
// outlive it — guaranteed because the image holds a reference to src.
int sharpgo_thumbnail_source(
    VipsSource *src,
    int width, int height,
    int size, int crop, int no_rotate,
    const char *import_profile,
    const char *export_profile,
    int intent,
    VipsImage **out);

// Per-format save-to-target wrappers. Each mirrors its save_buffer twin but
// streams bytes through the supplied VipsTargetCustom instead of accumulating
// them in memory.
int sharpgo_jpegsave_target(
    VipsImage *in, VipsTargetCustom *target,
    int quality, int progressive, int optimise_coding,
    int trellis_quantisation, int overshoot_deringing, int optimise_scans,
    int quantisation_table, int chroma_subsampling_444);

int sharpgo_pngsave_target(
    VipsImage *in, VipsTargetCustom *target,
    int compression, int progressive, int palette,
    int quality, int effort, int bitdepth);

// Strip metadata fields from `in` that are not retained by the given keep
// flags (VipsForeignKeep bitset). Modifies the image in place.
//
// keep_flags: bitset of VIPS_FOREIGN_KEEP_* values. 0 = strip everything.
void sharpgo_apply_keep(VipsImage *in, int keep_flags);

int sharpgo_dzsave(
    VipsImage *in,
    const char *filename,
    int layout,                       // VipsForeignDzLayout
    const char *suffix,
    int overlap,
    int tile_size,
    int depth,                        // VipsForeignDzDepth
    int container,                    // VipsForeignDzContainer
    int compression,                  // 0-9 (zip mode)
    int quality);                     // tile JPEG/WebP quality

#endif
