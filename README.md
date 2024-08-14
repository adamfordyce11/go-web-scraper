# Go Web Scraper

[![Open in Gitpod](https://gitpod.io/button/open-in-gitpod.svg)](https://gitpod.io/#https://github.com/adamfordyce11/go-web-scraper)

## Synopsis

linkcrawl - crawl a given url, recursing to each link found and printing the output to stdout

## Description

The linkcrawl program will take a given seed URL for a domain and scrape all of the links from the anchor nodes href attribute.
It keeps track of the links that have been scraped and the links that have been discovered but not yet scraped.

It only crawls through links that are for the same domain as the seed however it does not crawl through subdomains.

The program uses concurrency to speedup the processing of the pages, it is limited in the number of concurrent http.Get requests it can make to a domain so as not to overload it.
If a get request times out it pauses and retries up to three times before it gives up in that particular URL.

The output is formatted to with comma separated values so that it can be loaded into a program such as excel to filter and sort the results. There are two output types, data and error.

Sample output:

```text
normal,200,https://domain.com/,https://domain.com/path1
normal,200,https://domain.com/,https://domain.com/path2
normal,200,https://domain.com/,https://domain.com/path3
normal,200,https://domain.com/,https://domain.com/path4
```

Errors will be output for requests that have:

- timed out
- failed to parse as a URL
- where the content type is invalid

## Setup

1. Install Golang version >= 1.20.
2. Install Docker or a similar container runtime.

## Dependencies

- golang version 1.20
- module: "net/http"
- module: "net/http/httptest
- module: "net/url"
- module: "golang.org/x/net/html"

Installing the modules:

```bash
cd /path/to/code
go mod tidy
```

## Building

```go
go build . -o linkcrawl
```

## Running

```bash
go run main.go -domain https://domain.com
# [CTRL+C] to exit
```

### From the compiled binary

```bash
./linkcrawl -domain https://domain.com
# [CTRL+C] to exit
```

## Testing

```bash
go test ./...
go test ./... -cover # Show test coverage to stdout
```

Note: The test coverage could be improved


## Using a container

### Building the container

Review the contents of the Dockerfile, this has the essential steps of taking the source code and building the application in a multi stage Dockerfile.

```bash
cd go-web-scraper
docker build . -t go-web-scraper:latest
```

### Verify the container using docker

```bash
# Start the container in the background
container_id=$(docker run -it --rm --detach go-web-scraper:latest)

# Execute a command in the container
docker exec $container_id /app/go-web-scraper

# Review container logs
docker logs $container_id

# Review processes running inside the container
docker top $container_id
```

## References

- The Go Programming Language: Sections 5 and 8
(ISBN-10: 0-13-419044-0)
- General Documentation: [https://pkg.go.dev](https://pkg.go.dev/)
- ChatGPT - conversational input around some concurrency and testing elements i.e. show examples of using `net/http/httptest`

## Author

Adam Fordyce - Aug 2024
