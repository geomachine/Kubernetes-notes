# Go Channels & Concurrency — A Complete Guide

*Compiled from a deep-dive conversation covering channels, goroutines, directional types, timeouts, graceful shutdown, and rate limiting.*

---

## Table of Contents

1. [Directional Channel Types](#1-directional-channel-types)
2. [Goroutines: `go func(){}()` vs `go func(){}`](#2-goroutines)
3. [Directional Channels in Real Function Signatures](#3-directional-channels-in-real-function-signatures)
4. [The `make` Built-in Function](#4-the-make-built-in-function)
5. [Why `defer resp.Body.Close()` Matters](#5-why-defer-respbodyclose-matters)
6. [Timeout Pattern with Channels + `select`](#6-timeout-pattern-with-channels--select)
7. [The Modern Version: `context.WithTimeout`](#7-the-modern-version-contextwithtimeout)
8. [Graceful Server Shutdown](#8-graceful-server-shutdown)
9. [Rate Limiter / Semaphore Pattern](#9-rate-limiter--semaphore-pattern)
10. [Closed Channels: Behavior & Zero Values](#10-closed-channels-behavior--zero-values)
11. [Send & Receive — From Absolute Zero](#11-send--receive--from-absolute-zero)
12. [Buffered vs Unbuffered — The Core Mental Model](#12-buffered-vs-unbuffered--the-core-mental-model)
13. [Real-World Use Cases](#13-real-world-use-cases)
14. [Backpressure & Performance Myths](#14-backpressure--performance-myths)
15. [Quick Reference Cheat Sheet](#15-quick-reference-cheat-sheet)

---

## 1. Directional Channel Types

A channel parameter can be restricted to only sending or only receiving:

```go
func fetchPrice(ch chan<- Result) {} // send-only channel
```

Breaking down `ch chan<- Result`:
- `ch` — parameter name
- `chan<- Result` — a **send-only** channel of `Result`

| Syntax | Meaning |
|---|---|
| `chan Result` | Bidirectional — can send and receive |
| `chan<- Result` | Send-only — arrow points *into* the channel |
| `<-chan Result` | Receive-only — arrow points *out of* the channel |

```go
func fetchPrice(ch chan<- Result) {
    ch <- Result{Source: "api", Data: "42.00"} // OK
    r := <-ch                                   // compile error!
}

func consume(ch <-chan Result) {
    r := <-ch      // OK
    ch <- Result{} // compile error!
}
```

**Why bother?** It's a compiler-enforced contract. If you pass a bidirectional `chan Result` into a function expecting `chan<-`, Go automatically narrows it — no cast needed. This documents intent and catches bugs (like a writer accidentally reading) at compile time.

---

## 2. Goroutines

### Valid — a function call

```go
go func() {
    defer close(out)
    for _, l := range logs {
        out <- l
    }
}()
```

The `()` at the end **calls** the anonymous function; `go` runs that call in a new goroutine. Steps:
1. New goroutine starts.
2. Iterates over `logs`, sending each line into `out`.
3. When the loop finishes, `defer close(out)` runs, closing the channel.
4. Closing signals receivers (`range out` or `v, ok := <-out`) that no more values are coming.

### Invalid — missing `()`

```go
go func() {
    defer close(out)
    for _, l := range logs {
        out <- l
    }
}
```

Without `()`, this is just a function *value*, not a function *call*. Go requires `go` and `defer` to be followed by an actual invocation.

```
go requires function call, not conversion
```

This is a **compile-time error** — nothing runs at all.

**Rule of thumb:** `go` and `defer` both need something being *invoked right now* — `f()`, `obj.Method()`, or `func(){...}()`.

---

## 3. Directional Channels in Real Function Signatures

```go
func generateLogs(logs []string) <-chan string
func filterErrors(in <-chan string) <-chan string
func fetchPrice(ch chan<- Result)
```

### `generateLogs` — Producer
Returns `<-chan string`: creates its own channel internally, sends into it, returns a receive-only view so callers can't send or close it.

```go
func generateLogs(logs []string) <-chan string {
    out := make(chan string)
    go func() {
        defer close(out)
        for _, l := range logs {
            out <- l
        }
    }()
    return out // implicitly narrowed to <-chan string
}
```

### `filterErrors` — Pipeline stage
Takes `<-chan string` (can only receive from upstream) and returns `<-chan string` (creates and owns its own output).

```go
func filterErrors(in <-chan string) <-chan string {
    out := make(chan string)
    go func() {
        defer close(out)
        for l := range in {
            if !strings.Contains(l, "ERROR") {
                out <- l
            }
        }
    }()
    return out
}
```

### `fetchPrice` — Worker
Takes `chan<- Result` — a channel it doesn't own, only allowed to write into it.

```go
func fetchPrice(ch chan<- Result) {
    ch <- Result{Source: "api", Data: "42.00"}
}

func main() {
    ch := make(chan Result)
    go fetchPrice(ch)
    go fetchPrice(ch)
    r1, r2 := <-ch, <-ch
}
```

### Summary Table

| Function | Direction | Role |
|---|---|---|
| `generateLogs` | `<-chan` as **return** | Producer — owns and returns a readable channel |
| `filterErrors` | `<-chan` as **param and return** | Pipeline stage — reads upstream, produces its own output |
| `fetchPrice` | `chan<-` as **param** | Worker — writes into a channel owned by someone else |

**General pattern:** whoever *creates* a channel keeps send/close rights and hands out `<-chan` to consumers. Whoever is just a *worker writing into someone else's channel* takes `chan<-`, so it can't accidentally close or read something it doesn't own.

---

## 4. The `make` Built-in Function

`make` isn't a normal function — it's a built-in whose argument count depends on the type:

```go
func make(t Type, size ...IntegerType) Type
```

| Type | Min args | Max args | Extra args mean |
|---|---|---|---|
| slice | 2 | 3 | length, then capacity |
| map | 1 | 2 | size hint |
| channel | 1 | 2 | buffer size |

```go
make([]int, 5)       // length 5
make([]int, 5, 10)   // length 5, capacity 10
make(map[string]int)       // no size hint
make(map[string]int, 100)  // size hint of 100
make(chan int)      // unbuffered
make(chan int, 10)  // buffered, capacity 10
```

The type itself always counts as the first argument — `make` is never called with zero arguments.

---

## 5. Why `defer resp.Body.Close()` Matters

`resp.Body` is an open network connection (an `io.ReadCloser`), not just data in memory. Without closing it:

1. **The connection can't be reused** — Go's `http.Transport` pools connections (keep-alive), but only once the body is read *and* closed.
2. **File descriptors / sockets leak** — eventually causing `too many open files` errors.
3. **Memory can leak** too, since internal buffers may not free until `Close()` runs.

```go
resp, err := http.Get(url)
if err != nil {
    return err
}
defer resp.Body.Close() // placed AFTER the error check — resp may be nil if err != nil
```

`defer` guarantees `Close()` runs on every exit path — early return, panic, multiple branches.

**Subtlety:** for full connection reuse, the body should also be **drained**:

```go
defer resp.Body.Close()
io.Copy(io.Discard, resp.Body) // drain unread bytes
```

A partially-read body often gets discarded rather than reused by the transport.

---

## 6. Timeout Pattern with Channels + `select`

```go
func fetchWithTimeout(url string, timeout time.Duration) (string, error) {
    resultChan := make(chan string, 1)
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
        return "", fmt.Errorf("request timed out after %v", timeout)
    }
}
```

### The delivery analogy
You order food and say: *"If it doesn't arrive in 30 minutes, I'll eat noodles instead."*
- The goroutine = the delivery guy, actually fetching the HTTP response.
- `resultChan` = the doorbell (success).
- `errChan` = a phone call saying the order failed.
- `time.After(timeout)` = your kitchen timer.
- `select` = you, waiting for **whichever happens first**.

### Why the channels MUST be buffered (`make(..., 1)`)

If unbuffered, and the timeout fires first, nobody is left listening on `resultChan`. When the goroutine eventually finishes and tries `resultChan <- result`, **it blocks forever** — a permanent goroutine leak, since a send on an unbuffered channel needs an active receiver.

With a buffer of 1, the goroutine can drop its result into the buffer and exit cleanly, even if nobody ever reads it — like a delivery guy who can leave food in a basket by the door instead of waiting for you to answer.

### The critical flaw
Even though the *caller* stops waiting at the timeout, the actual HTTP request **keeps running in the background** to completion. This pattern stops you from waiting — it does not cancel the real work.

---

## 7. The Modern Version: `context.WithTimeout`

```go
func fetchWithContext(url string, timeout time.Duration) (string, error) {
    ctx, cancel := context.WithTimeout(context.Background(), timeout)
    defer cancel() // always call cancel to avoid a resource leak

    req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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
```

This fixes the exact flaw above: binding `ctx` to the request tells the HTTP machinery to **actually abort the in-flight connection** when the deadline passes — not just make the caller stop waiting.

### Comparison

| | Goroutine + `select` version | `context.WithTimeout` version |
|---|---|---|
| Lines of code | ~20 | ~10 |
| Manual channels needed | Yes (2) | No |
| Actually cancels the network call | **No** | **Yes** |
| Risk of goroutine leak | Yes, unless buffered | No |
| Error on timeout | Custom string | Standard `context.DeadlineExceeded` |
| Idiomatic for HTTP timeouts | No (learning exercise) | **Yes** |

`defer cancel()` releases the context's internal resources as soon as the function returns, rather than waiting for the timer to expire naturally.

---

## 8. Graceful Server Shutdown

```go
func runServer() {
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

    server := &http.Server{Addr: ":8080"}

    go func() {
        if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("server error: %v", err)
        }
    }()

    <-quit // blocks until a signal arrives
    log.Println("Shutting down gracefully...")

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    server.Shutdown(ctx) // gives in-flight requests up to 5s to finish
}
```

### Step by step
1. `signal.Notify` intercepts `SIGINT` (Ctrl+C) and `SIGTERM` (sent by `kill`, Docker, Kubernetes) and delivers them into `quit` instead of letting the OS kill the process instantly.
2. `server.ListenAndServe()` runs in a goroutine because it **blocks forever** — running it directly would prevent reaching the signal-handling code.
3. `err != http.ErrServerClosed` filters out the *expected* error that occurs when `Shutdown()` is deliberately called — anything else is a real failure.
4. `<-quit` parks the main goroutine until a real OS signal arrives.
5. `server.Shutdown(ctx)`:
   - Stops accepting new connections.
   - Waits for in-flight requests to finish naturally.
   - The 5-second context is a safety valve — if requests hang, `Shutdown` gives up and returns anyway (returning `context.DeadlineExceeded`).

### Important clarification: Go does NOT do this automatically
`http.Server` provides the *tools* (`Shutdown`, and `signal.Notify` from the OS package) but wires none of it up for you. Skipping this code means Ctrl+C or `SIGTERM` kills the process instantly, mid-request.

### Production picture (two layers)

**Layer 1 — your app code:** the signal-catching + `Shutdown()` pattern above.

**Layer 2 — the orchestrator/supervisor** (external to your Go binary):

- **Kubernetes:** sends `SIGTERM` → waits up to `terminationGracePeriodSeconds` (default 30s) → sends `SIGKILL` if still alive.
- **systemd:** `systemctl stop` sends `SIGTERM` → waits `TimeoutStopSec` → `SIGKILL`.
- **Docker:** `docker stop` sends `SIGTERM` → waits 10s by default → `SIGKILL`.
- **Load balancers:** perform their own connection draining — stop routing new traffic, let existing connections finish.

```
Deploy tool says "stop this pod"
        ↓
Load balancer stops routing NEW traffic (connection draining)
        ↓
Kubernetes sends SIGTERM
        ↓
signal.Notify catches it
        ↓
server.Shutdown(ctx) — finish in-flight requests, within the grace period
        ↓
If not finished in time → SIGKILL, no more cleanup possible
```

**Mux clarification:** `http.DefaultServeMux` (request routing) is a completely separate concern from shutdown handling — routing decides *which handler* runs; shutdown decides *when the server stops*.

---

## 9. Rate Limiter / Semaphore Pattern

```go
type RateLimiter struct {
    tokens chan struct{}
}

func NewRateLimiter(maxConcurrent int) *RateLimiter {
    return &RateLimiter{tokens: make(chan struct{}, maxConcurrent)}
}

func (rl *RateLimiter) Execute(task func()) {
    rl.tokens <- struct{}{}        // acquire a slot
    defer func() { <-rl.tokens }() // release the slot when done
    task()
}

func main() {
    limiter := NewRateLimiter(3) // max 3 concurrent
    var wg sync.WaitGroup

    for i := 1; i <= 20; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            limiter.Execute(func() {
                fmt.Printf("Processing payment %d\n", id)
                time.Sleep(500 * time.Millisecond)
            })
        }(i)
    }
    wg.Wait()
}
```

### What is a semaphore?

A general concurrency concept (not Go-specific): a counter of available "permits."
- **Acquire** — take a permit; if none left, wait.
- **Release** — give a permit back, freeing up a waiter.

**Analogy:** a parking garage with 3 spots. Cars entering decrease the count; the barrier won't open once it hits 0. Cars leaving increase the count and let the next waiting car in.

### Mapping to the code

| Parking garage | Go code |
|---|---|
| "3 spots" counter | `make(chan struct{}, 3)` |
| Car enters (count −1) | `rl.tokens <- struct{}{}` |
| Car leaves (count +1) | `<-rl.tokens` |
| Barrier won't open if full | Send blocks if buffer is full |

`struct{}{}` is used because it takes **zero bytes** — we only care about the *count* of items in the channel, not their value.

`defer func() { <-rl.tokens }()` guarantees the slot is released even if `task()` panics — otherwise a lost slot permanently shrinks the semaphore's capacity.

### Trace through `main()`
1. First 3 goroutines acquire tokens instantly, start processing. Other 17 block, waiting.
2. After ~500ms, one finishes, releases its token.
3. One of the 17 waiting goroutines (order not guaranteed) immediately grabs the freed slot.
4. Repeats until all 20 are processed — roughly `ceil(20/3) × 500ms ≈ 3.3s` total.

`sync.WaitGroup` is unrelated to the rate limiting — it only tracks when *all* 20 goroutines have finished so `main()` doesn't exit early.

---

## 10. Closed Channels: Behavior & Zero Values

### A closed channel is NOT `nil`

```go
ch := make(chan int)
close(ch)
fmt.Println(ch == nil) // false
```

- **`nil` channel** — never `make()`d; a variable holding no real channel.
- **Closed channel** — a real channel that had `close()` called on it; still a valid, non-nil reference, just flagged as "done."

Receiving from a `nil` channel **blocks forever**. Receiving from a *closed* channel returns **immediately** — these behave oppositely.

### What you get when reading a closed channel

Once closed and fully drained, every future receive returns the **zero value** of the channel's element type — not literally "0" unless the type is numeric:

| Type | Zero value |
|---|---|
| `int` | `0` |
| `string` | `""` |
| `bool` | `false` |
| pointer / slice / map / chan / func / interface | `nil` |
| `struct{}` | `struct{}{}` |
| custom struct | each field's own zero value |

```go
ch := make(chan string)
close(ch)
v := <-ch // "" — not 0!
```

### Distinguishing a real value from the zero-value fallback

```go
v, ok := <-ch
// ok == true  → real value that was actually sent
// ok == false → channel closed and drained; v is just the zero value
```

This is exactly what powers `for v := range ch` — it stops automatically when a receive comes back with `ok == false`.

### Buffered vs. unbuffered closed channels — timing difference

**Buffered:** real values are returned first, in order, until the buffer is drained — only *then* does it start returning zero-value + `false`.

```go
ch := make(chan int, 3)
ch <- 1; ch <- 2
close(ch)
v, ok := <-ch // 1, true
v, ok = <-ch  // 2, true
v, ok = <-ch  // 0, false — buffer now empty
```

**Unbuffered:** since nothing was ever buffered, closing means every receive is *immediately* zero-value + `false`.

```go
ch := make(chan int)
close(ch)
v, ok := <-ch // 0, false — immediately
```

### What panics vs. what's safe

| Operation on a closed channel | Result |
|---|---|
| Receive (`<-ch`) | Always safe, never panics |
| Send (`ch <- v`) | **Panics**: "send on closed channel" |
| Close it again | **Panics** |
| Close a `nil` channel | **Panics** |

Being freely readable-forever after closing is the entire point — it's how a producer says "I'm done, drain what's left and stop," which is exactly what makes `for range ch` terminate cleanly.

---

## 11. Send & Receive — From Absolute Zero

A **channel** is a pipe connecting two goroutines:

```
Goroutine A  --->  [ PIPE ]  --->  Goroutine B
     (send)                        (receive)
```

```go
ch <- 5    // SEND: put 5 into the channel
x := <-ch  // RECEIVE: take a value out, store it in x
```

**Trick to remember direction:** look at where the arrow points relative to `ch`.

```go
ch <- 5     // arrow points AWAY from ch, INTO the channel → sending
x := <-ch   // arrow points OUT of ch                       → receiving
```

### Minimal working example

```go
func main() {
    ch := make(chan string)

    go func() {
        ch <- "hello" // goroutine B sends
    }()

    msg := <-ch // main (A) waits and receives
    fmt.Println(msg) // hello
}
```

---

## 12. Buffered vs Unbuffered — The Core Mental Model

### Unbuffered — handing a letter directly into someone's hand

```go
ch := make(chan string) // no number = unbuffered = zero storage
```

You must place the letter **directly in their hand** — you stand there holding it until your friend shows up. The moment the handoff finishes, you *know* they have it. This is a **guarantee** (a handshake).

### Buffered — dropping a letter in a mailbox

```go
ch := make(chan string, 3) // room for 3 items
```

You drop the letter in a mailbox slot and walk away immediately, without waiting for anyone. You don't know exactly *when* they'll read it — only that it's sitting there, safely, until they do.

### Direct comparison

| | Unbuffered | Buffered |
|---|---|---|
| Do you wait for the other side? | Yes, always | No — as long as there's room |
| Do you know they got it the instant you're done? | Yes, guaranteed | No, just know it's stored somewhere |

### When a send blocks

- **Unbuffered:** blocks unless a receiver is actively waiting *right now*.
- **Buffered:** blocks only once the storage is **completely full**.

```go
ch := make(chan int, 2)
ch <- 1 // fits, instant
ch <- 2 // fits, instant
ch <- 3 // FULL — now blocks until someone removes one
```

### 20 goroutines example (the concrete trace)

**Unbuffered:**
```go
ch := make(chan string) // unbuffered

for i := 1; i <= 20; i++ {
    go func(id int) {
        ch <- fmt.Sprintf("hello from %d", id)
    }(i)
}
```
All 20 goroutines immediately try to send. Since there's no storage and no one receiving yet, **all 20 block**. Only a `<-ch` performed by `main` unblocks *one* of them at a time. If `main` only receives once, the other **19 leak forever** — permanently stuck.

**Buffered with room for 20:**
```go
ch := make(chan string, 20)
```
All 20 sends succeed instantly — nobody blocks, since there's a slot for every one of them. The values just sit in the buffer until someone eventually reads them out.

### The one-sentence summary

> "Waiting" means a goroutine tried to send or receive but couldn't yet, so it's paused until the situation changes. Unbuffered channels make senders wait for an actual receiver. Buffered channels let senders skip that wait — up to the buffer's size — by giving them somewhere to drop the value even if nobody's there yet.

---

## 13. Real-World Use Cases

### Buffered channels

| Use case | Why buffered |
|---|---|
| **Rate limiting / concurrency semaphore** | Buffer size *is* the business rule (e.g. max 3 concurrent payment API calls) |
| **Non-blocking event/metrics logging** | `select` + `default`: log if there's room, drop silently if full — logging must never slow down a request |
| **Absorbing bursty traffic** | Producer doesn't stall during spikes; lets workers catch up over time |
| **Result collection in fan-in** | Fast workers aren't stalled waiting on a slower collector |

```go
select {
case events <- Event{Path: r.URL.Path}:
    // logged
default:
    // buffer full, drop it rather than block the request
}
```

### Unbuffered channels

| Use case | Why unbuffered |
|---|---|
| **Startup readiness signal** | Must *guarantee* setup is truly complete before serving traffic |
| **Request/response "ask a goroutine a question"** | Need the real answer before proceeding — no sense in a mailbox |
| **Graceful shutdown coordination** | Need proof a cleanup goroutine has *actually* finished |
| **Job dispatch with natural backpressure** | Producer should genuinely slow down when all workers are busy, protecting memory |
| **Mutex replacement (strict turn-taking)** | Passing a token hand-to-hand between goroutines |

```go
type query struct {
    key      string
    response chan string // unbuffered — one-shot request/response
}

queries := make(chan query)

go func() { // owns the map exclusively — no mutex needed
    cache := map[string]string{}
    for q := range queries {
        q.response <- cache[q.key]
    }
}()

func Get(key string) string {
    resp := make(chan string)
    queries <- query{key: key, response: resp}
    return <-resp
}
```

### Fan-out/fan-in and pipelines — a mix, not purely one or the other

```go
jobs := make(chan Job)             // often unbuffered — natural throttling
results := make(chan Result, 100)  // often buffered — workers don't stall on the collector
```

Classic pipelines (`generateLogs -> filterErrors -> ...`) default to **unbuffered**, so a fast stage doesn't outpace what downstream can consume. Small buffers are sometimes added as a performance tweak, not as the defining trait.

---

## 14. Backpressure & Performance Myths

### Myth: "Buffering a channel gives you a performance/throughput gain"

**Not necessarily true.** Consider:

```go
jobs := make(chan Job, 1000)
var wg sync.WaitGroup

for w := 0; w < 8; w++ {
    go worker(jobs, results, &wg)
}
```

If the **8 workers** are the bottleneck (fixed processing speed per job), a 1000-size buffer doesn't make them process any faster — it only lets the *producer* dump all jobs into the buffer instantly instead of pacing itself. **Total processing time is essentially unchanged.**

What actually determines throughput:
1. **Number of workers** — more workers (if resources allow and work is parallelizable) increases throughput.
2. **Per-job processing time** — I/O-bound work may benefit from more workers than CPU cores; CPU-bound work usually doesn't.

Buffering only helps smooth out **bursty timing**, not the fundamental rate of work getting done.

### Backpressure — the real reason to avoid over-buffering

**Scenario:** Infinite jobs being created, 8 workers, unbuffered `jobs` channel.

- All workers busy → producer's `jobs <- j` **blocks immediately**. No drops — the producer is simply paused until a worker frees up. Memory stays minimal.

**With a buffer of 1000 instead:**
- The producer can keep cramming jobs into the buffer even while all workers are busy, building up a backlog of up to 1000 unprocessed jobs in memory *before* it finally blocks.
- If job creation genuinely outpaces workers (e.g. 50,000 jobs incoming), the buffer just **postpones** the blocking — it doesn't eliminate it, and doesn't magically make jobs disappear or get dropped.

### Corrected mapping

| Channel type | Behavior when all workers are busy, jobs keep arriving |
|---|---|
| **Unbuffered** | Producer blocks immediately — true backpressure, minimal memory use |
| **Buffered (size N)** | Producer fills up to N jobs first, **then** blocks — backlog builds in memory |

**Key takeaway:** an oversized buffer on a producer→worker channel doesn't protect against a slow consumer — it just delays the symptom while quietly growing memory usage in the meantime. Real production incidents (OOM crashes) can come from buffering "just in case" without a specific reason tied to the actual constraint you're modeling.

---

## 15. Quick Reference Cheat Sheet

### Channel type syntax
```go
chan T      // bidirectional
chan<- T    // send-only
<-chan T    // receive-only
```

### Send vs receive
```go
ch <- v   // send  (arrow points INTO channel)
v := <-ch // receive (arrow points OUT of channel)
```

### Buffered vs unbuffered
```go
make(chan T)      // unbuffered — handshake, blocks until a receiver is ready
make(chan T, N)   // buffered   — mailbox with N slots, blocks only when full
```

### Closed channel rules
- Receiving from a closed channel: **always safe**, returns zero value + `ok == false` once drained.
- Sending to a closed channel: **panics**.
- Closing an already-closed or `nil` channel: **panics**.
- Closed ≠ `nil`.

### Decision guide: buffered or unbuffered?

Ask: **"Do I need to know the other side actually received/processed this before I move on?"**
- **Yes** → lean unbuffered (handshake / guarantee).
- **No, just need to hand it off and keep going** → buffered (mailbox), with a buffer size tied to an actual reason (a concurrency limit, a tolerable backlog), not a random number.

### Key patterns covered
| Pattern | Channel style |
|---|---|
| Timeout via `select` + `time.After` | Buffered (size 1) result/error channels to avoid goroutine leaks |
| `context.WithTimeout` (modern, preferred) | No manual channels needed — cancels the real work |
| Graceful shutdown | `os.Signal` channel (buffered, size 1) + `context.WithTimeout` for the shutdown deadline |
| Rate limiter / semaphore | Buffered channel of `struct{}`, capacity = concurrency limit |
| Pipeline stages | Typically unbuffered by default |
| Startup readiness | Unbuffered / `close()`-based signal |

### What the Go runtime itself is built on
The runtime's own internals (scheduler, GC, memory allocator) are built using **lower-level primitives** — mutexes, atomics, futexes — not the `chan` type itself. This is because channels are *implemented by* the runtime (depend on the scheduler existing), so the scheduler can't be built using channels without circularity. The `chan` type is a feature the runtime *provides* to user code, not a tool the runtime is internally constructed from.