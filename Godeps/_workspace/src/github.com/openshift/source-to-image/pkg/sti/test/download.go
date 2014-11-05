package test

import (
	"net/url"
	"sync"
)

type FakeDownloader struct {
	URL   []url.URL
	File  []string
	Err   map[string]error
	mutex sync.Mutex
}

func (f *FakeDownloader) DownloadFile(url *url.URL, targetFile string) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()

	f.URL = append(f.URL, *url)
	f.File = append(f.File, targetFile)

	return f.Err[url.String()]
}
