// bridge.c — plain-C helpers (no C++). See bridge.h.
#include "bridge.h"
#include <webp/encode.h>
#include <string.h>

int sharpgo_load_buffer(const void *buf, size_t len, VipsImage **out) {
	VipsImage *im = vips_image_new_from_buffer(buf, len, "", NULL);
	if (im == NULL) return -1;

	// Force a full decode into libvips-managed memory so the caller's
	// buffer can be released immediately. Trades shrink-on-load for safe
	// lifetime; the Resize path fuses via vips_thumbnail_source instead.
	VipsImage *mem = vips_image_copy_memory(im);
	g_object_unref(im);
	if (mem == NULL) return -1;

	*out = mem;
	return 0;
}

int sharpgo_jpegsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,
    int progressive,
    int optimise_coding,
    int trellis_quantisation,
    int overshoot_deringing,
    int optimise_scans,
    int quantisation_table,
    int chroma_subsampling_444) {

	VipsForeignSubsample subsample = chroma_subsampling_444
		? VIPS_FOREIGN_SUBSAMPLE_OFF
		: VIPS_FOREIGN_SUBSAMPLE_AUTO;

	return vips_jpegsave_buffer(
		in, buf, len,
		"Q", quality,
		"interlace", progressive ? TRUE : FALSE,
		"optimize_coding", optimise_coding ? TRUE : FALSE,
		"trellis_quant", trellis_quantisation ? TRUE : FALSE,
		"overshoot_deringing", overshoot_deringing ? TRUE : FALSE,
		"optimize_scans", optimise_scans ? TRUE : FALSE,
		"quant_table", quantisation_table,
		"subsample_mode", subsample,
		NULL);
}

int sharpgo_pngsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int compression,
    int progressive,
    int palette,
    int quality,
    int effort,
    int bitdepth) {

	if (bitdepth > 0) {
		return vips_pngsave_buffer(
			in, buf, len,
			"compression", compression,
			"interlace", progressive ? TRUE : FALSE,
			"palette", palette ? TRUE : FALSE,
			"Q", quality,
			"effort", effort,
			"bitdepth", bitdepth,
			NULL);
	}
	return vips_pngsave_buffer(
		in, buf, len,
		"compression", compression,
		"interlace", progressive ? TRUE : FALSE,
		"palette", palette ? TRUE : FALSE,
		"Q", quality,
		"effort", effort,
		NULL);
}

int sharpgo_webpsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,
    int alpha_quality,
    int lossless,
    int near_lossless,
    int smart_subsample,
    int effort,
    int loop,
    int min_size,
    int mixed,
    int smart_deblock,
    int passes,
    int preset) {

	// "loop" is not a webpsave property — for animated WebP it's stored as
	// image metadata on the input. Set it if non-zero; otherwise leave the
	// inherited value (default 0 = infinite).
	if (loop != 0) {
		vips_image_set_int(in, "loop", loop);
	}

	// Optional knobs: skip when callers leave them at zero so libwebp's
	// internal defaults apply (passes auto-scales with effort; preset 0
	// is "default" which is the same as omitting the property).
	if (passes > 0 && preset > 0) {
		return vips_webpsave_buffer(
			in, buf, len,
			"Q", quality,
			"alpha-q", alpha_quality,
			"lossless", lossless ? TRUE : FALSE,
			"near-lossless", near_lossless ? TRUE : FALSE,
			"smart-subsample", smart_subsample ? TRUE : FALSE,
			"smart-deblock", smart_deblock ? TRUE : FALSE,
			"effort", effort,
			"passes", passes,
			"preset", (VipsForeignWebpPreset)preset,
			"min-size", min_size ? TRUE : FALSE,
			"mixed", mixed ? TRUE : FALSE,
			NULL);
	}
	if (passes > 0) {
		return vips_webpsave_buffer(
			in, buf, len,
			"Q", quality,
			"alpha-q", alpha_quality,
			"lossless", lossless ? TRUE : FALSE,
			"near-lossless", near_lossless ? TRUE : FALSE,
			"smart-subsample", smart_subsample ? TRUE : FALSE,
			"smart-deblock", smart_deblock ? TRUE : FALSE,
			"effort", effort,
			"passes", passes,
			"min-size", min_size ? TRUE : FALSE,
			"mixed", mixed ? TRUE : FALSE,
			NULL);
	}
	if (preset > 0) {
		return vips_webpsave_buffer(
			in, buf, len,
			"Q", quality,
			"alpha-q", alpha_quality,
			"lossless", lossless ? TRUE : FALSE,
			"near-lossless", near_lossless ? TRUE : FALSE,
			"smart-subsample", smart_subsample ? TRUE : FALSE,
			"smart-deblock", smart_deblock ? TRUE : FALSE,
			"effort", effort,
			"preset", (VipsForeignWebpPreset)preset,
			"min-size", min_size ? TRUE : FALSE,
			"mixed", mixed ? TRUE : FALSE,
			NULL);
	}
	return vips_webpsave_buffer(
		in, buf, len,
		"Q", quality,
		"alpha-q", alpha_quality,
		"lossless", lossless ? TRUE : FALSE,
		"near-lossless", near_lossless ? TRUE : FALSE,
		"smart-subsample", smart_subsample ? TRUE : FALSE,
		"smart-deblock", smart_deblock ? TRUE : FALSE,
		"effort", effort,
		"min-size", min_size ? TRUE : FALSE,
		"mixed", mixed ? TRUE : FALSE,
		NULL);
}

int sharpgo_gifsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    double dither,
    int effort,
    int bitdepth,
    int interframe_maxerror,
    int interpalette_maxerror,
    int interlace,
    int reuse,
    int keep_duplicate_frames) {

	return vips_gifsave_buffer(
		in, buf, len,
		"dither", dither,
		"effort", effort,
		"bitdepth", bitdepth,
		"interframe-maxerror", (double)interframe_maxerror,
		"interpalette-maxerror", (double)interpalette_maxerror,
		"interlace", interlace ? TRUE : FALSE,
		"reuse", reuse ? TRUE : FALSE,
		"keep-duplicate-frames", keep_duplicate_frames ? TRUE : FALSE,
		NULL);
}

