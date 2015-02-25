package test

import (
	"net/url"
	"sync"
)

// FakeDownloader provides a fake downloader interface
type FakeDownloader struct {
	URL    []url.URL
	Target []string
	Err    map[string]error
	mutex  sync.Mutex
}

// Download downloads a fake file from the URL
func (f *FakeDownloader) Download(url *url.URL, target string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.URL = append(f.URL, *url)
	f.Target = append(f.Target, target)

	return f.Err[url.String()]
}
