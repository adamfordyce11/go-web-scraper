package fetcher

// Make concurrent http get requests and return the http.Response to
// the calling functions.

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Fetcher structure holds the configuration and the channels that can be
// accessed by the package methods.
type Fetcher struct {
	Workers    int
	Timeout    time.Duration
	RetryCount int
	Fetch      chan *http.Response
	Err        chan<- error
	Out        chan<- string
	Requests   chan string
	Done       chan struct{}
}

// Initialise a fetcher.Fetcher object, accepting parameters from the calling
// function.
func NewFetcher(workers, retries int, timeout time.Duration, output chan<- string, errors chan<- error, fetch chan *http.Response, done chan struct{}) *Fetcher {
	requests := make(chan string)
	fetcher := &Fetcher{
		Workers:    workers,
		RetryCount: retries,
		Timeout:    timeout,
		Out:        output,
		Err:        errors,
		Fetch:      fetch,
		Requests:   requests,
		Done:       done,
	}
	return fetcher
}

// Convenience method to allow a new http.Get request to be made
func (f *Fetcher) NewRequest(url string) {
	f.Requests <- url
}

// Create a worker pool ready to start fetching URLs when the
// fetcher.Requests channel is updated
func (f *Fetcher) StartFetching(wg *sync.WaitGroup) {
	defer wg.Done()
	for i := 0; i < f.Workers; i++ {
		wg.Add(1)
		go f.worker(wg)
	}
	wg.Wait()
}

// worker - private method that will perform the http.Get requests
// the http.Response is written to the fetcher.Fetch channel
func (f *Fetcher) worker(wg *sync.WaitGroup) {
	defer wg.Done()

	for {
		select {
		case <-f.Done:
			return
		case url, ok := <-f.Requests:
			if !ok {
				return
			}

			var resp *http.Response
			var err error

			for retries := 0; retries <= f.RetryCount; retries++ {
				ctx, cancel := context.WithTimeout(context.Background(), f.Timeout)
				defer cancel()

				resp, err = http.Get(url)
				if err != nil {
					f.Err <- fmt.Errorf("Failed to fetch: %v", err)
					if retries < f.RetryCount {
						time.Sleep(1 * time.Second)
						continue
					}
					break
				}

				if ctx.Err() == context.DeadlineExceeded {
					f.Err <- fmt.Errorf("Timed out fetching %s after %d retries", url, retries)
					if retries < f.RetryCount {
						time.Sleep(1 * time.Second)
						continue
					}
					break
				}

				f.Fetch <- resp
				break
			}
			if resp != nil {
				//f.Out <- fmt.Sprintf("Fetched %s successfully", url)
			}
		}
	}
}
