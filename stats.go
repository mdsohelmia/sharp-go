package sharp

import (
	"context"

	"github.com/sohelmia/sharp-go/internal/vips"
)

// Stats holds per-channel statistics computed across the full image.
type Stats struct {
	Channels []ChannelStats
}

// ChannelStats are the per-channel values returned by vips_stats.
type ChannelStats struct {
	Min       float64
	Max       float64
	Sum       float64
	SumSquare float64
	Mean      float64
	Deviation float64
}

// Stats computes per-channel statistics. Pixels are decoded fully.
func (im *Image) Stats(ctx context.Context) (Stats, error) {
	if im.err != nil {
		return Stats{}, im.err
	}
	if err := ctx.Err(); err != nil {
		return Stats{}, err
	}
	if err := vips.InitError(); err != nil {
		return Stats{}, err
	}

	buf, err := readInput(im.in)
	if err != nil {
		return Stats{}, err
	}
	vimg, err := vips.LoadBuffer(buf)
	if err != nil {
		return Stats{}, err
	}

	bands, err := vips.Stats(vimg)
	if err != nil {
		return Stats{}, err
	}

	res := Stats{Channels: make([]ChannelStats, len(bands))}
	for i, b := range bands {
		res.Channels[i] = ChannelStats(b)
	}
	return res, nil
}