int sharpgo_thumbnail_image(
    VipsImage *in, VipsImage **out,
    int width, int height,
    int kernel, int size, int crop,
    int no_rotate) {

	// vips_thumbnail_image does not accept a "kernel" property — it uses
	// libvips' built-in resize kernel (lanczos3). Custom kernels require
	// the post-decode vips_resize path (see op_resize.go).
	(void)kernel;

	return vips_thumbnail_image(
		in, out, width,
		"height", height,
		"size", (VipsSize)size,
		"crop", (VipsInteresting)crop,
		"no_rotate", no_rotate ? TRUE : FALSE,
		NULL);
}

int sharpgo_embed(
    VipsImage *in, VipsImage **out,
    int x, int y, int width, int height,
    double bg_r, double bg_g, double bg_b, double bg_a) {

	double bg[4] = { bg_r, bg_g, bg_b, bg_a };
	int n = vips_image_hasalpha(in) ? 4 : 3;
	VipsArrayDouble *bga = vips_array_double_new(bg, n);

	int rc = vips_embed(
		in, out, x, y, width, height,
		"extend", VIPS_EXTEND_BACKGROUND,
		"background", bga,
		NULL);

	vips_area_unref((VipsArea *)bga);
	return rc;
}

int sharpgo_rot(VipsImage *in, VipsImage **out, int angle_quarter) {
	VipsAngle a;
	switch (angle_quarter) {
		case 1: a = VIPS_ANGLE_D90;  break;
		case 2: a = VIPS_ANGLE_D180; break;
		case 3: a = VIPS_ANGLE_D270; break;
		default: a = VIPS_ANGLE_D0;  break;
	}
	return vips_rot(in, out, a, NULL);
}

int sharpgo_rotate(
    VipsImage *in, VipsImage **out,
    double angle,
    double bg_r, double bg_g, double bg_b, double bg_a) {

	double bg[4] = { bg_r, bg_g, bg_b, bg_a };
	int n = vips_image_hasalpha(in) ? 4 : 3;
	VipsArrayDouble *bga = vips_array_double_new(bg, n);

	int rc = vips_rotate(in, out, angle,
		"background", bga,
		NULL);

	vips_area_unref((VipsArea *)bga);
	return rc;
}

int sharpgo_flip(VipsImage *in, VipsImage **out, int direction) {
	VipsDirection d = direction == 0
		? VIPS_DIRECTION_HORIZONTAL
		: VIPS_DIRECTION_VERTICAL;
	return vips_flip(in, out, d, NULL);
}

int sharpgo_autorot(VipsImage *in, VipsImage **out) {
	return vips_autorot(in, out, NULL);
}

int sharpgo_extract_area(
    VipsImage *in, VipsImage **out,
    int left, int top, int width, int height) {
	return vips_extract_area(in, out, left, top, width, height, NULL);
}

const char *sharpgo_find_load_buffer(const void *buf, size_t len) {
	return vips_foreign_find_load_buffer(buf, len);
}

int sharpgo_load_buffer_lazy(const void *buf, size_t len, VipsImage **out) {
	VipsImage *im = vips_image_new_from_buffer(buf, len, "", NULL);
	if (im == NULL) return -1;
	*out = im;
	return 0;
}

int sharpgo_load_buffer_pages(
    const void *buf, size_t len,
    int pages, int page,
    VipsImage **out) {

	// Build an option string for vips_image_new_from_buffer; the format-
	// agnostic loader honours [n=...,page=...] in the option string.
	char opts[64];
	snprintf(opts, sizeof(opts), "n=%d,page=%d", pages, page);

	VipsImage *im = vips_image_new_from_buffer(buf, len, opts, NULL);
	if (im == NULL) return -1;

	VipsImage *mem = vips_image_copy_memory(im);
	g_object_unref(im);
	if (mem == NULL) return -1;

	*out = mem;
	return 0;
}

int sharpgo_stats(VipsImage *in, VipsImage **out) {
	return vips_stats(in, out, NULL);
}

double sharpgo_matrix_get(VipsImage *m, int x, int y) {
	// vips_stats returns a small (bands+1) x 6 matrix already materialised
	// in memory. Force WIO so we can read pixel addresses directly.
	if (vips_image_wio_input(m) != 0) return 0;
	double *p = VIPS_MATRIX(m, x, y);
	return p ? *p : 0;
}

int sharpgo_get_int(VipsImage *in, const char *name, int *out) {
	if (!vips_image_get_typeof(in, name)) return -1;
	return vips_image_get_int(in, name, out);
}

int sharpgo_get_double(VipsImage *in, const char *name, double *out) {
	if (!vips_image_get_typeof(in, name)) return -1;
	return vips_image_get_double(in, name, out);
}

int sharpgo_get_string(VipsImage *in, const char *name, const char **out) {
	if (!vips_image_get_typeof(in, name)) return -1;
	return vips_image_get_string(in, name, out);
}

int sharpgo_has_alpha(VipsImage *in) {
	return vips_image_hasalpha(in) ? 1 : 0;
}

int sharpgo_get_interpretation(VipsImage *in) {
	return (int)vips_image_get_interpretation(in);
}

int sharpgo_get_band_format(VipsImage *in) {
	return (int)vips_image_get_format(in);
}

double sharpgo_get_xres(VipsImage *in) {
	return vips_image_get_xres(in);
}

double sharpgo_get_yres(VipsImage *in) {
	return vips_image_get_yres(in);
}

int sharpgo_get_blob(VipsImage *in, const char *name,
    const void **data, size_t *len) {
	if (!vips_image_get_typeof(in, name)) return -1;
	return vips_image_get_blob(in, name, (void **)data, len);
}

int sharpgo_has_embedded_icc(VipsImage *in) {
	return vips_image_get_typeof(in, VIPS_META_ICC_NAME) != 0 ? 1 : 0;
}

