package server

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	_ "github.com/docker/distribution/registry/storage/driver/inmemory"

	registryclient "github.com/openshift/origin/pkg/dockerregistry/server/client"
	registrytest "github.com/openshift/origin/pkg/dockerregistry/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"
	"github.com/openshift/origin/pkg/image/importer"
)

func TestPullthroughServeBlob(t *testing.T) {
	backgroundCtx := context.Background()
	backgroundCtx = registrytest.WithTestLogger(backgroundCtx, t)

	namespace, name := "user", "app"
	repoName := fmt.Sprintf("%s/%s", namespace, name)
	installFakeAccessController(t)
	setPassthroughBlobDescriptorServiceFactory()

	testImage, err := registrytest.NewImageForManifest(repoName, registrytest.SampleImageManifestSchema1, "", false)
	if err != nil {
		t.Fatal(err)
	}

	remoteRegistryServer := createTestRegistryServer(t, backgroundCtx)
	defer remoteRegistryServer.Close()

	serverURL, err := url.Parse(remoteRegistryServer.URL)
	if err != nil {
		t.Fatalf("error parsing server url: %v", err)
	}
	os.Setenv("OPENSHIFT_DEFAULT_REGISTRY", serverURL.Host)
	testImage.DockerImageReference = fmt.Sprintf("%s/%s@%s", serverURL.Host, repoName, testImage.Name)

	fos, imageClient := registrytest.NewFakeOpenShiftWithClient(backgroundCtx)
	registrytest.AddImageStream(t, fos, namespace, name, map[string]string{
		imageapi.InsecureRepositoryAnnotation: "true",
	})
	registrytest.AddImage(t, fos, testImage, namespace, name, "latest")

	blob1Desc, blob1Content, err := registrytest.UploadRandomTestBlob(backgroundCtx, serverURL, nil, repoName)
	if err != nil {
		t.Fatal(err)
	}
	blob2Desc, blob2Content, err := registrytest.UploadRandomTestBlob(backgroundCtx, serverURL, nil, repoName)
	if err != nil {
		t.Fatal(err)
	}

	blob1Storage := map[digest.Digest][]byte{blob1Desc.Digest: blob1Content}
	blob2Storage := map[digest.Digest][]byte{blob2Desc.Digest: blob2Content}

	for _, tc := range []struct {
		name                       string
		method                     string
		blobDigest                 digest.Digest
		localBlobs                 map[digest.Digest][]byte
		expectedStatError          error
		expectedContentLength      int64
		expectedBytesServed        int64
		expectedBytesServedLocally int64
		expectedLocalCalls         map[string]int
	}{
		{
			name:                  "stat local blob",
			method:                "HEAD",
			blobDigest:            blob1Desc.Digest,
			localBlobs:            blob1Storage,
			expectedContentLength: int64(len(blob1Content)),
			expectedLocalCalls: map[string]int{
				"Stat":      1,
				"ServeBlob": 1,
			},
		},

		{
			name:                       "serve local blob",
			method:                     "GET",
			blobDigest:                 blob1Desc.Digest,
			localBlobs:                 blob1Storage,
			expectedContentLength:      int64(len(blob1Content)),
			expectedBytesServed:        int64(len(blob1Content)),
			expectedBytesServedLocally: int64(len(blob1Content)),
			expectedLocalCalls: map[string]int{
				"Stat":      1,
				"ServeBlob": 1,
			},
		},

		{
			name:                  "stat remote blob",
			method:                "HEAD",
			blobDigest:            blob1Desc.Digest,
			localBlobs:            blob2Storage,
			expectedContentLength: int64(len(blob1Content)),
			expectedLocalCalls: map[string]int{
				"Stat":      1,
				"ServeBlob": 1,
			},
		},

		{
			name:                  "serve remote blob",
			method:                "GET",
			blobDigest:            blob1Desc.Digest,
			expectedContentLength: int64(len(blob1Content)),
			expectedBytesServed:   int64(len(blob1Content)),
			expectedLocalCalls: map[string]int{
				"Stat":      1,
				"ServeBlob": 1,
			},
		},

		{
			name:               "unknown blob digest",
			method:             "GET",
			blobDigest:         unknownBlobDigest,
			expectedStatError:  distribution.ErrBlobUnknown,
			expectedLocalCalls: map[string]int{"Stat": 1},
		},
	} {
		localBlobStore := newTestBlobStore(nil, tc.localBlobs)

		ctx := WithTestPassthroughToUpstream(backgroundCtx, false)
		repo := newTestRepository(ctx, t, namespace, name, testRepositoryOptions{
			client:            registryclient.NewFakeRegistryAPIClient(nil, imageClient),
			enablePullThrough: true,
		})
		ptbs := &pullthroughBlobStore{
			BlobStore: localBlobStore,
			repo:      repo,
		}

		req, err := http.NewRequest(tc.method, fmt.Sprintf("http://example.org/v2/user/app/blobs/%s", tc.blobDigest), nil)
		if err != nil {
			t.Fatalf("[%s] failed to create http request: %v", tc.name, err)
		}
		w := httptest.NewRecorder()

		dgst := digest.Digest(tc.blobDigest)

		_, err = ptbs.Stat(ctx, dgst)
		if err != tc.expectedStatError {
			t.Errorf("[%s] Stat returned unexpected error: %#+v != %#+v", tc.name, err, tc.expectedStatError)
		}
		if err != nil || tc.expectedStatError != nil {
			continue
		}
		err = ptbs.ServeBlob(ctx, w, req, dgst)
		if err != nil {
			t.Errorf("[%s] unexpected ServeBlob error: %v", tc.name, err)
			continue
		}

		clstr := w.Header().Get("Content-Length")
		if cl, err := strconv.ParseInt(clstr, 10, 64); err != nil {
			t.Errorf(`[%s] unexpected Content-Length: %q != "%d"`, tc.name, clstr, tc.expectedContentLength)
		} else {
			if cl != tc.expectedContentLength {
				t.Errorf("[%s] Content-Length does not match expected size: %d != %d", tc.name, cl, tc.expectedContentLength)
			}
		}
		if w.Header().Get("Content-Type") != "application/octet-stream" {
			t.Errorf("[%s] Content-Type does not match expected: %q != %q", tc.name, w.Header().Get("Content-Type"), "application/octet-stream")
		}

		body := w.Body.Bytes()
		if int64(len(body)) != tc.expectedBytesServed {
			t.Errorf("[%s] unexpected size of body: %d != %d", tc.name, len(body), tc.expectedBytesServed)
		}

		for name, expCount := range tc.expectedLocalCalls {
			count := localBlobStore.calls[name]
			if count != expCount {
				t.Errorf("[%s] expected %d calls to method %s of local blob store, not %d", tc.name, expCount, name, count)
			}
		}
		for name, count := range localBlobStore.calls {
			if _, exists := tc.expectedLocalCalls[name]; !exists {
				t.Errorf("[%s] expected no calls to method %s of local blob store, got %d", tc.name, name, count)
			}
		}

		if localBlobStore.bytesServed != tc.expectedBytesServedLocally {
			t.Errorf("[%s] unexpected number of bytes served locally: %d != %d", tc.name, localBlobStore.bytesServed, tc.expectedBytesServed)
		}
	}
}

