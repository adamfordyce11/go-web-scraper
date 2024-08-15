package crawler

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

var seedDomain string = "https://example.com"

// test function for the cleanUrl method in the crawler package
// take a list of URLs and test to make sure the output matches what
// is expected.
func Test_cleanUrl(t *testing.T) {
	// key is the URL to test
	// value is the expected result
	testCases := map[string]string{
		"example.com:8080":       "https://example.com:8080",
		"example.com":            "https://example.com",
		" example.com":           "https://example.com",
		"example.com/":           "https://example.com",
		"subdomain.example.com/": "",
		" https://example.com":   "https://example.com",
		"#fragment":              "https://example.com",
		"/relative/path":         "https://example.com/relative/path",
		"/":                      "https://example.com",
		"google.com":             "",
		"https://google.com":     "",
	}

	url, err := url.Parse(seedDomain)
	if err != nil {
		t.Errorf("Failed to parse seedDomain: %s", seedDomain)
	}
	c := Crawler{
		Domain: url,
	}

	for url, expected := range testCases {
		// Ignore the errors when calling cleanUrl, just want to test the output.
		cleaned, err := c.cleanUrl(url)

		if err != nil {
			t.Errorf("cleaned URL [%s] failed: %v", url, err)
		}

		// Fail the test if there is a mismatch between expected and cleaned
		if cleaned != expected {
			t.Errorf("cleaned URL [%s] does not match the expected [%s]", cleaned, expected)
		}
	}
}

func Test_filteredLinks(t *testing.T) {
	rawList := []string{
		"https://example.com",
		"https://example.com",
		"https://example.com",
		"https://example.com/path",
		"https://example.com/path",
		"https://example.com/path",
	}

	filteredList := filteredLinks(rawList)
	if len(filteredList) != 2 {
		t.Error("The filteredLinks test should only return two unique URLs")
	}

}

func Test_startFindLinks(t *testing.T) {
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

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Error("Failed to get html from httptest server")
	}
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Errorf("Failed to get body from html response: %v", err)
	}

	output := make(chan string)
	errors := make(chan error)
	fetch := make(chan *http.Response)
	c := NewCrawler(ts.URL, output, errors, fetch)
	links, err := c.startFindLinks(body)
	if err != nil {
		t.Errorf("Failed to get links from sample html: %v", err)
	}

	if len(links) != 7 {
		t.Errorf("Failed to get all links in sample html from %s: %d", ts.URL, len(links))
	}
}

func Test_ProcessResponse(t *testing.T) {
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

	go func() {
		for {
			select {
			case msg := <-output:
				fmt.Printf("%s\n", msg)
			case err := <-errors:
				fmt.Printf("%v\n", err)
			}
		}
	}()
	c := NewCrawler(ts.URL, output, errors, fetch)

	res, err := http.Get(ts.URL)
	if err != nil {
		t.Error("Failed to get html from httptest server")
	}

	links, err := c.ProcessResponse(res)
	if err != nil {
		t.Errorf("Failed to get links from sample html: %v", err)
	}

	if len(links) != 7 {
		t.Errorf("Failed to get all links in sample html from %s: %d", ts.URL, len(links))
	}

}