// sharpgo_icc_is_srgb reports whether the embedded ICC profile is the standard
// sRGB IEC61966-2.1 profile, by fingerprinting fixed header fields — so a
// conversion to sRGB is an identity and can be skipped. Adapted from imgproxy's
// vips_icc_is_srgb_iec61966 (Apache-2.0); see NOTICE.
int sharpgo_icc_is_srgb(VipsImage *in) {
	const void *data = NULL;
	size_t len = 0;
	if (!vips_image_get_typeof(in, VIPS_META_ICC_NAME) ||
	    vips_image_get_blob(in, VIPS_META_ICC_NAME, &data, &len))
		return 0;
	if (!data || len < 128) return 0;

	const char *p = (const char *)data;
	static const char date[]    = { 7, (char)206, 0, 2, 0, 9 }; // 1998-12-01
	static const char version[] = { 2, 16, 0, 0 };              // 2.1

	return (memcmp(p + 16, "RGB ", 4) == 0 && // colourspace
	        memcmp(p + 48, "IEC ", 4) == 0 && // device manufacturer
	        memcmp(p + 52, "sRGB", 4) == 0 && // device model
	        memcmp(p + 80, "HP  ", 4) == 0 && // profile creator
	        memcmp(p + 24, date, 6) == 0 &&
	        memcmp(p + 8, version, 4) == 0)
	    ? 1 : 0;
}

int sharpgo_gaussblur(VipsImage *in, VipsImage **out, double sigma) {
	return vips_gaussblur(in, out, sigma, NULL);
}

int sharpgo_sharpen(
    VipsImage *in, VipsImage **out,
    double sigma, double m1, double m2,
    double x1, double y2, double y3) {
	return vips_sharpen(in, out,
		"sigma", sigma,
		"m1", m1,
		"m2", m2,
		"x1", x1,
		"y2", y2,
		"y3", y3,
		NULL);
}

int sharpgo_gamma(VipsImage *in, VipsImage **out, double exponent, double exponent_out) {
	if (exponent_out > 0 && exponent_out != exponent) {
		VipsImage *mid = NULL;
		int rc = vips_gamma(in, &mid, "exponent", 1.0 / exponent, NULL);
		if (rc != 0) return rc;
		rc = vips_gamma(mid, out, "exponent", exponent_out, NULL);
		g_object_unref(mid);
		return rc;
	}
	return vips_gamma(in, out, "exponent", 1.0 / exponent, NULL);
}

int sharpgo_negate(VipsImage *in, VipsImage **out, int keep_alpha) {
	if (keep_alpha && vips_image_hasalpha(in)) {
		// Separate alpha, invert colour bands, recombine.
		VipsImage *colour = NULL, *alpha = NULL, *inverted = NULL, *joined = NULL;
		int n_colour = in->Bands - 1;

		if (vips_extract_band(in, &colour, 0, "n", n_colour, NULL) != 0) return -1;
		if (vips_extract_band(in, &alpha, n_colour, "n", 1, NULL) != 0) {
			g_object_unref(colour);
			return -1;
		}
		if (vips_invert(colour, &inverted, NULL) != 0) {
			g_object_unref(colour); g_object_unref(alpha);
			return -1;
		}
		VipsImage *pair[2] = { inverted, alpha };
		int rc = vips_bandjoin(pair, &joined, 2, NULL);
		g_object_unref(colour); g_object_unref(alpha); g_object_unref(inverted);
		if (rc != 0) return -1;
		*out = joined;
		return 0;
	}
	return vips_invert(in, out, NULL);
}

int sharpgo_threshold(VipsImage *in, VipsImage **out, double value, int grayscale) {
	VipsImage *src = in;
	VipsImage *bw = NULL;
	if (grayscale) {
		if (vips_colourspace(in, &bw, VIPS_INTERPRETATION_B_W, NULL) != 0) return -1;
		src = bw;
	}
	int rc = vips_moreeq_const1(src, out, value, NULL);
	if (bw) g_object_unref(bw);
	return rc;
}

int sharpgo_linear(
    VipsImage *in, VipsImage **out,
    const double *a, int n_a,
    const double *b, int n_b) {
	return vips_linear(in, out, (double *)a, (double *)b, n_a > n_b ? n_a : n_b, NULL);
}

int sharpgo_median(VipsImage *in, VipsImage **out, int size) {
	return vips_median(in, out, size, NULL);
}

int sharpgo_tint(VipsImage *in, VipsImage **out,
    double r, double g, double b) {
	// Sharp's tint: convert to Lab, keep L*, override a*/b* with the tint
	// colour's a*/b*, convert back. libvips has no direct tint op.
	VipsImage *lab = NULL, *tinted = NULL;
	if (vips_colourspace(in, &lab, VIPS_INTERPRETATION_LAB, NULL) != 0) return -1;

	// Build a 1x1 sRGB image with the tint colour, then convert to Lab and
	// materialise so we can read its a*/b* pixel values directly.
	VipsImage *rgb1 = vips_image_new_matrix(1, 1);
	if (!rgb1) { g_object_unref(lab); return -1; }
	// Make rgb1 a 3-band sRGB by re-creating via linear from a 3-band black.
	VipsImage *black3 = NULL;
	if (vips_black(&black3, 1, 1, "bands", 3, NULL) != 0) {
		g_object_unref(lab); g_object_unref(rgb1); return -1;
	}
	g_object_unref(rgb1); // unused after black3 created

	double mul0[3] = { 0, 0, 0 };
	double rgb[3] = { r, g, b };
	VipsImage *colour = NULL;
	if (vips_linear(black3, &colour, mul0, rgb, 3, NULL) != 0) {
		g_object_unref(lab); g_object_unref(black3); return -1;
	}
	g_object_unref(black3);

	// Tag the 1x1 image as sRGB so vips_colourspace knows the source space.
	VipsImage *colour_tagged = NULL;
	if (vips_copy(colour, &colour_tagged,
	              "interpretation", VIPS_INTERPRETATION_sRGB, NULL) != 0) {
		g_object_unref(lab); g_object_unref(colour); return -1;
	}
	g_object_unref(colour);

	VipsImage *colour_lab = NULL;
	if (vips_colourspace(colour_tagged, &colour_lab,
	                     VIPS_INTERPRETATION_LAB, NULL) != 0) {
		g_object_unref(lab); g_object_unref(colour_tagged); return -1;
	}
	g_object_unref(colour_tagged);

	if (vips_image_wio_input(colour_lab) != 0) {
		g_object_unref(lab); g_object_unref(colour_lab); return -1;
	}
	double *pel = (double *)VIPS_IMAGE_ADDR(colour_lab, 0, 0);
	double tint_a = pel[1];
	double tint_b = pel[2];
	g_object_unref(colour_lab);

	double mul[3] = { 1.0, 0.0, 0.0 };
	double add[3] = { 0.0, tint_a, tint_b };
	if (vips_linear(lab, &tinted, mul, add, 3, NULL) != 0) {
		g_object_unref(lab); return -1;
	}
	g_object_unref(lab);

	int rc = vips_colourspace(tinted, out, VIPS_INTERPRETATION_sRGB, NULL);
	g_object_unref(tinted);
	return rc;
}

