package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
)

type FakeHttpGet struct {
	url        string
	content    string
	err        error
	body       io.ReadCloser
	statusCode int
}

type FakeCloser struct {
	io.Reader
}

func (f *FakeCloser) Close() error {
	// No-op
	return nil
}

func (f *FakeHttpGet) get(url string) (*http.Response, error) {
	f.url = url
	f.body = &FakeCloser{strings.NewReader(f.content)}
	return &http.Response{
		Body:       f.body,
		StatusCode: f.statusCode,
	}, f.err
}

func getHttpReader() (*HttpURLReader, *FakeHttpGet) {
	sr := &HttpURLReader{}
	g := &FakeHttpGet{content: "test content", statusCode: 200}
	sr.httpGet = g.get
	return sr, g
}

func TestHTTPRead(t *testing.T) {
	u, _ := url.Parse("http://test.url/test")
	sr, fg := getHttpReader()
	rc, err := sr.Read(u)
	if rc != fg.body {
		t.Errorf("Unexpected readcloser returned: %#v", rc)
	}
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
}

func TestHTTPReadGetError(t *testing.T) {
	u, _ := url.Parse("http://test.url/test")
	sr, fg := getHttpReader()
	fg.err = fmt.Errorf("URL Error")
	rc, err := sr.Read(u)
	if rc != nil {
		t.Errorf("Unexpected stream returned: %#v", rc)
	}
	if err != fg.err {
		t.Errorf("Unexpected error returned: %#v", err)
	}
}

func TestHTTPReadErrorCode(t *testing.T) {
	u, _ := url.Parse("http://test.url/test")
	sr, fg := getHttpReader()
	fg.statusCode = 500
	rc, err := sr.Read(u)
	if rc != nil {
		t.Errorf("Unexpected stream returned: %#v", rc)
	}
	if err == nil {
		t.Errorf("Error expeccted and not returned")
	}
}

type FakeSchemeReader struct {
	content string
	err     error
}

func (f *FakeSchemeReader) Read(url *url.URL) (io.ReadCloser, error) {
	return &FakeCloser{strings.NewReader(f.content)}, f.err
}

func getDownloader() (Downloader, *FakeSchemeReader) {
	fakeReader := &FakeSchemeReader{}
	return &downloader{
		schemeReaders: map[string]schemeReader{
			"http":  fakeReader,
			"https": fakeReader,
			"file":  fakeReader,
		},
	}, fakeReader
}

func TestDownloadFile(t *testing.T) {
	dl, fr := getDownloader()
	fr.content = "test file content"
	temp, err := ioutil.TempFile("", "testdownload")
	if err != nil {
		t.Fatalf("Cannot create temp directory for test: %v", err)
	}
	defer os.Remove(temp.Name())
	u, _ := url.Parse("http://www.test.url/a/file")
	temp.Close()
	err = dl.DownloadFile(u, temp.Name())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	content, _ := ioutil.ReadFile(temp.Name())
	if string(content) != fr.content {
		t.Errorf("Unexpected file content: %s", string(content))
	}
}
