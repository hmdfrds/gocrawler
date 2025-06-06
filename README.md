# GoCrawler

Basic concurrent web crawler.

## Features

* Accepts a starting seed URL as a command-line argument.
* Fetches the HTML content of the given URL.
* Parses the HTML to extract all hyperlink URLs found in `<a>` tags.
* **Link Filtering & Processing:**
  * Resolves relative URLs into absolute URLs.
  * Ignores non-HTTP/HTTPS URLs.
  * **Crucially:** Only crawls pages within the **same domain** as the seed URL.
  * Normalizes URLs (removes fragment identifiers like `#section`).
* Manages state (visited URLs) concurrently and safely using a map protected by a `sync.Mutex`.
* Uses multiple goroutines to fetch and process pages concurrently.
* Limits the maximum number of concurrent fetch operations using a buffered channel as a semaphore.
* Waits for all initiated crawl tasks to complete before exiting using `sync.WaitGroup`.
* Provides basic logging output for monitoring progress.

## Getting Started

1. **Build the executable:**

    ```bash
    go build .
    ```

    This will create an executable file named `gocrawler` (or `gocrawler.exe` on Windows).

2. **Run the crawler:**
    Provide the seed URL as a command-line argument.

    * **On Windows:**

        ```bash
        .\gocrawler.exe <seed_url>
        ```

    * **On Linux / macOS:**

        ```bash
        ./gocrawler <seed_url>
        ```

    **Example:**

    ```bash
    # Using Go By Example website (usually safe for simple crawlers)
    ./gocrawler [https://gobyexample.com/](https://gobyexample.com/)
    ```

## Important Note

⚠️ This crawler is intended **for educational purposes only** to demonstrate Go concurrency patterns.

* It does **not** respect `robots.txt` rules.
* It does not include politeness delays between requests.
* It has basic error handling.