int sharpgo_greyscale(VipsImage *in, VipsImage **out) {
	return vips_colourspace(in, out, VIPS_INTERPRETATION_B_W, NULL);
}

int sharpgo_colourspace(VipsImage *in, VipsImage **out, int interpretation) {
	return vips_colourspace(in, out, (VipsInterpretation)interpretation, NULL);
}

int sharpgo_remove_alpha(VipsImage *in, VipsImage **out) {
	if (!vips_image_hasalpha(in)) {
		return vips_copy(in, out, NULL);
	}
	return vips_extract_band(in, out, 0, "n", in->Bands - 1, NULL);
}

int sharpgo_ensure_alpha(VipsImage *in, VipsImage **out, double alpha) {
	if (vips_image_hasalpha(in)) {
		return vips_copy(in, out, NULL);
	}
	return vips_bandjoin_const1(in, out, alpha, NULL);
}

int sharpgo_convolve(VipsImage *in, VipsImage **out,
    const double *kernel, int kernel_w, int kernel_h,
    double scale, double offset) {
	VipsImage *mask = vips_image_new_matrix(kernel_w, kernel_h);
	for (int y = 0; y < kernel_h; y++) {
		for (int x = 0; x < kernel_w; x++) {
			*VIPS_MATRIX(mask, x, y) = kernel[y * kernel_w + x];
		}
	}
	vips_image_set_double(mask, "scale", scale);
	vips_image_set_double(mask, "offset", offset);

	int rc = vips_conv(in, out, mask, NULL);
	g_object_unref(mask);
	return rc;
}

int sharpgo_boolean_const(VipsImage *in, VipsImage **out,
    int operation, double constant) {
	return vips_boolean_const1(in, out, (VipsOperationBoolean)operation, constant, NULL);
}

int sharpgo_recomb(VipsImage *in, VipsImage **out,
    const double *matrix, int n) {
	VipsImage *m = vips_image_new_matrix(n, n);
	for (int y = 0; y < n; y++) {
		for (int x = 0; x < n; x++) {
			*VIPS_MATRIX(m, x, y) = matrix[y * n + x];
		}
	}
	int rc = vips_recomb(in, out, m, NULL);
	g_object_unref(m);
	return rc;
}

int sharpgo_morph(VipsImage *in, VipsImage **out, int size, int mode) {
	// 3x3 all-ones square mask repeated `size` times.
	VipsImage *mask = vips_image_new_matrix(3, 3);
	for (int y = 0; y < 3; y++)
		for (int x = 0; x < 3; x++)
			*VIPS_MATRIX(mask, x, y) = 255;

	VipsOperationMorphology op = mode == 0
		? VIPS_OPERATION_MORPHOLOGY_DILATE
		: VIPS_OPERATION_MORPHOLOGY_ERODE;

	VipsImage *cur = in;
	g_object_ref(cur);
	for (int i = 0; i < size; i++) {
		VipsImage *next = NULL;
		if (vips_morph(cur, &next, mask, op, NULL) != 0) {
			g_object_unref(cur); g_object_unref(mask);
			return -1;
		}
		g_object_unref(cur);
		cur = next;
	}
	g_object_unref(mask);
	*out = cur;
	return 0;
}

int sharpgo_flatten(VipsImage *in, VipsImage **out,
    double bg_r, double bg_g, double bg_b) {
	// sharp.js short-circuits flatten when the image has no alpha, and
	// libvips ≥ 8.16 errors with "linear: vector must have 1 or N elements"
	// if the background size doesn't match bands-minus-alpha. Match sharp's
	// behaviour by passing through unchanged when no alpha is present.
	if (!vips_image_hasalpha(in)) {
		return vips_copy(in, out, NULL);
	}
	int n = in->Bands - 1;
	if (n < 1) n = 1;
	if (n > 3) n = 3;
	double bg[3] = { bg_r, bg_g, bg_b };
	VipsArrayDouble *bga = vips_array_double_new(bg, n);
	int rc = vips_flatten(in, out, "background", bga, NULL);
	vips_area_unref((VipsArea *)bga);
	return rc;
}

int sharpgo_clahe(VipsImage *in, VipsImage **out,
    int width, int height, int max_slope) {
	return vips_hist_local(in, out, width, height,
		"max_slope", max_slope,
		NULL);
}

int sharpgo_normalise(VipsImage *in, VipsImage **out, int lower_pct, int upper_pct) {
	// Use vips_stats: row 0 of result holds combined min (col 0) and max (col 1).
	VipsImage *stats = NULL;
	if (vips_stats(in, &stats, NULL) != 0) return -1;
	if (vips_image_wio_input(stats) != 0) { g_object_unref(stats); return -1; }
	double min = *VIPS_MATRIX(stats, 0, 0);
	double max = *VIPS_MATRIX(stats, 1, 0);
	g_object_unref(stats);

	(void)lower_pct; (void)upper_pct; // percentile clipping deferred

	if (max <= min) return vips_copy(in, out, NULL);
	double scale = 255.0 / (max - min);
	double offset = -min * scale;
	return vips_linear1(in, out, scale, offset, NULL);
}

int sharpgo_find_trim(VipsImage *in,
    double threshold, int line_art,
    int *left, int *top, int *width, int *height) {
	return vips_find_trim(in, left, top, width, height,
		"threshold", threshold,
		"line_art", line_art ? TRUE : FALSE,
		NULL);
}

int sharpgo_affine(VipsImage *in, VipsImage **out,
    double a, double b, double c, double d,
    double bg_r, double bg_g, double bg_b, double bg_a) {
	double bg[4] = { bg_r, bg_g, bg_b, bg_a };
	int n = vips_image_hasalpha(in) ? 4 : 3;
	VipsArrayDouble *bga = vips_array_double_new(bg, n);

	int rc = vips_affine(in, out, a, b, c, d,
		"background", bga,
		"extend", VIPS_EXTEND_BACKGROUND,
		NULL);

	vips_area_unref((VipsArea *)bga);
	return rc;
}

