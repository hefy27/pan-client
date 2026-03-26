package internal

import (
	"log/slog"
	"sync/atomic"
)

var logPtr atomic.Pointer[slog.Logger]

func init() {
	logPtr.Store(slog.Default())
}

// GetLogger returns the current global logger (thread-safe).
func GetLogger() *slog.Logger {
	return logPtr.Load()
}

// SetLogger replaces the global logger (thread-safe).
func SetLogger(l *slog.Logger) {
	if l != nil {
		logPtr.Store(l)
	}
}
