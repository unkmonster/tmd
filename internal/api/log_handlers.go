package api

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
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
	disableSSEWriteTimeout(w)

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

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case <-ctx.Done():
			return
		case line := <-ch:
			if line == "" {
				continue
			}
			if !matchLogFilters(line, levelStr, search) {
				continue
			}
			writeSSEData(w, line)
			flusher.Flush()
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

func writeSSEData(w http.ResponseWriter, line string) {
	for _, part := range strings.Split(line, "\n") {
		fmt.Fprintf(w, "data: %s\n", strings.TrimSuffix(part, "\r"))
	}
	fmt.Fprint(w, "\n")
}
