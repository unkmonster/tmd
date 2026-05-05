package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	log "github.com/sirupsen/logrus"
)

func (s *Server) handleSSETasks(w http.ResponseWriter, r *http.Request) {
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
	s.writeSSENamedEvent(w, flusher, "tasks", tasks)

	if sched := s.getScheduler(); sched != nil {
		s.writeSSENamedEvent(w, flusher, "schedules", map[string]interface{}{
			"scheduler_running": sched.IsRunning(),
			"entries":           sched.GetStatuses(),
		})
	}

	ch, unsubscribe := s.eventBus.Subscribe()
	defer unsubscribe()

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if err := s.writeSSENamedEvent(w, flusher, evt.Event, evt.Data); err != nil {
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
		return nil
	}
	if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, jsonData); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
