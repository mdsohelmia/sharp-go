package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/sohelmia/sharp-go"
	"github.com/sohelmia/sharp-go/format"
)

func TestExtractChannel(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		ExtractChannel(1). // green
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("ExtractChannel: %v", err)
	}
	if info.Channels != 1 {
		t.Errorf("Channels = %d, want 1", info.Channels)
	}
}

func TestJoinChannel(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	mask := readFixture(t, "Flag_of_the_Netherlands.png")
	// JoinChannel requires same dimensions; resize mask to match.
	maskResized, _, err := sharp.FromBytes(mask).
		Resize(sharp.ResizeOptions{Width: 320, Height: 240, Fit: sharp.FitFill}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("prep mask: %v", err)
	}
	_, info, err := sharp.FromBytes(in).
		JoinChannel(sharp.JoinChannelOptions{Inputs: [][]byte{maskResized}}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("JoinChannel: %v", err)
	}
	if info.Channels < 4 {
		t.Errorf("Channels = %d, want >= 4", info.Channels)
	}
}

func TestBandbool(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, info, err := sharp.FromBytes(in).
		Threshold(sharp.ThresholdOptions{Value: 128}).
		Bandbool(sharp.BandboolOptions{Op: sharp.BooleanOr}).
		PNG(format.PNGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("Bandbool: %v", err)
	}
	if info.Channels != 1 {
		t.Errorf("Channels = %d, want 1", info.Channels)
	}
}

func TestPipelineColourspace(t *testing.T) {
	in := readFixture(t, "320x240.jpg")
	_, _, err := sharp.FromBytes(in).
		PipelineColourspace(sharp.ColourspaceLAB).
		Blur(sharp.BlurOptions{Sigma: 1.0}).
		ToColourspace(sharp.ColourspaceSRGB).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("PipelineColourspace: %v", err)
	}
}
