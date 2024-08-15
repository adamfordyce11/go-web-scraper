package main

// Entry point and main routine structure for the linkcrawl module
// This will take a given URL and so long as it is valid, it will
// crawl through all the links from the anchor fields, printing them to
// stdout until of the pages and the links they contain have been printed
// It does not recurse to subdomains or to URLs that have a different domain
// to the one that has been supplied.

import (
	"flag"
	"fmt"
	"linkcrawl/crawler"
	"linkcrawl/fetcher"
	"linkcrawl/fronter"
	"net/http"
	"os"
	"runtime"
	"sync"
	"time"
)

// Create a goroutine to print the output from the crawler object.
// Note: this will receive data from multiple goroutines.
func stream(output <-chan string, errors <-chan error, done <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case msg := <-output:
			if 1 > 0 {
				fmt.Printf("data,%v\n", msg)
			}
		case err := <-errors:
			fmt.Printf("error,%v\n", err)
		case <-done:
			return
		}
	}
}

// main function - This performs the following steps
//   - Parses and checks for user input to get the domain
//   - Creates channels for logging output and errors
//   - Initialises a new Crawler object from the crawler package
//   - Create a worklist channel to receive all the URLs from the crawling
//   - Create an unseenUrls channel for non-scraped URLs to be passed to while
//     processing the data from the worklist channel
//   - spawn a goroutine to receive logging and errors
//   - spawn a goroutine to write the starting(seed) URL to the worklist chan
//   - spawn a 20 worker goroutines to handle concurrent URL Crawling
//   - Initialise a new Data object from the data package, it uses a mutex so
//     it can be locked and unlocked to protect it from concurrent R/W access
//   - spawn a goroutine to Process the output from the calls to crawler.Crawl
//     method, writing any unseen URLs to the unseenUrls channel
//   - Monitor the status of the crawling and when no new URLs are being
//     scraped, close the channels, signal to the waitgroup that the processing
//     is done and exit.
//
// References: The Go Programming Language: Section 8.6
// ISBN-10: 0-13-419044-0
func main() {
	domain := flag.String("domain", "", "The domain to crawl")
	flag.Parse()

	if *domain == "" {
		fmt.Printf("Error, please pass a domain using -domain https://domain.com")
		os.Exit(1)
	}

	var wgStream sync.WaitGroup
	var wgFetch sync.WaitGroup
	var wgFront sync.WaitGroup
	done := make(chan struct{}) // Signal go routines to exit
	output := make(chan string) // Channel to send output to
	errors := make(chan error)  // Channel to send errors to
	fetch := make(chan *http.Response)

	// Initialise a new web crawler from the crawler package.
	crawler := crawler.NewCrawler(*domain, output, errors, fetch)
	fetcher := fetcher.NewFetcher(10, 3, 5*time.Second, output, errors, fetch, done)
	fronter := fronter.NewFronter(20, 3, 5*time.Second, fetcher, crawler, output, errors, done)

	wgStream.Add(1)
	go stream(output, errors, done, &wgStream)
	fetcher.StartFetching(&wgFetch)
	fronter.StartFronting(&wgFront)
	fronter.Seed(*domain)

	timer := time.NewTicker(1 * time.Second)
	//locked := false
	for {
		select {
		case <-timer.C:
			fmt.Printf("process,%d goroutines\n", runtime.NumGoroutine())

		case <-done:
			wgStream.Wait() // Wait for the processing to complete
			fmt.Printf("process,wgStream done - %d goroutines\n", runtime.NumGoroutine())
			wgFetch.Wait() // Wait for the processing to complete
			fmt.Printf("process,wgFetch done - %d goroutines\n", runtime.NumGoroutine())
			wgFront.Wait() // Wait for the processing to complete
			fmt.Printf("process,wgFront done - %d goroutines\n", runtime.NumGoroutine())
			close(fetch)
			close(errors)
			close(output)
			return
		}
	}

}
