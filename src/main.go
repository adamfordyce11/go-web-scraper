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
	"os"
	"runtime"
	"sync"
	"time"
)

// crawl will call the crawler Crawl method to get the requested URL and
// return a list of links on the page returned for the same domain as the
// one used to initialise the crawler object.
func crawl(c *crawler.Crawler, url string) []string {
	list, err := c.Crawl(url)
	if err != nil {
		c.Err <- err
	}
	return list
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
// TODO: Sometimes the initial seed call to the domain does not return a valid
// response, this is not currently dealt with.
//
// TODO: Optimise the code, further modularising it into packages as required
// TODO: Calculate some statistics for the crawling
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

	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case <-ticker.C:
				fmt.Printf("Spawned %d goroutines\n", runtime.NumGoroutine())
			}
		}
	}()

	// Initialise a new web crawler from our package.
	output := make(chan string)
	errors := make(chan error)
	c := crawler.NewCrawler(*domain, output, errors)

	worklist := make(chan []string)
	// Channel where to send urls that need to be scraped
	unseenUrls := make(chan string)
	// Signal channel to allow worker go routines to exit
	done := make(chan struct{})

	var wg sync.WaitGroup

	// Create a goroutine to print the output from the crawler object.
	// Note: this will receive data from multiple goroutines.
	go func() {
		for {
			select {
			case msg := <-output:
				fmt.Printf("[NORMAL] %v\n", msg)
			case err := <-errors:
				fmt.Printf("[ERROR] %v\n", err)
			}
		}
	}()

	// Take the supplied URL as the `seed` and enqueue it into the worklist
	// channel for processing.
	wg.Add(1)
	go func() {
		defer wg.Done()
		worklist <- []string{*domain}
	}()

	// Spawn the goroutines to form the worker pool.
	// TODO: Make the number of workers configurable
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case link, ok := <-unseenUrls:
					if !ok {
						return
					}
					foundLinks := crawl(c, link)
					wg.Add(1)
					go func() {
						defer wg.Done()
						worklist <- foundLinks
					}()
				case <-done:
					return
				}
			}
		}()
	}

	// Retrieve the data that is returned from the crawler.Crawl method and
	// process the links that are returned storing them and their visited state
	// in a thread safe data.Data.Links structure.
	// Unseen URLs are written to the unseenUrls channel
	visited := data.NewData()
	wg.Add(1)
	go func() {
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
	}()

	// Monitor for completion
	// The number of URLs (keys) in the data.Links map indicate the number of
	// URLs that have been found. Using this structure along with checking the
	// size of the worker queue gives indication when the program can exit
	// Three chances are given with a pregnant pause in between to make sure that
	// the crawling really is finished and that no more data will be written to
	// to the channels once they are closed.
	wg.Add(1)
	go func() {
		defer wg.Done()
		chances := 0
		lastSeen := 0
		lastVisited := 0
		for {

			time.Sleep(1 * time.Second)
			var seen []string

			// Lock the data structure from further read/writes before processing its
			// contents
			visited.Mu.Lock()

			// Store the URLs that have been crawled
			for url, been := range visited.Links {
				if been {
					seen = append(seen, url)
				}
			}

			// Reset the chances counter if the crawling starts again
			if len(seen) != len(visited.Links) {
				// DEBUG
				//fmt.Printf("Reset chances Seen(%d) != Visited(%d)", len(seen), len(visited.Links))
				chances = 0
			}

			if len(seen) == len(visited.Links) && len(seen) == lastSeen && len(visited.Links) == lastVisited && len(visited.Links) > 0 {
				chances++
				// DEBUG
				//fmt.Printf("Attempt %d of %d, No work queued, checking again in 3 seconds\n", chances, 3)
				time.Sleep(3 * time.Second)
			}

			// DEBUG
			//fmt.Printf("Visited [%d](%d) :: Seen[%d](%d) :: UnseenURLs(%d) Chances(%d)\n", lastVisited, len(visited.Links), lastSeen, len(seen), len(unseenUrls), chances)

			// Unlock the data structure
			visited.Mu.Unlock()

			// The conditions to exit the program have been met, stop all running
			// goroutines and exit
			if chances == 3 {
				fmt.Println("All work done. Exiting...")
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
	}()

	// Wait for the running goroutines that have been added to the WaitGroup
	// to finish then close the remaining channels.
	wg.Wait()
	close(errors)
	close(output)
}
