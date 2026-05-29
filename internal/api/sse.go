package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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

	tasks := s.taskManager.GetAllTasks()
	if err := s.writeSSENamedEvent(w, flusher, "tasks", tasks); err != nil {
		return
	}

	if sched := s.getScheduler(); sched != nil {
		if err := s.writeSSENamedEvent(w, flusher, "schedules", map[string]interface{}{
			"scheduler_running": sched.IsRunning(),
			"entries":           sched.GetStatuses(),
		}); err != nil {
			return
		}
	}

	ch, unsubscribe := s.eventBus.Subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(sseHeartbeatInterval)
	defer heartbeat.Stop()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if err := s.writeSSENamedEvent(w, flusher, evt.Event, evt.Data); err != nil {
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
	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Warnf("[SSE] Failed to marshal event %s: %v", event, err)
		return err
	}
	return writeSSEFrame(w, flusher, func() error {
		_, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData)
		return err
	})
}

func writeSSEHeartbeat(w http.ResponseWriter) error {
	_, err := fmt.Fprint(w, ": heartbeat\n\n")
	return err
}
