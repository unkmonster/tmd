package api

import (
	"sync"
)

type SSEEvent struct {
	Event string
	Data  interface{}
}

type EventBus struct {
	mu          sync.Mutex
	subscribers map[chan SSEEvent]struct{}
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[chan SSEEvent]struct{}),
	}
}

func (b *EventBus) Subscribe() (<-chan SSEEvent, func()) {
	ch := make(chan SSEEvent, 64)
	b.mu.Lock()
	b.subscribers[ch] = struct{}{}
	b.mu.Unlock()

	return ch, func() {
		b.mu.Lock()
		delete(b.subscribers, ch)
		close(ch)
		b.mu.Unlock()
	}
}

func (b *EventBus) Publish(event string, data interface{}) {
	evt := SSEEvent{Event: event, Data: data}
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subscribers {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (b *EventBus) PublishTasks(tasks []*Task) {
	b.Publish("tasks", tasks)
}

func (b *EventBus) PublishNotification(notifType, message string, detail interface{}) {
	b.Publish("notification", map[string]interface{}{
		"type":    notifType,
		"message": message,
		"detail":  detail,
	})
}

func (b *EventBus) PublishServerShutdown(message string) {
	b.Publish("server_shutdown", map[string]string{"message": message})
}
