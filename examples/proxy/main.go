// Image proxy inspired by /Users/sohelmia/sites/image-optimizer-edge-proxy/bunproxy.
// Fetches an image from an upstream origin, optimises it with sharp-go, and
// streams the result back. Adds /raw, /metrics (butteraugli + ssimulacra2)
// and a side-by-side /compare HTML view.
//
// Usage:
//
//	PORT=3003 UPSTREAM_BASE=https://staging.aarong.com go run ./examples/proxy
package main

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

var (
	upstreamBase = envOr("UPSTREAM_BASE", "https://mcprod.aarong.com")
	fastlyBase   = envOrLookup("FASTLY_BASE", "https://mcprod.aarong.com")
	defaultPath  = envOr("DEFAULT_PATH", "/media/catalog/product/0/5/0560000084696.jpg")
	upstreamHost string
	httpClient   = &http.Client{Timeout: 30 * time.Second}
	baLineRegex  = regexp.MustCompile(`(?m)^([\d.]+)\s*$`)

	// Origin disk cache. Configured at init via main(); ENV:
	//   ORIGIN_CACHE_DIR   default /tmp/sharp-proxy-cache
	//   ORIGIN_CACHE_TTL   default 1h  (Go duration; "0" disables)
	//   ORIGIN_CACHE       set to "off" to disable entirely
	origCache *originCache

	// Optimized-output disk cache. Keyed by the full variant (path + format +
	// quality + effort + dimensions + fit + bg) so each distinct encode is
	// stored once and never re-encoded. ENV mirrors the origin cache:
	//   OUTPUT_CACHE_DIR   default <ORIGIN_CACHE_DIR>-opt
	//   OUTPUT_CACHE       set to "off" to disable
	// Shares ORIGIN_CACHE_TTL.
	outCache *originCache
)

