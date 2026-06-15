package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	levelStr := r.URL.Query().Get("level")
	if levelStr != "" && !isValidLogLevel(levelStr) {
		s.writeError(w, http.StatusBadRequest, "Invalid log level: "+levelStr)
		return
	}
	search := r.URL.Query().Get("q")
	ctx := r.Context()
	ch, unsubscribe := s.logHub.Subscribe()
	defer unsubscribe()

	if err := writeSSEFrame(w, flusher, func() error {
		_, err := fmt.Fprint(w, ": connected\n\n")
		return err
	}); err != nil {
		return
	}
	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			if err := writeSSEFrame(w, flusher, func() error {
				return writeSSEHeartbeat(w)
			}); err != nil {
				return
			}
		case line := <-ch:
			if line == "" {
				continue
			}
			if !matchLogFilters(line, levelStr, search) {
				continue
			}
			if err := writeSSEFrame(w, flusher, func() error {
				return writeSSEData(w, line)
			}); err != nil {
				return
			}
		}
	}
}

func writeSSEData(w http.ResponseWriter, line string) error {
	if _, err := fmt.Fprint(w, "event: log\n"); err != nil {
		return err
	}
	for _, part := range strings.Split(line, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", strings.TrimSuffix(part, "\r")); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(w, "\n")
	return err
}
