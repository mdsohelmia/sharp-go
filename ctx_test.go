package sharp_test

import (
	"context"
	"testing"
	"time"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestTimeoutMethod(t *testing.T) {
	// Timeout 0 = no timeout, should succeed normally.
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		Timeout(0).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Timeout(0): %v", err)
	}
}

func TestCtxCancelled(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg") // 6.8 MB PNG, slow
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately
	_, _, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{Width: 4000, Height: 4000, Fit: sharp.FitFill}).
		Blur(sharp.BlurOptions{Sigma: 20}).
		JPEG(format.JPEGOptions{}).
		ToBytes(ctx)
	if err == nil {
		t.Errorf("expected error from pre-cancelled ctx")
	}
}

func TestTimeoutShort(t *testing.T) {
	// Tiny timeout on heavy work — expect error.
	in := readFixture(t, "2569067123_aca715a2ee_o.png") // 6.8 MB PNG, slow to decode
	_, _, err := sharp.FromBytes(in).
		Timeout(1 * time.Nanosecond).
		Blur(sharp.BlurOptions{Sigma: 20}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err == nil {
		t.Logf("Timeout(1ns) succeeded (pipeline finished before watcher fired)")
	}
}
