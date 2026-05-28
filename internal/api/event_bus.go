package api

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

const eventBusSubscriberBuffer = 4096

type SSEEvent struct {
	Event string
	Data  interface{}
}

type eventSubscriber struct {
	mu     sync.Mutex
	ch     chan SSEEvent
	closed bool
}

func newEventSubscriber() *eventSubscriber {
	return &eventSubscriber{
		ch: make(chan SSEEvent, eventBusSubscriberBuffer),
	}
}

func (s *eventSubscriber) send(evt SSEEvent) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return true
	}

	select {
	case s.ch <- evt:
		return true
	default:
		return false
	}
}

func (s *eventSubscriber) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	close(s.ch)
	s.closed = true
}

type EventBus struct {
	mu          sync.Mutex
	subscribers map[*eventSubscriber]struct{}
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[*eventSubscriber]struct{}),
	}
}

func (b *EventBus) Subscribe() (<-chan SSEEvent, func()) {
	sub := newEventSubscriber()
	b.mu.Lock()
	b.subscribers[sub] = struct{}{}
	b.mu.Unlock()

	return sub.ch, func() {
		b.mu.Lock()
		_, ok := b.subscribers[sub]
		if ok {
			delete(b.subscribers, sub)
		}
		b.mu.Unlock()

		if ok {
			sub.close()
		}
	}
}

func (b *EventBus) Publish(event string, data interface{}) {
	evt := SSEEvent{Event: event, Data: data}
	b.mu.Lock()
	subscribers := make([]*eventSubscriber, 0, len(b.subscribers))
	for sub := range b.subscribers {
		subscribers = append(subscribers, sub)
	}
	b.mu.Unlock()

	for _, sub := range subscribers {
		if !sub.send(evt) {
			log.Warnf("[SSE] subscriber queue full, dropping %s event", event)
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
