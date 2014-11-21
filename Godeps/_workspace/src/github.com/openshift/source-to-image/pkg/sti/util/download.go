package util

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/golang/glog"
)

// Downloader downloads the specified URL to the target file location
type Downloader interface {
	DownloadFile(url *url.URL, targetFile string) error
}

// schemeReader creates an io.Reader from the given url.
type schemeReader interface {
	Read(*url.URL) (io.ReadCloser, error)
}

// HttpURLReader retrieves a response from a given http(s) URL
type HttpURLReader struct {
	httpGet func(url string) (*http.Response, error)
}

// FileURLReader opens a specified file and returns its stream
type FileURLReader struct{}

type downloader struct {
	schemeReaders map[string]schemeReader
}

// NewDownloader creates an instance of the default Downloader implementation
func NewDownloader() Downloader {
	httpReader := NewHttpReader()
	return &downloader{
		schemeReaders: map[string]schemeReader{
			"http":  httpReader,
			"https": httpReader,
			"file":  &FileURLReader{},
		},
	}
}

// NewHttpReader creates an instance of the HttpURLReader
func NewHttpReader() schemeReader {
	r := &HttpURLReader{}
	r.httpGet = http.Get
	return r
}

// Read produces an io.Reader from an http(s) URL.
func (h *HttpURLReader) Read(url *url.URL) (io.ReadCloser, error) {
	resp, err := h.httpGet(url.String())
	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
		}
		return nil, err
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return resp.Body, nil
	} else {
		return nil, fmt.Errorf("Failed to retrieve %s, response code %d", url.String(), resp.StatusCode)
	}
}

// Read produces an io.Reader from a file URL
func (f *FileURLReader) Read(url *url.URL) (io.ReadCloser, error) {
	return os.Open(url.Path)
}

// DownloadFile downloads the file pointed to by URL into local targetFile
func (d *downloader) DownloadFile(url *url.URL, targetFile string) error {
	sr := d.schemeReaders[url.Scheme]

	if sr == nil {
		glog.Errorf("No URL handler found for %s", url.String())
		return fmt.Errorf("No URL handler found url %s", url.String())
	}
	reader, err := sr.Read(url)
	if err != nil {
		glog.V(2).Infof("Unable to download %s (%s)", url.String(), err)
		return err
	}
	defer reader.Close()

	out, err := os.Create(targetFile)
	defer out.Close()

	if err != nil {
		defer os.Remove(targetFile)
		glog.Errorf("Unable to create target file %s (%s)", targetFile, err)
		return err
	}

	if _, err = io.Copy(out, reader); err != nil {
		defer os.Remove(targetFile)
		glog.Errorf("Skipping file %s due to error copying from source: %s", targetFile, err)
	}

	glog.V(2).Infof("Downloaded '%s'", url.String())
	return nil
}