func TestPullthroughServeNotSeekableBlob(t *testing.T) {
	repoName := "foorepo"
	blob, err := registrytest.CreateRandomTarFile()
	if err != nil {
		t.Fatalf("unexpected error generating test layer file: %v", err)
	}
	dgst := digest.FromBytes(blob)

	externalRegistry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("external registry got %s %s", r.Method, r.URL.Path)

		w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")

		switch r.URL.Path {
		case "/v2/":
			w.Write([]byte(`{}`))
		case "/v2/" + repoName + "/blobs/" + dgst.String():
			w.Header().Set("Content-Length", fmt.Sprintf("%d", len(blob)))
			w.Header().Set("Docker-Content-Digest", dgst.String())

			if r.Method == "HEAD" {
				w.WriteHeader(http.StatusOK)
			} else {
				// We need to return any return code between 200 and 399,
				// except 200 and 206 [1].
				//
				// In this case the docker client library will make a not
				// truly seekable response [2].
				//
				// [1]: https://github.com/docker/distribution/blob/7484e51bf6af0d3b1a849644cdaced3cfcf13617/registry/client/transport/http_reader.go#L239
				// [2]: https://github.com/docker/distribution/blob/7484e51bf6af0d3b1a849644cdaced3cfcf13617/registry/client/transport/http_reader.go#L119-L121
				w.WriteHeader(http.StatusNonAuthoritativeInfo)
				w.Write(blob)
			}
		default:
			panic(fmt.Errorf("unexpected request: %#+v", r))
		}
	}))
	defer externalRegistry.Close()

	externalRegistryURL, err := url.Parse(externalRegistry.URL)
	if err != nil {
		t.Fatal("error parsing test server url:", err)
	}

	ctx := context.Background()
	ctx = registrytest.WithTestLogger(ctx, t)

	retriever := importer.NewContext(http.DefaultTransport, http.DefaultTransport).WithCredentials(importer.NoCredentials)
	repo, err := retriever.Repository(ctx, externalRegistryURL, repoName, true)
	if err != nil {
		t.Fatal(err)
	}

	repoBlobs := repo.Blobs(ctx)

	// Test that the reader is not seekable.
	remoteBlob, err := repoBlobs.Open(ctx, dgst)
	if err != nil {
		t.Fatalf("failed to Open blob %s: %v", dgst, err)
	}
	defer remoteBlob.Close()

	if _, err := remoteBlob.Seek(0, os.SEEK_END); err == nil {
		t.Fatal("expected non-seekable blob reader, but Seek(0, os.SEEK_END) succeed")
	}

	// Test that the blob can be fetched.
	ptbs := &pullthroughBlobStore{
		BlobStore: newTestBlobStore(nil, nil),
		repo: &repository{
			remoteBlobGetter: repoBlobs,
		},
		mirror: false,
	}

	req := httptest.NewRequest("GET", "/unused", nil)
	w := httptest.NewRecorder()

	if err = ptbs.ServeBlob(ctx, w, req, dgst); err != nil {
		t.Fatalf("ServeBlob failed: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("unexpected status code: got %d, want %d", w.Code, http.StatusOK)
	}

	clstr := w.Header().Get("Content-Length")
	if cl, err := strconv.ParseInt(clstr, 10, 64); err != nil || cl != int64(len(blob)) {
		t.Errorf("Content-Length does not match the expected size: got %s, want %d", clstr, len(blob))
	}

	if w.Body.Len() != len(blob) {
		t.Errorf("unexpected size of body: got %d, want %d", w.Body.Len(), len(blob))
	}
}

