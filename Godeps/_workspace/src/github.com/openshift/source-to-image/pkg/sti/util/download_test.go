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

type FakeHTTPGet struct {
	url        string
	content    string
	err        error
	body       io.ReadCloser
	statusCode int
}

func (f *FakeHTTPGet) get(url string) (*http.Response, error) {
	f.url = url
	f.body = ioutil.NopCloser(strings.NewReader(f.content))
	return &http.Response{
		Body:       f.body,
		StatusCode: f.statusCode,
	}, f.err
}

func getHTTPReader() (*HttpURLReader, *FakeHTTPGet) {
	sr := &HttpURLReader{}
	g := &FakeHTTPGet{content: "test content", statusCode: 200}
	sr.httpGet = g.get
	return sr, g
}

func TestHTTPRead(t *testing.T) {
	u, _ := url.Parse("http://test.url/test")
	sr, fg := getHTTPReader()
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
	sr, fg := getHTTPReader()
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
	sr, fg := getHTTPReader()
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
	return ioutil.NopCloser(strings.NewReader(f.content)), f.err
}

func (f *FakeSchemeReader) IsFromImage() bool {
	return false
}

type FakeImageSchemeReader struct{}

func (f *FakeImageSchemeReader) Read(url *url.URL) (io.ReadCloser, error) {
	return nil, nil
}

func (f *FakeImageSchemeReader) IsFromImage() bool {
	return true
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
	_, err = dl.DownloadFile(u, temp.Name())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	content, _ := ioutil.ReadFile(temp.Name())
	if string(content) != fr.content {
		t.Errorf("Unexpected file content: %s", string(content))
	}
}

func TestNoDownloadFile(t *testing.T) {
	dl := &downloader{
		schemeReaders: map[string]schemeReader{
			"image": &FakeImageSchemeReader{},
		},
	}
	u, _ := url.Parse("image:///tmp/testfile")
	download, err := dl.DownloadFile(u, "")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if download {
		t.Error("Expected no download, but got true!")
	}
}

func TestNoDownloader(t *testing.T) {
	dl := &downloader{
		schemeReaders: map[string]schemeReader{},
	}
	u, _ := url.Parse("http://www.test.url/a/file")
	_, err := dl.DownloadFile(u, "")
	if err == nil {
		t.Errorf("Expected error, got nil!")
	}
}
