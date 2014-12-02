package httpgzip_test

import (
	"bytes"
	"github.com/daaku/go.httpgzip"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func stubHandler(response string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(response))
	})
}

func TestWithoutGzip(t *testing.T) {
	const resp = "hello"
	handler := httpgzip.NewHandler(stubHandler(resp))
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
	handler := httpgzip.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler := httpgzip.NewHandler(stubHandler("hello"))
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
	handler := httpgzip.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	handler := httpgzip.NewHandler(stubHandler(raw))
	server := httptest.NewServer(handler)
	defer server.Close()
	resp, err := http.Get(server.URL)
	if err != nil {
		t.Fatalf("failed http request: %s", err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if string(body) != raw {
		t.Fatalf(`did not find expected "%s" but got "%s" instead`, raw, resp)
	}
	vary := resp.Header["Vary"]
	if !reflect.DeepEqual(vary, []string{"Accept-Encoding"}) {
		t.Fatalf("invalid Vary headers, got %#v", vary)
	}
}

func TestWithGzipRealAndMultipleVaryHeaders(t *testing.T) {
	const raw = "hello"
	handler := httpgzip.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
		t.Fatalf(`did not find expected "%s" but got "%s" instead`, raw, resp)
	}
	vary := resp.Header["Vary"]
	if !reflect.DeepEqual(vary, []string{"Accept-Encoding", "Foo"}) {
		t.Fatalf("invalid Vary headers, got %#v", vary)
	}
}

func TestWithGzipDoubleWrite(t *testing.T) {
	handler := httpgzip.NewHandler(
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