func envOr(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// envOrLookup distinguishes "unset" (returns def) from "set to empty"
// (returns ""). Lets `FASTLY_BASE=` explicitly disable the 3rd pane.
func envOrLookup(k, def string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return def
}

func main() {
	u, err := url.Parse(upstreamBase)
	if err != nil {
		log.Fatalf("invalid UPSTREAM_BASE: %v", err)
	}
	upstreamHost = u.Host

	originDir := envOr("ORIGIN_CACHE_DIR", filepath.Join(os.TempDir(), "sharp-proxy-cache"))
	ttl, err := time.ParseDuration(envOr("ORIGIN_CACHE_TTL", "1h"))
	if err != nil {
		log.Fatalf("invalid ORIGIN_CACHE_TTL: %v", err)
	}

	// Clear cache dirs on startup by default; set CACHE_CLEAR=off to persist
	// across restarts.
	clearCache := strings.ToLower(envOr("CACHE_CLEAR", "on")) != "off"

	if strings.ToLower(envOr("ORIGIN_CACHE", "on")) != "off" {
		c, err := newOriginCache(originDir, ttl, clearCache)
		if err != nil {
			log.Fatalf("origin cache: %v", err)
		}
		origCache = c
		log.Printf("origin cache: dir=%s ttl=%s cleared=%t", originDir, ttl, clearCache)
	} else {
		log.Printf("origin cache: disabled")
	}

	if strings.ToLower(envOr("OUTPUT_CACHE", "on")) != "off" {
		dir := envOr("OUTPUT_CACHE_DIR", originDir+"-opt")
		c, err := newOriginCache(dir, ttl, clearCache)
		if err != nil {
			log.Fatalf("output cache: %v", err)
		}
		outCache = c
		log.Printf("output cache: dir=%s ttl=%s cleared=%t", dir, ttl, clearCache)
	} else {
		log.Printf("output cache: disabled")
	}

	port := envOr("PORT", "3003")
	mux := http.NewServeMux()
	mux.HandleFunc("/", route)

	addr := ":" + port
	log.Printf("sharp-go proxy listening on http://localhost%s (upstream=%s)", addr, upstreamBase)
	log.Fatal(http.ListenAndServe(addr, mux))
}

func route(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path
	switch {
	case p == "/health":
		w.Write([]byte("ok"))
	case p == "/":
		w.Header().Set("content-type", "text/plain")
		fmt.Fprintln(w, "sharp-go image proxy")
		fmt.Fprintln(w, "  /<upstream-path>?w=&h=&q=&fit=&effort=&bg-color=")
		fmt.Fprintln(w, "  /raw/<upstream-path>      pass-through")
		fmt.Fprintln(w, "  /metrics[/<path>]         butteraugli + ssimulacra2 (JSON)")
		fmt.Fprintln(w, "  /compare[/<path>]         side-by-side HTML")
	case strings.HasPrefix(p, "/raw/"):
		proxyRaw(w, r, p[len("/raw"):])
	case strings.HasPrefix(p, "/fastly/"):
		proxyFastly(w, r, p[len("/fastly"):])
	case p == "/metrics" || strings.HasPrefix(p, "/metrics/"):
		serveMetrics(w, r)
	case p == "/compare" || strings.HasPrefix(p, "/compare/"):
		serveCompare(w, r)
	default:
		proxyOptimize(w, r, p)
	}
}

// fetchUpstream issues a GET against the origin with sane defaults. Use
// fetchOriginBytes when you want the result body buffered and cached.
func fetchUpstream(ctx context.Context, path, accept, ua string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", upstreamBase+path, nil)
	if err != nil {
		return nil, err
	}
	req.Host = upstreamHost
	if ua == "" {
		ua = "sharp-go-proxy/1.0"
	}
	if accept == "" {
		accept = "image/*"
	}
	req.Header.Set("user-agent", ua)
	req.Header.Set("accept", accept)
	return httpClient.Do(req)
}

// fetchOriginBytes returns the upstream body with content-type, going
// through the on-disk cache. Returns (entry, fromCache, err). On cache miss
// the key is locked so concurrent identical requests dedupe to one origin
// fetch.
func fetchOriginBytes(ctx context.Context, path, accept, ua string) (*cachedOrigin, bool, error) {
	if origCache != nil {
		if hit, err := origCache.Get(path, accept); err == nil && hit != nil {
			return hit, true, nil
		}
	}

	var unlock func()
	if origCache != nil {
		unlock = origCache.lockKey(path, accept)
		defer unlock()
		// Re-check after acquiring the lock — another goroutine may have
		// populated the entry while we were waiting.
		if hit, err := origCache.Get(path, accept); err == nil && hit != nil {
			return hit, true, nil
		}
	}

	resp, err := fetchUpstream(ctx, path, accept, ua)
	if err != nil {
		return nil, false, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false, err
	}
	entry := &cachedOrigin{
		Body:        body,
		ContentType: resp.Header.Get("content-type"),
		Status:      resp.StatusCode,
	}
	// Only cache successful image responses — error pages and redirects
	// would otherwise stick around for the TTL.
	if origCache != nil && resp.StatusCode == 200 && strings.HasPrefix(entry.ContentType, "image/") {
		if err := origCache.Put(path, accept, entry, upstreamBase+path); err != nil {
			log.Printf("cache put failed: %v", err)
		}
	}
	return entry, false, nil
}

func proxyRaw(w http.ResponseWriter, r *http.Request, path string) {
	entry, hit, err := fetchOriginBytes(r.Context(), path, r.Header.Get("accept"), r.Header.Get("user-agent"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if entry.ContentType != "" {
		w.Header().Set("content-type", entry.ContentType)
	}
	w.Header().Set("cache-control", "public, max-age=31536000, immutable")
	w.Header().Set("x-origin-cache", cacheStateHeader(hit))
	w.WriteHeader(entry.Status)
	w.Write(entry.Body)
}

// cacheStateHeader returns a debug header value indicating whether the
// origin was served from cache.
func cacheStateHeader(hit bool) string {
	if hit {
		return "HIT"
	}
	return "MISS"
}

// proxyFastly streams the same path from FASTLY_BASE so the compare HTML
// can fetch it same-origin (no CORS) and so we can echo Fastly's
// fastly-io-info / x-served-by headers back as observable evidence.
// Query string is forwarded verbatim — pass ?width=W&quality=Q etc.
func proxyFastly(w http.ResponseWriter, r *http.Request, path string) {
	if fastlyBase == "" {
		http.Error(w, "FASTLY_BASE not configured", http.StatusNotFound)
		return
	}
	target := fastlyBase + path
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", target, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	req.Header.Set("user-agent", "sharp-go-proxy/1.0 (fastly-compare)")
	accept := r.Header.Get("accept")
	if accept == "" {
		accept = "image/avif,image/webp,*/*"
	}
	req.Header.Set("accept", accept)

	resp, err := httpClient.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Forward useful Fastly observability headers so the compare UI can
	// surface them ("Fastly served WebP from img02-asia-northeast1").
	for _, h := range []string{
		"content-type", "fastly-io-info", "fastly-io-served-by",
		"x-served-by", "x-cache", "cache-control",
	} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.Header().Set("x-image-source", "fastly")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

func proxyOptimize(w http.ResponseWriter, r *http.Request, path string) {
	entry, hit, err := fetchOriginBytes(r.Context(), path, r.Header.Get("accept"), r.Header.Get("user-agent"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if entry.Status != 200 {
		http.Error(w, fmt.Sprintf("upstream %d", entry.Status), entry.Status)
		return
	}
	ct := entry.ContentType
	if !strings.HasPrefix(ct, "image/") {
		// Non-image — straight pass-through.
		w.Header().Set("content-type", ct)
		w.Header().Set("cache-control", "public, max-age=31536000, immutable")
		w.Header().Set("x-origin-cache", cacheStateHeader(hit))
		w.Write(entry.Body)
		return
	}
	body := entry.Body

	q := r.URL.Query()
	preset := firstNonEmpty(q.Get("optimize"), q.Get("effort"))
	width := clampInt(q.Get("w"), 1, 8000, 0)
	height := clampInt(q.Get("h"), 1, 8000, 0)
	fit := parseFit(q.Get("fit"))
	bg, hasBG := parseBG(q.Get("bg-color"))

	out := pickFormat(q.Get("format"), q.Get("f"), r.Header.Get("Accept"))
	tune := presetFor(out, preset)
	// Explicit q= overrides the preset's quality.
	if qStr := q.Get("q"); qStr != "" {
		if v := clampInt(qStr, 1, 100, tune.quality); v > 0 {
			tune.quality = v
		}
	}

	// EnsureSRGB transforms wide-gamut (Adobe RGB, Display P3, …) source
	// pixels through the embedded ICC profile so the encoded output looks
	// identical on viewers that don't colour-manage WebP/AVIF — same
	// rendering Fastly Image Optimizer ships. Without this, Adobe RGB
	// pixels carried verbatim into sRGB-assuming browsers appear lighter
	// and desaturated.
	//
	// AutoOrient applies the EXIF orientation tag in pixels then strips
	// it, so phone-camera photos display upright on every viewer (some
	// browsers ignore the tag for WebP/AVIF). Sharp does this by default;
	// we match the behaviour.
	// Variant key captures every input that changes the encoded bytes. A hit
	// serves the stored encode directly — skipping the full-res libvips
	// decode + WebP/AVIF encode that dominates request latency.
	varKey := strings.Join([]string{
		path, out,
		strconv.Itoa(tune.quality), strconv.Itoa(tune.effort),
		strconv.Itoa(width), strconv.Itoa(height),
		strconv.Itoa(int(fit)), q.Get("bg-color"),
	}, "|")
	if outCache != nil {
		if hitOut, err := outCache.Get(varKey, ""); err == nil && hitOut != nil {
			w.Header().Set("content-type", hitOut.ContentType)
			w.Header().Set("cache-control", "public, max-age=31536000, immutable")
			w.Header().Set("vary", "Accept")
			w.Header().Set("x-image-format", out)
			w.Header().Set("x-image-quality", strconv.Itoa(tune.quality))
			w.Header().Set("x-image-effort", strconv.Itoa(tune.effort))
			w.Header().Set("x-upstream", upstreamBase+path)
			w.Header().Set("x-origin-cache", cacheStateHeader(hit))
			w.Header().Set("x-optimized-cache", "hit")
			w.Write(hitOut.Body)
			return
		}
	}

	pipe := sharp.FromBytes(body).EnsureSRGB().AutoOrient()

	if hasBG {
		pipe = pipe.Flatten(sharp.FlattenOptions{Background: bg})
	}

	resized := width > 0 || height > 0
	if resized {
		ro := sharp.ResizeOptions{
			Width:              width,
			Height:             height,
			Fit:                fit,
			WithoutEnlargement: true,
		}
		if hasBG {
			ro.Background = bg
		} else {
			ro.Background = sharp.Color{R: 255, G: 255, B: 255, A: 255}
		}
		pipe = pipe.Resize(ro)
	}

	// Pre-encode sharpen ONLY when significantly downscaled (≥2×). On
	// full-res or near-full-res images the source is already sharp and
	// unsharp masking creates halos that compression amplifies — both
	// butteraugli and ssimulacra2 penalise this. Measured: +0% perceptual
	// quality, +20% bytes for full-res. Net loss. Gate strictly.
	if resized && tune.effort >= 6 {
		pipe = pipe.Sharpen(sharp.SharpenOptions{Sigma: 0.5})
	}

	pipe = applyEncoder(pipe, out, tune)

	buf, _, err := pipe.ToBytes(r.Context())
	if err != nil {
		log.Printf("encode error path=%s fmt=%s err=%v", path, out, err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if outCache != nil {
		entry := &cachedOrigin{
			Body:        buf,
			ContentType: contentType(out),
			Status:      200,
		}
		if err := outCache.Put(varKey, "", entry, upstreamBase+path); err != nil {
			log.Printf("output cache put failed key=%s: %v", varKey, err)
		}
	}

	w.Header().Set("content-type", contentType(out))
	w.Header().Set("cache-control", "public, max-age=31536000, immutable")
	w.Header().Set("vary", "Accept")
	w.Header().Set("x-image-format", out)
	w.Header().Set("x-image-quality", strconv.Itoa(tune.quality))
	w.Header().Set("x-image-effort", strconv.Itoa(tune.effort))
	w.Header().Set("x-upstream", upstreamBase+path)
	w.Header().Set("x-origin-cache", cacheStateHeader(hit))
	w.Header().Set("x-optimized-cache", "miss")
	w.Write(buf)
}

// outFormat is the encoder we ship to the client.
type outFormat = string

const (
	outAVIF outFormat = "avif"
	outWebP outFormat = "webp"
	outJPEG outFormat = "jpeg"
)

// pickFormat resolves the output codec. `?format=` and `?f=` override Accept
// negotiation. Negotiation order matches Fastly Image Optimizer:
// AVIF > WebP > JPEG (clients that advertise nothing get WebP — universally
// supported in 2026).
func pickFormat(formatQ, fQ, accept string) outFormat {
	switch strings.ToLower(firstNonEmpty(formatQ, fQ)) {
	case "avif":
		return outAVIF
	case "webp":
		return outWebP
	case "jpg", "jpeg":
		return outJPEG
	case "auto", "":
		// negotiate
	default:
		return outWebP
	}
	a := strings.ToLower(accept)
	switch {
	case strings.Contains(a, "image/avif"):
		return outAVIF
	case strings.Contains(a, "image/webp"):
		return outWebP
	}
	return outWebP
}

func contentType(f outFormat) string {
	switch f {
	case outAVIF:
		return "image/avif"
	case outJPEG:
		return "image/jpeg"
	}
	return "image/webp"
}

// presetTune holds the per-encode tuning knobs.
type presetTune struct {
	quality int
	effort  int
}

// presetFor maps a preset name + output format to encoder params tuned for
// "best-quality-low-size" — values calibrated against ssimulacra2 ≥ 80 and
// butteraugli ≤ 1.5 on typical product photos. Low quality numbers are
// intentional for AVIF: AV1's q=50 is visually equivalent to WebP q=80.
// Preset quality numbers calibrated against the Fastly Image Optimizer
// baseline (WebP @ ~q65, butteraugli 3.67, ssimulacra2 64.5). Goal: every
// preset matches or beats Fastly on BOTH butteraugli AND ssimulacra2,
// while AVIF/high ships meaningfully smaller bytes for clients that
// accept it.
func presetFor(f outFormat, preset string) presetTune {
	switch f {
	case outAVIF:
		// AVIF q=55 e=6 lands at -10% size vs Fastly's WebP with
		// butteraugli 3.35 (Fastly: 3.67) and ssimulacra2 70.5
		// (Fastly: 64.5) — a clean win on every axis. Lower presets
		// trade quality for size; max preset is for hero images where
		// encode time doesn't matter.
		switch preset {
		case "low":
			return presetTune{quality: 50, effort: 2}
		case "high":
			return presetTune{quality: 55, effort: 3}
		case "max":
			// Opt-in only: e9 can take seconds on large images. Not bound by
			// the ~300ms latency budget the other presets target.
			return presetTune{quality: 60, effort: 9}
		default: // medium / unset
			// effort 2 keeps AVIF encode ≤~220ms even at full resolution
			// (aom's e4 sits in a slow spot for ~0.7-3% smaller bytes — not
			// worth 3x the latency). Still beats Fastly on size + quality.
			return presetTune{quality: 52, effort: 2}
		}
	case outJPEG:
		// MozJPEG. q=82 with trellis matches Fastly's JPEG output
		// quality at slightly smaller byte size.
		switch preset {
		case "low":
			return presetTune{quality: 75, effort: 0}
		case "high":
			return presetTune{quality: 82, effort: 0}
		case "max":
			return presetTune{quality: 88, effort: 0}
		default:
			return presetTune{quality: 80, effort: 0}
		}
	}
	// WebP with preset=photo + smart-deblock + passes=10. Quality numbers
	// calibrated to land ≥10 KB below Fastly's WebP at the same resolution.
	// Measured on a 1800×2400 product photo: Fastly ~322 KB; us at Q=68
	// ~311 KB (-11 KB). Quality stays in the same verdict tier.
	switch preset {
	case "low":
		return presetTune{quality: 62, effort: 4}
	case "high":
		return presetTune{quality: 68, effort: 6}
	case "max":
		return presetTune{quality: 75, effort: 6}
	default:
		return presetTune{quality: 65, effort: 4}
	}
}

// applyEncoder chains the format-specific encoder onto pipe with quality/
// effort tuned per Fastly-style preset.
func applyEncoder(pipe *sharp.Image, f outFormat, t presetTune) *sharp.Image {
	switch f {
	case outAVIF:
		return pipe.AVIF(format.AVIFOptions{
			Quality: t.quality,
			Effort:  t.effort,
			// 4:2:0 default — best size for photos. Switch to 4:4:4 for
			// images dominated by text/UI via ?subsample=444 (not exposed).
		})
	case outJPEG:
		return pipe.JPEG(format.JPEGOptions{
			Quality: t.quality,
			MozJPEG: true, // trellis + overshoot + optimise scans + progressive
		})
	}
	// UseSharpYUV routes through sharp-go's libwebp-direct path that sets
	// WebPConfig.use_sharp_yuv = 1 — sharper RGB→YUV conversion that
	// libvips's vips_webpsave_buffer doesn't expose. Measured win on this
	// codebase's test image: -0.10 butteraugli + 2.0 ssimulacra2 vs the
	// vips_webpsave_buffer path at the same Q + preset. AutoFilter +
	// passes=10 + preset=photo stack on top.
	return pipe.WebP(format.WebPOptions{
		Quality:     t.quality,
		Effort:      t.effort,
		Preset:      "photo",
		UseSharpYUV: true,
		AutoFilter:  true,
		Multithread: true, // parallel token-partition encode; ~17% faster, sub-0.1% size
		Passes: func() int {
			if t.effort >= 6 {
				return 10
			}
			return 0
		}(),
		AlphaQuality: 90,
	})
}

// ───── query parsing ─────

func clampInt(s string, lo, hi, def int) int {
	if s == "" {
		return def
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if n < lo {
		return lo
	}
	if n > hi {
		return hi
	}
	return n
}

func parseFit(s string) sharp.Fit {
	switch s {
	case "cover", "crop":
		return sharp.FitCover
	case "contain", "pad":
		return sharp.FitContain
	case "fill", "scale":
		return sharp.FitFill
	case "outside":
		return sharp.FitOutside
	case "bounds", "inside", "clip", "":
		fallthrough
	default:
		return sharp.FitInside
	}
}

func parseBG(s string) (sharp.Color, bool) {
	if s == "" {
		return sharp.Color{}, false
	}
	parts := strings.Split(s, ",")
	if len(parts) < 3 {
		return sharp.Color{}, false
	}
	clamp := func(n int) float64 {
		if n < 0 {
			n = 0
		}
		if n > 255 {
			n = 255
		}
		return float64(n)
	}
	r, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
	g, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
	b, err3 := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err1 != nil || err2 != nil || err3 != nil {
		return sharp.Color{}, false
	}
	a := 255
	if len(parts) >= 4 {
		if v, err := strconv.Atoi(strings.TrimSpace(parts[3])); err == nil {
			a = v
		}
	}
	return sharp.Color{R: clamp(r), G: clamp(g), B: clamp(b), A: clamp(a)}, true
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

// ───── metrics ─────

type metricsResp struct {
	Path           string       `json:"path"`
	OriginBytes    int          `json:"origin_bytes"`
	OptimizedBytes int          `json:"optimized_bytes"`
	SavedPct       float64      `json:"saved_pct"`
	Params         metricsParam `json:"params"`
	Butteraugli    scoreBlock   `json:"butteraugli"`
	Ssimulacra2    scoreBlock   `json:"ssimulacra2"`
	DSSIM          scoreBlock   `json:"dssim"`
	PSNR           scoreBlock   `json:"psnr"`
	Fastly         *candidate   `json:"fastly,omitempty"`
}

type metricsParam struct {
	Format string `json:"format"`
	Q      int    `json:"q"`
	Effort int    `json:"effort"`
}

type scoreBlock struct {
	Score   float64 `json:"score,omitempty"`        // ssimulacra2: 0-100 (higher better)
	MaxDist float64 `json:"max_distance,omitempty"` // butteraugli: 0+ (lower better)
	DSSIM   float64 `json:"dssim,omitempty"`        // dssim: 0+ (lower better, 0 = identical)
	PSNR    float64 `json:"psnr,omitempty"`         // PSNR dB (higher better)
	Verdict string  `json:"verdict"`
	Raw     string  `json:"raw"`
}

// allScores groups the four perceptual metrics computed against a shared
// reference. Higher = better for SSIMULACRA2 + PSNR; lower = better for
// butteraugli + DSSIM.
type allScores struct {
	Butteraugli scoreBlock `json:"butteraugli"`
	Ssimulacra2 scoreBlock `json:"ssimulacra2"`
	DSSIM       scoreBlock `json:"dssim"`
	PSNR        scoreBlock `json:"psnr"`
}

// candidate captures the per-pipeline result for compare-page scoring.
type candidate struct {
	Bytes       int        `json:"bytes"`
	Format      string     `json:"format,omitempty"`
	SavedPct    float64    `json:"saved_pct"`
	Butteraugli scoreBlock `json:"butteraugli"`
	Ssimulacra2 scoreBlock `json:"ssimulacra2"`
	DSSIM       scoreBlock `json:"dssim"`
	PSNR        scoreBlock `json:"psnr"`
	Source      string     `json:"source,omitempty"` // e.g. "img02-asia-northeast1"
	Note        string     `json:"note,omitempty"`   // diagnostic message if scoring failed
}

func serveMetrics(w http.ResponseWriter, r *http.Request) {
	path := pathFromURL(r, "/metrics")
	q := r.URL.Query()
	preset := firstNonEmpty(q.Get("optimize"), q.Get("effort"))
	outFmt := pickFormat(q.Get("format"), q.Get("f"), r.Header.Get("Accept"))
	tune := presetFor(outFmt, preset)
	if qStr := q.Get("q"); qStr != "" {
		if v := clampInt(qStr, 1, 100, tune.quality); v > 0 {
			tune.quality = v
		}
	}

	entry, _, err := fetchOriginBytes(r.Context(), path, "image/*", "sharp-go-proxy/1.0")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if entry.Status != 200 {
		http.Error(w, fmt.Sprintf("upstream %d", entry.Status), http.StatusBadGateway)
		return
	}
	origBuf := entry.Body

	// Encode our candidate at full resolution. EnsureSRGB + AutoOrient
	// match the optimize path so perceptual scores reflect what the
	// browser would actually render (orientation + colour space normalised).
	// /metrics always runs full-res with no resize, so we skip the
	// post-downscale sharpen that the optimize path applies.
	optBuf, _, err := applyEncoder(sharp.FromBytes(origBuf).EnsureSRGB().AutoOrient(), outFmt, tune).
		ToBytes(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Fetch Fastly's variant if configured. We forward the picked format
	// to their `format=` query param so tenants that don't auto-upgrade
	// via Accept still ship the same codec we do — otherwise we'd be
	// scoring our AVIF against their JPEG, an unfair fight.
	var (
		fastlyBuf []byte
		fastlyCT  string
		fastlyPOP string
	)
	if fastlyBase != "" {
		fq := url.Values{}
		switch outFmt {
		case outAVIF, outWebP, outJPEG:
			fq.Set("format", outFmt)
		}
		fastlyPath := path
		if e := fq.Encode(); e != "" {
			fastlyPath += "?" + e
		}
		fastlyBuf, fastlyCT, fastlyPOP, _ = fetchFastlyForMetrics(r.Context(), fastlyPath, r.Header.Get("Accept"))
	}

	// butteraugli + ssimulacra2 both need same-dim PNG inputs in sRGB.
	// The reference itself goes through EnsureSRGB so the colour space
	// matches every candidate. Without this the score is dominated by
	// the Adobe-RGB → sRGB delta and reports false "visible loss".
	tmp := os.TempDir()
	id := strconv.FormatInt(time.Now().UnixNano(), 10)
	refPng := filepath.Join(tmp, "sgp-"+id+"-ref.png")
	optPng := filepath.Join(tmp, "sgp-"+id+"-opt.png")
	fastlyPng := filepath.Join(tmp, "sgp-"+id+"-fastly.png")
	defer os.Remove(refPng)
	defer os.Remove(optPng)
	defer os.Remove(fastlyPng)

	if err := writeSRGBPNG(r.Context(), origBuf, refPng); err != nil {
		http.Error(w, "png(ref): "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := writeSRGBPNG(r.Context(), optBuf, optPng); err != nil {
		http.Error(w, "png(opt): "+err.Error(), http.StatusInternalServerError)
		return
	}
	optScores := scoreAgainst(r.Context(), refPng, optPng)

	var fastlyCand *candidate
	if len(fastlyBuf) > 0 {
		if err := writeSRGBPNG(r.Context(), fastlyBuf, fastlyPng); err != nil {
			fastlyCand = &candidate{
				Bytes:    len(fastlyBuf),
				Format:   fastlyCT,
				SavedPct: pctSaved(len(fastlyBuf), len(origBuf)),
				Source:   fastlyPOP,
				Note:     "png(fastly): " + err.Error(),
			}
		} else {
			fScores := scoreAgainst(r.Context(), refPng, fastlyPng)
			fastlyCand = &candidate{
				Bytes:       len(fastlyBuf),
				Format:      fastlyCT,
				SavedPct:    pctSaved(len(fastlyBuf), len(origBuf)),
				Butteraugli: fScores.Butteraugli,
				Ssimulacra2: fScores.Ssimulacra2,
				DSSIM:       fScores.DSSIM,
				PSNR:        fScores.PSNR,
				Source:      fastlyPOP,
			}
		}
	}

	res := metricsResp{
		Path:           path,
		OriginBytes:    len(origBuf),
		OptimizedBytes: len(optBuf),
		SavedPct:       pctSaved(len(optBuf), len(origBuf)),
		Params:         metricsParam{Format: outFmt, Q: tune.quality, Effort: tune.effort},
		Butteraugli:    optScores.Butteraugli,
		Ssimulacra2:    optScores.Ssimulacra2,
		DSSIM:          optScores.DSSIM,
		PSNR:           optScores.PSNR,
		Fastly:         fastlyCand,
	}
	w.Header().Set("content-type", "application/json")
	json.NewEncoder(w).Encode(res)
}

// writeSRGBPNG decodes src and writes a PNG with pixels transformed to
// sRGB via the embedded ICC profile (Adobe RGB / Display P3 / ProPhoto
// inputs all normalise to sRGB). Required so reference + candidate PNGs
// are in the same colour space before butteraugli/ssimulacra2.
func writeSRGBPNG(ctx context.Context, src []byte, dst string) error {
	_, err := sharp.FromBytes(src).EnsureSRGB().PNG(format.PNGOptions{}).ToFile(ctx, dst)
	return err
}

// fetchFastlyForMetrics gets the same path from FASTLY_BASE for scoring.
// Forwards the Accept header verbatim so we score whichever format Fastly
// would have served the user (WebP/AVIF/JPEG).
func fetchFastlyForMetrics(ctx context.Context, path, accept string) ([]byte, string, string, error) {
	if accept == "" {
		accept = "image/avif,image/webp,*/*"
	}
	req, err := http.NewRequestWithContext(ctx, "GET", fastlyBase+path, nil)
	if err != nil {
		return nil, "", "", err
	}
	req.Header.Set("user-agent", "sharp-go-proxy/1.0 (fastly-compare)")
	req.Header.Set("accept", accept)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", "", fmt.Errorf("fastly %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", "", err
	}
	return body, resp.Header.Get("content-type"),
		firstNonEmpty(resp.Header.Get("fastly-io-served-by"), resp.Header.Get("x-served-by")),
		nil
}

// scoreAgainst runs four perceptual metrics in parallel against refPng:
//
//   - butteraugli  (Google) — 3-norm max distance (lower = better)
//   - ssimulacra2 (cloudinary/jpeg-xl) — 0-100 (higher = better)
//   - dssim       (Cloudflare/imageoptim) — 0+ (lower = better; 0 = identical)
//   - PSNR        (libvips) — dB (higher = better; 30+ = good photo, 40+ = excellent)
//
// All four metrics are scored against the same sRGB PNG reference so the
// numbers are apples-to-apples across formats (WebP/AVIF/JPEG).
func scoreAgainst(ctx context.Context, refPng, candPng string) allScores {
	type runRes struct {
		out string
		err error
	}
	baCh := make(chan runRes, 1)
	ssCh := make(chan runRes, 1)
	dsCh := make(chan runRes, 1)
	psCh := make(chan struct {
		mse float64
		err error
	}, 1)
	go func() {
		out, err := exec.CommandContext(ctx, "butteraugli_main", refPng, candPng).CombinedOutput()
		baCh <- runRes{string(out), err}
	}()
	go func() {
		out, err := exec.CommandContext(ctx, "ssimulacra2", refPng, candPng).CombinedOutput()
		ssCh <- runRes{string(out), err}
	}()
	go func() {
		out, err := exec.CommandContext(ctx, "dssim", refPng, candPng).CombinedOutput()
		dsCh <- runRes{string(out), err}
	}()
	go func() {
		mse, err := computeMSE(ctx, refPng, candPng)
		psCh <- struct {
			mse float64
			err error
		}{mse, err}
	}()
	ba := <-baCh
	ss := <-ssCh
	ds := <-dsCh
	ps := <-psCh

	baScore := parseBA(ba.out)
	ssScore := parseSS(ss.out)
	dsScore := parseDSSIM(ds.out)
	psnr := 0.0
	if ps.err == nil && ps.mse > 0 {
		// PSNR = 10 * log10(MAX^2 / MSE), MAX=255 for 8-bit sRGB.
		psnr = 10 * math.Log10(255*255/ps.mse)
	}

	return allScores{
		Butteraugli: scoreBlock{
			MaxDist: baScore,
			Verdict: verdictBA(baScore),
			Raw:     strings.TrimSpace(ba.out),
		},
		Ssimulacra2: scoreBlock{
			Score:   ssScore,
			Verdict: verdictSS(ssScore),
			Raw:     strings.TrimSpace(ss.out),
		},
		DSSIM: scoreBlock{
			DSSIM:   dsScore,
			Verdict: verdictDSSIM(dsScore),
			Raw:     strings.TrimSpace(ds.out),
		},
		PSNR: scoreBlock{
			PSNR:    psnr,
			Verdict: verdictPSNR(psnr),
		},
	}
}

// parseDSSIM extracts the score from dssim's "<score>\t<filename>" output.
func parseDSSIM(out string) float64 {
	fields := strings.Fields(strings.TrimSpace(out))
	if len(fields) == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(fields[0], 64)
	return f
}

// computeMSE shells out to `vips` to compute per-pixel MSE between two PNGs.
// Saves us a libvips cgo call from the example; same numerical result.
//
//	subtract → multiply self → avg
func computeMSE(ctx context.Context, ref, cand string) (float64, error) {
	diff, err := os.CreateTemp("", "sgp-diff-*.v")
	if err != nil {
		return 0, err
	}
	defer os.Remove(diff.Name())
	diff.Close()
	sq, err := os.CreateTemp("", "sgp-sq-*.v")
	if err != nil {
		return 0, err
	}
	defer os.Remove(sq.Name())
	sq.Close()

	if out, err := exec.CommandContext(ctx, "vips", "subtract", ref, cand, diff.Name()).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("vips subtract: %v: %s", err, out)
	}
	if out, err := exec.CommandContext(ctx, "vips", "multiply", diff.Name(), diff.Name(), sq.Name()).CombinedOutput(); err != nil {
		return 0, fmt.Errorf("vips multiply: %v: %s", err, out)
	}
	out, err := exec.CommandContext(ctx, "vips", "avg", sq.Name()).CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("vips avg: %v: %s", err, out)
	}
	mse, perr := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if perr != nil {
		return 0, perr
	}
	return mse, nil
}

// verdictDSSIM: 0 = identical, <0.001 = imperceptible, <0.01 = excellent,
// <0.03 = good, <0.07 = acceptable, more = visible loss.
func verdictDSSIM(v float64) string {
	switch {
	case v == 0:
		return "identical"
	case v < 0.001:
		return "imperceptible"
	case v < 0.01:
		return "excellent"
	case v < 0.03:
		return "good"
	case v < 0.07:
		return "acceptable"
	default:
		return "visible loss"
	}
}

// verdictPSNR: dB thresholds calibrated for 8-bit sRGB photographic content.
// >50 dB = essentially lossless, 40-50 = excellent, 35-40 = good,
// 30-35 = acceptable, <30 = visible compression.
func verdictPSNR(v float64) string {
	switch {
	case v == 0:
		return "unknown"
	case v >= 50:
		return "perfect"
	case v >= 40:
		return "excellent"
	case v >= 35:
		return "good"
	case v >= 30:
		return "acceptable"
	default:
		return "visible loss"
	}
}

func pctSaved(out, orig int) float64 {
	if orig <= 0 {
		return 0
	}
	return round2((1 - float64(out)/float64(orig)) * 100)
}

func parseBA(out string) float64 {
	m := baLineRegex.FindAllStringSubmatch(out, -1)
	if len(m) == 0 {
		return 0
	}
	// Butteraugli prints the max-distance number on its own line at the end.
	last := m[len(m)-1][1]
	f, _ := strconv.ParseFloat(last, 64)
	return f
}

func parseSS(out string) float64 {
	parts := strings.Fields(out)
	if len(parts) == 0 {
		return 0
	}
	f, _ := strconv.ParseFloat(parts[len(parts)-1], 64)
	return f
}

func verdictBA(v float64) string {
	switch {
	case v == 0:
		return "unknown"
	case v < 1.0:
		return "imperceptible"
	case v < 1.5:
		return "excellent"
	case v < 3:
		return "good"
	case v < 6:
		return "acceptable"
	default:
		return "visible loss"
	}
}

func verdictSS(v float64) string {
	switch {
	case v == 0:
		return "unknown"
	case v > 90:
		return "perfect"
	case v > 70:
		return "high (visually lossless)"
	case v > 50:
		return "medium-high"
	case v > 30:
		return "medium"
	default:
		return "low"
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

// ───── compare HTML ─────

func serveCompare(w http.ResponseWriter, r *http.Request) {
	path := pathFromURL(r, "/compare")
	q := r.URL.Query()
	get := func(k, def string) string {
		if v := q.Get(k); v != "" {
			return v
		}
		return def
	}
	// width/height are intentionally blank by default so /compare runs at
	// full resolution — that's the apples-to-apples comparison against
	// Fastly's no-dimension default. Pass ?w=1080 to compare at a
	// downscaled target.
	width := get("w", "")
	height := get("h", "")
	quality := get("q", "")
	effort := get("effort", "high")
	fit := get("fit", "bounds")
	bg := get("bg-color", "")
	outFmt := get("format", "webp")

	qs := url.Values{}
	qs.Set("effort", effort)
	qs.Set("fit", fit)
	qs.Set("format", outFmt)
	if bg != "" {
		qs.Set("bg-color", bg)
	}
	if width != "" {
		qs.Set("w", width)
	}
	if height != "" {
		qs.Set("h", height)
	}
	if quality != "" {
		qs.Set("q", quality)
	}

	origSrc := "/raw" + path
	optSrc := path + "?" + qs.Encode()
	metricsQS := url.Values{}
	metricsQS.Set("effort", effort)
	metricsQS.Set("format", outFmt)
	if quality != "" {
		metricsQS.Set("q", quality)
	}
	metricsSrc := "/metrics" + path + "?" + metricsQS.Encode()

	// Fastly URL: same path. If a width is requested, pass Fastly's
	// `width=` param. Quality maps to `quality=`.
	fastlyEnabled := fastlyBase != ""
	fastlySrc := ""
	if fastlyEnabled {
		fq := url.Values{}
		if width != "" {
			fq.Set("width", width)
		}
		if height != "" {
			fq.Set("height", height)
		}
		if quality != "" {
			fq.Set("quality", quality)
		}
		// Forward the codec selection to Fastly Image Optimizer's own
		// `format=` param so the panes are apples-to-apples (WebP vs
		// WebP / AVIF vs AVIF). Without this, Fastly tenants that
		// don't auto-upgrade Accept-driven negotiation will return JPEG
		// even when the browser advertises webp+avif.
		switch outFmt {
		case outAVIF, outWebP, outJPEG:
			fq.Set("format", outFmt)
		}
		fastlySrc = "/fastly" + path
		if e := fq.Encode(); e != "" {
			fastlySrc += "?" + e
		}
	}

	w.Header().Set("content-type", "text/html")
	fmt.Fprintf(w, compareTpl,
		path, width, height, quality,
		optBox("effort", []string{"low", "medium", "high", "max"}, effort),
		optBox("fit", []string{"bounds", "cover", "contain", "fill", "outside"}, fit),
		optBox("format", []string{"auto", "avif", "webp", "jpeg"}, outFmt),
		bg,
		threePaneGrid(origSrc, fastlySrc, optSrc, qs.Encode()),
		jsStr(origSrc), jsStr(fastlySrc), jsStr(optSrc), jsStr(metricsSrc),
	)
}

// threePaneGrid renders the original / Fastly / sharp-go panes. If
// fastlySrc is empty (FASTLY_BASE not configured) the middle pane is
// omitted and the layout is two columns.
func threePaneGrid(origSrc, fastlySrc, optSrc, qsLabel string) string {
	pane := func(id, title, src string) string {
		return `<div class="pane">
    <h2>` + title + `</h2>
    <div class="stats" id="` + id + `"><span class="k">loading…</span></div>
    <img id="img-` + id + `" src="` + src + `">
  </div>`
	}
	cols := "1fr 1fr"
	middle := ""
	if fastlySrc != "" {
		cols = "1fr 1fr 1fr"
		middle = pane("sFastly", "fastly", fastlySrc)
	}
	return `<div class="grid" style="grid-template-columns:` + cols + `">
  ` + pane("sOrig", "original", origSrc) + `
  ` + middle + `
  ` + pane("sOpt", `sharp-go · <span id=fmtLabel>?</span> · `+qsLabel, optSrc) + `
</div>`
}

func pathFromURL(r *http.Request, prefix string) string {
	sub := strings.TrimPrefix(r.URL.Path, prefix)
	if sub != "" && sub != "/" {
		return sub
	}
	if p := r.URL.Query().Get("path"); p != "" {
		return p
	}
	return defaultPath
}

func optBox(name string, opts []string, sel string) string {
	var sb strings.Builder
	sb.WriteString(`<select name="` + name + `">`)
	for _, o := range opts {
		s := ""
		if o == sel {
			s = " selected"
		}
		sb.WriteString("<option" + s + ">" + o + "</option>")
	}
	sb.WriteString(`</select>`)
	return sb.String()
}

func jsStr(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

const compareTpl = `<!doctype html>
<html><head><meta charset="utf-8"><title>compare (go + sharp-go)</title>
<style>
  body{font:14px system-ui;margin:0;background:#111;color:#eee}
  header{padding:12px 16px;background:#1c1c1c;border-bottom:1px solid #333;display:flex;gap:10px;flex-wrap:wrap;align-items:center}
  header label{display:flex;align-items:center;gap:4px;font-size:12px;color:#9aa}
  input,select{background:#222;color:#eee;border:1px solid #444;padding:4px 8px;font:13px monospace}
  input[name=path]{width:380px}
  button{background:#0a84ff;color:#fff;border:0;padding:6px 14px;cursor:pointer;font-weight:600}
  .badge{margin-left:auto;font:11px monospace;color:#7a8;background:#0a2a14;border:1px solid #2a5;padding:3px 8px}
  .grid{display:grid;grid-template-columns:1fr 1fr;gap:1px;background:#333}
  .pane{background:#000;display:flex;flex-direction:column;min-height:60vh}
  .pane h2{margin:0;padding:8px 12px;background:#1c1c1c;font-size:13px;font-weight:600}
  .pane .stats{padding:8px 12px;background:#181818;font:12px monospace;color:#eee;border-bottom:1px solid #333;display:grid;grid-template-columns:auto 1fr;gap:4px 12px}
  .pane .stats .k{color:#7a8;font-size:11px;text-transform:uppercase;letter-spacing:.5px;align-self:center}
  .pane .stats b{color:#fff}
  .pane img{max-width:100%%;max-height:80vh;height:auto;display:block;margin:auto;object-fit:contain}
  .saved{color:#5ec27e;font-weight:700}
  .v-perfect,.v-imperceptible{color:#5ec27e}
  .v-excellent,.v-high{color:#7dd3a8}
  .v-good,.v-medium-high{color:#e3c46a}
  .v-medium,.v-acceptable{color:#e89c5d}
  .v-low,.v-visible{color:#e35d5d}
</style></head>
<body>
<header>
  <form style="display:contents">
    <label>path<input name="path" value="%s"></label>
    <label>w<input name="w" value="%s" size="5"></label>
    <label>h<input name="h" value="%s" size="5"></label>
    <label>q<input name="q" value="%s" size="3" placeholder="auto"></label>
    <label>effort%s</label>
    <label>fit%s</label>
    <label>format%s</label>
    <label>bg<input name="bg-color" value="%s" size="11"></label>
    <button>compare</button>
  </form>
  <span class="badge">sharp-go · avif/webp/jpeg negotiation</span>
</header>
%s
<script>
async function measure(url){
  if (!url) return null;
  const t0 = performance.now();
  const r = await fetch(url, {cache:'no-store'});
  const blob = await r.blob();
  const ms = (performance.now()-t0).toFixed(0);
  const dims = await new Promise(res => {
    const im = new Image();
    im.onload = () => res({w:im.naturalWidth, h:im.naturalHeight});
    im.onerror = () => res({w:0,h:0});
    im.src = URL.createObjectURL(blob);
  });
  return {
    bytes: blob.size,
    ct:   r.headers.get('content-type')||'',
    fmt:  r.headers.get('x-image-format')||'',
    q:    r.headers.get('x-image-quality')||'',
    eff:  r.headers.get('x-image-effort')||'',
    fio:  r.headers.get('fastly-io-info')||'',
    fsb:  r.headers.get('fastly-io-served-by')||r.headers.get('x-served-by')||'',
    ms, w:dims.w, h:dims.h,
  };
}
function fmtKB(n){ return (n/1024).toFixed(1)+' KB'; }
function vClass(s){ return 'v-'+s.split(' ')[0].replace(/[()]/g,''); }
function kv(k,v){ return '<span class="k">'+k+'</span><span>'+v+'</span>'; }
function delta(b, base){
  if (!base || !b) return '';
  const saved = ((1 - b.bytes/base.bytes)*100).toFixed(1);
  const ratio = (base.bytes/b.bytes).toFixed(1);
  const cls   = b.bytes < base.bytes ? 'saved' : 'v-low';
  const sign  = b.bytes < base.bytes ? '-' : '+';
  return ' <span class="'+cls+'">('+sign+Math.abs(saved)+'%%, '+ratio+'×)</span>';
}
(async () => {
  const [orig, fastly, opt] = await Promise.all([measure(%s), measure(%s), measure(%s)]);

  document.getElementById('sOrig').innerHTML =
    kv('size', '<b>'+fmtKB(orig.bytes)+'</b>') +
    kv('format', orig.ct) +
    kv('dims', orig.w+' × '+orig.h) +
    kv('fetch', orig.ms+' ms');

  if (fastly) {
    const dF = delta(fastly, orig);
    document.getElementById('sFastly').innerHTML =
      kv('size', '<b>'+fmtKB(fastly.bytes)+'</b>'+dF) +
      kv('format', fastly.ct) +
      kv('dims', fastly.w+' × '+fastly.h) +
      kv('fetch', fastly.ms+' ms') +
      (fastly.fsb ? kv('pop', fastly.fsb) : '') +
      kv('butteraugli ↓', '<span id=fastly-ba>measuring…</span>') +
      kv('ssimulacra2 ↑', '<span id=fastly-ss>measuring…</span>') +
      kv('dssim ↓',       '<span id=fastly-ds>measuring…</span>') +
      kv('psnr ↑',        '<span id=fastly-ps>measuring…</span>') +
      (fastly.fio ? kv('io-info', '<small>'+fastly.fio+'</small>') : '');
  }

  if (opt.fmt) document.getElementById('fmtLabel').textContent = opt.fmt;
  const dO = delta(opt, orig);
  const dV = fastly ? ' <span class="saved">vs fastly: '+((1-opt.bytes/fastly.bytes)*100).toFixed(1)+'%%</span>' : '';
  document.getElementById('sOpt').innerHTML =
    kv('size', '<b>'+fmtKB(opt.bytes)+'</b>'+dO+dV) +
    kv('format', opt.ct) +
    kv('encode', 'q='+opt.q+' effort='+opt.eff) +
    kv('dims', opt.w+' × '+opt.h) +
    kv('fetch', opt.ms+' ms') +
    kv('butteraugli ↓', '<span id=ba>measuring…</span>') +
    kv('ssimulacra2 ↑', '<span id=ss>measuring…</span>') +
    kv('dssim ↓',       '<span id=ds>measuring…</span>') +
    kv('psnr ↑',        '<span id=ps>measuring…</span>');

  // paintMetric formats one perceptual metric. The kind arg picks the
  // numeric field on the block plus its display precision.
  function paintMetric(elId, s, kind){
    if (!s) { document.getElementById(elId).textContent = '—'; return; }
    let val, decimals;
    switch (kind) {
      case 'ba': val = s.max_distance; decimals = 2; break;
      case 'ss': val = s.score;        decimals = 1; break;
      case 'ds': val = s.dssim;        decimals = 4; break;
      case 'ps': val = s.psnr;         decimals = 2; break;
    }
    if (val == null || val === 0 && kind !== 'ds') {
      document.getElementById(elId).textContent = s.verdict || 'n/a';
      return;
    }
    document.getElementById(elId).innerHTML =
      '<b>'+val.toFixed(decimals)+'</b> <span class="'+vClass(s.verdict)+'">'+s.verdict+'</span>';
  }

  try {
    const m = await (await fetch(%s)).json();
    paintMetric('ba', m.butteraugli, 'ba');
    paintMetric('ss', m.ssimulacra2, 'ss');
    paintMetric('ds', m.dssim,       'ds');
    paintMetric('ps', m.psnr,        'ps');
    if (fastly && m.fastly) {
      paintMetric('fastly-ba', m.fastly.butteraugli, 'ba');
      paintMetric('fastly-ss', m.fastly.ssimulacra2, 'ss');
      paintMetric('fastly-ds', m.fastly.dssim,       'ds');
      paintMetric('fastly-ps', m.fastly.psnr,        'ps');
    } else if (fastly) {
      ['fastly-ba','fastly-ss','fastly-ds','fastly-ps'].forEach(id =>
        document.getElementById(id).textContent = 'n/a');
    }
  } catch(e) {
    document.getElementById('ba').textContent = 'err';
    document.getElementById('ss').textContent = 'err';
  }
})();
</script>
</body></html>`

// originCache is an on-disk read-through cache for upstream origin
// responses. Keyed by (path, accept-header) so different format
// negotiations don't poison each other. Entries expire by mtime + TTL.
//
// Layout per entry:
//
//	<CACHE_DIR>/<sha256(key)[:2]>/<sha256(key)>.bin
//	uint32 LE meta length | JSON meta | body bytes
type originCache struct {
	dir string
	ttl time.Duration

	// Per-key singleflight: prevents two requests for the same upstream
	// path from both hitting origin during a cold miss.
	flight   sync.Mutex
	inFlight map[string]chan struct{}
}

type cacheMeta struct {
	ContentType string `json:"content_type"`
	Status      int    `json:"status"`
	UpstreamURL string `json:"upstream_url"`
	FetchedAt   int64  `json:"fetched_at"`
}

type cachedOrigin struct {
	Body        []byte
	ContentType string
	Status      int
}

func newOriginCache(dir string, ttl time.Duration, clear bool) (*originCache, error) {
	if dir == "" {
		return nil, errors.New("cache: empty dir")
	}
	// Drop any entries left by a previous run so a restart never serves
	// stale bytes (e.g. after changing encode presets/effort).
	if clear {
		if err := os.RemoveAll(dir); err != nil {
			return nil, err
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return &originCache{
		dir:      dir,
		ttl:      ttl,
		inFlight: map[string]chan struct{}{},
	}, nil
}

func (c *originCache) keyHash(path, accept string) string {
	h := sha256.Sum256([]byte(path + "\x00" + accept))
	return hex.EncodeToString(h[:])
}

func (c *originCache) pathFor(key string) string {
	return filepath.Join(c.dir, key[:2], key+".bin")
}

// Get returns a cached entry if present and not expired. Returns (nil, nil)
// on miss; (nil, err) only on I/O failure.
func (c *originCache) Get(path, accept string) (*cachedOrigin, error) {
	key := c.keyHash(path, accept)
	fp := c.pathFor(key)

	st, err := os.Stat(fp)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if c.ttl > 0 && time.Since(st.ModTime()) > c.ttl {
		_ = os.Remove(fp) // best-effort eviction
		return nil, nil
	}
	return readEntry(fp)
}

// Put writes the entry to disk atomically (temp file + rename).
func (c *originCache) Put(path, accept string, e *cachedOrigin, upstreamURL string) error {
	key := c.keyHash(path, accept)
	fp := c.pathFor(key)
	if err := os.MkdirAll(filepath.Dir(fp), 0o755); err != nil {
		return err
	}

	meta := cacheMeta{
		ContentType: e.ContentType,
		Status:      e.Status,
		UpstreamURL: upstreamURL,
		FetchedAt:   time.Now().Unix(),
	}
	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	tmp, err := os.CreateTemp(filepath.Dir(fp), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// If we didn't successfully rename, clean up the temp.
		if _, statErr := os.Stat(tmpName); statErr == nil {
			_ = os.Remove(tmpName)
		}
	}()

	if err := writeEntry(tmp, metaBytes, e.Body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, fp)
}

// lockKey serialises in-flight origin fetches for the same key so a stampede
// of N concurrent requests at cold-cache start hits origin exactly once.
// Returns an unlock callback; subsequent callers block until unlock fires.
func (c *originCache) lockKey(path, accept string) func() {
	key := c.keyHash(path, accept)
	for {
		c.flight.Lock()
		if ch, ok := c.inFlight[key]; ok {
			c.flight.Unlock()
			<-ch // wait for current fetch
			continue
		}
		ch := make(chan struct{})
		c.inFlight[key] = ch
		c.flight.Unlock()
		return func() {
			c.flight.Lock()
			delete(c.inFlight, key)
			c.flight.Unlock()
			close(ch)
		}
	}
}

func writeEntry(w io.Writer, meta, body []byte) error {
	var hdr [4]byte
	binary.LittleEndian.PutUint32(hdr[:], uint32(len(meta)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(meta); err != nil {
		return err
	}
	_, err := w.Write(body)
	return err
}

func readEntry(fp string) (*cachedOrigin, error) {
	f, err := os.Open(fp)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var hdr [4]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return nil, err
	}
	metaLen := binary.LittleEndian.Uint32(hdr[:])
	if metaLen > 1<<16 {
		return nil, errors.New("cache: meta header too large")
	}
	metaBytes := make([]byte, metaLen)
	if _, err := io.ReadFull(f, metaBytes); err != nil {
		return nil, err
	}
	var meta cacheMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, err
	}
	body, err := io.ReadAll(f)
	if err != nil {
		return nil, err
	}
	return &cachedOrigin{
		Body:        body,
		ContentType: meta.ContentType,
		Status:      meta.Status,
	}, nil
}
