// Package logging wires up the core's structured logger.
//
// Logs are emitted as JSON lines to ~/.biene/logs/core-YYYYMMDD.log and, in
// parallel, as a human-readable text stream to stderr (which Electron's main
// process captures). Level is controlled by BIENE_LOG_LEVEL
// (debug|info|warn|error, default info). The file is opened in append mode so
// multiple runs on the same day share one file.
package logging

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"biene/internal/bienehome"
)

const logsDirName = "logs"

var (
	initOnce sync.Once
	logFile  *os.File
)

// Init configures slog.Default() and returns the path of the log file (or
// an empty string if file logging could not be set up). Safe to call
// multiple times — only the first call takes effect.
func Init() string {
	var path string
	initOnce.Do(func() {
		path = initOnce0()
	})
	return path
}

func initOnce0() string {
	level := parseLevel(os.Getenv("BIENE_LOG_LEVEL"))

	handlers := []slog.Handler{
		slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}),
	}

	logPath, err := openLogFile(time.Now())
	if err != nil {
		fmt.Fprintf(os.Stderr, "logging: file handler disabled: %v\n", err)
	} else {
		handlers = append(handlers, slog.NewJSONHandler(logFile, &slog.HandlerOptions{Level: level}))
	}

	slog.SetDefault(slog.New(newFanout(handlers...)))
	return logPath
}

// Close flushes and closes the log file. Intended for graceful shutdown.
func Close() {
	if logFile != nil {
		_ = logFile.Sync()
		_ = logFile.Close()
		logFile = nil
	}
}

// Session returns a logger pre-bound to a session id.
func Session(id string) *slog.Logger {
	return slog.Default().With("session_id", id)
}

func openLogFile(now time.Time) (string, error) {
	home, err := bienehome.HomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, logsDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("core-%s.log", now.Format("20060102"))
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	logFile = f
	return path, nil
}

func parseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// fanout dispatches every record to all wrapped handlers. It lets us tee
// formatted text to stderr and JSON to a file from a single logger.
type fanout struct {
	handlers []slog.Handler
}

func newFanout(hs ...slog.Handler) slog.Handler {
	return &fanout{handlers: hs}
}

func (f *fanout) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (f *fanout) Handle(ctx context.Context, r slog.Record) error {
	var firstErr error
	for _, h := range f.handlers {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if err := h.Handle(ctx, r.Clone()); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (f *fanout) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithAttrs(attrs)
	}
	return &fanout{handlers: next}
}

func (f *fanout) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		next[i] = h.WithGroup(name)
	}
	return &fanout{handlers: next}
}
