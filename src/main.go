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
	"linkcrawl/data"
	"linkcrawl/fetcher"
	"net/http"
	"os"
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
			fmt.Printf("data,%v\n", msg)
		case err := <-errors:
			fmt.Printf("error,%v\n", err)
		case <-done:
			return
		}
	}
}

// Take the supplied URL as the `seed` and enqueue it into the worklist
// channel for processing.
func seed(domain *string, worklist chan<- []string, wg *sync.WaitGroup) {
	defer wg.Done()
	worklist <- []string{*domain}
}

func worker(c *crawler.Crawler, unseenUrls <-chan string, worklist chan<- []string, fetcher *fetcher.Fetcher, done chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case link, ok := <-unseenUrls:
			if !ok {
				return
			}

			fetcher.NewRequest(link)

			select {
			case resp := <-fetcher.Fetch:
				foundLinks, err := c.ProcessResponse(resp)
				if err != nil {
					fetcher.Err <- err
				} else {
					wg.Add(1)
					go func() {
						defer wg.Done()
						worklist <- foundLinks
					}()
				}
			case <-done:
				return
			}
		case <-done:
			return
		}
	}
}

// Retrieve the data that is returned from the crawler.Crawl method and
// process the links that are returned storing them and their visited state
// in a thread safe data.Data.Links structure.
// Unseen URLs are written to the unseenUrls channel
func cache(visited *data.Data, unseenUrls chan<- string, worklist <-chan []string, done <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case list, ok := <-worklist:
			if !ok {
				return
			}
			for _, link := range list {
				visited.Mu.Lock()
				if !visited.Links[link] {
					visited.Links[link] = true
					unseenUrls <- link
				}
				visited.Mu.Unlock()
			}
		case <-done:
			return
		}
	}
}

// Monitor for completion
// The number of URLs (keys) in the data.Links map indicate the number of
// URLs that have been found. Using this structure along with checking the
// size of the worker queue gives indication when the program can exit
// Three chances are given with a pregnant pause in between to make sure that
// the crawling really is finished and that no more data will be written to
// to the channels once they are closed.
func monitor(visited *data.Data, unseenUrls chan string, worklist chan []string, done chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	chances := 0
	lastSeen := 0
	lastVisited := 0
	for {
		time.Sleep(1 * time.Second)
		var seen []string

		// Lock data structure from further R/W before processing its contents
		visited.Mu.Lock()

		// Store the URLs that have been crawled
		for url, been := range visited.Links {
			if been {
				seen = append(seen, url)
			}
		}

		// Reset the chances counter if the crawling starts again
		if len(seen) != len(visited.Links) {
			chances = 0
		}

		if len(seen) == len(visited.Links) && len(seen) == lastSeen && len(visited.Links) == lastVisited && len(visited.Links) > 0 {
			chances++
			time.Sleep(3 * time.Second)
		}

		// Unlock the data structure
		visited.Mu.Unlock()

		// The conditions to exit the program have been met, stop all running
		// goroutines and exit
		if chances == 3 {
			close(done)
			close(worklist)
			close(unseenUrls)
			return
		}

		// Update the last seen and last visited variables use in the next loop
		// iteration
		lastSeen = len(seen)
		lastVisited = len(visited.Links)

		// Reset the seen list so that on the next loop iteration it is filled
		// with a fresh list of URLs that have now been scraped
		seen = seen[:0]
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

	/*
		// Debugging
		go func() {
			timer := time.NewTicker(1 * time.Second)
			for {
				select {
				case <-timer.C:
					fmt.Printf("process,%d goroutines\n", runtime.NumGoroutine())
				}
			}
		}()
	*/
	domain := flag.String("domain", "", "The domain to crawl")
	flag.Parse()

	if *domain == "" {
		fmt.Printf("Error, please pass a domain using -domain https://domain.com")
		os.Exit(1)
	}

	var wg sync.WaitGroup
	visited := data.NewData()
	worklist := make(chan []string) // Data returned from crawling
	unseenUrls := make(chan string) // URLs to scrape
	done := make(chan struct{})     // Signal go routines to exit
	output := make(chan string)     // Channel to send output to
	errors := make(chan error)      // Channel to send errors to
	fetch := make(chan *http.Response)

	// Initialise a new web crawler from the crawler package.
	c := crawler.NewCrawler(*domain, output, errors, fetch)

	fetcher := fetcher.NewFetcher(5, 3, 5*time.Second, output, errors, fetch, done)

	wg.Add(1)
	go fetcher.StartFetching(&wg)

	// Spawn the goroutines to form the worker pool.
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go worker(c, unseenUrls, worklist, fetcher, done, &wg)
	}

	wg.Add(4)
	go stream(output, errors, done, &wg)
	go seed(domain, worklist, &wg)
	go cache(visited, unseenUrls, worklist, done, &wg)
	go monitor(visited, unseenUrls, worklist, done, &wg)

	wg.Wait() // Wait for the processing to complete
	close(fetch)
	close(errors)
	close(output)
}
