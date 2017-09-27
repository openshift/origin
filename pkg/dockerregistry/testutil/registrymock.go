package testutil

import (
	"net/http"
	"regexp"

	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/reference"
)

func BlobServer(blob []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(blob)
	})
}

func ManifestServer(manifest []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/vnd.docker.distribution.manifest.v2+json")
		w.Write(manifest)
	})
}

type TaggedHandler struct {
	Tags []string
	http.Handler
}

type TaggedHandlers []TaggedHandler

func (hs TaggedHandlers) ServeHTTPTag(w http.ResponseWriter, r *http.Request, tag string) {
	for _, th := range hs {
		for _, t := range th.Tags {
			if t == tag {
				th.ServeHTTP(w, r)
				return
			}
		}
	}
	http.NotFound(w, r)
}

type RepositoryMock struct {
	Blobs     TaggedHandlers
	Manifests TaggedHandlers
}

type RegistryMockHandler map[string]RepositoryMock

var blobsRe = regexp.MustCompile("^/v2/(" + reference.NameRegexp.String() + ")/blobs/(" + digest.DigestRegexp.String() + ")$")
var manifestsRe = regexp.MustCompile("^/v2/(" + reference.NameRegexp.String() + ")/manifests/(" + reference.TagRegexp.String() + "|" + digest.DigestRegexp.String() + ")$")

func (h RegistryMockHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path

	if match := blobsRe.FindStringSubmatch(path); match != nil {
		repoName, dgst := match[1], match[2]
		if repo, ok := h[repoName]; ok {
			repo.Blobs.ServeHTTPTag(w, r, dgst)
			return
		}
	} else if match := manifestsRe.FindStringSubmatch(path); match != nil {
		repoName, tag := match[1], match[2]
		if repo, ok := h[repoName]; ok {
			repo.Manifests.ServeHTTPTag(w, r, tag)
			return
		}
	}

	http.NotFound(w, r)
}
