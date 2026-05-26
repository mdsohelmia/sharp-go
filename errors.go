package sharp

import "errors"

var (
	ErrUnsupportedInput   = errors.New("sharp: unsupported input")
	ErrTimeout            = errors.New("sharp: operation timed out")
	ErrFeatureUnavailable = errors.New("sharp: feature not available in this libvips build")
)
