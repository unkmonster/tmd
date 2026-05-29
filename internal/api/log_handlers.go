package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"
)

const (
	defaultLogsPageSize = 100
	maxLogsPageSize     = 200
)

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	levelStr := query.Get("level")
	search := query.Get("q")
	pagination := NewPaginationWithDefaults(r, defaultLogsPageSize, maxLogsPageSize, defaultPaginationSort, defaultSortOrder)

	filtered := reverseLogLines(filterLogLines(s.logHub.Snapshot(), levelStr, search))

	total := len(filtered)
	start := pagination.Offset
	end := start + pagination.PageSize
	if start >= total {
		filtered = []string{}
	} else if end > total {
		filtered = filtered[start:]
	} else {
		filtered = filtered[start:end]
	}

	totalPages := 1
	if total > 0 {
		totalPages = (total + pagination.PageSize - 1) / pagination.PageSize
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(LogsResponse{
		Logs:       filtered,
		Total:      total,
		Page:       pagination.Page,
		PageSize:   pagination.PageSize,
		TotalPages: totalPages,
	}))
}

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

func filterLogLines(lines []string, level, search string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if !matchLogFilters(line, level, search) {
			continue
		}
		result = append(result, line)
	}
	return result
}

func reverseLogLines(lines []string) []string {
	reversed := make([]string, len(lines))
	for i := range lines {
		reversed[i] = lines[len(lines)-1-i]
	}
	return reversed
}

func matchLogFilters(line, level, search string) bool {
	if level != "" && level != "all" && !matchLogLevel(line, level) {
		return false
	}
	if search != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(search)) {
		return false
	}
	return true
}

func matchLogLevel(line, level string) bool {
	line = stripAnsiCodes(line)
	level = strings.ToLower(strings.TrimSpace(level))
	if level == "" || level == "all" {
		return true
	}

	lowerLine := strings.ToLower(line)
	if strings.Contains(lowerLine, "level="+level+" ") ||
		strings.Contains(lowerLine, "level="+level+"\n") ||
		strings.Contains(lowerLine, "level="+level+"\t") {
		return true
	}

	return strings.HasPrefix(line, logLevelPrefix(level)+"[")
}

func logLevelPrefix(level string) string {
	switch level {
	case "debug":
		return "DEBU"
	case "info":
		return "INFO"
	case "warn", "warning":
		return "WARN"
	case "error":
		return "ERRO"
	default:
		return strings.ToUpper(level)
	}
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsiCodes(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func writeSSEData(w http.ResponseWriter, line string) error {
	for _, part := range strings.Split(line, "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", strings.TrimSuffix(part, "\r")); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(w, "\n")
	return err
}