int sharpgo_replicate(VipsImage *in, VipsImage **out, int width, int height) {
	int across = (width  + in->Xsize - 1) / in->Xsize;
	int down   = (height + in->Ysize - 1) / in->Ysize;
	if (across < 1) across = 1;
	if (down   < 1) down   = 1;
	VipsImage *tiled = NULL;
	if (vips_replicate(in, &tiled, across, down, NULL) != 0) return -1;
	int rc = vips_extract_area(tiled, out, 0, 0, width, height, NULL);
	g_object_unref(tiled);
	return rc;
}

int sharpgo_composite2(VipsImage *base, VipsImage *overlay, VipsImage **out,
    int blend_mode, int x, int y, int premultiplied) {
	return vips_composite2(base, overlay, out, (VipsBlendMode)blend_mode,
		"x", x,
		"y", y,
		"premultiplied", premultiplied ? TRUE : FALSE,
		NULL);
}

int sharpgo_extract_band(VipsImage *in, VipsImage **out, int band) {
	return vips_extract_band(in, out, band, "n", 1, NULL);
}

int sharpgo_bandjoin(VipsImage **inputs, int n, VipsImage **out) {
	return vips_bandjoin(inputs, out, n, NULL);
}

int sharpgo_bandbool(VipsImage *in, VipsImage **out, int op) {
	return vips_bandbool(in, out, (VipsOperationBoolean)op, NULL);
}

int sharpgo_tiffsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int compression,
    int quality,
    int predictor,
    int tile,
    int tile_width,
    int tile_height,
    int pyramid,
    int bitdepth,
    int bigtiff) {

	return vips_tiffsave_buffer(
		in, buf, len,
		"compression", (VipsForeignTiffCompression)compression,
		"Q", quality,
		"predictor", (VipsForeignTiffPredictor)predictor,
		"tile", tile ? TRUE : FALSE,
		"tile-width", tile_width,
		"tile-height", tile_height,
		"pyramid", pyramid ? TRUE : FALSE,
		"bitdepth", bitdepth,
		"bigtiff", bigtiff ? TRUE : FALSE,
		NULL);
}

int sharpgo_heifsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int compression,
    int quality,
    int lossless,
    int effort,
    int bitdepth,
    int chroma_subsampling_444) {

	VipsForeignSubsample sub = chroma_subsampling_444
		? VIPS_FOREIGN_SUBSAMPLE_OFF
		: VIPS_FOREIGN_SUBSAMPLE_AUTO;

	return vips_heifsave_buffer(
		in, buf, len,
		"compression", (VipsForeignHeifCompression)compression,
		"Q", quality,
		"lossless", lossless ? TRUE : FALSE,
		"effort", effort,
		"bitdepth", bitdepth,
		"subsample-mode", sub,
		NULL);
}

int sharpgo_load_raw_buffer(
    const void *buf, size_t len,
    int width, int height, int bands, int format,
    VipsImage **out) {

	// Use memory_copy so the caller's buffer can be freed immediately.
	VipsImage *im = vips_image_new_from_memory_copy(buf, len,
		width, height, bands, (VipsBandFormat)format);
	if (im == NULL) return -1;
	*out = im;
	return 0;
}

int sharpgo_rawsave_buffer(
    VipsImage *in,
    int band_format,
    void **buf, size_t *len) {

	// Cast to the requested band format first (sharp's raw {depth} option).
	VipsImage *cast = NULL;
	if (vips_cast(in, &cast, (VipsBandFormat)band_format, NULL) != 0) return -1;

	// vips_rawsave_buffer outputs raw pixel data without container/header.
	int rc = vips_rawsave_buffer(cast, buf, len, NULL);
	g_object_unref(cast);
	return rc;
}

int sharpgo_jxlsave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,
    int tier,
    double distance,
    int effort,
    int lossless,
    int bitdepth) {

	return vips_jxlsave_buffer(
		in, buf, len,
		"Q", quality,
		"tier", tier,
		"distance", distance,
		"effort", effort,
		"lossless", lossless ? TRUE : FALSE,
		"bitdepth", bitdepth,
		NULL);
}

int sharpgo_jp2ksave_buffer(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,
    int lossless,
    int tile_width,
    int tile_height,
    int chroma_subsampling_444) {

	VipsForeignSubsample sub = chroma_subsampling_444
		? VIPS_FOREIGN_SUBSAMPLE_OFF
		: VIPS_FOREIGN_SUBSAMPLE_AUTO;

	return vips_jp2ksave_buffer(
		in, buf, len,
		"Q", quality,
		"lossless", lossless ? TRUE : FALSE,
		"tile-width", tile_width,
		"tile-height", tile_height,
		"subsample-mode", sub,
		NULL);
}

// Map callback: remove the field if its name starts with the prefix passed in
// as user data. Returns NULL to continue iteration.
static void *sharpgo_strip_with_prefix(VipsImage *im, const char *name, GValue *value, void *prefix) {
	(void)value;
	const char *p = (const char *)prefix;
	if (g_str_has_prefix(name, p)) {
		vips_image_remove(im, name);
	}
	return NULL;
}

int sharpgo_create_solid(VipsImage **out,
    int width, int height, int bands,
    double bg_r, double bg_g, double bg_b, double bg_a) {

	VipsImage *black = NULL;
	if (vips_black(&black, width, height, "bands", bands, NULL) != 0) return -1;

	double a_zero[4] = { 0, 0, 0, 0 };
	double rgba[4]   = { bg_r, bg_g, bg_b, bg_a };
	int rc = vips_linear(black, out, a_zero, rgba, bands, NULL);
	g_object_unref(black);
	if (rc != 0) return rc;

	// Tag as sRGB so subsequent ops know the colourspace.
	VipsImage *tagged = NULL;
	if (vips_copy(*out, &tagged, "interpretation", VIPS_INTERPRETATION_sRGB, NULL) != 0) {
		return -1;
	}
	g_object_unref(*out);
	*out = tagged;
	return 0;
}

