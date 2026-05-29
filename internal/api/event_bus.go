package api

import (
	"sync"

	log "github.com/sirupsen/logrus"
)

const eventBusSubscriberBuffer = 4096

var coalescedSSEEvents = []string{"tasks", "schedules"}

type SSEEvent struct {
	Event string
	Data  interface{}
}

type eventSubscriber struct {
	mu      sync.Mutex
	ch      chan SSEEvent
	wake    chan struct{}
	done    chan struct{}
	closeMu sync.Once
	closed  bool
	queue   []SSEEvent
	latest  map[string]SSEEvent
}

func newEventSubscriber() *eventSubscriber {
	sub := &eventSubscriber{
		ch:     make(chan SSEEvent),
		wake:   make(chan struct{}, 1),
		done:   make(chan struct{}),
		queue:  make([]SSEEvent, 0, eventBusSubscriberBuffer),
		latest: make(map[string]SSEEvent, len(coalescedSSEEvents)),
	}
	go sub.run()
	return sub
}

type eventSendResult int

const (
	eventSendQueued eventSendResult = iota
	eventSendClosed
	eventSendOverflow
)

func (s *eventSubscriber) send(evt SSEEvent) eventSendResult {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return eventSendClosed
	}

	if isCoalescedSSEEvent(evt.Event) {
		s.latest[evt.Event] = evt
		s.wakeLocked()
		return eventSendQueued
	}

	if len(s.queue) >= eventBusSubscriberBuffer {
		s.closed = true
		s.closeDone()
		return eventSendOverflow
	}

	s.queue = append(s.queue, evt)
	s.wakeLocked()
	return eventSendQueued
}

func (s *eventSubscriber) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}
	s.closed = true
	s.closeDone()
}

func (s *eventSubscriber) run() {
	defer close(s.ch)

	for {
		select {
		case <-s.wake:
		case <-s.done:
			return
		}

		for {
			evt, ok := s.nextEvent()
			if !ok {
				break
			}

			select {
			case s.ch <- evt:
			case <-s.done:
				return
			}
		}
	}
}

func (s *eventSubscriber) nextEvent() (SSEEvent, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.queue) > 0 {
		evt := s.queue[0]
		s.queue = s.queue[1:]
		return evt, true
	}

	for _, eventName := range coalescedSSEEvents {
		if evt, ok := s.latest[eventName]; ok {
			delete(s.latest, eventName)
			return evt, true
		}
	}

	return SSEEvent{}, false
}

func (s *eventSubscriber) wakeLocked() {
	select {
	case s.wake <- struct{}{}:
	default:
	}
}

func (s *eventSubscriber) closeDone() {
	s.closeMu.Do(func() {
		close(s.done)
	})
}

func isCoalescedSSEEvent(event string) bool {
	for _, name := range coalescedSSEEvents {
		if event == name {
			return true
		}
	}
	return false
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

	var overflowed []*eventSubscriber
	for _, sub := range subscribers {
		switch sub.send(evt) {
		case eventSendOverflow:
			overflowed = append(overflowed, sub)
		}
	}

	if len(overflowed) == 0 {
		return
	}

	b.mu.Lock()
	for _, sub := range overflowed {
		delete(b.subscribers, sub)
	}
	b.mu.Unlock()

	for range overflowed {
		log.Warnf("[SSE] closing slow subscriber after %s event queue overflow", event)
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
