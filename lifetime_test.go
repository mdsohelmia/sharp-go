package sharp_test

import (
	"bytes"
	"context"
	"io"
	"runtime"
	"sync"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

// readOnly hides the io.Seeker that *bytes.Reader implements, forcing the
// non-seekable streaming-source code path.
type readOnly struct{ r io.Reader }

func (ro readOnly) Read(p []byte) (int, error) { return ro.r.Read(p) }

// TestReaderInputResize exercises the streaming-source load + resize path.
// Before the source refactor, a fused thumbnail over a reader read lazily at
// encode time — after the input had been released. The input is now fully
// decoded within the load call, so resize-from-reader must succeed.
func TestReaderInputResize(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	r := readOnly{r: bytes.NewReader(in)}

	out, info, err := sharp.FromReader(r).
		Resize(sharp.ResizeOptions{Width: 100, Height: 75, Fit: sharp.FitFill}).
		JPEG(format.JPEGOptions{Quality: 80}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("empty output")
	}
	if info.Width != 100 || info.Height != 75 {
		t.Fatalf("dims = %dx%d, want 100x75", info.Width, info.Height)
	}
}

// TestFileInputResizeWithGC drives the buffer-fused-via-source path (FromFile
// reads bytes that are NOT retained by the Image). A GC between load and
// encode would surface the old use-after-free if the input weren't pinned to
// the image lifetime by the source machinery.
func TestFileInputResizeWithGC(t *testing.T) {
	path := fixturePath(t, "320x240.jpg")

	im := sharp.FromFile(path).
		Resize(sharp.ResizeOptions{Width: 80}).
		WebP(format.WebPOptions{Quality: 75})

	// Force a GC cycle so any unreachable input buffer would be collected
	// before the encode reads pixels from the (lazy) fused pipeline.
	runtime.GC()

	out, _, err := im.ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ToBytes: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("empty output")
	}
}

// TestConcurrentResizeNoRace runs many independent pipelines at once. Combined
// with `go test -race` it guards the cgo.Handle source and the thread-local
// error-buffer handling (LockOSThread) against data races.
func TestConcurrentResizeNoRace(t *testing.T) {
	in := readFixture(t, "320x240.jpg")

	const n = 16
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Distinct backing buffer per goroutine.
			buf := append([]byte(nil), in...)
			_, _, err := sharp.FromBytes(buf).
				Resize(sharp.ResizeOptions{Width: 120, Height: 90, Fit: sharp.FitFill}).
				WebP(format.WebPOptions{Quality: 70}).
				ToBytes(context.Background())
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent ToBytes: %v", err)
		}
	}
}

// TestCanceledContextErrors is a smoke test that a canceled context is honored
// by a terminal call rather than ignored or hanging.
func TestCanceledContextErrors(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, _, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 100}).
		JPEG(format.JPEGOptions{Quality: 80}).
		ToBytes(ctx)
	if err == nil {
		t.Fatal("expected error from canceled context, got nil")
	}
}