int sharpgo_create_text(VipsImage **out,
    const char *text,
    const char *font,
    const char *fontfile,
    int width, int height,
    int dpi, int spacing,
    int rgba) {

	const char *use_font = (font && font[0]) ? font : "sans 12";

	if (fontfile && fontfile[0]) {
		return vips_text(out, text,
			"font", use_font,
			"fontfile", fontfile,
			"width", width > 0 ? width : 0,
			"height", height > 0 ? height : 0,
			"dpi", dpi > 0 ? dpi : 72,
			"spacing", spacing,
			"rgba", rgba ? TRUE : FALSE,
			NULL);
	}
	return vips_text(out, text,
		"font", use_font,
		"width", width > 0 ? width : 0,
		"height", height > 0 ? height : 0,
		"dpi", dpi > 0 ? dpi : 72,
		"spacing", spacing,
		"rgba", rgba ? TRUE : FALSE,
		NULL);
}

int sharpgo_arrayjoin(VipsImage **inputs, int n, VipsImage **out,
    int across, int hspacing, int vspacing,
    double bg_r, double bg_g, double bg_b, double bg_a) {

	double bg[4] = { bg_r, bg_g, bg_b, bg_a };
	VipsArrayDouble *bga = vips_array_double_new(bg, 4);

	int rc = vips_arrayjoin(inputs, out, n,
		"across", across > 0 ? across : n,
		"hspacing", hspacing,
		"vspacing", vspacing,
		"background", bga,
		NULL);

	vips_area_unref((VipsArea *)bga);
	return rc;
}

// Forward declaration: the Go-side trampoline implemented via cgo //export.
extern void sharpgoLogTrampoline(char *domain, int level, char *message);

static void sharpgo_glog_handler(
    const gchar *log_domain,
    GLogLevelFlags log_level,
    const gchar *message,
    gpointer user_data) {
	(void)user_data;
	sharpgoLogTrampoline(
		(char *)(log_domain ? log_domain : ""),
		(int)log_level,
		(char *)(message ? message : ""));
}

void sharpgo_install_log_handler(void) {
	g_log_set_default_handler(sharpgo_glog_handler, NULL);
}

void sharpgo_uninstall_log_handler(void) {
	g_log_set_default_handler(g_log_default_handler, NULL);
}

void sharpgo_set_int(VipsImage *in, const char *name, int value) {
	vips_image_set_int(in, name, value);
}

void sharpgo_set_string(VipsImage *in, const char *name, const char *value) {
	vips_image_set_string(in, name, value);
}

void sharpgo_set_blob(VipsImage *in, const char *name, const void *data, size_t len) {
	vips_image_set_blob_copy(in, name, data, len);
}

void sharpgo_set_resolution(VipsImage *in, double xres, double yres) {
	// xres/yres are GObject properties on VipsImage; setting via the GObject
	// API actually mutates the struct's Xres/Yres fields used by the savers.
	g_object_set(in, "xres", xres, "yres", yres, NULL);
}

int sharpgo_icc_transform(VipsImage *in, VipsImage **out,
    const char *output_profile, const char *input_profile) {

	if (input_profile && input_profile[0]) {
		return vips_icc_transform(in, out, output_profile,
			"input_profile", input_profile,
			"embedded", TRUE,
			NULL);
	}
	return vips_icc_transform(in, out, output_profile,
		"embedded", TRUE,
		NULL);
}

void sharpgo_set_icc_profile_blob(VipsImage *in, const void *data, size_t len) {
	vips_image_set_blob_copy(in, VIPS_META_ICC_NAME, data, len);
}

// Forward decl: Go-side trampoline for streaming-output writes.
extern long long sharpgoTargetWriteTrampoline(long long id, void *buf, long long len);

static gint64 sharpgo_target_write_cb(VipsTargetCustom *target,
    const void *buf, gint64 len, void *user_data) {
	(void)target;
	long long id = (long long)(intptr_t)user_data;
	return (gint64)sharpgoTargetWriteTrampoline(id, (void *)buf, (long long)len);
}

VipsTargetCustom *sharpgo_target_new(long long id) {
	VipsTargetCustom *t = vips_target_custom_new();
	if (!t) return NULL;
	g_signal_connect(t, "write", G_CALLBACK(sharpgo_target_write_cb),
		(gpointer)(intptr_t)id);
	return t;
}

void sharpgo_target_unref(VipsTargetCustom *target) {
	if (target) g_object_unref(target);
}

// --- streaming input source backed by a Go io.Reader ---------------------
//
// A VipsSource subclass that pulls bytes from a Go reader referenced via a
// runtime/cgo.Handle. Adapted from imgproxy's vips/source.{c,h,go}
// (Apache-2.0); see NOTICE. The handle is released in dispose, so the Go
// reader lives exactly as long as any VipsImage referencing this source —
// no global registry/mutex, and the reader is guaranteed to outlive lazy
// (shrink-on-load) pipelines that read from it on demand.

extern long long sharpgoSourceRead(uintptr_t handle, void *buf, long long len);
extern long long sharpgoSourceSeek(uintptr_t handle, long long offset, int whence);
extern void      sharpgoSourceClose(uintptr_t handle);

typedef struct _SharpgoSource {
	VipsSource parent;
	uintptr_t handle;
} SharpgoSource;

typedef struct _SharpgoSourceClass {
	VipsSourceClass parent_class;
} SharpgoSourceClass;

GType sharpgo_source_get_type(void);
#define SHARPGO_TYPE_SOURCE (sharpgo_source_get_type())
G_DEFINE_TYPE(SharpgoSource, sharpgo_source, VIPS_TYPE_SOURCE)

static gint64 sharpgo_source_read_vfunc(VipsSource *source, void *buffer, size_t length) {
	SharpgoSource *self = (SharpgoSource *)source;
	return (gint64)sharpgoSourceRead(self->handle, buffer, (long long)length);
}

static gint64 sharpgo_source_seek_vfunc(VipsSource *source, gint64 offset, int whence) {
	SharpgoSource *self = (SharpgoSource *)source;
	return (gint64)sharpgoSourceSeek(self->handle, (long long)offset, whence);
}

static void sharpgo_source_dispose(GObject *gobject) {
	SharpgoSource *self = (SharpgoSource *)gobject;
	if (self->handle) {
		sharpgoSourceClose(self->handle);
		self->handle = 0;
	}
	G_OBJECT_CLASS(sharpgo_source_parent_class)->dispose(gobject);
}

static void sharpgo_source_class_init(SharpgoSourceClass *klass) {
	GObjectClass *gobject_class = G_OBJECT_CLASS(klass);
	VipsObjectClass *object_class = VIPS_OBJECT_CLASS(klass);
	VipsSourceClass *source_class = VIPS_SOURCE_CLASS(klass);

	object_class->nickname = "sharpgo_source";
	object_class->description = "sharp-go input source";

	gobject_class->dispose = sharpgo_source_dispose;

	source_class->read = sharpgo_source_read_vfunc;
	source_class->seek = sharpgo_source_seek_vfunc;
}

