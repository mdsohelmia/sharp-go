package sharp

import (
	"log/slog"

	"github.com/mdsohelmia/sharp-go/internal/vips"
)

// LogLevel mirrors the GLib log-level bits surfaced through libvips.
type LogLevel int

const (
	LogLevelError    LogLevel = LogLevel(vips.GLogLevelError)
	LogLevelCritical LogLevel = LogLevel(vips.GLogLevelCritical)
	LogLevelWarning  LogLevel = LogLevel(vips.GLogLevelWarning)
	LogLevelMessage  LogLevel = LogLevel(vips.GLogLevelMessage)
	LogLevelInfo     LogLevel = LogLevel(vips.GLogLevelInfo)
	LogLevelDebug    LogLevel = LogLevel(vips.GLogLevelDebug)
)

// LogSink receives a single message emitted by libvips or its GLib
// dependencies. domain is the GLib log domain (e.g. "VIPS", "GLib").
type LogSink func(domain string, level LogLevel, message string)

// SetLogSink installs a sink that receives all libvips/GLib log messages.
// Pass nil to restore GLib's default stderr handler.
func SetLogSink(sink LogSink) {
	if sink == nil {
		vips.SetLogSink(nil)
		return
	}
	vips.SetLogSink(func(domain string, level vips.GLogLevel, message string) {
		sink(domain, LogLevel(level), message)
	})
}

// SetSlogSink routes libvips/GLib log messages to slog. Levels are mapped:
//   Error/Critical -> slog.LevelError
//   Warning        -> slog.LevelWarn
//   Message/Info   -> slog.LevelInfo
//   Debug          -> slog.LevelDebug
//
// The GLib log domain is added as a "domain" attribute on each record.
func SetSlogSink(logger *slog.Logger) {
	vips.SetSlogSink(logger)
}
