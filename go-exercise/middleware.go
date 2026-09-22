package main

import (
	"net/http"
	"time"
)

// RateLimiter is the semaphore pattern from README §9, lifted directly:
// a buffered channel of struct{} where the buffer capacity IS the concurrency
// limit. struct{}{} costs zero bytes — only the count of tokens in flight matters.
type RateLimiter struct {
	tokens chan struct{}
}

func NewRateLimiter(maxConcurrent int) *RateLimiter {
	return &RateLimiter{tokens: make(chan struct{}, maxConcurrent)}
}

// Middleware caps how many requests are inside `next` at the same time.
// Acquire blocks (queues the request) once all tokens are taken; release
// happens via defer so a panicking handler still frees its slot.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rl.tokens <- struct{}{}        // acquire a slot (blocks if all 3 are taken)
		defer func() { <-rl.tokens }() // release the slot when this request is done
		next.ServeHTTP(w, r)
	})
}

// statusRecorder captures the status code a handler wrote, since http.ResponseWriter
// doesn't expose it after the fact.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// withLogging times every request and publishes an Event non-blockingly.
// It wraps the rate limiter, so the recorded duration includes any time spent
// queued for a token — that's real client-perceived latency, not noise.
func withLogging(publish func(Event), next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		publish(Event{
			Path:     r.URL.Path,
			Status:   rec.status,
			Duration: time.Since(start),
		})
	})
}
