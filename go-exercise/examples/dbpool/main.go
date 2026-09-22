// Pattern 8: Semaphore-limited bulk write — cap how many goroutines can be
// "using a database connection" at once. Real scenario: bulk-importing
// thousands of records when the DB only allows ~100 concurrent connections
// (e.g. PostgreSQL's default max_connections). Without the cap, spawning one
// goroutine per record can exhaust every connection and take the DB down.
//
// This uses a fake insertRecord (time.Sleep + an occasional simulated
// failure) instead of a real *sql.DB, so it runs standalone with no database
// required — the concurrency-control shape is identical either way.
package main

import (
	"fmt"
	"sync"
	"time"
)

type Record struct {
	ID int
}

// insertRecord stands in for a real DB write. Record 13 "fails" on purpose
// so you can see errors surface correctly through the pipeline.
func insertRecord(r Record) error {
	time.Sleep(50 * time.Millisecond) // simulate query latency
	if r.ID == 13 {
		return fmt.Errorf("record %d: unique constraint violation", r.ID)
	}
	return nil
}

func bulkInsert(records []Record) error {
	sem := make(chan struct{}, 10) // at most 10 "connections" in use at once
	var wg sync.WaitGroup
	errChan := make(chan error, len(records))

	for _, r := range records {
		wg.Add(1)
		sem <- struct{}{} // acquire a slot (blocks once 10 are already in flight)

		go func(rec Record) {
			defer wg.Done()
			defer func() { <-sem }() // release the slot no matter how this goroutine exits

			if err := insertRecord(rec); err != nil {
				errChan <- err
			}
		}(r)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err // report the first error found; the rest still ran to completion
		}
	}
	return nil
}

func main() {
	records := make([]Record, 50)
	for i := range records {
		records[i] = Record{ID: i}
	}

	start := time.Now()
	err := bulkInsert(records)
	elapsed := time.Since(start)

	fmt.Printf("processed %d records in %s (cap: 10 concurrent)\n", len(records), elapsed)
	if err != nil {
		fmt.Println("bulk insert failed:", err)
	} else {
		fmt.Println("bulk insert succeeded")
	}
}
