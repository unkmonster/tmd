package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	defaultLogsPageSize = 100
	maxLogsPageSize     = 200
)

// logrus 时间格式: time="2024-06-04T10:00:00+08:00"
var logTimeRegex = regexp.MustCompile(`time="([^"]+)"`)

func parseLogTime(line string) (time.Time, bool) {
	m := logTimeRegex.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02T15:04:05-07:00", m[1])
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

func parseFilterTime(s string) (time.Time, bool) {
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	levelStr := query.Get("level")
	search := query.Get("q")
	startStr := query.Get("start_time")
	endStr := query.Get("end_time")

	var startTime, endTime time.Time
	hasStart := false
	hasEnd := false
	if t, ok := parseFilterTime(startStr); ok {
		startTime = t
		hasStart = true
	}
	if t, ok := parseFilterTime(endStr); ok {
		endTime = t
		hasEnd = true
	}

	pagination := NewPaginationWithDefaults(r, defaultLogsPageSize, maxLogsPageSize, defaultPaginationSort, defaultSortOrder)

	filtered := filterLogLinesReverse(s.logHub.Snapshot(), levelStr, search, hasStart, startTime, hasEnd, endTime)

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

	resp := pagination.ToResponse(filtered, total)

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(LogsResponse{
		Logs:       filtered,
		Total:      resp.Total,
		Page:       resp.Page,
		PageSize:   resp.PageSize,
		TotalPages: resp.TotalPages,
	}))
}

func (s *Server) handleLogStats(w http.ResponseWriter, r *http.Request) {
	lines := s.logHub.Snapshot()
	stats := map[string]int{"debug": 0, "info": 0, "warn": 0, "error": 0, "total": len(lines)}
	for _, line := range lines {
		switch {
		case matchLogLevel(line, "error"):
			stats["error"]++
		case matchLogLevel(line, "warn"):
			stats["warn"]++
		case matchLogLevel(line, "debug"):
			stats["debug"]++
		default:
			stats["info"]++
		}
	}
	s.writeJSON(w, http.StatusOK, NewSuccessResponse(stats))
}

func (s *Server) handleLogExport(w http.ResponseWriter, r *http.Request) {
	logPath := filepath.Join(s.appRootPath, "tmd2.log")
	data, err := os.ReadFile(logPath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to read log file: "+err.Error())
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="tmd2-%s.log"`, time.Now().Format("20060102-150405")))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// filterLogLinesReverse 从尾向头遍历（天然逆序），过滤并直接产出结果，一次遍历完成。
func filterLogLinesReverse(lines []string, level, search string, hasStart bool, startTime time.Time, hasEnd bool, endTime time.Time) []string {
	result := make([]string, 0, len(lines))
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if !matchLogFilters(line, level, search) {
			continue
		}
		if hasStart || hasEnd {
			t, ok := parseLogTime(line)
			if ok {
				if hasStart && t.Before(startTime) {
					continue
				}
				if hasEnd && t.After(endTime) {
					continue
				}
			}
		}
		result = append(result, line)
	}
	return result
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


