// Pattern 1: Fan-Out / Fan-In — merge results from several independent sources.
// Real scenario: loading an e-commerce product page that needs data from three
// separate microservices (price, inventory, reviews). Calling them one after
// another would take 100+150+200 = 450ms; fanning them out in parallel and
// fanning the results back in takes only as long as the slowest one (~200ms).
package main

import (
	"fmt"
	"time"
)

type Result struct {
	Source string
	Data   string
	Err    error
}

func fetchPrice(ch chan<- Result) {
	time.Sleep(100 * time.Millisecond) // simulate a network call
	ch <- Result{Source: "price", Data: "$29.99"}
}

func fetchInventory(ch chan<- Result) {
	time.Sleep(150 * time.Millisecond)
	ch <- Result{Source: "inventory", Data: "In Stock"}
}

func fetchReviews(ch chan<- Result) {
	time.Sleep(200 * time.Millisecond)
	ch <- Result{Source: "reviews", Data: "4.5 stars"}
}

func GetProductPage() map[string]string {
	resultChan := make(chan Result, 3) // buffered so no sender blocks waiting on us

	go fetchPrice(resultChan)     // fan-out: three independent goroutines start...
	go fetchInventory(resultChan) // ...none of them know about each other,
	go fetchReviews(resultChan)   // they just all write into the same channel.

	page := make(map[string]string)
	for i := 0; i < 3; i++ { // fan-in: merge whichever result arrives, in arrival order
		res := <-resultChan
		page[res.Source] = res.Data
	}
	return page
}

func main() {
	start := time.Now()
	page := GetProductPage()
	elapsed := time.Since(start)

	fmt.Println("product page:", page)
	fmt.Printf("elapsed: %s (sequential would have been ~450ms)\n", elapsed)
}
