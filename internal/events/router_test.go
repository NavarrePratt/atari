package events

import (
	"sync"
	"testing"
	"time"
)

func TestNewRouter(t *testing.T) {
	t.Run("default buffer size", func(t *testing.T) {
		r := NewRouter(0)
		if r.bufferSize != DefaultBufferSize {
			t.Errorf("expected buffer size %d, got %d", DefaultBufferSize, r.bufferSize)
		}
	})

	t.Run("negative buffer size uses default", func(t *testing.T) {
		r := NewRouter(-10)
		if r.bufferSize != DefaultBufferSize {
			t.Errorf("expected buffer size %d, got %d", DefaultBufferSize, r.bufferSize)
		}
	})

	t.Run("custom buffer size", func(t *testing.T) {
		r := NewRouter(50)
		if r.bufferSize != 50 {
			t.Errorf("expected buffer size 50, got %d", r.bufferSize)
		}
	})
}

func TestRouterEmitSubscribe(t *testing.T) {
	t.Run("single subscriber receives event", func(t *testing.T) {
		r := NewRouter(10)
		defer r.Close()

		ch := r.Subscribe()
		event := &ClaudeTextEvent{
			BaseEvent: NewClaudeEvent(EventClaudeText),
			Text:      "Hello",
		}

		r.Emit(event)

		select {
		case received := <-ch:
			if received.Type() != EventClaudeText {
				t.Errorf("expected %s, got %s", EventClaudeText, received.Type())
			}
			textEvent, ok := received.(*ClaudeTextEvent)
			if !ok {
				t.Fatalf("expected *ClaudeTextEvent, got %T", received)
			}
			if textEvent.Text != "Hello" {
				t.Errorf("expected 'Hello', got %q", textEvent.Text)
			}
		case <-time.After(time.Second):
			t.Error("timeout waiting for event")
		}
	})

	t.Run("multiple subscribers each receive all events", func(t *testing.T) {
		r := NewRouter(10)
		defer r.Close()

		ch1 := r.Subscribe()
		ch2 := r.Subscribe()
		ch3 := r.Subscribe()

		events := []Event{
			&ClaudeTextEvent{BaseEvent: NewClaudeEvent(EventClaudeText), Text: "Event 1"},
			&ClaudeTextEvent{BaseEvent: NewClaudeEvent(EventClaudeText), Text: "Event 2"},
			&ClaudeTextEvent{BaseEvent: NewClaudeEvent(EventClaudeText), Text: "Event 3"},
		}

		for _, e := range events {
			r.Emit(e)
		}

		// Each subscriber should receive all 3 events
		for _, ch := range []<-chan Event{ch1, ch2, ch3} {
			for i := 0; i < 3; i++ {
				select {
				case <-ch:
					// Event received
				case <-time.After(time.Second):
					t.Errorf("timeout waiting for event %d", i)
				}
			}
		}
	})
}

func TestRouterSubscribeBuffered(t *testing.T) {
	r := NewRouter(10)
	defer r.Close()

	// Subscribe with larger buffer
	ch := r.SubscribeBuffered(1000)

	// Fill the buffer with events
	for i := 0; i < 500; i++ {
		r.Emit(&ClaudeTextEvent{
			BaseEvent: NewClaudeEvent(EventClaudeText),
			Text:      "test",
		})
	}

	// All 500 events should be buffered
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 500 {
		t.Errorf("expected 500 buffered events, got %d", count)
	}
}

func TestRouterUnsubscribe(t *testing.T) {
	t.Run("unsubscribe removes subscriber", func(t *testing.T) {
		r := NewRouter(10)
		defer r.Close()

		ch1 := r.Subscribe()
		ch2 := r.Subscribe()

		// Unsubscribe ch1
		r.Unsubscribe(ch1)

		// Emit an event
		r.Emit(&ClaudeTextEvent{BaseEvent: NewClaudeEvent(EventClaudeText), Text: "test"})

		// ch1 should be closed
		select {
		case _, ok := <-ch1:
			if ok {
				t.Error("expected ch1 to be closed")
			}
		default:
			t.Error("ch1 should be readable (closed)")
		}

		// ch2 should receive the event
		select {
		case <-ch2:
			// Event received
		case <-time.After(time.Second):
			t.Error("timeout waiting for event on ch2")
		}
	})

	t.Run("unsubscribe unknown channel is safe", func(t *testing.T) {
		r := NewRouter(10)
		defer r.Close()

		unknownCh := make(chan Event)
		// Should not panic
		r.Unsubscribe(unknownCh)
	})
}

