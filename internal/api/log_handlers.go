package api

import (
	"bytes"
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

const (
	logFileName                 = "tmd2.log"
	defaultLogsPage             = 1
	defaultLogsPageSize         = 100
	maxLogsPageSize             = 200
	logTailLineLimit            = 5000
	logStreamPollInterval       = time.Second
	logTailChunkSize      int64 = 4096
)

func (s *Server) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	levelStr := query.Get("level")
	search := query.Get("q")
	pageStr := query.Get("page")
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = defaultLogsPage
	}

	pageSizeStr := query.Get("pageSize")
	pageSize, err := strconv.Atoi(pageSizeStr)
	if err != nil || pageSize < 1 || pageSize > maxLogsPageSize {
		pageSize = defaultLogsPageSize
	}

	logPath := filepath.Join(s.appRootPath, logFileName)
	lines, err := readLogLinesTail(logPath, logTailLineLimit)
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

type logFollower struct {
	path    string
	file    *os.File
	offset  int64
	pending string
}

func newLogFollower(path string) (*logFollower, error) {
	follower := &logFollower{path: path}
	if err := follower.openAtEnd(); err != nil {
		return nil, err
	}
	return follower, nil
}

func (f *logFollower) Close() error {
	if f.file == nil {
		return nil
	}
	err := f.file.Close()
	f.file = nil
	return err
}

func (f *logFollower) open(offset int64) error {
	if err := f.Close(); err != nil {
		return err
	}

	file, err := os.Open(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			f.offset = 0
			f.pending = ""
			return nil
		}
		return err
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		file.Close()
		return err
	}

	f.file = file
	f.offset = offset
	f.pending = ""
	return nil
}

func (f *logFollower) openAtEnd() error {
	info, err := os.Stat(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return f.open(info.Size())
}

func (f *logFollower) prepare() error {
	pathInfo, err := os.Stat(f.path)
	if err != nil {
		if os.IsNotExist(err) {
			return f.Close()
		}
		return err
	}

	if f.file == nil {
		return f.open(0)
	}

	currentInfo, err := f.file.Stat()
	if err != nil {
		return f.open(0)
	}

	if !os.SameFile(currentInfo, pathInfo) || pathInfo.Size() < f.offset {
		return f.open(0)
	}

	return nil
}

func (f *logFollower) ReadNewLines() ([]string, error) {
	if err := f.prepare(); err != nil {
		return nil, err
	}
	if f.file == nil {
		return nil, nil
	}

	data, err := io.ReadAll(f.file)
	if err != nil {
		return nil, err
	}
	f.offset += int64(len(data))
	if len(data) == 0 {
		return nil, nil
	}

	chunk := f.pending + string(data)
	parts := strings.Split(chunk, "\n")
	if strings.HasSuffix(chunk, "\n") {
		f.pending = ""
	} else {
		f.pending = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	}

	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSuffix(part, "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		lines = append(lines, line)
	}
	return lines, nil
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
	logPath := filepath.Join(s.appRootPath, logFileName)

	follower, err := newLogFollower(logPath)
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, "Failed to open log stream: "+err.Error())
		return
	}
	defer follower.Close()

	ticker := time.NewTicker(logStreamPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			lines, err := follower.ReadNewLines()
			if err != nil {
				continue
			}

			if len(lines) == 0 {
				fmt.Fprint(w, ": ping\n\n")
				flusher.Flush()
				continue
			}

			for _, line := range lines {
				if levelStr != "" && !matchLogLevel(line, levelStr) {
					continue
				}
				line = stripAnsiCodes(line)
				fmt.Fprintf(w, "data: %s\n\n", jsonEscape(line))
				flusher.Flush()
			}
		}
	}
}

func readLogLinesTail(path string, n int) ([]string, error) {
	if n <= 0 {
		return []string{}, nil
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return []string{}, nil
	}

	offset := info.Size()
	buf := make([]byte, 0)
	newlineCount := 0

	for offset > 0 && newlineCount <= n {
		start := offset - logTailChunkSize
		if start < 0 {
			start = 0
		}

		chunk := make([]byte, offset-start)
		if _, err := file.ReadAt(chunk, start); err != nil && err != io.EOF {
			return nil, err
		}
		buf = append(chunk, buf...)
		newlineCount += bytes.Count(chunk, []byte{'\n'})
		offset = start
	}

	parts := bytes.Split(buf, []byte{'\n'})
	if len(parts) > 0 && len(parts[len(parts)-1]) == 0 {
		parts = parts[:len(parts)-1]
	}
	if offset > 0 && len(parts) > 0 {
		parts = parts[1:]
	}
	if len(parts) > n {
		parts = parts[len(parts)-n:]
	}

	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		lines = append(lines, strings.TrimSuffix(string(part), "\r"))
	}
	return lines, nil
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
