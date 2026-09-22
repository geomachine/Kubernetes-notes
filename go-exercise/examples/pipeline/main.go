// Pattern 2: Pipeline — chain of stages, each its own goroutine, connected by
// directional channels. One stage's <-chan output becomes the next stage's
// <-chan input. Used for log aggregation (Datadog/Splunk-style), ETL, and
// media transcoding pipelines (upload -> validate -> transcode -> thumbnail).
package main

import (
	"fmt"
	"strings"
	"time"
)

type LogEntry struct {
	Message   string
	Timestamp time.Time
}

// Stage 1 — producer: owns `out`, hands callers a receive-only view of it.
func generateLogs(logs []string) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out) // signals downstream "no more input" once we're done
		for _, l := range logs {
			out <- l
		}
	}()
	return out
}

// Stage 2 — filter: reads upstream, produces its own output channel.
func filterErrors(in <-chan string) <-chan string {
	out := make(chan string)
	go func() {
		defer close(out)
		for line := range in { // exits automatically when `in` closes
			if strings.Contains(line, "ERROR") {
				out <- line
			}
		}
	}()
	return out
}

// Stage 3 — enrich: transforms each surviving item into a richer type.
func parseAndEnrich(in <-chan string) <-chan LogEntry {
	out := make(chan LogEntry)
	go func() {
		defer close(out)
		for line := range in {
			out <- LogEntry{Message: line, Timestamp: time.Now()}
		}
	}()
	return out
}

func main() {
	logs := []string{"INFO: started", "ERROR: db timeout", "ERROR: disk full", "INFO: healthy"}

	stage1 := generateLogs(logs)  // [raw logs]
	stage2 := filterErrors(stage1) // -> [only ERROR lines]
	stage3 := parseAndEnrich(stage2) // -> [enriched entries]

	for entry := range stage3 {
		fmt.Printf("%s | %s\n", entry.Timestamp.Format(time.RFC3339Nano), entry.Message)
	}
}