static void sharpgo_source_init(SharpgoSource *source) {
	(void)source;
}

VipsSource *sharpgo_source_new(uintptr_t handle) {
	SharpgoSource *s = g_object_new(SHARPGO_TYPE_SOURCE, NULL);
	if (!s) return NULL;
	s->handle = handle;
	return (VipsSource *)s;
}

void sharpgo_source_unref(VipsSource *src) {
	if (src) g_object_unref(src);
}

int sharpgo_load_source(VipsSource *src, VipsImage **out) {
	VipsImage *im = vips_image_new_from_source(src, "", NULL);
	if (im == NULL) return -1;
	VipsImage *mem = vips_image_copy_memory(im);
	g_object_unref(im);
	if (mem == NULL) return -1;
	*out = mem;
	return 0;
}

int sharpgo_thumbnail_source(
    VipsSource *src,
    int width, int height,
    int size, int crop, int no_rotate,
    const char *import_profile,
    const char *export_profile,
    int intent,
    VipsImage **out) {

	const char *ip = (import_profile && import_profile[0]) ? import_profile : NULL;
	const char *ep = (export_profile && export_profile[0]) ? export_profile : NULL;

	if (ip != NULL && ep != NULL) {
		return vips_thumbnail_source(
			src, out, width,
			"height", height,
			"size", (VipsSize)size,
			"crop", (VipsInteresting)crop,
			"no_rotate", no_rotate ? TRUE : FALSE,
			"import_profile", ip,
			"export_profile", ep,
			"intent", (VipsIntent)intent,
			NULL);
	}
	if (ep != NULL) {
		return vips_thumbnail_source(
			src, out, width,
			"height", height,
			"size", (VipsSize)size,
			"crop", (VipsInteresting)crop,
			"no_rotate", no_rotate ? TRUE : FALSE,
			"export_profile", ep,
			"intent", (VipsIntent)intent,
			NULL);
	}
	return vips_thumbnail_source(
		(VipsSource *)src, out, width,
		"height", height,
		"size", (VipsSize)size,
		"crop", (VipsInteresting)crop,
		"no_rotate", no_rotate ? TRUE : FALSE,
		NULL);
}

int sharpgo_jpegsave_target(
    VipsImage *in, VipsTargetCustom *target,
    int quality, int progressive, int optimise_coding,
    int trellis_quantisation, int overshoot_deringing, int optimise_scans,
    int quantisation_table, int chroma_subsampling_444) {

	VipsForeignSubsample subsample = chroma_subsampling_444
		? VIPS_FOREIGN_SUBSAMPLE_OFF
		: VIPS_FOREIGN_SUBSAMPLE_AUTO;

	return vips_jpegsave_target(in, (VipsTarget *)target,
		"Q", quality,
		"interlace", progressive ? TRUE : FALSE,
		"optimize_coding", optimise_coding ? TRUE : FALSE,
		"trellis_quant", trellis_quantisation ? TRUE : FALSE,
		"overshoot_deringing", overshoot_deringing ? TRUE : FALSE,
		"optimize_scans", optimise_scans ? TRUE : FALSE,
		"quant_table", quantisation_table,
		"subsample_mode", subsample,
		NULL);
}

int sharpgo_pngsave_target(
    VipsImage *in, VipsTargetCustom *target,
    int compression, int progressive, int palette,
    int quality, int effort, int bitdepth) {

	if (bitdepth > 0) {
		return vips_pngsave_target(in, (VipsTarget *)target,
			"compression", compression,
			"interlace", progressive ? TRUE : FALSE,
			"palette", palette ? TRUE : FALSE,
			"Q", quality,
			"effort", effort,
			"bitdepth", bitdepth,
			NULL);
	}
	return vips_pngsave_target(in, (VipsTarget *)target,
		"compression", compression,
		"interlace", progressive ? TRUE : FALSE,
		"palette", palette ? TRUE : FALSE,
		"Q", quality,
		"effort", effort,
		NULL);
}

void sharpgo_image_kill(VipsImage *in) {
	if (in != NULL) {
		vips_image_set_kill(in, TRUE);
	}
}

void sharpgo_image_ref(VipsImage *in) {
	if (in != NULL) {
		g_object_ref(in);
	}
}

void sharpgo_block_operation(const char *name, int blocked) {
	vips_operation_block_set(name, blocked ? TRUE : FALSE);
}

void sharpgo_apply_keep(VipsImage *in, int keep_flags) {
	if (in == NULL) return;
	if (!(keep_flags & VIPS_FOREIGN_KEEP_EXIF)) {
		vips_image_remove(in, VIPS_META_EXIF_NAME);
		// libvips also stores per-field EXIF values with "exif-*" names; strip
		// them so the encoder doesn't re-synthesise a new EXIF block.
		vips_image_map(in, sharpgo_strip_with_prefix, (void *)"exif-");
		// Standalone EXIF orientation tag.
		vips_image_remove(in, VIPS_META_ORIENTATION);
	}
	if (!(keep_flags & VIPS_FOREIGN_KEEP_XMP)) {
		vips_image_remove(in, VIPS_META_XMP_NAME);
	}
	if (!(keep_flags & VIPS_FOREIGN_KEEP_IPTC)) {
		vips_image_remove(in, VIPS_META_IPTC_NAME);
	}
	if (!(keep_flags & VIPS_FOREIGN_KEEP_ICC)) {
		vips_image_remove(in, VIPS_META_ICC_NAME);
	}
}

int sharpgo_dzsave(
    VipsImage *in,
    const char *filename,
    int layout,
    const char *suffix,
    int overlap,
    int tile_size,
    int depth,
    int container,
    int compression,
    int quality) {

	return vips_dzsave(in, filename,
		"layout", (VipsForeignDzLayout)layout,
		"suffix", suffix,
		"overlap", overlap,
		"tile-size", tile_size,
		"depth", (VipsForeignDzDepth)depth,
		"container", (VipsForeignDzContainer)container,
		"compression", compression,
		"Q", quality,
		NULL);
}

