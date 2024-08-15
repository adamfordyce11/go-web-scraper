package crawler

// crawler package implements the Crawl method which accepts a seed url
// All text/html content from the seed is parsed and links are scraped from
// the html body, the calling function receives a list of all the links
// that were found on the page that was crawled and scraped.

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

// The Crawler struct contains the seed domain to start crawling and the
// channels used for error and standard output reporting.
type Crawler struct {
	Domain *url.URL
	Out    chan<- string
	Err    chan<- error
	Fetch  chan<- *http.Response
}

// NewCrawler, returns a pointer to a crawler.Crawler object, it is initialised
// with a seed domain, output channel and an error channel
func NewCrawler(domain string, output chan<- string, errors chan<- error, fetch chan<- *http.Response) *Crawler {
	// The seed domain is stored in a url.URL object, so it is parsed to begin
	// with, if there is an error then the code panics and exits so that it
	// does not proceed with a bad "seed".
	url, err := url.Parse(domain)
	if err != nil {
		errors <- fmt.Errorf("Error parsing domain")
		panic(err)
	}

	return &Crawler{
		Domain: url,
		Out:    output,
		Err:    errors,
		Fetch:  fetch,
	}
}

// The private cleanUrl - method will take a rawUrl that has been scraped
// from an HTML page from the anchor nodes href attribute and run the following
// steps on it.
//   - Strip any leading or trailing spaces, this can confuse the net/url Parser
//   - Detect if the link is a fragment i.e. #something, if it is then return the
//     seed domain.
//   - Detect if the link supplied is a relative path, if it is then rebuild the
//     url from the seed domain i.e. /home/blog -> https://domain.com/home/blog
//   - Use the net/url url.Parse method to load the url into a url.URL object
//   - Check and ensure the domain in the URL is the same as the one supplied in
//     the seed.
//   - Ensure the protocol scheme is set on the URL, if not then use "https"
//
// Once all the checks have been complete, the url is reconstructed to ensure
// there are no trailing `/` and to add any query string back onto it.
func (c *Crawler) cleanUrl(rawUrl string) (string, error) {
	rawUrl = strings.TrimSpace(rawUrl)

	// If a fragment then return the host for the supplied domain
	if strings.HasPrefix(rawUrl, "#") {
		rawUrl = c.Domain.Scheme + "://" + c.Domain.Hostname()
	}

	// If the URL is relative, prepend the scheme and domain
	if strings.HasPrefix(rawUrl, "/") {
		rawUrl = c.Domain.Scheme + "://" + c.Domain.Hostname() + rawUrl
	}

	if strings.Contains(rawUrl, "127.0.0.1") && !strings.HasPrefix(rawUrl, "http") {
		rawUrl = "http://" + rawUrl
	}

	if !strings.HasPrefix(rawUrl, "http") {
		rawUrl = "https://" + rawUrl
	}

	// Parse the URL
	u, err := url.Parse(rawUrl)
	if err != nil {
		return "", fmt.Errorf("Error parsing URL: %v", err)
	}

	// Set the protocol scheme to https if its not set
	if u.Scheme == "" {
		if u.Host == "127.0.0.1" {
			u.Scheme = "http"
		} else {
			u.Scheme = "https"
		}
	}

	// Check the requests hostname is in the same domain as the seed
	if u.Hostname() != c.Domain.Hostname() {
		return "", nil
	}

	query := ""
	if len(u.RawQuery) > 0 {
		query = "?" + u.RawQuery
	}

	path := ""
	if len(u.Path) > 0 && u.Path != "/" {
		path = u.Path
	}

	// Reconstruct the URL to ensure it's normalized, i.e. no fragments, no relative paths etc
	return u.Scheme + "://" + u.Host + path + query, nil
}

// fitleredLinks takes a list of URLs that may contain duplicates or empty
// values. The function should remove any empty strings and de-duplicate the
// entries, returning a list of unique URLs to the calling function.
func filteredLinks(links []string) []string {
	uniqueLinks := []string{}
	seen := make(map[string]bool)
	for _, link := range links {
		// Don't want empty links
		if len(link) == 0 {
			continue
		}
		// Get the unique links
		if _, exists := seen[link]; !exists {
			seen[link] = true
			uniqueLinks = append(uniqueLinks, link)
		}
	}
	return uniqueLinks
}

// The ProcessResponse method accepts the response from an http.Get request
// The body is extracted from the response and processed to
// locate all of the links in the html body.
// If the content type is not as expected or the body is not able to be read
// Then an error is returned
func (c *Crawler) ProcessResponse(resp *http.Response) ([]string, error) {
	defer resp.Body.Close()
	url := resp.Request.RequestURI
	if len(url) == 0 {
		url = resp.Request.Response.Request.URL.String()
	}
	var found []string

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "text/plain") {
		return found, fmt.Errorf("%d,%s,Invalid Content Type: %s", resp.StatusCode, url, contentType)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return found, fmt.Errorf("%d,Error reading response body: %v", resp.StatusCode, err)
	}

	// Parse through the body and return all the links that have been found
	links, err := c.startFindLinks(body)
	if err != nil {
		return found, fmt.Errorf("%d,Error finding links: %v", resp.StatusCode, err)
	}

	// Send all the unique links found to the output
	for _, link := range filteredLinks(links) {
		foundUrl, _ := c.cleanUrl(link)
		found = append(found, foundUrl)
		c.Out <- fmt.Sprintf("%d,%s,%s", resp.StatusCode, url, link)
	}

	// Return the found URLs to enqueue for future processing
	return found, nil
}

// startFindLinks takes the html body inside a []byte slice and get an
// html.Node using html.Parse
// - recurse through all the elements in the html.Node
// - for each link that is discovered, clean the URLs
// - return a []string with all the URLs discovered
func (c *Crawler) startFindLinks(body []byte) ([]string, error) {
	var links []string
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return []string{}, fmt.Errorf("Error parsing HTML: %v", err)
	}
	for _, a := range c.findLinks(nil, doc) {
		url, err := c.cleanUrl(a)
		if err != nil {
			// TODO: Do not ignore failed URL cleaning
			continue
		}
		links = append(links, url)
	}
	return links, nil
}

// findLinks extracts all the anchor elements in an html node, extracts the
// href attribute and updates the slice passed in with the links on it, it
// the html node is looped over and if there are more children in the node, it
// recurses calling itself until all the nodes have been seen and had their
// links extracted.
// It returns a slice with all the links that were found in the seed html node
func (c *Crawler) findLinks(links []string, n *html.Node) []string {
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key != "href" {
				continue
			}
			links = append(links, a.Val)
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		links = c.findLinks(links, child)
	}
	return links
}
