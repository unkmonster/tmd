package consolelog

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"
)

const DefaultLimit = 5000

var (
	defaultHub = NewHub(DefaultLimit)

	captureMu      sync.Mutex
	activeCapture  *captureSession
	startCaptureFn = startCaptureSession
	ansiRegex      = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
)

// captureSession owns the redirected stdout/stderr pipes for the lifetime of
// an installed capture. In the current application flow capture stays active
// until process exit, but tests may stop it explicitly to restore stdio.
type captureSession struct {
	originalStdout *os.File
	originalStderr *os.File
	stdoutReader   *os.File
	stdoutWriter   *os.File
	stderrReader   *os.File
	stderrWriter   *os.File
}

type Hub struct {
	mu          sync.Mutex
	lines       []string
	start       int
	count       int
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
		lines:       make([]string, limit),
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

	if h.count < h.limit {
		idx := (h.start + h.count) % h.limit
		h.lines[idx] = line
		h.count++
	} else {
		h.lines[h.start] = line
		h.start = (h.start + 1) % h.limit
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

	lines := make([]string, h.count)
	if h.count == 0 {
		return lines
	}

	if h.start+h.count <= h.limit {
		copy(lines, h.lines[h.start:h.start+h.count])
		return lines
	}

	n := copy(lines, h.lines[h.start:])
	copy(lines[n:], h.lines[:(h.start+h.count)%h.limit])
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

	captureMu.Lock()
	defer captureMu.Unlock()

	if activeCapture != nil {
		return nil
	}

	session, err := startCaptureFn(h)
	if err != nil {
		return err
	}

	activeCapture = session
	return nil
}

func startCaptureSession(h *Hub) (*captureSession, error) {
	originalStdout := os.Stdout
	originalStderr := os.Stderr

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		stdoutReader.Close()
		stdoutWriter.Close()
		return nil, err
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	go capturePipe(stdoutReader, originalStdout, h)
	go capturePipe(stderrReader, originalStderr, h)

	return &captureSession{
		originalStdout: originalStdout,
		originalStderr: originalStderr,
		stdoutReader:   stdoutReader,
		stdoutWriter:   stdoutWriter,
		stderrReader:   stderrReader,
		stderrWriter:   stderrWriter,
	}, nil
}

func stopCaptureLocked() {
	if activeCapture == nil {
		return
	}

	os.Stdout = activeCapture.originalStdout
	os.Stderr = activeCapture.originalStderr

	if activeCapture.stdoutWriter != nil {
		_ = activeCapture.stdoutWriter.Close()
	}
	if activeCapture.stderrWriter != nil {
		_ = activeCapture.stderrWriter.Close()
	}
	if activeCapture.stdoutReader != nil {
		_ = activeCapture.stdoutReader.Close()
	}
	if activeCapture.stderrReader != nil {
		_ = activeCapture.stderrReader.Close()
	}

	activeCapture = nil
}

func capturePipe(reader io.Reader, output *os.File, h *Hub) {
	buf := make([]byte, 4096)
	var line strings.Builder

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			chunk := buf[:n]
			_, _ = output.Write(chunk)

			remaining := chunk
			for len(remaining) > 0 {
				idx := bytes.IndexByte(remaining, '\n')
				if idx < 0 {
					_, _ = line.Write(remaining)
					break
				}

				_, _ = line.Write(remaining[:idx])
				h.Add(strings.TrimSuffix(line.String(), "\r"))
				line.Reset()
				remaining = remaining[idx+1:]
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
