//go:build !cgo

package vips

import "errors"

// InitError reports that cgo was disabled at build time.
func InitError() error {
	return errors.New("sharp-go requires cgo (CGO_ENABLED=1) and a system libvips installation")
}

func Version() (int, int, int)   { return 0, 0, 0 }
func VersionString() string      { return "0.0.0" }
func SetConcurrency(int)         {}
func Concurrency() int           { return 0 }
func SetCache(int, int, uint64)  {}
