package api

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEventBusPublishSubscribe(t *testing.T) {
	bus := NewEventBus()
	ch, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	bus.Publish("test", "hello")

	select {
	case evt := <-ch:
		assert.Equal(t, "test", evt.Event)
		assert.Equal(t, "hello", evt.Data)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Did not receive event")
	}
}

func TestEventBusUnsubscribe(t *testing.T) {
	bus := NewEventBus()
	ch1, unsub1 := bus.Subscribe()
	ch2, unsub2 := bus.Subscribe()

	bus.Publish("test", "before")

	// drain both channels
	<-ch1
	<-ch2

	unsub1()

	bus.Publish("test", "after")

	select {
	case evt, ok := <-ch2:
		assert.True(t, ok)
		assert.Equal(t, "after", evt.Data)
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Active subscriber should receive event")
	}

	unsub2()
}

func TestEventBusUnsubscribeClosesChannel(t *testing.T) {
	bus := NewEventBus()
	ch, unsubscribe := bus.Subscribe()

	unsubscribe()

	select {
	case _, ok := <-ch:
		assert.False(t, ok)
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Unsubscribed channel should be closed")
	}
}

func TestEventBusBuffersBurstWithoutDropping(t *testing.T) {
	bus := NewEventBus()
	ch, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	for i := 0; i < 200; i++ {
		bus.Publish("test", i)
	}

	received := 0
	timeout := time.After(50 * time.Millisecond)
	for {
		select {
		case <-ch:
			received++
		case <-timeout:
			goto done
		}
	}
done:

	assert.Equal(t, 200, received)
}

func TestEventBusConcurrentPublish(t *testing.T) {
	bus := NewEventBus()
	ch, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				bus.Publish("test", n*100+j)
			}
		}(i)
	}
	wg.Wait()

	timeout := time.After(200 * time.Millisecond)
	received := 0
	for {
		select {
		case <-ch:
			received++
		case <-timeout:
			goto done2
		}
	}
done2:
	assert.Equal(t, 1000, received)
}

func TestEventBusCoalescesTasksEvents(t *testing.T) {
	bus := NewEventBus()
	ch, unsubscribe := bus.Subscribe()
	defer unsubscribe()

	for i := 0; i < eventBusSubscriberBuffer+100; i++ {
		bus.Publish("tasks", i)
	}

	values := make([]int, 0, 2)
	timeout := time.After(200 * time.Millisecond)
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				t.Fatal("Subscriber channel closed unexpectedly")
			}
			value, ok := evt.Data.(int)
			if !ok {
				t.Fatalf("Unexpected event payload type %T", evt.Data)
			}
			values = append(values, value)
		case <-timeout:
			goto done3
		}
	}
done3:

	assert.NotEmpty(t, values)
	assert.LessOrEqual(t, len(values), 2)
	assert.Equal(t, eventBusSubscriberBuffer+99, values[len(values)-1])
}

func TestEventBusClosesSlowSubscriberOnRegularQueueOverflow(t *testing.T) {
	bus := NewEventBus()
	_, _ = bus.Subscribe()

	for i := 0; i < eventBusSubscriberBuffer*2; i++ {
		bus.Publish("notification", i)
	}

	assert.Eventually(t, func() bool {
		bus.mu.Lock()
		defer bus.mu.Unlock()
		return len(bus.subscribers) == 0
	}, 200*time.Millisecond, 10*time.Millisecond, "Slow subscriber should be removed after queue overflow")
}

func TestEventBusMultipleSubscribers(t *testing.T) {
	bus := NewEventBus()
	ch1, unsub1 := bus.Subscribe()
	defer unsub1()
	ch2, unsub2 := bus.Subscribe()
	defer unsub2()

	bus.Publish("test", "hello")

	select {
	case evt := <-ch1:
		assert.Equal(t, "test", evt.Event)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Subscriber 1 did not receive event")
	}

	select {
	case evt := <-ch2:
		assert.Equal(t, "test", evt.Event)
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Subscriber 2 did not receive event")
	}
}
