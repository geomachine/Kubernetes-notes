package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/fetch", fetchHandler)
	mux.HandleFunc("/slow", slowHandler)

	// Background logger (README §11/§10): a goroutine that owns the receive
	// end of a channel and runs until that channel is closed and drained.
	publish, events := newEventPublisher(100)
	var wg sync.WaitGroup
	wg.Add(1)
	go runLogger(&wg, events)

	// At most 3 requests execute concurrently; the rest queue on the token
	// channel (README §9).
	limiter := NewRateLimiter(3)
	handler := withLogging(publish, limiter.Middleware(mux))

	server := &http.Server{
		Addr:    ":8080",
		Handler: handler,
	}

	// README §8: catch SIGINT/SIGTERM ourselves — Go does NOT wire this up
	// automatically. Without it, Ctrl+C or `docker stop` kills the process
	// mid-request.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Println("listening on :8080 (try /health, /fetch?url=..., /slow?d=3s)")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server error: %v", err)
		}
	}()

	<-quit
	log.Println("shutdown signal received, draining in-flight requests...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown did not finish cleanly: %v", err)
	}

	// Only safe to close `events` now: Shutdown has returned, so no handler
	// can still be running and no one will call publish() again. Closing lets
	// runLogger's `for range` drain whatever's left and exit (README §10).
	close(events)
	wg.Wait()

	log.Println("shutdown complete")
}
