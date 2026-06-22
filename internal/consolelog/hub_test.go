package consolelog

import (
	"errors"
	"io"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHubKeepsLineLimit(t *testing.T) {
	hub := NewHub(3)

	hub.Add("line1")
	hub.Add("line2")
	hub.Add("line3")
	hub.Add("line4")

	assert.Equal(t, []string{"line2", "line3", "line4"}, hub.Snapshot())
}

func TestHubSubscribeReceivesNewLines(t *testing.T) {
	hub := NewHub(10)
	ch, unsubscribe := hub.Subscribe()
	defer unsubscribe()

	hub.Add("live line")

	assert.Eventually(t, func() bool {
		select {
		case got := <-ch:
			return got == "live line"
		default:
			return false
		}
	}, 200*time.Millisecond, 10*time.Millisecond, "expected subscribed log line")
}

func TestHubSubscribeUnsubscribeClosesChannel(t *testing.T) {
	hub := NewHub(10)
	ch, unsubscribe := hub.Subscribe()

	unsubscribe()

	assert.Eventually(t, func() bool {
		select {
		case _, ok := <-ch:
			return !ok
		default:
			return false
		}
	}, 200*time.Millisecond, 10*time.Millisecond, "expected unsubscribed log channel to be closed")
}

func TestHubNormalizesLines(t *testing.T) {
	hub := NewHub(10)

	hub.Add(" \x1b[31mERRO[2026] failed\x1b[0m ")
	hub.Add("")

	assert.Equal(t, []string{"ERRO[2026] failed"}, hub.Snapshot())
}

func TestHubUsesFixedRingBuffer(t *testing.T) {
	hub := NewHub(3)

	for i := 0; i < 10; i++ {
		hub.Add(string(rune('a' + i)))
	}

	assert.Equal(t, 3, len(hub.lines))
	assert.Equal(t, 3, cap(hub.lines))
	assert.Equal(t, 3, hub.count)
	assert.Equal(t, []string{"h", "i", "j"}, hub.Snapshot())
}

func TestHubRemovesSlowSubscriberOnOverflow(t *testing.T) {
	hub := NewHub(10)
	_, _ = hub.Subscribe()

	for i := 0; i < subscriberBuffer*2; i++ {
		hub.Add("line")
	}

	assert.Eventually(t, func() bool {
		hub.mu.Lock()
		defer hub.mu.Unlock()
		return len(hub.subscribers) == 0
	}, 200*time.Millisecond, 10*time.Millisecond, "Slow log subscriber should be removed after queue overflow")
}

type chunkedReader struct {
	chunks [][]byte
	index  int
}

func (r *chunkedReader) Read(p []byte) (int, error) {
	if r.index >= len(r.chunks) {
		return 0, io.EOF
	}

	chunk := r.chunks[r.index]
	r.index++
	n := copy(p, chunk)
	return n, nil
}

func TestCapturePipe_ReassemblesLinesAcrossChunks(t *testing.T) {
	hub := NewHub(10)
	output, err := os.CreateTemp("", "capture-pipe-output")
	require.NoError(t, err)
	defer os.Remove(output.Name())
	defer output.Close()

	reader := &chunkedReader{
		chunks: [][]byte{
			[]byte("line1\r"),
			[]byte("\nline"),
			[]byte("2\nline3"),
		},
	}

	capturePipe(reader, output, hub)

	_, err = output.Seek(0, 0)
	require.NoError(t, err)

	data, err := io.ReadAll(output)
	require.NoError(t, err)

	assert.Equal(t, "line1\r\nline2\nline3", string(data))
	assert.Equal(t, []string{"line1", "line2", "line3"}, hub.Snapshot())
}

func TestStartCaptureRetriesAfterFailure(t *testing.T) {
	StopCapture()
	captureMu.Lock()
	startCaptureFn = startCaptureSession
	captureMu.Unlock()
	t.Cleanup(func() {
		StopCapture()
		captureMu.Lock()
		startCaptureFn = startCaptureSession
		captureMu.Unlock()
	})

	hub := NewHub(10)
	calls := 0
	startCaptureFn = func(h *Hub) (*captureSession, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("temporary start failure")
		}
		return &captureSession{
			originalStdout: os.Stdout,
			originalStderr: os.Stderr,
		}, nil
	}

	err := StartCapture(hub)
	require.EqualError(t, err, "temporary start failure")

	err = StartCapture(hub)
	require.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestStartCaptureIsIdempotentAfterSuccess(t *testing.T) {
	StopCapture()
	captureMu.Lock()
	startCaptureFn = startCaptureSession
	captureMu.Unlock()
	t.Cleanup(func() {
		StopCapture()
		captureMu.Lock()
		startCaptureFn = startCaptureSession
		captureMu.Unlock()
	})

	hub := NewHub(10)
	calls := 0
	startCaptureFn = func(h *Hub) (*captureSession, error) {
		calls++
		return &captureSession{
			originalStdout: os.Stdout,
			originalStderr: os.Stderr,
		}, nil
	}

	require.NoError(t, StartCapture(hub))
	require.NoError(t, StartCapture(hub))
	assert.Equal(t, 1, calls)
}
