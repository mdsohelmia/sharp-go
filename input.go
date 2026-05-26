package sharp

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"

	"github.com/sohelmia/sharp-go/format"
)

// FromBytes creates a new Image whose input is the given byte slice. The
// slice is referenced until the pipeline executes; the caller should not
// mutate it in the interim.
func FromBytes(buf []byte) *Image {
	im := acquireImage()
	if len(buf) == 0 {
		im.stickyErr(errors.New("sharp: empty input buffer"))
		return im
	}
	im.in.bytes = buf
	return im
}

// FromRawBytes wraps an in-memory pixel buffer with the given layout. Mirrors
// sharp(buf, { raw: { width, height, channels } }).
func FromRawBytes(buf []byte, layout format.RawInput) *Image {
	im := acquireImage()
	if len(buf) == 0 {
		im.stickyErr(errors.New("sharp: empty raw input buffer"))
		return im
	}
	if layout.Width <= 0 || layout.Height <= 0 || layout.Channels <= 0 {
		im.stickyErr(errors.New("sharp: FromRawBytes requires positive Width, Height, Channels"))
		return im
	}
	im.in.bytes = buf
	r := layout
	im.in.raw = &r
	return im
}

// FromFile creates a new Image whose input is the file at path. The file is
// read fully into memory at terminal-call time.
func FromFile(path string) *Image {
	im := acquireImage()
	if path == "" {
		im.stickyErr(errors.New("sharp: empty input path"))
		return im
	}
	im.in.path = path
	return im
}

// Animated configures the input to load all pages/frames of a multi-page
// image (animated GIF/WebP/HEIF/TIFF). Equivalent to Pages(-1).
func (im *Image) Animated() *Image {
	if im.err != nil {
		return im
	}
	im.in.pages = -1
	return im
}

// Pages sets the number of pages to load from a multi-page input. -1 loads
// all pages. Default (0) loads only the first page.
func (im *Image) Pages(n int) *Image {
	if im.err != nil {
		return im
	}
	im.in.pages = n
	return im
}

// Page sets the starting page index (0-based) for multi-page input.
func (im *Image) Page(idx int) *Image {
	if im.err != nil {
		return im
	}
	im.in.page = idx
	return im
}

// FromReader records r as a streaming input. Unlike earlier versions, this
// no longer reads the stream eagerly — libvips pulls bytes via a custom
// source at pipeline execution time. The reader is consumed once; pair with
// FromReadSeeker for formats whose decoder needs to rewind (TIFF, multi-page
// HEIF). Errors from r surface at the terminal call (ToBytes/ToFile/...).
func FromReader(r io.Reader) *Image {
	im := acquireImage()
	if r == nil {
		im.stickyErr(errors.New("sharp: nil reader"))
		return im
	}
	im.in.reader = r
	return im
}

// FromReadSeeker is FromReader for seekable streams (os.File, bytes.Reader).
// Seekability lets libvips handle decoders that probe / rewind the source.
func FromReadSeeker(rs io.ReadSeeker) *Image {
	im := acquireImage()
	if rs == nil {
		im.stickyErr(errors.New("sharp: nil reader"))
		return im
	}
	im.in.reader = rs
	return im
}

// FromURL fetches url with client and records the response body as a
// streaming input. The body is closed when the pipeline terminates. If
// client is nil, http.DefaultClient is used. Non-2xx responses produce a
// sticky error.
func FromURL(ctx context.Context, url string, client *http.Client) *Image {
	im := acquireImage()
	if url == "" {
		im.stickyErr(errors.New("sharp: empty url"))
		return im
	}
	if client == nil {
		client = http.DefaultClient
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		im.stickyErr(err)
		return im
	}
	resp, err := client.Do(req)
	if err != nil {
		im.stickyErr(err)
		return im
	}
	if resp.StatusCode/100 != 2 {
		_ = resp.Body.Close()
		im.stickyErr(errors.New("sharp: " + resp.Status))
		return im
	}
	im.in.reader = resp.Body
	im.in.closer = resp.Body
	return im
}

// readInput resolves the recorded input to a byte slice. Synth-backed
// inputs return nil bytes — caller checks inputSource.synth. Reader inputs
// are drained via io.ReadAll as a buffering fallback for code paths that
// need bytes (Metadata, Stats, composite InputPath); the streaming entry in
// buildPipelineImage routes the reader path before calling this.
func readInput(in inputSource) ([]byte, error) {
	switch {
	case in.synth != nil:
		return nil, nil
	case in.bytes != nil:
		return in.bytes, nil
	case in.path != "":
		return os.ReadFile(in.path)
	case in.reader != nil:
		return io.ReadAll(in.reader)
	default:
		return nil, errors.New("sharp: no input source")
	}
}
