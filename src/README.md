# linkcrawl

## Synopsis

- Crawl a given URL and print out all links that are found on the page and traverse to each of the links that have been found. Only recurse for links with the same domain and do not traverse for subdomains.

## Dependencies

- golang version 1.20
- module: "net/http"
- module: "net/url"
-	module: "golang.org/x/net/html"

## Building

```go
go build . -o linkcrawl
```

## Running

```go
./linkcrawl -domain https://domain.com
```

## Testing

// TODO

## References

The Go Programming Language

## Author

Adam Fordyce - Aug 2024