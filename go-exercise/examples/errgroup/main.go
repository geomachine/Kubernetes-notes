// Pattern 7: errgroup — run several goroutines, get back the first error, and
// have the rest properly waited-on for you. Real scenario: batch-validating
// N uploaded files where one invalid file should reject the whole batch.
//
// This is what sync.WaitGroup + a manual mutex-guarded error slice is doing
// under the hood, packaged into ~3 lines instead of ~15.
package main

import (
	"fmt"

	"golang.org/x/sync/errgroup"
)

func validateAndProcess(file string) error {
	if file == "corrupt.csv" {
		return fmt.Errorf("%s: checksum mismatch", file)
	}
	fmt.Printf("validated %s\n", file)
	return nil
}

func processFiles(files []string) error {
	var g errgroup.Group

	for _, file := range files {
		g.Go(func() error {
			return validateAndProcess(file)
		})
	}

	return g.Wait() // returns the first non-nil error, after every goroutine has finished
}

func main() {
	files := []string{"orders.csv", "customers.csv", "corrupt.csv", "products.csv"}

	if err := processFiles(files); err != nil {
		fmt.Println("batch rejected:", err)
		return
	}
	fmt.Println("batch accepted")
}
