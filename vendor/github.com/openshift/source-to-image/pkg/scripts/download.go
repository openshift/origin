package scripts

import (
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"

	"github.com/openshift/source-to-image/pkg/api"
	s2ierr "github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/scm/git"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
)

var glog = utilglog.StderrLog

// Downloader downloads the specified URL to the target file location
type Downloader interface {
	Download(url *url.URL, target string) (*git.SourceInfo, error)
}

// schemeReader creates an io.Reader from the given url.
type schemeReader interface {
	Read(*url.URL) (io.ReadCloser, error)
}

type downloader struct {
	schemeReaders map[string]schemeReader
}

// NewDownloader creates an instance of the default Downloader implementation
func NewDownloader(proxyConfig *api.ProxyConfig) Downloader {
	httpReader := NewHTTPURLReader(proxyConfig)
	return &downloader{
		schemeReaders: map[string]schemeReader{
			"http":  httpReader,
			"https": httpReader,
			"file":  &FileURLReader{},
			"image": &ImageReader{},
		},
	}
}

// Download downloads the file pointed to by URL into local targetFile
// Returns information a boolean flag informing whether any download/copy operation
// happened and an error if there was a problem during that operation
func (d *downloader) Download(url *url.URL, targetFile string) (*git.SourceInfo, error) {
	r := d.schemeReaders[url.Scheme]
	info := &git.SourceInfo{}
	if r == nil {
		glog.Errorf("No URL handler found for %s", url.String())
		return nil, s2ierr.NewURLHandlerError(url.String())
	}

	reader, err := r.Read(url)
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	out, err := os.Create(targetFile)
	defer out.Close()

	if err != nil {
		glog.Errorf("Unable to create target file %s (%v)", targetFile, err)
		return nil, err
	}

	if _, err = io.Copy(out, reader); err != nil {
		os.Remove(targetFile)
		glog.Warningf("Skipping file %s due to error copying from source: %v", targetFile, err)
		return nil, err
	}

	glog.V(2).Infof("Downloaded '%s'", url.String())
	info.Location = url.String()
	return info, nil
}

// HTTPURLReader retrieves a response from a given HTTP(S) URL.
type HTTPURLReader struct {
	Get func(url string) (*http.Response, error)
}

var transportMap map[api.ProxyConfig]*http.Transport
var transportMapMutex sync.Mutex

func init() {
	transportMap = make(map[api.ProxyConfig]*http.Transport)
}

// NewHTTPURLReader returns a new HTTPURLReader.
func NewHTTPURLReader(proxyConfig *api.ProxyConfig) *HTTPURLReader {
	getFunc := http.Get
	if proxyConfig != nil {
		transportMapMutex.Lock()
		transport, ok := transportMap[*proxyConfig]
		if !ok {
			transport = &http.Transport{
				Proxy: func(req *http.Request) (*url.URL, error) {
					if proxyConfig.HTTPSProxy != nil && req.URL.Scheme == "https" {
						return proxyConfig.HTTPSProxy, nil
					}
					return proxyConfig.HTTPProxy, nil
				},
			}
			transportMap[*proxyConfig] = transport
		}
		transportMapMutex.Unlock()
		client := &http.Client{
			Transport: transport,
		}
		getFunc = client.Get
	}
	return &HTTPURLReader{Get: getFunc}
}

// Read produces an io.Reader from an http(s) URL.
func (h *HTTPURLReader) Read(url *url.URL) (io.ReadCloser, error) {
	resp, err := h.Get(url.String())
	if err != nil {
		if resp != nil {
			defer resp.Body.Close()
		}
		return nil, err
	}
	if resp.StatusCode == 200 || resp.StatusCode == 201 {
		return resp.Body, nil
	}
	return nil, s2ierr.NewDownloadError(url.String(), resp.StatusCode)
}

// FileURLReader opens a specified file and returns its stream
type FileURLReader struct{}

// Read produces an io.Reader from a file URL
func (*FileURLReader) Read(url *url.URL) (io.ReadCloser, error) {
	// for some reason url.Host may contain information about the ./ or ../ when
	// specifying relative path, thus using that value as well
	return os.Open(filepath.Join(url.Host, url.Path))
}

// ImageReader just returns information the URL is from inside the image
type ImageReader struct{}

// Read throws Not implemented error
func (*ImageReader) Read(url *url.URL) (io.ReadCloser, error) {
	return nil, s2ierr.NewScriptsInsideImageError(url.String())
}
