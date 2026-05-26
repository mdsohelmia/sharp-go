//go:build cgo

package vips

/*
#include "bridge.h"
*/
import "C"

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
)

// GLogLevel mirrors GLib's log-level bitset enough to map to slog levels.
type GLogLevel int

const (
	GLogLevelError    GLogLevel = 1 << 2
	GLogLevelCritical GLogLevel = 1 << 3
	GLogLevelWarning  GLogLevel = 1 << 4
	GLogLevelMessage  GLogLevel = 1 << 5
	GLogLevelInfo     GLogLevel = 1 << 6
	GLogLevelDebug    GLogLevel = 1 << 7
)

// LogSink receives a single log entry from libvips/GLib.
type LogSink func(domain string, level GLogLevel, message string)

var (
	logMu   sync.RWMutex
	logSink LogSink
	logOn   atomic.Bool
)

// SetLogSink installs sink as the receiver for libvips/GLib log messages.
// Passing nil restores GLib's default handler (stderr output). Safe to call
// from any goroutine.
func SetLogSink(sink LogSink) {
	logMu.Lock()
	defer logMu.Unlock()
	if sink == nil {
		if logOn.Load() {
			C.sharpgo_uninstall_log_handler()
			logOn.Store(false)
		}
		logSink = nil
		return
	}
	logSink = sink
	if !logOn.Load() {
		C.sharpgo_install_log_handler()
		logOn.Store(true)
	}
}

// SetSlogSink routes log messages to slog at appropriate levels.
func SetSlogSink(logger *slog.Logger) {
	if logger == nil {
		SetLogSink(nil)
		return
	}
	SetLogSink(func(domain string, level GLogLevel, message string) {
		l := slog.LevelInfo
		switch {
		case level&(GLogLevelError|GLogLevelCritical) != 0:
			l = slog.LevelError
		case level&GLogLevelWarning != 0:
			l = slog.LevelWarn
		case level&GLogLevelDebug != 0:
			l = slog.LevelDebug
		}
		logger.Log(context.Background(), l, message, "domain", domain)
	})
}

//export sharpgoLogTrampoline
func sharpgoLogTrampoline(domain *C.char, level C.int, message *C.char) {
	logMu.RLock()
	sink := logSink
	logMu.RUnlock()
	if sink == nil {
		return
	}
	sink(C.GoString(domain), GLogLevel(level), C.GoString(message))
}
