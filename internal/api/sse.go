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

func disableSSEWriteTimeout(w http.ResponseWriter) {
	if err := http.NewResponseController(w).SetWriteDeadline(time.Time{}); err != nil {
		if errors.Is(err, http.ErrNotSupported) {
			return
		}
		log.Warnf("[SSE] Failed to disable write deadline: %v", err)
	}
}

func (s *Server) handleSSETasks(w http.ResponseWriter, r *http.Request) {
	disableSSEWriteTimeout(w)

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "Streaming unsupported")
		return
	}

	fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()

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
			if err := writeSSEHeartbeat(w); err != nil {
				return
			}
			flusher.Flush()
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
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

func writeSSEHeartbeat(w http.ResponseWriter) error {
	_, err := fmt.Fprint(w, ": heartbeat\n\n")
	return err
}