func TestPullthroughServeBlobInsecure(t *testing.T) {
	namespace := "user"
	repo1 := "app1"
	repo2 := "app2"
	repo1Name := fmt.Sprintf("%s/%s", namespace, repo1)
	repo2Name := fmt.Sprintf("%s/%s", namespace, repo2)

	installFakeAccessController(t)
	setPassthroughBlobDescriptorServiceFactory()

	backgroundCtx := context.Background()
	backgroundCtx = registrytest.WithTestLogger(backgroundCtx, t)

	remoteRegistryServer := createTestRegistryServer(t, backgroundCtx)
	defer remoteRegistryServer.Close()

	serverURL, err := url.Parse(remoteRegistryServer.URL)
	if err != nil {
		t.Fatalf("error parsing server url: %v", err)
	}

	m1dgst, m1canonical, m1cfg, m1manifest, err := registrytest.CreateAndUploadTestManifest(
		backgroundCtx, registrytest.ManifestSchema2, 2, serverURL, nil, repo1Name, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, m1payload, err := m1manifest.Payload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("m1dgst=%s, m1manifest: %s", m1dgst, m1canonical)
	m2dgst, m2canonical, m2cfg, m2manifest, err := registrytest.CreateAndUploadTestManifest(
		backgroundCtx, registrytest.ManifestSchema2, 2, serverURL, nil, repo2Name, "bar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, m2payload, err := m2manifest.Payload()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("m2dgst=%s, m2manifest: %s", m2dgst, m2canonical)

	m1img, err := registrytest.NewImageForManifest(repo1Name, string(m1payload), m1cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	m1img.DockerImageReference = fmt.Sprintf("%s/%s/%s@%s", serverURL.Host, namespace, repo1, m1img.Name)
	m1img.DockerImageManifest = ""
	m2img, err := registrytest.NewImageForManifest(repo2Name, string(m2payload), m2cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	m2img.DockerImageReference = fmt.Sprintf("%s/%s/%s@%s", serverURL.Host, namespace, repo2, m2img.Name)
	m2img.DockerImageManifest = ""

	for _, tc := range []struct {
		name                       string
		method                     string
		blobDigest                 digest.Digest
		localBlobs                 map[digest.Digest][]byte
		fakeOpenShiftInit          func(fos *registrytest.FakeOpenShift)
		expectedStatError          error
		expectedContentLength      int64
		expectedBytesServed        int64
		expectedBytesServedLocally int64
		expectedLocalCalls         map[string]int
	}{
		{
			name:       "stat remote blob with insecure repository",
			method:     "HEAD",
			blobDigest: digest.Digest(m1img.DockerImageLayers[0].Name),
			fakeOpenShiftInit: func(fos *registrytest.FakeOpenShift) {
				registrytest.AddImageStream(t, fos, namespace, repo1, map[string]string{
					imageapi.InsecureRepositoryAnnotation: "true",
				})
				registrytest.AddImage(t, fos, m1img, namespace, repo1, "tag1")
			},
			expectedContentLength: int64(m1img.DockerImageLayers[0].LayerSize),
			expectedLocalCalls:    map[string]int{"Stat": 1, "ServeBlob": 1},
		},

		{
			name:       "serve remote blob with insecure repository",
			method:     "GET",
			blobDigest: digest.Digest(m1img.DockerImageLayers[0].Name),
			fakeOpenShiftInit: func(fos *registrytest.FakeOpenShift) {
				registrytest.AddImageStream(t, fos, namespace, repo1, map[string]string{
					imageapi.InsecureRepositoryAnnotation: "true",
				})
				registrytest.AddImage(t, fos, m1img, namespace, repo1, "tag1")
			},
			expectedContentLength: int64(m1img.DockerImageLayers[0].LayerSize),
			expectedBytesServed:   int64(m1img.DockerImageLayers[0].LayerSize),
			expectedLocalCalls:    map[string]int{"Stat": 1, "ServeBlob": 1},
		},

		{
			name:       "stat remote blob with secure repository",
			method:     "HEAD",
			blobDigest: digest.Digest(m1img.DockerImageLayers[0].Name),
			fakeOpenShiftInit: func(fos *registrytest.FakeOpenShift) {
				registrytest.AddImageStream(t, fos, namespace, repo1, map[string]string{
					imageapi.InsecureRepositoryAnnotation: "false",
				})
				registrytest.AddImage(t, fos, m1img, namespace, repo1, "tag1")
			},
			expectedStatError: distribution.ErrBlobUnknown,
		},

		{
			name:       "serve remote blob with secure repository",
			method:     "GET",
			blobDigest: digest.Digest(m1img.DockerImageLayers[0].Name),
			fakeOpenShiftInit: func(fos *registrytest.FakeOpenShift) {
				registrytest.AddImageStream(t, fos, namespace, repo1, map[string]string{
					imageapi.InsecureRepositoryAnnotation: "false",
				})
				registrytest.AddImage(t, fos, m1img, namespace, repo1, "tag1")
			},
			expectedStatError: distribution.ErrBlobUnknown,
		},

		{
			name:       "stat remote blob with with insecure tag",
			method:     "HEAD",
			blobDigest: digest.Digest(m2img.DockerImageLayers[0].Name),
			fakeOpenShiftInit: func(fos *registrytest.FakeOpenShift) {
				registrytest.AddUntaggedImage(t, fos, m1img)
				registrytest.AddUntaggedImage(t, fos, m2img)
				registrytest.AddImageStream(t, fos, namespace, repo1, map[string]string{
					imageapi.InsecureRepositoryAnnotation: "false",
				})
				registrytest.AddImageStreamTag(t, fos, m1img, namespace, repo1, &imageapiv1.TagReference{
					Name:         "tag1",
					ImportPolicy: imageapiv1.TagImportPolicy{Insecure: false},
				})
				registrytest.AddImageStreamTag(t, fos, m2img, namespace, repo1, &imageapiv1.TagReference{
					Name:         "tag2",
					ImportPolicy: imageapiv1.TagImportPolicy{Insecure: true},
				})
			},
			expectedContentLength: int64(m2img.DockerImageLayers[0].LayerSize),
			expectedLocalCalls:    map[string]int{"Stat": 1, "ServeBlob": 1},
		},

		{
			name:       "serve remote blob with insecure tag",
			method:     "GET",
			blobDigest: digest.Digest(m2img.DockerImageLayers[0].Name),
			fakeOpenShiftInit: func(fos *registrytest.FakeOpenShift) {
				registrytest.AddUntaggedImage(t, fos, m1img)
				registrytest.AddUntaggedImage(t, fos, m2img)
				registrytest.AddImageStream(t, fos, namespace, repo1, map[string]string{
					imageapi.InsecureRepositoryAnnotation: "false",
				})
				registrytest.AddImageStreamTag(t, fos, m1img, namespace, repo1, &imageapiv1.TagReference{
					Name:         "tag1",
					ImportPolicy: imageapiv1.TagImportPolicy{Insecure: false},
				})
				registrytest.AddImageStreamTag(t, fos, m2img, namespace, repo1, &imageapiv1.TagReference{
					Name:         "tag2",
					ImportPolicy: imageapiv1.TagImportPolicy{Insecure: true},
				})
			},
			expectedLocalCalls:    map[string]int{"Stat": 1, "ServeBlob": 1},
			expectedContentLength: int64(m2img.DockerImageLayers[0].LayerSize),
			expectedBytesServed:   int64(m2img.DockerImageLayers[0].LayerSize),
		},

		{
			name:       "insecure flag propagates to all repositories of the registry",
			method:     "GET",
			blobDigest: digest.Digest(m2img.DockerImageLayers[0].Name),
			fakeOpenShiftInit: func(fos *registrytest.FakeOpenShift) {
				registrytest.AddUntaggedImage(t, fos, m1img)
				registrytest.AddUntaggedImage(t, fos, m2img)
				registrytest.AddImageStream(t, fos, namespace, repo1, map[string]string{
					imageapi.InsecureRepositoryAnnotation: "false",
				})
				registrytest.AddImageStreamTag(t, fos, m1img, namespace, repo1, &imageapiv1.TagReference{
					Name: "tag1",
					// This value will propagate to the other tag as well.
					ImportPolicy: imageapiv1.TagImportPolicy{Insecure: true},
				})
				registrytest.AddImageStreamTag(t, fos, m2img, namespace, repo1, &imageapiv1.TagReference{
					Name:         "tag2",
					ImportPolicy: imageapiv1.TagImportPolicy{},
				})
			},
			expectedLocalCalls:    map[string]int{"Stat": 1, "ServeBlob": 1},
			expectedContentLength: int64(m2img.DockerImageLayers[0].LayerSize),
			expectedBytesServed:   int64(m2img.DockerImageLayers[0].LayerSize),
		},

		{
			name:       "serve remote blob with secure tag",
			method:     "GET",
			blobDigest: digest.Digest(m1img.DockerImageLayers[0].Name),
			fakeOpenShiftInit: func(fos *registrytest.FakeOpenShift) {
				registrytest.AddUntaggedImage(t, fos, m1img)
				registrytest.AddUntaggedImage(t, fos, m2img)

				m2docker := *m2img
				ref, err := imageapi.ParseDockerImageReference(m2img.DockerImageReference)
				if err != nil {
					t.Fatal(err)
				}
				// The two references must differ because all repositories of particular registry are
				// considered insecure if there's at least one insecure flag for the registry.
				ref.Registry = "docker.io"
				m2docker.DockerImageReference = ref.DockerClientDefaults().Exact()

				registrytest.AddImageStream(t, fos, namespace, repo1, map[string]string{
					imageapi.InsecureRepositoryAnnotation: "false",
				})
				registrytest.AddImageStreamTag(t, fos, m1img, namespace, repo1, &imageapiv1.TagReference{
					Name:         "tag1",
					ImportPolicy: imageapiv1.TagImportPolicy{Insecure: false},
				})
				registrytest.AddImageStreamTag(t, fos, &m2docker, namespace, repo1, &imageapiv1.TagReference{
					Name:         "tag2",
					ImportPolicy: imageapiv1.TagImportPolicy{Insecure: true},
				})
			},
			expectedStatError: distribution.ErrBlobUnknown,
		},

		{
			name:       "serve remote blob with 2 tags pointing to the same image",
			method:     "GET",
			blobDigest: digest.Digest(m1img.DockerImageLayers[0].Name),
			fakeOpenShiftInit: func(fos *registrytest.FakeOpenShift) {
				registrytest.AddUntaggedImage(t, fos, m1img)
				registrytest.AddImageStream(t, fos, namespace, repo1, map[string]string{
					imageapi.InsecureRepositoryAnnotation: "false",
				})
				registrytest.AddImageStreamTag(t, fos, m1img, namespace, repo1, &imageapiv1.TagReference{
					Name:         "tag1",
					ImportPolicy: imageapiv1.TagImportPolicy{Insecure: false},
				})
				registrytest.AddImageStreamTag(t, fos, m1img, namespace, repo1, &imageapiv1.TagReference{
					Name:         "tag2",
					ImportPolicy: imageapiv1.TagImportPolicy{Insecure: true},
				})
			},
			expectedLocalCalls:    map[string]int{"Stat": 1, "ServeBlob": 1},
			expectedContentLength: int64(m1img.DockerImageLayers[0].LayerSize),
			expectedBytesServed:   int64(m1img.DockerImageLayers[0].LayerSize),
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := registrytest.WithTestLogger(backgroundCtx, t)

			fos, imageClient := registrytest.NewFakeOpenShiftWithClient(ctx)

			tc.fakeOpenShiftInit(fos)

			localBlobStore := newTestBlobStore(nil, tc.localBlobs)

			ctx = WithTestPassthroughToUpstream(ctx, false)

			repo := newTestRepository(ctx, t, namespace, repo1, testRepositoryOptions{
				client:            registryclient.NewFakeRegistryAPIClient(nil, imageClient),
				enablePullThrough: true,
			})

			ptbs := &pullthroughBlobStore{
				BlobStore: localBlobStore,
				repo:      repo,
			}

			req, err := http.NewRequest(tc.method, fmt.Sprintf("http://example.org/v2/user/app/blobs/%s", tc.blobDigest), nil)
			if err != nil {
				t.Fatalf("[%s] failed to create http request: %v", tc.name, err)
			}
			w := httptest.NewRecorder()

			dgst := digest.Digest(tc.blobDigest)

			_, err = ptbs.Stat(ctx, dgst)
			if err != tc.expectedStatError {
				t.Fatalf("[%s] Stat returned unexpected error: %#+v != %#+v", tc.name, err, tc.expectedStatError)
			}
			if err != nil || tc.expectedStatError != nil {
				return
			}
			err = ptbs.ServeBlob(ctx, w, req, dgst)
			if err != nil {
				t.Errorf("[%s] unexpected ServeBlob error: %v", tc.name, err)
				return
			}

			clstr := w.Header().Get("Content-Length")
			if cl, err := strconv.ParseInt(clstr, 10, 64); err != nil {
				t.Errorf(`[%s] unexpected Content-Length: %q != "%d"`, tc.name, clstr, tc.expectedContentLength)
			} else {
				if cl != tc.expectedContentLength {
					t.Errorf("[%s] Content-Length does not match expected size: %d != %d", tc.name, cl, tc.expectedContentLength)
				}
			}
			if w.Header().Get("Content-Type") != "application/octet-stream" {
				t.Errorf("[%s] Content-Type does not match expected: %q != %q", tc.name, w.Header().Get("Content-Type"), "application/octet-stream")
			}

			body := w.Body.Bytes()
			if int64(len(body)) != tc.expectedBytesServed {
				t.Errorf("[%s] unexpected size of body: %d != %d", tc.name, len(body), tc.expectedBytesServed)
			}

			for name, expCount := range tc.expectedLocalCalls {
				count := localBlobStore.calls[name]
				if count != expCount {
					t.Errorf("[%s] expected %d calls to method %s of local blob store, not %d", tc.name, expCount, name, count)
				}
			}
			for name, count := range localBlobStore.calls {
				if _, exists := tc.expectedLocalCalls[name]; !exists {
					t.Errorf("[%s] expected no calls to method %s of local blob store, got %d", tc.name, name, count)
				}
			}

			if localBlobStore.bytesServed != tc.expectedBytesServedLocally {
				t.Errorf("[%s] unexpected number of bytes served locally: %d != %d", tc.name, localBlobStore.bytesServed, tc.expectedBytesServed)
			}
		})
	}
}

const (
	unknownBlobDigest = "sha256:bef57ec7f53a6d40beb640a780a639c83bc29ac8a9816f1fc6c5c6dcd93c4721"
)

func makeDigestFromBytes(data []byte) digest.Digest {
	return digest.Digest(fmt.Sprintf("sha256:%x", sha256.Sum256(data)))
}

type blobContents map[digest.Digest][]byte
type blobDescriptors map[digest.Digest]distribution.Descriptor

type testBlobStore struct {
	blobDescriptors blobDescriptors
	// blob digest mapped to content
	blobs blobContents
	// method name mapped to number of invocations
	calls       map[string]int
	bytesServed int64
}

var _ distribution.BlobStore = &testBlobStore{}

func newTestBlobStore(blobDescriptors blobDescriptors, blobs blobContents) *testBlobStore {
	bs := make(map[digest.Digest][]byte)
	for d, content := range blobs {
		bs[d] = content
	}
	bds := make(map[digest.Digest]distribution.Descriptor)
	for d, desc := range blobDescriptors {
		bds[d] = desc
	}
	return &testBlobStore{
		blobDescriptors: bds,
		blobs:           bs,
		calls:           make(map[string]int),
	}
}

func (t *testBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	t.calls["Stat"]++
	desc, exists := t.blobDescriptors[dgst]
	if exists {
		return desc, nil
	}

	content, exists := t.blobs[dgst]
	if !exists {
		return distribution.Descriptor{}, distribution.ErrBlobUnknown
	}
	return distribution.Descriptor{
		MediaType: schema1.MediaTypeManifestLayer,
		Size:      int64(len(content)),
		Digest:    makeDigestFromBytes(content),
	}, nil
}

func (t *testBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	t.calls["Get"]++
	content, exists := t.blobs[dgst]
	if !exists {
		return nil, distribution.ErrBlobUnknown
	}
	return content, nil
}

func (t *testBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	t.calls["Open"]++
	content, exists := t.blobs[dgst]
	if !exists {
		return nil, distribution.ErrBlobUnknown
	}
	return &testBlobFileReader{
		bs:      t,
		content: content,
	}, nil
}

func (t *testBlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	t.calls["Put"]++
	return distribution.Descriptor{}, fmt.Errorf("method not implemented")
}

func (t *testBlobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	t.calls["Create"]++
	return nil, fmt.Errorf("method not implemented")
}

func (t *testBlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	t.calls["Resume"]++
	return nil, fmt.Errorf("method not implemented")
}

func (t *testBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	t.calls["ServeBlob"]++
	content, exists := t.blobs[dgst]
	if !exists {
		return distribution.ErrBlobUnknown
	}
	reader := bytes.NewReader(content)
	setResponseHeaders(w, int64(len(content)), "application/octet-stream", dgst)
	http.ServeContent(w, req, dgst.String(), time.Time{}, reader)
	n, err := reader.Seek(0, 1)
	if err != nil {
		return err
	}
	t.bytesServed = n
	return nil
}

func (t *testBlobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	t.calls["Delete"]++
	return fmt.Errorf("method not implemented")
}

type testBlobFileReader struct {
	bs      *testBlobStore
	content []byte
	offset  int64
}

var _ distribution.ReadSeekCloser = &testBlobFileReader{}

func (fr *testBlobFileReader) Read(p []byte) (n int, err error) {
	fr.bs.calls["ReadSeakCloser.Read"]++
	n = copy(p, fr.content[fr.offset:])
	fr.offset += int64(n)
	fr.bs.bytesServed += int64(n)
	return n, nil
}

func (fr *testBlobFileReader) Seek(offset int64, whence int) (int64, error) {
	fr.bs.calls["ReadSeakCloser.Seek"]++

	newOffset := fr.offset

	switch whence {
	case os.SEEK_CUR:
		newOffset += int64(offset)
	case os.SEEK_END:
		newOffset = int64(len(fr.content)) + offset
	case os.SEEK_SET:
		newOffset = int64(offset)
	}

	var err error
	if newOffset < 0 {
		err = fmt.Errorf("cannot seek to negative position")
	} else {
		// No problems, set the offset.
		fr.offset = newOffset
	}

	return fr.offset, err
}

func (fr *testBlobFileReader) Close() error {
	fr.bs.calls["ReadSeakCloser.Close"]++
	return nil
}
