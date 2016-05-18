package assets

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func stubHandler(response string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	})
}

func TestWebConsoleConfigTemplate(t *testing.T) {
	handler, err := GeneratedConfigHandler(WebConsoleConfig{}, WebConsoleVersion{}, WebConsoleExtensionProperties{})
	if err != nil {
		t.Fatalf("expected a handler, got error %v", err)
	}
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, &http.Request{Method: "GET"})
	if writer.Body == nil {
		t.Fatal("expected a body")
	}
	response := writer.Body.String()
	if !strings.Contains(response, "OPENSHIFT_CONFIG") {
		t.Errorf("body does not have OPENSHIFT_CONFIG:\n%s", response)
	}
	if strings.Contains(response, "limitRequestOverrides") {
		t.Errorf("LimitRequestOverrides should be omitted from the body:\n%s", response)
	}
}

func TestWithoutGzip(t *testing.T) {
	const resp = "hello"
	handler := GzipHandler(stubHandler(resp))
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, &http.Request{Method: "GET"})
	if writer.Body == nil {
		t.Fatal("expected a body")
	}
	if l := writer.Body.Len(); l != len(resp) {
		t.Fatalf("invalid body length, got %d", l)
	}
	vary := writer.Header()["Vary"]
	if !reflect.DeepEqual(vary, []string{"Accept-Encoding"}) {
		t.Fatalf("expected a Vary header with value Accept-Encoding, got %v", vary)
	}
}

func TestWithoutGzipWithMultipleVaryHeaders(t *testing.T) {
	const resp = "hello"
	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Foo")
		w.Write([]byte(resp))
	}))
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, &http.Request{Method: "GET"})
	if writer.Body == nil {
		t.Fatal("expected a body")
	}
	if l := writer.Body.Len(); l != len(resp) {
		t.Fatalf("invalid body length, got %d", l)
	}
	vary := writer.Header()["Vary"]
	if !reflect.DeepEqual(vary, []string{"Accept-Encoding", "Foo"}) {
		t.Fatalf("invalid Vary headers, got %#v", vary)
	}
}

func TestWithGzip(t *testing.T) {
	handler := GzipHandler(stubHandler("hello"))
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, &http.Request{
		Method: "GET",
		Header: http.Header{
			"Accept-Encoding": []string{"gzip"},
		},
	})
	if writer.Body == nil {
		t.Fatal("expected a body")
	}
	if l := writer.Body.Len(); l != 29 {
		t.Fatalf("invalid body length, got %d", l)
	}
	vary := writer.Header()["Vary"]
	if !reflect.DeepEqual(vary, []string{"Accept-Encoding"}) {
		t.Fatalf("invalid Vary headers, got %#v", vary)
	}
}

func TestWithGzipAndMultipleVaryHeader(t *testing.T) {
	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Foo")
		w.Write([]byte("hello"))
	}))
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, &http.Request{
		Method: "GET",
		Header: http.Header{
			"Accept-Encoding": []string{"gzip"},
		},
	})
	if writer.Body == nil {
		t.Fatal("expected a body")
	}
	if l := writer.Body.Len(); l != 29 {
		t.Fatalf("invalid body length, got %d", l)
	}
	vary := writer.Header()["Vary"]
	if !reflect.DeepEqual(vary, []string{"Accept-Encoding", "Foo"}) {
		t.Fatalf("invalid Vary headers, got %#v", vary)
	}
}

func TestWithGzipReal(t *testing.T) {
	const raw = "hello"
	handler := GzipHandler(stubHandler(raw))
	server := httptest.NewServer(handler)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed http request: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if string(body) != raw {
		t.Fatalf(`did not find expected "%s" but got "%s" instead`, raw, string(body))
	}
	vary := resp.Header["Vary"]
	if !reflect.DeepEqual(vary, []string{"Accept-Encoding"}) {
		t.Fatalf("invalid Vary headers, got %#v", vary)
	}
}

func TestWithGzipRealAndMultipleVaryHeaders(t *testing.T) {
	const raw = "hello"
	handler := GzipHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Foo")
		w.Write([]byte(raw))
	}))
	server := httptest.NewServer(handler)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed http request: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if string(body) != raw {
		t.Fatalf(`did not find expected "%s" but got "%s" instead`, raw, string(body))
	}
	vary := resp.Header["Vary"]
	if !reflect.DeepEqual(vary, []string{"Accept-Encoding", "Foo"}) {
		t.Fatalf("invalid Vary headers, got %#v", vary)
	}
}

func TestWithGzipDoubleWrite(t *testing.T) {
	handler := GzipHandler(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Write(bytes.Repeat([]byte("foo"), 1000))
			w.Write(bytes.Repeat([]byte("bar"), 1000))
		}))
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, &http.Request{
		Method: "GET",
		Header: http.Header{
			"Accept-Encoding": []string{"gzip"},
		},
	})
	if writer.Body == nil {
		t.Fatal("expected a body")
	}
	if l := writer.Body.Len(); l != 54 {
		t.Fatalf("invalid body length, got %d", l)
	}
}

func TestGenerateEtag(t *testing.T) {
	etag := generateEtag(
		&http.Request{
			Method: "GET",
			Header: http.Header{
				"Foo": []string{"123"},
				"Bar": []string{"456"},
				"Baz": []string{"789"},
			},
		},
		"1234",
		[]string{"Foo", "Bar"},
	)
	expected := "W/\"1234_313233343536\""
	if etag != expected {
		t.Fatalf("Expected %s, got %s", expected, etag)
	}
}

func TestCacheWithoutEtag(t *testing.T) {
	handler := CacheControlHandler("1234", stubHandler("hello"))
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, &http.Request{
		Method: "GET",
		Header: http.Header{},
	})
	if writer.Header().Get("ETag") == "" {
		t.Fatal("ETag header was not set")
	}
}

func TestCacheWithInvalidEtag(t *testing.T) {
	handler := CacheControlHandler("1234", stubHandler("hello"))
	writer := httptest.NewRecorder()
	handler.ServeHTTP(writer, &http.Request{
		Method: "GET",
		Header: http.Header{
			"If-None-Match": []string{"123"},
		},
	})
	if writer.Code == 304 {
		t.Fatal("Set status to Not Modified (304) on an invalid etag")
	}
}

func TestCacheWithValidEtag(t *testing.T) {
	handler := CacheControlHandler("1234", stubHandler("hello"))
	writer := httptest.NewRecorder()
	r := http.Request{
		Method: "GET",
		Header: http.Header{},
	}
	etag := generateEtag(&r, "1234", []string{})
	r.Header.Set("If-None-Match", etag)
	handler.ServeHTTP(writer, &r)
	if writer.Code != 304 {
		t.Fatalf("Expected status to be Not Modified (304), got %d.  Expected etag was %s, actual was %s", writer.Code, etag, writer.Header().Get("ETag"))
	}
}
