package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("ok\n"))
}

// fetchHandler is README §7's context.WithTimeout pattern, wired into a real
// handler. Note it derives ctx from r.Context() rather than context.Background():
// that means the upstream fetch is cancelled not just on our own timeout, but
// also if the original client disconnects — a step beyond what the guide covers.
func fetchHandler(w http.ResponseWriter, r *http.Request) {
	target := r.URL.Query().Get("url")
	if target == "" {
		http.Error(w, "missing ?url= query param", http.StatusBadRequest)
		return
	}

	timeout := 5 * time.Second
	if raw := r.URL.Query().Get("timeout"); raw != "" {
		if d, err := time.ParseDuration(raw); err == nil {
			timeout = d
		}
	}

	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel() // release resources the instant we return, not when the timer expires

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			http.Error(w, fmt.Sprintf("upstream fetch timed out after %s", timeout), http.StatusGatewayTimeout)
			return
		}
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close() // README §5 — always after the error check, always via defer

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // cap at 1MB so this stays a toy
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(resp.StatusCode)
	fmt.Fprintf(w, "upstream status: %d\ncontent-length: %d bytes\n\n%s\n", resp.StatusCode, len(body), body)
}

// slowHandler simulates slow work so you can watch two things happen live:
//  1. curl it 4+ times in parallel and see the rate limiter (README §9) queue
//     the 4th request behind the first 3.
//  2. curl it, then Ctrl+C the server, and watch graceful shutdown (README §8)
//     let this request finish instead of cutting it off mid-response.
func slowHandler(w http.ResponseWriter, r *http.Request) {
	d := 2 * time.Second
	if raw := r.URL.Query().Get("d"); raw != "" {
		if parsed, err := time.ParseDuration(raw); err == nil {
			d = parsed
		}
	}

	select {
	case <-time.After(d):
		fmt.Fprintf(w, "slept for %s\n", d)
	case <-r.Context().Done():
		// Only fires if the CLIENT disconnects early — server.Shutdown waits
		// for in-flight handlers rather than cancelling their context.
		log.Printf("slow handler: client went away early (%v)", r.Context().Err())
	}
}
