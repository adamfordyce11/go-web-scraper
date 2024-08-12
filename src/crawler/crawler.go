package crawler

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type Crawler struct {
	Domain *url.URL
	Out    chan<- string
	Err    chan<- error
}

func NewCrawler(domain string, output chan<- string, errors chan<- error) *Crawler {
	url, err := url.Parse(domain)
	if err != nil {
		errors <- fmt.Errorf("Error parsing domain")
		panic(err)
	}
	return &Crawler{
		Domain: url,
		Out:    output,
		Err:    errors,
	}
}

func (c *Crawler) CleanURL(rawUrl string) (string, error) {
	rawUrl = strings.TrimSpace(rawUrl)

	// If a fragment then return the host for the supplied domain
	if strings.HasPrefix(rawUrl, "#") {
		rawUrl = c.Domain.Scheme + "://" + c.Domain.Hostname()
	}

	// If the URL is relative, prepend the scheme and domain
	if strings.HasPrefix(rawUrl, "/") {
		rawUrl = c.Domain.Scheme + "://" + c.Domain.Hostname() + rawUrl
	}

	// Parse the URL
	u, err := url.Parse(rawUrl)
	if err != nil {
		return "", fmt.Errorf("Error parsing URL: %v", err)
	}

	// Check the requests hostname is in the same domain as the seed
	if u.Hostname() != c.Domain.Hostname() {
		return "", fmt.Errorf("request is for a subdomain or different domain")
	}

	// Set the protocol scheme to https if its not set
	if u.Scheme == "" {
		u.Scheme = "https"
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

func (c *Crawler) Crawl(url string) ([]string, error) {

	var found []string

	// Fetch the URL
	//fmt.Printf("Fetching %s\n", url)
	c.Out <- "Fetching " + url
	resp, err := http.Get(url)
	if err != nil {
		return found, fmt.Errorf("Error fetching URL: %v", err)
	}
	defer resp.Body.Close()

	header := resp.Header.Get("Content-Type")
	if !strings.Contains(header, "text/html") {
		return found, fmt.Errorf("URL is not for text/html: %s(%s)", url, header)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return found, fmt.Errorf("Error reading response body: %v", err)
	}

	// Mark the URL as seen (early)
	links, err := c.startFindLinks(body)
	if err != nil {
		return found, fmt.Errorf("Error finding links: %v", err)
	}

	c.Out <- fmt.Sprintf("%s", url)
	for _, link := range filteredLinks(links) {
		foundUrl, _ := c.CleanURL(link)
		found = append(found, foundUrl)
		c.Out <- fmt.Sprintf(" - %s", link)
	}
	return found, nil
}

func (c *Crawler) FindLinks(links []string, n *html.Node) []string {
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, a := range n.Attr {
			if a.Key != "href" {
				continue
			}
			links = append(links, a.Val)
		}
	}
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		links = c.FindLinks(links, child)
	}
	return links
}

func (c *Crawler) startFindLinks(body []byte) ([]string, error) {
	var links []string
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return []string{}, fmt.Errorf("Error parsing HTML: %v", err)
	}
	for _, a := range c.FindLinks(nil, doc) {
		url, err := c.CleanURL(a)
		if err != nil {
			// TODO: Do not ignore failed URL cleaning
			continue
		}
		links = append(links, url)
	}
	return links, nil
}
