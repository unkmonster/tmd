package consolelog

import (
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
)

const DefaultLimit = 5000

var (
	defaultHub = NewHub(DefaultLimit)

	captureOnce sync.Once
	captureErr  error
	ansiRegex   = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
)

type Hub struct {
	mu          sync.Mutex
	lines       []string
	limit       int
	subscribers map[chan string]struct{}
}

func DefaultHub() *Hub {
	return defaultHub
}

func NewHub(limit int) *Hub {
	if limit <= 0 {
		limit = DefaultLimit
	}

	return &Hub{
		limit:       limit,
		subscribers: make(map[chan string]struct{}),
	}
}

func (h *Hub) Add(line string) {
	line = strings.TrimSpace(stripANSI(line))
	if line == "" {
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	h.lines = append(h.lines, line)
	if len(h.lines) > h.limit {
		copy(h.lines, h.lines[len(h.lines)-h.limit:])
		h.lines = h.lines[:h.limit]
	}

	for ch := range h.subscribers {
		select {
		case ch <- line:
		default:
		}
	}
}

func (h *Hub) Snapshot() []string {
	h.mu.Lock()
	defer h.mu.Unlock()

	lines := make([]string, len(h.lines))
	copy(lines, h.lines)
	return lines
}

func (h *Hub) Subscribe() (<-chan string, func()) {
	ch := make(chan string, 100)

	h.mu.Lock()
	h.subscribers[ch] = struct{}{}
	h.mu.Unlock()

	return ch, func() {
		h.mu.Lock()
		delete(h.subscribers, ch)
		close(ch)
		h.mu.Unlock()
	}
}

func StartCapture(h *Hub) error {
	if h == nil {
		h = DefaultHub()
	}

	captureOnce.Do(func() {
		captureErr = startCapture(h)
	})
	return captureErr
}

func startCapture(h *Hub) error {
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		return err
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	go capturePipe(stdoutReader, originalStdout, h)
	go capturePipe(stderrReader, originalStderr, h)
	return nil
}

func capturePipe(reader io.Reader, output *os.File, h *Hub) {
	buf := make([]byte, 4096)
	var line strings.Builder

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = output.Write(chunk)

			for _, b := range chunk {
				if b == '\n' {
					h.Add(strings.TrimSuffix(line.String(), "\r"))
					line.Reset()
					continue
				}
				line.WriteByte(b)
			}
		}

		if err != nil {
			if line.Len() > 0 {
				h.Add(strings.TrimSuffix(line.String(), "\r"))
			}
			return
		}
	}
}

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}
