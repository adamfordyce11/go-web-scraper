package data

import (
	"sync"
	"testing"
)

// Test the creation of the data.Data structure returned by
// the NewData function.
func Test_NewData(t *testing.T) {
	d := NewData()

	if d.Mu == nil {
		t.Error("The mutex is not initialised in the data.Data structure")
	}

	if d.Links == nil {
		t.Error("The Links map is not initialised in the data.Data structure")
	}
}

// Test concurrent access to the data.Data structure
func Test_Concurrency(t *testing.T) {
	d := NewData()

	var wg sync.WaitGroup

	url := "https://example.com"

	for i := 0; i < 100; i++ {
		wg.Add(1)
		state := false
		if i%2 == 0 {
			state = true
		}
		go func() {
			defer wg.Done()
			d.Mu.Lock()
			d.Links[url] = state
			d.Mu.Unlock()
		}()

	}

	wg.Wait()

	if _, exists := d.Links[url]; !exists {
		t.Error("The url " + url + " is expected to be in the data.Data.Links structure, but it isn't")
	}
}
