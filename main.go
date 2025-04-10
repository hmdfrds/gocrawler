package main

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"

	"golang.org/x/net/html"
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

// Store the hostname of the original seed URL to ensure we stay on the same domain.
var seedHostname string

func main() {
	// --- 1. Argument Parsing and Validation ---
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
					log.Printf("Fetcher goroutine starting for: %s", urlToProcess)
					processURL(urlToProcess, &wg, &visitedMutex, visited, taskQueue, concurrencyLimiter, seedHostname)
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

func processURL(urlToCrawl string, wg *sync.WaitGroup, visitedMutex *sync.Mutex, visited map[string]bool, taskQueue chan<- string, limiter chan struct{}, baseHostname string) {
	defer wg.Done()
	defer func() {
		<-limiter
		log.Printf("Limiter slot released for: %s", urlToCrawl)
	}()

	visitedMutex.Lock()
	if _, exists := visited[urlToCrawl]; exists {
		visitedMutex.Unlock()
		log.Printf("Skipping already visited (in-flight check): %s", urlToCrawl)
		return
	}
	visited[urlToCrawl] = true
	visitedMutex.Unlock()

	log.Printf("Fetching: %s", urlToCrawl)

	resp, err := http.Get(urlToCrawl)
	if err != nil {
		log.Printf("ERROR: Failed to fetch %s: %v", urlToCrawl, err)
		return
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("SKIP: Non-OK status code %d for %s", resp.StatusCode, urlToCrawl)
		return
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") {
		log.Printf("SKIP: Non-HTML content type '%s' for %s", contentType, urlToCrawl)
		return
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: Failed to read body for %s: %v", urlToCrawl, err)
		return
	}

	doc, err := html.Parse(bytes.NewReader(bodyBytes))
	if err != nil {
		log.Printf("Error: Failed to parse HTML for %s: %v", urlToCrawl, err)
		return
	}

	baseParsedURL, _ := url.Parse(urlToCrawl)

	log.Printf("Parsing links on: %s", urlToCrawl)
	extractAndQueueLinks(doc, baseParsedURL, wg, visitedMutex, visited, taskQueue, baseHostname)
	log.Printf("Finished processing: %s", urlToCrawl)
}

func extractAndQueueLinks(node *html.Node, base *url.URL, wg *sync.WaitGroup, visitedMutex *sync.Mutex, visited map[string]bool, taskQueue chan<- string, baseHostname string) {
	if node == nil {
		return
	}

	if node.Type == html.ElementNode && node.Data == "a" {
		for _, attr := range node.Attr {
			if attr.Key == "href" {
				href := strings.TrimSpace(attr.Val)
				if href == "" {
					continue
				}

				absoluteURL := resolveAndFilter(href, base, baseHostname)

				if absoluteURL != "" {
					visitedMutex.Lock()
					if _, exists := visited[absoluteURL]; !exists {
						visited[absoluteURL] = true
						wg.Add(1)
						taskQueue <- absoluteURL
						log.Printf("QUEUED: %s (from %s)", absoluteURL, base.String())
					}
					visitedMutex.Unlock()
				}
				break
			}
		}
	}

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		extractAndQueueLinks(c, base, wg, visitedMutex, visited, taskQueue, baseHostname)
	}
}

func resolveAndFilter(href string, base *url.URL, baseHostname string) string {
	resolvedURL, err := base.Parse(href)
	if err != nil {
		return ""
	}

	if resolvedURL.Scheme != "http" && resolvedURL.Scheme != "https" {
		return ""
	}

	if resolvedURL.Hostname() != baseHostname {
		return ""
	}

	resolvedURL.Fragment = ""

	return resolvedURL.String()
}
