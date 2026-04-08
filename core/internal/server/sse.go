package server

import (
	"fmt"
	"net/http"

	"biene/internal/session"
)

// writeSSE writes one SSE frame to w and flushes if possible.
func writeSSE(w http.ResponseWriter, frame session.Frame) {
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", frame.EventType, frame.Data)
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}
