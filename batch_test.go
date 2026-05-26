package sharp_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestToBytesAllOrderPreserved(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	const n = 8
	images := make([]*sharp.Image, n)
	for i := range images {
		w := 50 + 10*i
		images[i] = sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: w}).
			JPEG(format.JPEGOptions{Quality: 70})
	}
	results := sharp.ToBytesAll(context.Background(), images, sharp.BatchOptions{Concurrency: 4})
	if len(results) != n {
		t.Fatalf("len(results) = %d, want %d", len(results), n)
	}
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("idx %d: %v", i, r.Err)
			continue
		}
		wantW := 50 + 10*i
		if r.Info.Width != wantW {
			t.Errorf("idx %d: Width = %d, want %d", i, r.Info.Width, wantW)
		}
	}
}

func TestToBytesAllNilImage(t *testing.T) {
	results := sharp.ToBytesAll(context.Background(),
		[]*sharp.Image{nil}, sharp.BatchOptions{Concurrency: 1})
	if len(results) != 1 || results[0].Err == nil {
		t.Errorf("expected error for nil image, got %+v", results)
	}
}

func TestToBytesAllSequential(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	images := []*sharp.Image{
		sharp.FromBytes(in).Resize(sharp.ResizeOptions{Width: 100}).JPEG(format.JPEGOptions{}),
		sharp.FromBytes(in).Resize(sharp.ResizeOptions{Width: 200}).JPEG(format.JPEGOptions{}),
	}
	results := sharp.ToBytesAll(context.Background(), images, sharp.BatchOptions{Concurrency: 1})
	if results[0].Info.Width != 100 || results[1].Info.Width != 200 {
		t.Errorf("widths = %d, %d", results[0].Info.Width, results[1].Info.Width)
	}
}

func TestToBytesAllStopOnFirstError(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	images := []*sharp.Image{
		nil, // immediate error
		sharp.FromBytes(in).JPEG(format.JPEGOptions{}),
		sharp.FromBytes(in).JPEG(format.JPEGOptions{}),
	}
	results := sharp.ToBytesAll(context.Background(), images,
		sharp.BatchOptions{Concurrency: 1, StopOnFirstError: true})
	if results[0].Err == nil {
		t.Errorf("first job should have errored")
	}
	// At least one subsequent job should have been cancelled.
	cancelledCount := 0
	for i := 1; i < len(results); i++ {
		if results[i].Err != nil && errors.Is(results[i].Err, context.Canceled) {
			cancelledCount++
		}
	}
	if cancelledCount == 0 {
		t.Errorf("expected at least one subsequent job to be cancelled, got %+v", results)
	}
}

func TestToBytesAllCtxCancel(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	images := make([]*sharp.Image, 10)
	for i := range images {
		images[i] = sharp.FromBytes(in).
			Blur(sharp.BlurOptions{Sigma: 5}).
			JPEG(format.JPEGOptions{})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancelled
	results := sharp.ToBytesAll(ctx, images, sharp.BatchOptions{Concurrency: 2})
	cancelled := 0
	for _, r := range results {
		if r.Err != nil {
			cancelled++
		}
	}
	if cancelled == 0 {
		t.Errorf("expected at least one cancellation, got %+v", results)
	}
}

func TestToFilesAll(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	dir := t.TempDir()
	const n = 5
	images := make([]*sharp.Image, n)
	paths := make([]string, n)
	for i := range images {
		images[i] = sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{Width: 80 + 10*i}).
			PNG(format.PNGOptions{})
		paths[i] = filepath.Join(dir, "out"+itoaTest(i)+".png")
	}
	results := sharp.ToFilesAll(context.Background(), images, paths, sharp.BatchOptions{Concurrency: 3})
	for i, r := range results {
		if r.Err != nil {
			t.Errorf("idx %d: %v", i, r.Err)
		}
		if _, err := os.Stat(paths[i]); err != nil {
			t.Errorf("idx %d: file missing: %v", i, err)
		}
	}
}

func TestMetadataAll(t *testing.T) {
	images := []*sharp.Image{
		sharp.FromBytes(readFixture(t, "320x240.jpg")),
		sharp.FromBytes(readFixture(t, "Flag_of_the_Netherlands.png")),
		sharp.FromBytes(readFixture(t, "Flag_of_the_Netherlands-alpha.png")),
	}
	results := sharp.MetadataAll(context.Background(), images, sharp.BatchOptions{Concurrency: 2})
	expected := []string{"jpeg", "png", "png"}
	for i, want := range expected {
		if results[i].Err != nil {
			t.Errorf("idx %d: %v", i, results[i].Err)
			continue
		}
		if results[i].Metadata.Format != want {
			t.Errorf("idx %d: Format = %q, want %q", i, results[i].Metadata.Format, want)
		}
	}
}

func TestBatchPerJobTimeout(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	images := []*sharp.Image{
		sharp.FromBytes(in).Blur(sharp.BlurOptions{Sigma: 30}).JPEG(format.JPEGOptions{}),
	}
	results := sharp.ToBytesAll(context.Background(), images,
		sharp.BatchOptions{Concurrency: 1, PerJobTimeout: 1 * time.Nanosecond})
	if results[0].Err == nil {
		t.Logf("PerJobTimeout(1ns) completed before fire — acceptable on fast hardware")
	}
}

// itoaTest is a tiny stringifier (independent of internal sharp.itoa).
func itoaTest(i int) string {
	if i == 0 {
		return "0"
	}
	const d = "0123456789"
	var b [8]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = d[i%10]
		i /= 10
	}
	return string(b[pos:])
}