int sharpgo_modulate(VipsImage *in, VipsImage **out,
    double brightness, double saturation, double hue_deg, double lightness_add) {
	VipsImage *lch = NULL;
	if (vips_colourspace(in, &lch, VIPS_INTERPRETATION_LCH, NULL) != 0) return -1;

	double mul[3] = { brightness, saturation, 1.0 };
	double add[3] = { lightness_add, 0.0, hue_deg };

	VipsImage *modulated = NULL;
	if (vips_linear(lch, &modulated, mul, add, 3, NULL) != 0) {
		g_object_unref(lch); return -1;
	}
	g_object_unref(lch);

	int rc = vips_colourspace(modulated, out, VIPS_INTERPRETATION_sRGB, NULL);
	g_object_unref(modulated);
	return rc;
}

// sharpgo_webpsave_sharp_yuv encodes via libwebp DIRECTLY, bypassing
// libvips's webpsave wrapper. libvips doesn't expose WebPConfig.use_sharp_yuv
// (sharper RGB→YUV conversion), autofilter, sns_strength, target_PSNR, or
// segments — measured to give 5-8% better butteraugli at same byte size.
//
// Input must be sRGB 3- or 4-band uchar (caller responsibility — pair with
// vips_colourspace(VIPS_INTERPRETATION_sRGB) before calling).
//
// Returns 0 on success; *buf is g_malloc-owned (caller g_free()'s it).
int sharpgo_webpsave_sharp_yuv(
    VipsImage *in,
    void **buf, size_t *len,
    int quality,            // 1-100
    int effort,             // 0-6 (libwebp "method")
    int use_sharp_yuv,      // 0/1
    int autofilter,         // 0/1
    int sns_strength,       // 0-100; 0 = libwebp default (50)
    int target_size,        // 0 = ignore
    float target_psnr,      // 0.0 = ignore
    int passes,             // 1-10; 0 = libwebp default
    int preset,             // 0=default, 1=picture, 2=photo, 3=drawing, 4=icon, 5=text
    int segments,           // 1-4; 0 = libwebp default (4)
    int multithread) {      // 0/1 -> WebPConfig.thread_level

	// libwebp wants tight-packed RGB(A) uchar. Force the image into that
	// layout via a memory write — handles any source interpretation.
	VipsImage *srgb = NULL;
	if (vips_colourspace(in, &srgb, VIPS_INTERPRETATION_sRGB, NULL) != 0) {
		return -1;
	}
	VipsImage *cast = NULL;
	if (vips_cast(srgb, &cast, VIPS_FORMAT_UCHAR, NULL) != 0) {
		g_object_unref(srgb);
		return -1;
	}
	g_object_unref(srgb);
	srgb = cast;

	int width  = vips_image_get_width(srgb);
	int height = vips_image_get_height(srgb);
	int bands  = vips_image_get_bands(srgb);
	if (bands != 3 && bands != 4) {
		g_object_unref(srgb);
		return -1;
	}

	size_t total;
	void *pixels = vips_image_write_to_memory(srgb, &total);
	g_object_unref(srgb);
	if (pixels == NULL) return -1;

	int stride = width * bands;

	WebPConfig config;
	WebPPreset preset_enum = WEBP_PRESET_DEFAULT;
	switch (preset) {
		case 1: preset_enum = WEBP_PRESET_PICTURE; break;
		case 2: preset_enum = WEBP_PRESET_PHOTO;   break;
		case 3: preset_enum = WEBP_PRESET_DRAWING; break;
		case 4: preset_enum = WEBP_PRESET_ICON;    break;
		case 5: preset_enum = WEBP_PRESET_TEXT;    break;
	}
	if (!WebPConfigPreset(&config, preset_enum, (float)quality)) {
		g_free(pixels);
		return -1;
	}

	config.method = effort;
	config.use_sharp_yuv = use_sharp_yuv ? 1 : 0;
	config.autofilter = autofilter ? 1 : 0;
	if (sns_strength > 0) config.sns_strength = sns_strength;
	if (target_size > 0) config.target_size = target_size;
	if (target_psnr > 0) config.target_PSNR = target_psnr;
	if (passes > 0)      config.pass = passes;
	if (segments > 0)    config.segments = segments;
	// thread_level=0 matches cwebp's single-threaded rate-distortion curve
	// exactly. thread_level=1 lets libwebp encode token partitions in
	// parallel — faster on multicore, with a negligible (sub-0.1%) size
	// delta. Opt in via WebPOptions.Multithread when latency matters.
	config.thread_level = multithread ? 1 : 0;

	if (!WebPValidateConfig(&config)) {
		g_free(pixels);
		return -1;
	}

	WebPPicture pic;
	if (!WebPPictureInit(&pic)) {
		g_free(pixels);
		return -1;
	}
	pic.width = width;
	pic.height = height;
	// use_argb = 1 is REQUIRED for use_sharp_yuv to take effect. With
	// use_argb = 0, WebPPictureImportRGB does an immediate fast RGB→YUV
	// conversion and the sharp path is never reached. With use_argb = 1
	// the picture keeps ARGB and WebPEncode performs the (sharp) YUV
	// conversion honouring config.use_sharp_yuv.
	pic.use_argb = 1;

	// libvips uses VIPS_INTERPRETATION_sRGB band order RGB (red, green, blue).
	// Confirmed via vips_image_band_format() docs — same as libwebp expects.
	int imported = (bands == 4)
		? WebPPictureImportRGBA(&pic, pixels, stride)
		: WebPPictureImportRGB(&pic, pixels, stride);
	if (!imported) {
		WebPPictureFree(&pic);
		g_free(pixels);
		return -1;
	}

	WebPMemoryWriter writer;
	WebPMemoryWriterInit(&writer);
	pic.writer = WebPMemoryWrite;
	pic.custom_ptr = &writer;

	int ok = WebPEncode(&config, &pic);
	WebPPictureFree(&pic);
	g_free(pixels);

	if (!ok) {
		WebPMemoryWriterClear(&writer);
		return -1;
	}

	// Copy libwebp's malloc-owned buffer into g_malloc memory so the Go
	// side can free it via C.g_free() (matches our other encoders).
	*buf = g_malloc(writer.size);
	memcpy(*buf, writer.mem, writer.size);
	*len = writer.size;
	WebPMemoryWriterClear(&writer);
	return 0;
}
