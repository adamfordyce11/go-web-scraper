#=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-
# First Stage
#=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-
FROM golang:1.22-alpine3.20

# Set the working directory and copy the source files into the container
WORKDIR /app

COPY src .

# Build the Golang program with the following flags
# -a: force rebuilding of packages that are already up-to-date
# -mod readonly: disallow any changes to go.mod and go.sum
# -ldflags '-extldflags "-static"': link the Go program statically
# This will create a binary called printval
RUN CGO_ENABLED=0 GOOS=linux go build -a -mod readonly -ldflags '-extldflags "-static"' .

#=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-
# Second Stage
#=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-=-

# Multi stage container image, to reduce the size of the final image
FROM alpine:latest

# Install the required packages and update the certificates
RUN apk --no-cache add ca-certificates curl tzdata bash
RUN apk upgrade libssl3 libcrypto3
RUN update-ca-certificates

# Set the working directory and copy the binary from the first stage
COPY --from=0 /app/go-web-scraper /app/go-web-scraper

# Set the entrypoint for the container
CMD ["/app/go-web-scraper"]
