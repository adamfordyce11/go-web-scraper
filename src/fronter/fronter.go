package fronter

import (
	"fmt"
	"linkcrawl/crawler"
	"linkcrawl/data"
	"linkcrawl/fetcher"
	"sync"
	"time"
)

type Fronter struct {
	Workers   int
	Retries   int
	RetryWait time.Duration
	Out       chan string
	Err       chan error
	Done      chan struct{}
	Worklist  chan []string
	Seen      *data.Data
	Fetcher   *fetcher.Fetcher
	Crawler   *crawler.Crawler
	Unseen    chan string
}

func NewFronter(workers int, retries int, pause time.Duration, fetcher *fetcher.Fetcher, crawler *crawler.Crawler, out chan string, err chan error, done chan struct{}) *Fronter {

	worklist := make(chan []string)
	unseen := make(chan string)
	seen := data.NewData()

	return &Fronter{
		Workers:   workers,
		Retries:   retries,
		RetryWait: pause,
		Out:       out,
		Err:       err,
		Done:      done,
		Worklist:  worklist,
		Seen:      seen,
		Unseen:    unseen,
		Crawler:   crawler,
		Fetcher:   fetcher,
	}
}

// Take the supplied URL as the `seed` and enqueue it into the worklist
// channel for processing.
func (f *Fronter) Seed(domain string) {
	go func() {
		f.Worklist <- []string{domain}
	}()
}

func (f *Fronter) worker(wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case link, ok := <-f.Unseen:
			if !ok {
				return
			}

			f.Fetcher.NewRequest(link)
			select {
			case resp := <-f.Fetcher.Fetch:
				foundLinks, err := f.Crawler.ProcessResponse(resp)
				if err != nil {
					f.Err <- err
				} else {
					go func() {
						f.Worklist <- foundLinks
					}()
				}
			case <-f.Done:
				return
			}
		case <-f.Done:
			return
		}
	}
}

// Retrieve the data that is returned from the crawler.Crawl method and
// process the links that are returned storing them and their visited state
// in a thread safe data.Data.Links structure.
// Unseen URLs are written to the unseenUrls channel
func (f *Fronter) cache(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case list, ok := <-f.Worklist:
				if !ok {
					return
				}
				for _, link := range list {
					f.Seen.Mu.Lock()
					if !f.Seen.Links[link] {
						f.Seen.Links[link] = false
						f.Unseen <- link
					}
					f.Seen.Mu.Unlock()
				}
			case <-f.Done:
				return
			}
		}
	}()
}

// Monitor for completion
// The number of URLs (keys) in the data.Links map indicate the number of
// URLs that have been found. Using this structure along with checking the
// size of the worker queue gives indication when the program can exit
// Three chances are given with a pregnant pause in between to make sure that
// the crawling really is finished and that no more data will be written to
// to the channels once they are closed.
func (f *Fronter) monitor(wg *sync.WaitGroup) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		chances := 0
		lastSeen := 0
		lastVisited := 0
		for {
			time.Sleep(1 * time.Second)
			var seen []string

			// Lock data structure from further R/W before processing its contents
			f.Seen.Mu.Lock()

			// Store the URLs that have been crawled
			for url, been := range f.Seen.Links {
				if been {
					seen = append(seen, url)
				}
			}

			// Reset the chances counter if the crawling starts again
			if len(seen) != len(f.Seen.Links) {
				chances = 0
			}

			if len(seen) == len(f.Seen.Links) && len(seen) == lastSeen && len(f.Seen.Links) == lastVisited && len(f.Seen.Links) > 0 {
				chances++
				time.Sleep(f.RetryWait)
			}

			// Unlock the data structure
			f.Seen.Mu.Unlock()

			// The conditions to exit the program have been met, stop all running
			// goroutines and exit
			if chances == f.Retries {
				close(f.Done)
				fmt.Printf("done signalled\n")
				close(f.Worklist)
				fmt.Printf("worklist signalled\n")
				close(f.Unseen)
				fmt.Printf("unseen signalled\n")
				return
			}

			// Update the last seen and last visited variables use in the next loop
			// iteration
			lastSeen = len(seen)
			lastVisited = len(f.Seen.Links)
		}
	}()
}

func (f *Fronter) StartFronting(wg *sync.WaitGroup) {
	f.cache(wg)
	f.monitor(wg)

	for i := 0; i < f.Workers; i++ {
		wg.Add(1)
		go f.worker(wg)
	}
}
