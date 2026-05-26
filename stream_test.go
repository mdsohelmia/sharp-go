package sharp_test

import (
	"bytes"
	"context"
	"io"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func TestStreamJPEG(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	var buf bytes.Buffer
	info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 160, Height: 120}).
		JPEG(format.JPEGOptions{Quality: 70}).
		ToWriter(context.Background(), &buf)
	if err != nil {
		t.Fatalf("ToWriter: %v", err)
	}
	if info.Format != "jpeg" {
		t.Errorf("Format = %q, want jpeg", info.Format)
	}
	if info.Width != 160 || info.Height != 120 {
		t.Errorf("dims = %dx%d, want 160x120", info.Width, info.Height)
	}
	got := buf.Bytes()
	if len(got) < 100 {
		t.Errorf("output suspiciously small: %d bytes", len(got))
	}
	// JPEG SOI marker
	if got[0] != 0xFF || got[1] != 0xD8 {
		t.Errorf("not JPEG: %x", got[:2])
	}
}

func TestStreamPNG(t *testing.T) {
	in := readFixture(t, "Flag_of_the_Netherlands.png")
	var buf bytes.Buffer
	info, err := sharp.FromBytes(in).
		PNG(format.PNGOptions{}).
		ToWriter(context.Background(), &buf)
	if err != nil {
		t.Fatalf("ToWriter: %v", err)
	}
	if info.Format != "png" {
		t.Errorf("Format = %q, want png", info.Format)
	}
	got := buf.Bytes()
	want := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	for i, b := range want {
		if got[i] != b {
			t.Errorf("not PNG (signature mismatch at %d)", i)
			break
		}
	}
}

// errWriter fails after writing some bytes — verifies stream error handling.
type errWriter struct {
	limit int
	n     int
}

func (e *errWriter) Write(p []byte) (int, error) {
	if e.n+len(p) > e.limit {
		room := e.limit - e.n
		if room > 0 {
			e.n += room
			return room, io.ErrShortWrite
		}
		return 0, io.ErrShortWrite
	}
	e.n += len(p)
	return len(p), nil
}

func TestFromReaderStreaming(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	r := bytes.NewReader(in)
	out, info, err := sharp.FromReader(r).
		Resize(sharp.ResizeOptions{Width: 400, Height: 300}).
		JPEG(format.JPEGOptions{Quality: 80}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes via reader: %v", err)
	}
	if info.Width != 400 || info.Height != 300 {
		t.Errorf("dims = %dx%d, want 400x300", info.Width, info.Height)
	}
	if len(out) < 100 || out[0] != 0xFF || out[1] != 0xD8 {
		t.Errorf("output not a JPEG: %d bytes head=%x", len(out), out[:min(2, len(out))])
	}
}

func TestFromReaderNoResize(t *testing.T) {
	// Non-fused streaming path: no resize → LoadSource + full decode.
	in := readFixture(t, "320x240.jpg")
	r := bytes.NewReader(in)
	_, info, err := sharp.FromReader(r).
		JPEG(format.JPEGOptions{Quality: 75}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes via reader (no resize): %v", err)
	}
	if info.Width != 320 || info.Height != 240 {
		t.Errorf("dims = %dx%d, want 320x240", info.Width, info.Height)
	}
}

func TestStreamWriterError(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	w := &errWriter{limit: 100}
	_, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 400, Height: 300}).
		JPEG(format.JPEGOptions{}).
		ToWriter(context.Background(), w)
	if err == nil {
		t.Errorf("expected error from short writer")
	}
}
