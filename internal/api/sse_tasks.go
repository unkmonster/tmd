package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const sseHeartbeatInterval = 25 * time.Second
const sseWriteTimeout = 10 * time.Second

func writeSSEFrame(w http.ResponseWriter, flusher http.Flusher, write func() error) error {
	controller := http.NewResponseController(w)
	if err := controller.SetWriteDeadline(time.Now().Add(sseWriteTimeout)); err != nil && !errors.Is(err, http.ErrNotSupported) {
		log.Warnf("[SSE] Failed to set write deadline: %v", err)
	}
	defer func() {
		if err := controller.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
			log.Warnf("[SSE] Failed to clear write deadline: %v", err)
		}
	}()

	if err := write(); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func (s *Server) handleSSETasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "Streaming unsupported")
		return
	}

	if err := writeSSEFrame(w, flusher, func() error {
		_, err := fmt.Fprint(w, ": connected\n\n")
		return err
	}); err != nil {
		return
	}

	lastEventID := parseLastEventID(r.Header.Get("Last-Event-ID"))
	ch, replay, unsubscribe := s.eventBus.SubscribeWithReplay(lastEventID)
	defer unsubscribe()

	tasks := s.taskManager.GetAllTasks()
	if err := s.writeSSENamedEvent(w, flusher, "tasks", tasks); err != nil {
		return
	}

	if sched := s.getScheduler(); sched != nil {
		schedulesPath := filepath.Join(s.appRootPath, "schedules.yaml")
		exists := true
		if _, err := os.Stat(schedulesPath); os.IsNotExist(err) {
			exists = false
		}
		if err := s.writeSSENamedEvent(w, flusher, "schedules", map[string]interface{}{
			"scheduler_running": sched.IsRunning(),
			"entries":           sched.GetStatuses(),
			"exists":            exists,
		}); err != nil {
			return
		}
	}

	for _, evt := range replay {
		if err := s.writeSSEEvent(w, flusher, evt); err != nil {
			return
		}
	}
	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if err := s.writeSSEEvent(w, flusher, evt); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := writeSSEFrame(w, flusher, func() error {
				return writeSSEHeartbeat(w)
			}); err != nil {
				return
			}
		case <-r.Context().Done():
			return
		}
	}
}

func (s *Server) writeSSENamedEvent(w http.ResponseWriter, flusher http.Flusher, event string, data interface{}) error {
	return s.writeSSEEvent(w, flusher, SSEEvent{Event: event, Data: data})
}

func (s *Server) writeSSEEvent(w http.ResponseWriter, flusher http.Flusher, evt SSEEvent) error {
	jsonData := evt.Raw
	if jsonData == nil {
		var err error
		jsonData, err = json.Marshal(evt.Data)
		if err != nil {
			log.Warnf("[SSE] Failed to marshal event %s: %v", evt.Event, err)
			return err
		}
	}
	return writeSSEFrame(w, flusher, func() error {
		if evt.ID > 0 {
			if _, err := fmt.Fprintf(w, "id: %d\n", evt.ID); err != nil {
				return err
			}
		}
		_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Event, jsonData)
		return err
	})
}

func parseLastEventID(raw string) uint64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func writeSSEHeartbeat(w http.ResponseWriter) error {
	_, err := fmt.Fprint(w, ": heartbeat\n\n")
	return err
}
