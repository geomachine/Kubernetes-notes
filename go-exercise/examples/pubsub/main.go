// Pattern 6: Pub-Sub — broadcast one event to many independent consumers.
// Real scenario: live score updates, chat message fan-out over WebSocket,
// a stock ticker — one publish, many subscribers, each with their own channel.
package main

import (
	"fmt"
	"sync"
	"time"
)

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]chan string
}

func NewEventBus() *EventBus {
	return &EventBus{subscribers: make(map[string][]chan string)}
}

func (eb *EventBus) Subscribe(topic string) <-chan string {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ch := make(chan string, 10)
	eb.subscribers[topic] = append(eb.subscribers[topic], ch)
	return ch
}

func (eb *EventBus) Publish(topic string, message string) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()

	for _, ch := range eb.subscribers[topic] {
		select {
		case ch <- message: // delivered
		default:
			// This subscriber's buffer is full (a slow consumer) — drop the
			// message for THEM specifically rather than blocking Publish and
			// stalling every other subscriber too.
			fmt.Println("dropped message for a slow subscriber")
		}
	}
}

func main() {
	bus := NewEventBus()

	sub1 := bus.Subscribe("cricket-score")
	sub2 := bus.Subscribe("cricket-score")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for msg := range sub1 {
			fmt.Println("User1 received:", msg)
		}
	}()
	go func() {
		defer wg.Done()
		for msg := range sub2 {
			fmt.Println("User2 received:", msg)
		}
	}()

	bus.Publish("cricket-score", "Bangladesh 250/4")
	bus.Publish("cricket-score", "Bangladesh 255/4")
	time.Sleep(200 * time.Millisecond) // let the printers catch up before main exits

	// Note: this demo never closes the subscriber channels, since EventBus has
	// no Unsubscribe here — that's why we exit main() rather than range-waiting
	// on the goroutines (wg.Wait() would block forever). A real implementation
	// needs an Unsubscribe(topic, ch) that closes the channel so subscribers
	// can shut down cleanly (README §10).
}
