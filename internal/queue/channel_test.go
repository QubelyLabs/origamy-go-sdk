package queue

import (
	"sync"
	"testing"
	"time"
)

func TestChannelQueue_EnqueueDequeue(t *testing.T) {
	q := NewChannelQueue[string](WithCapacity[string](10))
	defer q.Close()

	if err := q.Enqueue("msg1"); err != nil {
		t.Fatalf("Enqueue failed: %v", err)
	}

	msg, ok := q.Dequeue()
	if !ok {
		t.Fatal("Dequeue returned false")
	}
	if msg != "msg1" {
		t.Errorf("Expected 'msg1', got '%s'", msg)
	}
}

func TestChannelQueue_TryEnqueue(t *testing.T) {
	q := NewChannelQueue[string](WithCapacity[string](1))
	defer q.Close()

	if err := q.TryEnqueue("msg1"); err != nil {
		t.Fatalf("TryEnqueue failed: %v", err)
	}

	if err := q.TryEnqueue("msg2"); err == nil {
		t.Fatal("Expected ErrQueueFull, got nil")
	}
}

func TestChannelQueue_TryDequeue(t *testing.T) {
	q := NewChannelQueue[string](WithCapacity[string](10))
	defer q.Close()

	_, ok := q.TryDequeue()
	if ok {
		t.Fatal("Expected false for empty queue")
	}

	q.Enqueue("msg1")
	msg, ok := q.TryDequeue()
	if !ok {
		t.Fatal("TryDequeue returned false")
	}
	if msg != "msg1" {
		t.Errorf("Expected 'msg1', got '%s'", msg)
	}
}

func TestChannelQueue_Len(t *testing.T) {
	q := NewChannelQueue[string](WithCapacity[string](10))
	defer q.Close()

	if q.Len() != 0 {
		t.Errorf("Expected Len() == 0, got %d", q.Len())
	}

	q.Enqueue("msg1")
	q.Enqueue("msg2")

	if q.Len() != 2 {
		t.Errorf("Expected Len() == 2, got %d", q.Len())
	}
}

func TestChannelQueue_Cap(t *testing.T) {
	q := NewChannelQueue[string](WithCapacity[string](50))
	defer q.Close()

	if q.Cap() != 50 {
		t.Errorf("Expected Cap() == 50, got %d", q.Cap())
	}
}

func TestChannelQueue_Close(t *testing.T) {
	q := NewChannelQueue[string](WithCapacity[string](10))

	if q.IsClosed() {
		t.Fatal("Queue should not be closed")
	}

	q.Close()

	if !q.IsClosed() {
		t.Fatal("Queue should be closed")
	}

	if err := q.Enqueue("msg"); err == nil {
		t.Fatal("Expected error after close")
	}
}

func TestChannelQueue_Drain(t *testing.T) {
	q := NewChannelQueue[string](WithCapacity[string](10))

	q.Enqueue("msg1")
	q.Enqueue("msg2")
	q.Enqueue("msg3")

	q.Close()

	var drained []string
	for msg := range q.Drain() {
		drained = append(drained, msg)
	}

	if len(drained) != 3 {
		t.Errorf("Expected 3 drained messages, got %d", len(drained))
	}
}

func TestChannelQueue_ConcurrentAccess(t *testing.T) {
	q := NewChannelQueue[int](WithCapacity[int](1000))

	var wg sync.WaitGroup
	numProducers := 10
	numMessages := 100

	for i := 0; i < numProducers; i++ {
		wg.Add(1)
		go func(producerID int) {
			defer wg.Done()
			for j := 0; j < numMessages; j++ {
				if err := q.Enqueue(producerID*numMessages + j); err != nil {
					t.Errorf("Enqueue failed: %v", err)
				}
			}
		}(i)
	}

	wg.Wait()

	if q.Len() != numProducers*numMessages {
		t.Errorf("Expected %d messages, got %d", numProducers*numMessages, q.Len())
	}

	q.Close()
}

func TestChannelQueue_Channel(t *testing.T) {
	q := NewChannelQueue[string](WithCapacity[string](10))

	q.Enqueue("msg1")

	select {
	case msg := <-q.Channel():
		if msg != "msg1" {
			t.Errorf("Expected 'msg1', got '%s'", msg)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message")
	}

	q.Close()
}

func TestNewChannelQueueWithConfig(t *testing.T) {
	config := Config{
		Capacity: 200,
	}
	q := NewChannelQueueWithConfig[string](config)
	defer q.Close()

	if q.Cap() != 200 {
		t.Errorf("Expected Cap() == 200, got %d", q.Cap())
	}
}

func TestChannelQueue_DefaultCapacity(t *testing.T) {
	q := NewChannelQueue[string]()
	defer q.Close()

	if q.Cap() != 100 {
		t.Errorf("Expected default Cap() == 100, got %d", q.Cap())
	}
}
