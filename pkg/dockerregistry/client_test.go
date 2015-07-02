package dockerregistry

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestConnect(t *testing.T) {
	c := NewClient()
	conn, err := c.Connect("docker.io", false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"index.docker.io", "https://docker.io", "https://index.docker.io"} {
		otherConn, err := c.Connect(s, false)
		if err != nil {
			t.Errorf("%s: can't connect: ", s, err)
			continue
		}
		if !reflect.DeepEqual(otherConn, conn) {
			t.Errorf("%s: did not reuse connection: %#v %#v", s, conn, otherConn)
		}
	}

	otherConn, err := c.Connect("index.docker.io:443", false)
	if err != nil || reflect.DeepEqual(otherConn, conn) {
		t.Errorf("should not have reused index.docker.io:443: %v", err)
	}

	if _, err := c.Connect("http://ba%3/", false); err == nil {
		t.Error("Unexpected non-error")
	}
}

func TestHTTPFallback(t *testing.T) {
	called := make(chan struct{}, 2)
	var uri *url.URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		if strings.HasSuffix(r.URL.Path, "/tags") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("X-Docker-Endpoints", uri.Host)
		w.WriteHeader(http.StatusOK)
	}))
	uri, _ = url.Parse(server.URL)
	conn, err := NewClient().Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ImageTags("foo", "bar"); !IsRepositoryNotFound(err) {
		t.Error(err)
	}
	<-called
	<-called
}

func TestInsecureHTTPS(t *testing.T) {
	called := make(chan struct{}, 2)
	var uri *url.URL
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		if strings.HasSuffix(r.URL.Path, "/tags") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("X-Docker-Endpoints", uri.Host)
		w.WriteHeader(http.StatusOK)
	}))
	uri, _ = url.Parse(server.URL)
	conn, err := NewClient().Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ImageTags("foo", "bar"); !IsRepositoryNotFound(err) {
		t.Error(err)
	}
	<-called
	<-called
}

func TestRegistryNotFound(t *testing.T) {
	conn, err := NewClient().Connect("localhost:65000", false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ImageByID("foo", "bar", "baz"); !IsRegistryNotFound(err) {
		t.Error(err)
	}
}

func TestImage(t *testing.T) {
	conn, err := NewClient().Connect("", false)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := conn.ImageByTag("openshift", "origin-not-found", "latest"); !IsRepositoryNotFound(err) {
		t.Errorf("unexpected error: %v", err)
	}

	image, err := conn.ImageByTag("openshift", "origin", "latest")
	if err != nil {
		t.Fatal(err)
	}
	if len(image.ContainerConfig.Entrypoint) == 0 {
		t.Errorf("unexpected image: %#v", image)
	}

	other, err := conn.ImageByID("openshift", "origin", image.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(other.ContainerConfig.Entrypoint, image.ContainerConfig.Entrypoint) {
		t.Errorf("unexpected image: %#v", other)
	}
}

func TestQuayIOImage(t *testing.T) {
	conn, err := NewClient().Connect("quay.io", false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = conn.ImageByTag("coreos", "etcd", "latest")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTokenExpiration(t *testing.T) {
	var uri *url.URL
	lastToken := ""
	tokenIndex := 0
	validToken := ""

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Docker-Token") == "true" {
			tokenIndex++
			lastToken = fmt.Sprintf("token%d", tokenIndex)
			validToken = lastToken
			w.Header().Set("X-Docker-Token", lastToken)
			w.Header().Set("X-Docker-Endpoints", uri.Host)
			return
		}

		auth := r.Header.Get("Authorization")
		parts := strings.Split(auth, " ")
		token := parts[1]
		if token != validToken {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)

		// ImageTags
		if strings.HasSuffix(r.URL.Path, "/tags") {
			fmt.Fprintln(w, `{"tag1":"image1"}`)
		}

		// get tag->image id
		if strings.HasSuffix(r.URL.Path, "latest") {
			fmt.Fprintln(w, `"image1"`)
		}

		// get image json
		if strings.HasSuffix(r.URL.Path, "json") {
			fmt.Fprintln(w, `{"id":"image1"}`)
		}
	}))

	uri, _ = url.Parse(server.URL)
	conn, err := NewClient().Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := conn.ImageTags("foo", "bar"); err != nil {
		t.Fatal(err)
	}

	// expire token, should get an error
	validToken = ""
	if _, err := conn.ImageTags("foo", "bar"); err == nil {
		t.Fatal("expected error")
	}
	// retry, should get a new token
	if _, err := conn.ImageTags("foo", "bar"); err != nil {
		t.Fatal(err)
	}

	// expire token, should get an error
	validToken = ""
	if _, err := conn.ImageByTag("foo", "bar", "latest"); err == nil {
		t.Fatal("expected error")
	}
	// retry, should get a new token
	if _, err := conn.ImageByTag("foo", "bar", "latest"); err != nil {
		t.Fatal(err)
	}

	// expire token, should get an error
	validToken = ""
	if _, err := conn.ImageByID("foo", "bar", "image1"); err == nil {
		t.Fatal("expected error")
	}
	// retry, should get a new token
	if _, err := conn.ImageByID("foo", "bar", "image1"); err != nil {
		t.Fatal(err)
	}
}
