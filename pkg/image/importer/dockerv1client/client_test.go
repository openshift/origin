package dockerv1client

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

// tests of running registries are done in the integration client test

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
	conn, err := NewClient(10*time.Second, true).Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}
	v2 := false
	conn.(*connection).isV2 = &v2
	if _, err := conn.ImageTags("foo", "bar"); !IsRepositoryNotFound(err) {
		t.Error(err)
	}
	<-called
	<-called
}

func TestV2Check(t *testing.T) {
	called := make(chan struct{}, 2)
	var uri *url.URL
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		if strings.HasSuffix(r.URL.Path, "/v2/") {
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/tags/list") {
			w.WriteHeader(http.StatusOK)
			fmt.Fprintln(w, `{"tags":["tag1","image1"]}`)
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
	}))
	uri, _ = url.Parse(server.URL)
	conn, err := NewClient(10*time.Second, true).Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := conn.ImageTags("foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if tags["tag1"] != "tag1" {
		t.Errorf("unexpected tags: %#v", tags)
	}
	if tags["image1"] != "image1" {
		t.Errorf("unexpected tags: %#v", tags)
	}

	<-called
	<-called
}

func TestV2CheckNoDistributionHeader(t *testing.T) {
	called := make(chan struct{}, 3)
	var uri *url.URL
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		if strings.HasSuffix(r.URL.Path, "/v2/") {
			w.Header().Set("Docker-Distribution-API-Version", "")
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("X-Docker-Endpoints", uri.Host)

		// Images
		if strings.HasSuffix(r.URL.Path, "/images") {
			return
		}

		// ImageTags
		if strings.HasSuffix(r.URL.Path, "/tags") {
			fmt.Fprintln(w, `{"tag1":"image1"}`)
			return
		}

		// get tag->image id
		if strings.HasSuffix(r.URL.Path, "latest") {
			fmt.Fprintln(w, `"image1"`)
			return
		}

		// get image json
		if strings.HasSuffix(r.URL.Path, "json") {
			fmt.Fprintln(w, `{"id":"image1"}`)
			return
		}
		t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
	}))
	uri, _ = url.Parse(server.URL)
	conn, err := NewClient(10*time.Second, true).Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}
	tags, err := conn.ImageTags("foo", "bar")
	if err != nil {
		t.Fatal(err)
	}
	if tags["tag1"] != "image1" {
		t.Errorf("unexpected tags: %#v", tags)
	}

	<-called
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
	conn, err := NewClient(10*time.Second, true).Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}
	v2 := false
	conn.(*connection).isV2 = &v2
	if _, err := conn.ImageTags("foo", "bar"); !IsRepositoryNotFound(err) {
		t.Error(err)
	}
	<-called
	<-called
}

func TestProxy(t *testing.T) {
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
	os.Setenv("HTTP_PROXY", "http.proxy.tld")
	os.Setenv("HTTPS_PROXY", "secure.proxy.tld")
	os.Setenv("NO_PROXY", "")
	uri, _ = url.Parse(server.URL)
	conn, err := NewClient(10*time.Second, true).Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}
	v2 := false
	conn.(*connection).isV2 = &v2
	if _, err := conn.ImageTags("foo", "bar"); !IsRepositoryNotFound(err) {
		t.Error(err)
	}
	<-called
	<-called
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
	conn, err := NewClient(10*time.Second, true).Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}
	v2 := false
	conn.(*connection).isV2 = &v2
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

func TestGetTagFallback(t *testing.T) {
	var uri *url.URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Docker-Endpoints", uri.Host)

		// get all tags
		if strings.HasSuffix(r.URL.Path, "/tags") {
			fmt.Fprintln(w, `{"tag1":"image1", "test":"image2"}`)
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/json") {
			fmt.Fprintln(w, `{"ID":"image2"}`)
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	uri, _ = url.Parse(server.URL)
	conn, err := NewClient(10*time.Second, true).Connect(uri.Host, true)
	c := conn.(*connection)
	if err != nil {
		t.Fatal(err)
	}
	repo := &v1repository{
		name:     "testrepo",
		endpoint: *uri,
	}
	// Case when tag is found
	img, _, err := repo.getTaggedImage(c, "test", "")
	if err != nil {
		t.Errorf("unexpected error getting tag: %v", err)
		return
	}
	if img.Image.ID != "image2" {
		t.Errorf("unexpected image for tag: %v", img)
	}
	// Case when tag is not found
	img, _, err = repo.getTaggedImage(c, "test2", "")
	if err == nil {
		t.Errorf("expected error")
	}
}

func TestImageManifest(t *testing.T) {
	manifestDigest := "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238"

	called := make(chan struct{}, 2)
	var uri *url.URL
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called <- struct{}{}
		t.Logf("got %s %s", r.Method, r.URL.Path)
		switch r.URL.Path {
		case "/v2/":
			w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
			w.Write([]byte(`{}`))
		case "/v2/test/image/manifests/latest", "/v2/test/image/manifests/" + manifestDigest:
			if r.Method == "HEAD" {
				w.Header().Set("Content-Length", fmt.Sprintf("%d", len(SampleImageManifestSchema1)))
				w.Header().Set("Docker-Content-Digest", manifestDigest)
				w.WriteHeader(http.StatusOK)
				return
			}
			w.Write([]byte(SampleImageManifestSchema1))
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.RequestURI())
			return
		}
	}))
	uri, _ = url.Parse(server.URL)
	conn, err := NewClient(10*time.Second, true).Connect(uri.Host, true)
	if err != nil {
		t.Fatal(err)
	}
	_, manifest, err := conn.ImageManifest("test", "image", "latest")
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest) == 0 {
		t.Errorf("empty manifest")
	}

	if string(manifest) != SampleImageManifestSchema1 {
		t.Errorf("unexpected manifest: %#v", manifest)
	}

	<-called
	<-called
}

const SampleImageManifestSchema1 = `{
   "schemaVersion": 1,
   "name": "nm/is",
   "tag": "latest",
   "architecture": "",
   "fsLayers": [
      {
         "blobSum": "sha256:b2c5513bd934a7efb412c0dd965600b8cb00575b585eaff1cb980b69037fe6cd"
      },
      {
         "blobSum": "sha256:2dde6f11a89463bf20dba3b47d8b3b6de7cdcc19e50634e95a18dd95c278768d"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"size\":18407936}"
      },
      {
         "v1Compatibility": "{\"size\":19387392}"
      }
   ],
   "signatures": [
      {
         "header": {
            "jwk": {
               "crv": "P-256",
               "kid": "5HTY:A24B:L6PG:TQ3G:GMAK:QGKZ:ICD4:S7ZJ:P5JX:UTMP:XZLK:ZXVH",
               "kty": "EC",
               "x": "j5YnDSyrVIt3NquUKvcZIpbfeD8HLZ7BVBFL4WutRBM",
               "y": "PBgFAZ3nNakYN3H9enhrdUrQ_HPYzb8oX5rtJxJo1Y8"
            },
            "alg": "ES256"
         },
         "signature": "1rXiEmWnf9eL7m7Wy3K4l25-Zv2XXl5GgqhM_yjT0ujPmTn0uwfHcCWlweHa9gput3sECj507eQyGpBOF5rD6Q",
         "protected": "eyJmb3JtYXRMZW5ndGgiOjQ4NSwiZm9ybWF0VGFpbCI6IkNuMCIsInRpbWUiOiIyMDE2LTA3LTI2VDExOjQ2OjQ2WiJ9"
      }
   ]
}`
