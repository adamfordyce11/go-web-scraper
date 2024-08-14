package fetcher

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// Spawn a test server using net/http/httptest
// Write a simple html structure out when it is queried with a number of links
// Pass the test server URL to the fetcher package and wait for the response
// Test the status code and body length of the response
func Test_Fetcher(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		html := `
		<html>
		<body>
		<p><a href="/">Link1</a></p>
		<p><a href="/home">Link2</a></p>
		<p><a href="/about">Link3</a></p>
		<p><a href="/blog">Link4</a></p>
		<p><a href="/contact">Link5</a></p>
		<p><a href="/privacy">Link6</a></p>
		<p><a href="/cookies">Link7</a></p>
		</body>
		</html>`
		fmt.Fprintf(w, html)
	}))
	defer ts.Close()

	output := make(chan string)
	errors := make(chan error)
	fetch := make(chan *http.Response)
	done := make(chan struct{})

	fetcher := NewFetcher(5, 3, 5*time.Second, output, errors, fetch, done)

	var wg sync.WaitGroup
	wg.Add(1)
	go fetcher.StartFetching(&wg)

	fetcher.NewRequest(ts.URL)

	select {
	case resp := <-fetch:
		if resp.StatusCode != 200 {
			t.Errorf("Invalid Status Code")
		}

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Errorf("Failed to read the http response body")
		}
		if len(body) == 0 {
			t.Error("The response body has an invalid size")
		}
	}
}