func TestRouterClose(t *testing.T) {
	t.Run("close closes all subscriber channels", func(t *testing.T) {
		r := NewRouter(10)

		ch1 := r.Subscribe()
		ch2 := r.Subscribe()

		r.Close()

		// Both channels should be closed
		for i, ch := range []<-chan Event{ch1, ch2} {
			select {
			case _, ok := <-ch:
				if ok {
					t.Errorf("expected channel %d to be closed", i)
				}
			default:
				t.Errorf("channel %d should be readable (closed)", i)
			}
		}
	})

	t.Run("emit after close is no-op", func(t *testing.T) {
		r := NewRouter(10)
		ch := r.Subscribe()
		r.Close()

		// Emit after close should not panic
		r.Emit(&ClaudeTextEvent{BaseEvent: NewClaudeEvent(EventClaudeText), Text: "test"})

		// Channel should be closed, not have a new event
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("expected channel to be closed, not receive event")
			}
		default:
			t.Error("channel should be readable (closed)")
		}
	})

	t.Run("subscribe after close returns closed channel", func(t *testing.T) {
		r := NewRouter(10)
		r.Close()

		ch := r.Subscribe()

		select {
		case _, ok := <-ch:
			if ok {
				t.Error("expected channel to be closed")
			}
		default:
			t.Error("channel should be readable (closed)")
		}
	})

	t.Run("close is idempotent", func(t *testing.T) {
		r := NewRouter(10)
		r.Subscribe()

		// Multiple closes should not panic
		r.Close()
		r.Close()
		r.Close()
	})
}

func TestRouterFullBuffer(t *testing.T) {
	// Use a very small buffer to test drop behavior
	r := NewRouter(2)
	defer r.Close()

	ch := r.SubscribeBuffered(2)

	// Emit more events than the buffer can hold
	for i := 0; i < 10; i++ {
		r.Emit(&ClaudeTextEvent{
			BaseEvent: NewClaudeEvent(EventClaudeText),
			Text:      "test",
		})
	}

	// Only 2 events should be in the buffer
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto done
		}
	}
done:
	if count != 2 {
		t.Errorf("expected 2 events (buffer full, rest dropped), got %d", count)
	}
}

func TestRouterConcurrency(t *testing.T) {
	r := NewRouter(100)
	defer r.Close()

	subscribers := make([]<-chan Event, 10)
	for i := range subscribers {
		subscribers[i] = r.Subscribe()
	}

	var wg sync.WaitGroup
	numEvents := 100

	// Concurrent emitters
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < numEvents; j++ {
				r.Emit(&ClaudeTextEvent{
					BaseEvent: NewClaudeEvent(EventClaudeText),
					Text:      "concurrent event",
				})
			}
		}()
	}

	// Concurrent subscriber/unsubscriber
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 10; j++ {
			ch := r.Subscribe()
			r.Unsubscribe(ch)
		}
	}()

	wg.Wait()

	// Drain channels to verify no panics occurred
	for _, ch := range subscribers {
		for {
			select {
			case <-ch:
			default:
				goto next
			}
		}
	next:
	}
}

func TestRouterEventTypes(t *testing.T) {
	r := NewRouter(10)
	defer r.Close()

	ch := r.Subscribe()

	// Test various event types
	events := []Event{
		&SessionStartEvent{BaseEvent: NewInternalEvent(EventSessionStart), BeadID: "bd-123"},
		&SessionEndEvent{BaseEvent: NewInternalEvent(EventSessionEnd), SessionID: "sess-456"},
		&ClaudeToolUseEvent{BaseEvent: NewClaudeEvent(EventClaudeToolUse), ToolName: "Bash"},
		&IterationStartEvent{BaseEvent: NewInternalEvent(EventIterationStart), BeadID: "bd-789"},
		&ErrorEvent{BaseEvent: NewInternalEvent(EventError), Message: "test error"},
	}

	for _, e := range events {
		r.Emit(e)
	}

	// Verify all events received in order
	for i, expected := range events {
		select {
		case received := <-ch:
			if received.Type() != expected.Type() {
				t.Errorf("event %d: expected type %s, got %s", i, expected.Type(), received.Type())
			}
		case <-time.After(time.Second):
			t.Errorf("timeout waiting for event %d", i)
		}
	}
}
