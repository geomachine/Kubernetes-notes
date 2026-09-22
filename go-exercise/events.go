package main

import (
	"log"
	"sync"
	"time"
)

// Event is one completed request, published for the background logger to consume.
// README §13: "Result collection" / "non-blocking event logging" use case.
type Event struct {
	Path     string
	Status   int
	Duration time.Duration
}

// newEventPublisher returns a publish function backed by a buffered channel.
// The buffer size is the channel's whole reason for existing here (README §13,
// §14): it's not "faster", it's a place to drop an event without ever slowing
// down the request that generated it.
func newEventPublisher(buffer int) (publish func(Event), events chan Event) {
	events = make(chan Event, buffer)

	publish = func(e Event) {
		select {
		case events <- e:
			// logged
		default:
			// Buffer full — drop rather than block the request that's waiting
			// on us. This is the exact select+default pattern from README §13.
			log.Printf("event buffer full, dropping event for %s", e.Path)
		}
	}

	return publish, events
}

// runLogger drains events until the channel is closed, then returns.
// README §10: `for range` on a channel exits cleanly the moment the channel
// is both closed AND drained — no extra signaling needed to know when to stop.
func runLogger(wg *sync.WaitGroup, events <-chan Event) {
	defer wg.Done()

	for e := range events {
		log.Printf("request path=%-12s status=%d duration=%s", e.Path, e.Status, e.Duration)
	}

	log.Println("logger: channel closed and drained, exiting")
}
