package main

import (
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
	if len(os.Args) != 2 {
		log.Fatalf("Usage: %s <seed URL>", os.Args[0])
	}
	seedUrl := os.Args[1]

	// Check if input string looks like a valid URI structure
	_, err := url.ParseRequestURI(seedUrl)
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

}
