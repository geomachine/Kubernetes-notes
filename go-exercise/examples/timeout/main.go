// Pattern 3: Timeout / Cancellation on an external call — two versions, run
// back to back against the same deliberately-slow local server, so you can
// see the difference land in real time instead of just reading about it.
//
// The naive channel+select version stops the CALLER from waiting, but the
// real HTTP request keeps running in the background — a goroutine leak.
// The context.WithTimeout version actually aborts the in-flight connection.
package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"time"
)

// --- naive version: README §6 ---

func fetchWithTimeout(url string, timeout time.Duration) (string, error) {
	resultChan := make(chan string, 1) // buffered: see README §6 on why this must be 1, not 0
	errChan := make(chan error, 1)

	go func() {
		resp, err := http.Get(url)
		if err != nil {
			errChan <- err
			return
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		resultChan <- string(body)
	}()

	select {
	case result := <-resultChan:
		return result, nil
	case err := <-errChan:
		return "", err
	case <-time.After(timeout):
		return "", fmt.Errorf("request timed out after %v (but the goroutine above is still running!)", timeout)
	}
}

// --- modern version: README §7 ---

func fetchWithContext(url string, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err // on timeout, wraps context.DeadlineExceeded
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func main() {
	// A local server that takes 2s to respond — slower than either timeout below,
	// so both calls are guaranteed to time out deterministically (no live network needed).
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.Write([]byte("done"))
	}))
	defer slow.Close()

	fmt.Println("--- naive channel+select version ---")
	start := time.Now()
	_, err := fetchWithTimeout(slow.URL, 300*time.Millisecond)
	fmt.Printf("returned after %s, err: %v\n", time.Since(start), err)
	fmt.Println("(the abandoned goroutine's http.Get is still blocked on the server right now)")

	fmt.Println()
	fmt.Println("--- context.WithTimeout version ---")
	start = time.Now()
	_, err = fetchWithContext(slow.URL, 300*time.Millisecond)
	fmt.Printf("returned after %s, err: %v\n", time.Since(start), err)
	fmt.Println("(this one actually cancelled the in-flight connection)")
}
