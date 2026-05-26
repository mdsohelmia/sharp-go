package sharp_test

import (
	"context"
	"testing"

	sharp "github.com/mdsohelmia/sharp-go"
	"github.com/mdsohelmia/sharp-go/format"
)

func TestResizeCoverEdgeGravities(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg") // 2725x2225
	positions := []sharp.Position{
		sharp.PositionNorth,
		sharp.PositionNorthEast,
		sharp.PositionEast,
		sharp.PositionSouthEast,
		sharp.PositionSouth,
		sharp.PositionSouthWest,
		sharp.PositionWest,
		sharp.PositionNorthWest,
	}
	for _, pos := range positions {
		_, info, err := sharp.FromBytes(in).
			Resize(sharp.ResizeOptions{
				Width: 200, Height: 200,
				Fit:      sharp.FitCover,
				Position: pos,
			}).
			JPEG(format.JPEGOptions{}).
			ToBytes(context.Background())
		if err != nil {
			t.Errorf("Position %v: %v", pos, err)
			continue
		}
		if info.Width != 200 || info.Height != 200 {
			t.Errorf("Position %v: dims = %dx%d, want 200x200", pos, info.Width, info.Height)
		}
	}
}

func TestResizeCoverNonSquareEdgeGravities(t *testing.T) {
	in := readFixture(t, "2569067123_aca715a2ee_o.jpg")
	_, info, err := sharp.FromBytes(in).
		Resize(sharp.ResizeOptions{
			Width: 300, Height: 100,
			Fit:      sharp.FitCover,
			Position: sharp.PositionNorth,
		}).
		JPEG(format.JPEGOptions{}).
		ToBytes(context.Background())
	if err != nil {
		t.Fatalf("North gravity: %v", err)
	}
	if info.Width != 300 || info.Height != 100 {
		t.Errorf("dims = %dx%d, want 300x100", info.Width, info.Height)
	}
}
