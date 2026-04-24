package server

import (
	"bufio"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"
)

// requestLogMiddleware emits one log line per HTTP request with method, path,
// status and duration. WebSocket upgrades are logged at Info on completion of
// the dial (status 101); inside handlers we prefer explicit slog calls for the
// long-lived connection events themselves.
func requestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		// Noise filter: /api/health is polled constantly by Electron while the
		// core starts up — demote to Debug to keep the default log readable.
		level := slog.LevelInfo
		if r.URL.Path == "/api/health" {
			level = slog.LevelDebug
		}

		slog.Default().Log(r.Context(), level, "http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", clientIP(r),
		)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Hijack passes through so the gorilla websocket upgrader still works.
func (r *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("http.Hijacker not supported")
}

func clientIP(r *http.Request) string {
	if addr := r.RemoteAddr; addr != "" {
		if i := strings.LastIndex(addr, ":"); i >= 0 {
			return addr[:i]
		}
		return addr
	}
	return ""
}
