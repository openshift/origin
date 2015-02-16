package test

import (
	"net/url"
	"sync"
)

// FakeDownloader provides a fake downloader interface
type FakeDownloader struct {
	URL   []url.URL
	File  []string
	Err   map[string]error
	mutex sync.Mutex
}

// DownloadFile downloads a fake file from the URL
func (f *FakeDownloader) DownloadFile(url *url.URL, targetFile string) (bool, error) {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.URL = append(f.URL, *url)
	f.File = append(f.File, targetFile)

	return true, f.Err[url.String()]
}
