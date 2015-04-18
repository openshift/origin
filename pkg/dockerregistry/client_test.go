package dockerregistry

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestConnect(t *testing.T) {
	c := NewClient()
	conn, err := c.Connect("docker.io")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"index.docker.io", "https://docker.io", "https://index.docker.io"} {
		otherConn, err := c.Connect(s)
		if err != nil {
			t.Errorf("%s: can't connect: ", s, err)
			continue
		}
		if !reflect.DeepEqual(otherConn, conn) {
			t.Errorf("%s: did not reuse connection: %#v %#v", s, conn, otherConn)
		}
	}

	otherConn, err := c.Connect("index.docker.io:443")
	if err != nil || reflect.DeepEqual(otherConn, conn) {
		t.Errorf("should not have reused index.docker.io:443: %v", err)
	}

	if _, err := c.Connect("http://ba%3/"); err == nil {
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
	conn, err := NewClient().Connect(uri.Host)
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
	conn, err := NewClient().Connect("localhost:65000")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.ImageByID("foo", "bar", "baz"); !IsRegistryNotFound(err) {
		t.Error(err)
	}
}

func TestImage(t *testing.T) {
	conn, err := NewClient().Connect("")
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
