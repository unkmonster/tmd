package consolelog

import (
	"bytes"
	"io"
	"os"
	"regexp"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
)

const DefaultLimit = 5000
const subscriberBuffer = 100

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
	subscribers map[*logSubscriber]struct{}
}

type logSubscriber struct {
	mu      sync.Mutex // guards close+send race
	closeMu sync.Once
	ch      chan string
	closed  bool
}

func newLogSubscriber() *logSubscriber {
	return &logSubscriber{
		ch: make(chan string, subscriberBuffer),
	}
}

type logSendResult int

const (
	logSendQueued logSendResult = iota
	logSendClosed
	logSendOverflow
)

func (s *logSubscriber) send(line string) logSendResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return logSendClosed
	}
	select {
	case s.ch <- line:
		return logSendQueued
	default:
		s.closed = true
		s.closeMu.Do(func() { close(s.ch) })
		return logSendOverflow
	}
}

func (s *logSubscriber) close() {
	s.closeMu.Do(func() {
		s.mu.Lock()
		s.closed = true
		close(s.ch)
		s.mu.Unlock()
	})
}

func (h *Hub) removeSubscribers(subscribers []*logSubscriber) {
	h.mu.Lock()
	for _, sub := range subscribers {
		delete(h.subscribers, sub)
	}
	h.mu.Unlock()
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
		subscribers: make(map[*logSubscriber]struct{}),
	}
}

func (h *Hub) Add(line string) {
	line = strings.TrimSpace(stripANSI(line))
	if line == "" {
		return
	}

	h.mu.Lock()

	if h.count < h.limit {
		idx := (h.start + h.count) % h.limit
		h.lines[idx] = line
		h.count++
	} else {
		h.lines[h.start] = line
		h.start = (h.start + 1) % h.limit
	}

	subscribers := make([]*logSubscriber, 0, len(h.subscribers))
	for sub := range h.subscribers {
		subscribers = append(subscribers, sub)
	}
	h.mu.Unlock()

	var overflowed []*logSubscriber
	for _, sub := range subscribers {
		switch sub.send(line) {
		case logSendOverflow:
			overflowed = append(overflowed, sub)
		}
	}

	if len(overflowed) > 0 {
		h.removeSubscribers(overflowed)
		for range overflowed {
			log.Warn("[consolelog] closing slow log subscriber after queue overflow")
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

// Close closes all active log subscribers, causing their SSE handlers to exit.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub := range h.subscribers {
		sub.close()
	}
	h.subscribers = make(map[*logSubscriber]struct{})
}

func (h *Hub) Subscribe() (<-chan string, func()) {
	sub := newLogSubscriber()

	h.mu.Lock()
	h.subscribers[sub] = struct{}{}
	h.mu.Unlock()

	return sub.ch, func() {
		h.mu.Lock()
		_, ok := h.subscribers[sub]
		if ok {
			delete(h.subscribers, sub)
		}
		h.mu.Unlock()

		if ok {
			sub.close()
		}
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

// StopCapture stops the active stdout/stderr capture session and restores
// the original stdout/stderr. It is safe to call multiple times (no-op
// when no capture is active).
func StopCapture() {
	captureMu.Lock()
	defer captureMu.Unlock()
	stopCaptureLocked()
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

