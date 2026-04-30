package api

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	query := r.URL.Query()
	levelStr := query.Get("level")
	search := query.Get("q")
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	pageSizeStr := query.Get("pageSize")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > 200 {
		pageSize = 100
	}

	logPath := filepath.Join(s.appRootPath, "tmd2.log")
	lines, err := readLogLinesTail(logPath, 5000)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to read logs: "+err.Error())
		return
	}

	filtered := filterLogLines(lines, levelStr, search)

	total := len(filtered)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= total {
		filtered = []string{}
	} else if end > total {
		filtered = filtered[start:]
	} else {
		filtered = filtered[start:end]
	}

	s.writeJSON(w, http.StatusOK, NewSuccessResponse(LogsResponse{
		Logs:       filtered,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: (total + pageSize - 1) / pageSize,
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
	ctx := r.Context()
	logPath := filepath.Join(s.appRootPath, "tmd2.log")

	var lastOffset int64 = 0
	if fi, err := os.Stat(logPath); err == nil {
		lastOffset = fi.Size()
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fi, err := os.Stat(logPath)
			if err != nil {
				continue
			}

			currentSize := fi.Size()

			if currentSize < lastOffset {
				lastOffset = 0
				continue
			}

			if currentSize == lastOffset {
				fmt.Fprint(w, ": ping\n\n")
				flusher.Flush()
				continue
			}

			file, err := os.Open(logPath)
			if err != nil {
				continue
			}

			_, err = file.Seek(lastOffset, io.SeekStart)
			if err != nil {
				file.Close()
				continue
			}

			reader := bufio.NewReader(file)
			scanner := bufio.NewScanner(reader)
			scanner.Buffer(make([]byte, 64*1024), 1024*1024)

			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" {
					continue
				}
				if levelStr != "" && !matchLogLevel(line, levelStr) {
					continue
				}
				line = stripAnsiCodes(line)
				fmt.Fprintf(w, "data: %s\n\n", jsonEscape(line))
				flusher.Flush()
			}

			newOffset, _ := file.Seek(0, io.SeekCurrent)
			file.Close()

			if newOffset > lastOffset {
				lastOffset = newOffset
			}
		}
	}
}

func readLogLinesTail(path string, n int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[len(lines)-n:]
		}
	}
	return lines, scanner.Err()
}

func filterLogLines(lines []string, level, search string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if level != "" && !matchLogLevel(line, level) {
			continue
		}
		if search != "" && !strings.Contains(strings.ToLower(line), strings.ToLower(search)) {
			continue
		}
		result = append(result, line)
	}
	return result
}

func matchLogLevel(line, level string) bool {
	target := "level=" + level
	return strings.Contains(line, target+" ") ||
		strings.Contains(line, target+"\n") ||
		strings.Contains(line, target+"\t")
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func stripAnsiCodes(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

func jsonEscape(s string) string {
	b, err := json.Marshal(s)
	if err != nil {
		return s
	}
	return string(b[1 : len(b)-1])
}
