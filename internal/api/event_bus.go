package api

import (
	"encoding/json"
	"sync"

	log "github.com/sirupsen/logrus"
)

const eventBusSubscriberBuffer = 4096
const eventBusReplayLimit = 256

var coalescedSSEEvents = []string{"tasks", "schedules"}
var replayableSSEEvents = []string{"notification", "server_shutdown"}

type SSEEvent struct {
	ID    uint64
	Event string
	Data  interface{} // 保留原始数据用于 replay
	Raw   []byte      // 预序列化的 JSON 字节缓存，避免对 N 个订阅者重复序列化
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

func isReplayableSSEEvent(event string) bool {
	for _, name := range replayableSSEEvents {
		if event == name {
			return true
		}
	}
	return false
}

type EventBus struct {
	mu            sync.Mutex
	subscribers   map[*eventSubscriber]struct{}
	nextEventID   uint64
	replayHistory []SSEEvent
}

func NewEventBus() *EventBus {
	return &EventBus{
		subscribers:   make(map[*eventSubscriber]struct{}),
		replayHistory: make([]SSEEvent, 0, eventBusReplayLimit),
	}
}

func (b *EventBus) Subscribe() (<-chan SSEEvent, func()) {
	ch, _, unsubscribe := b.SubscribeWithReplay(0)
	return ch, unsubscribe
}

func (b *EventBus) SubscribeWithReplay(lastEventID uint64) (<-chan SSEEvent, []SSEEvent, func()) {
	sub := newEventSubscriber()
	b.mu.Lock()
	replay := b.copyReplayAfterLocked(lastEventID)
	b.subscribers[sub] = struct{}{}
	b.mu.Unlock()

	return sub.ch, replay, func() {
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

func (b *EventBus) copyReplayAfterLocked(lastEventID uint64) []SSEEvent {
	if lastEventID == 0 || len(b.replayHistory) == 0 {
		return nil
	}

	replay := make([]SSEEvent, 0, len(b.replayHistory))
	for _, evt := range b.replayHistory {
		if evt.ID > lastEventID {
			replay = append(replay, evt)
		}
	}
	return replay
}

func (b *EventBus) Publish(event string, data interface{}) {
	// 预序列化：一份 JSON 字节切片供所有订阅者共享，避免 N 个订阅者重复 json.Marshal
	raw, err := json.Marshal(data)
	if err != nil {
		log.Warnf("[SSE] Failed to marshal event %s: %v", event, err)
		return
	}

	b.mu.Lock()
	b.nextEventID++
	evt := SSEEvent{ID: b.nextEventID, Event: event, Data: data, Raw: raw}
	if isReplayableSSEEvent(event) {
		if len(b.replayHistory) == eventBusReplayLimit {
			copy(b.replayHistory, b.replayHistory[1:])
			b.replayHistory[len(b.replayHistory)-1] = evt
		} else {
			b.replayHistory = append(b.replayHistory, evt)
		}
	}
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

	log.Warnf("[SSE] closing %d slow subscriber(s) after %s event queue overflow", len(overflowed), event)
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
