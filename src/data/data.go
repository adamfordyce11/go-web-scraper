package data

// The data package has been created to make the thread safe data.Data
// structure available to other packages.
// It returns an empty data.Data structure

import "sync"

// Data structure to hold the map of all the links that have been found and
// their value indicates if the link has or has not been scraped yet.
// The Mutex allows the structure to be locked so that only one process can
// read or write to the structure at any given moment.
type Data struct {
	Mu    *sync.Mutex
	Links map[string]bool
}

// NewData function returns a pointer to an empty data.Data structure
func NewData() *Data {
	return &Data{
		Mu:    &sync.Mutex{},
		Links: map[string]bool{},
	}
}
