package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
)

// Keys are normalized URLs, bool indicates presence.
// Protected by visitedMutex
var visited = make(map[string]bool)
var visitedMutex sync.Mutex

// Goroutines pick URLs from here to crawl.
// Buffered channel to avoid blocking sender if no workers are ready.
var taskQueue = make(chan string, 100)

var concurrencyLimiter = make(chan struct{}, 5) // Max 5 concurrent fetches

// To wait for all crawl tasks to complete
var wg sync.WaitGroup

// Store the hostname of the original seed URL to ensure we stay on the same dommain.
var seedHostname string

func main() {
	// --- 1. Argument Parsing and Validation ---
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <seed URL>", os.Args[0])
	}
	seedUrl := os.Args[1]

	// Check if input string looks like a valid URI structure
	parsedSeedURL, err := url.ParseRequestURI(seedUrl)
	if err != nil {
		log.Fatalf("Invalid seed URL format: %v", err)
	}

	// To get the scheme and hostname
	u, err := url.Parse(seedUrl)
	if err != nil {
		log.Fatalf("Could not parse seed URL components: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		log.Fatalf("Seed URL must use http or https scheme, got: %s", u.Scheme)
	}

	seedHostname = u.Hostname()
	if seedHostname == "" {
		log.Fatalf("Could not extract hostname from seed URL")
	}

	log.Printf("Crawler starting. Seed URL: %s, Domain Constraint: %s", seedUrl, seedHostname)

	// --- 2. Seed the Crawl ---
	// Mark the seed URL as visited before adding into the queue to prevent race conditions where multiple workers might try to add it.
	visitedMutex.Lock()
	visited[seedUrl] = true
	visitedMutex.Unlock()

	wg.Add(1)
	taskQueue <- seedUrl
	log.Printf("Queued initial seed URL: %s", seedUrl)

	// --- 3. Start Workers ---
	numWorkers := cap(concurrencyLimiter)
	log.Printf("Starting %d worker goroutines...", numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			log.Printf("Worker %d started", workerID)
			for urlToCrawl := range taskQueue {
				log.Printf("Worker %d received task: %s", workerID, urlToCrawl)

				concurrencyLimiter <- struct{}{}
				log.Printf("Worker %d acquired limiter slot for: %s", workerID, urlToCrawl)

				go func(urlToProcess string) {
					log.Printf("Fetcher goroutine starting for: %s", &urlToProcess)
					fmt.Printf("Simulating processing: %s\n", &urlToProcess)
					log.Printf("Fetcher goroutine finished for: %s. Releasing limiter.", &urlToProcess)
					<-concurrencyLimiter
					wg.Done()
				}(urlToCrawl)

			}
			log.Printf("Worker %d finished (task queue closed)", workerID)
		}(i)
	}

	// --- 4. Wait for Completion and Cleanup ---
	log.Println("Main goroutine: Waiting for all tasks to complete...")
	// Block until the wg counter = 0
	wg.Wait()
	log.Println("Main goroutine: All tasks completed.")

	log.Println("Main goroutine: Closing task queue.")

	close(taskQueue)
	close(concurrencyLimiter)

	log.Println("Crawler finished.")

}
