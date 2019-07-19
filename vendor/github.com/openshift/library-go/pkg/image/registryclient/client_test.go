package registryclient

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/time/rate"

	"github.com/docker/distribution"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/errcode"
	"github.com/opencontainers/go-digest"
	"golang.org/x/net/context"
)

type mockRetriever struct {
	repo     distribution.Repository
	insecure bool
	err      error
}

func (r *mockRetriever) Repository(ctx context.Context, registry *url.URL, repoName string, insecure bool) (distribution.Repository, error) {
	r.insecure = insecure
	return r.repo, r.err
}

type mockRepository struct {
	repoErr, getErr, getByTagErr, getTagErr, tagErr, untagErr, allTagErr, err error

	blobs *mockBlobStore

	manifest distribution.Manifest
	tags     map[string]string
}

func (r *mockRepository) Name() string { return "test" }
func (r *mockRepository) Named() reference.Named {
	named, _ := reference.WithName("test")
	return named
}

func (r *mockRepository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	return r, r.repoErr
}
func (r *mockRepository) Blobs(ctx context.Context) distribution.BlobStore { return r.blobs }
func (r *mockRepository) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	return false, r.getErr
}
func (r *mockRepository) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	for _, option := range options {
		if _, ok := option.(distribution.WithTagOption); ok {
			return r.manifest, r.getByTagErr
		}
	}
	return r.manifest, r.getErr
}
func (r *mockRepository) Delete(ctx context.Context, dgst digest.Digest) error {
	return fmt.Errorf("not implemented")
}
func (r *mockRepository) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	return "", fmt.Errorf("not implemented")
}
func (r *mockRepository) Tags(ctx context.Context) distribution.TagService {
	return &mockTagService{repo: r}
}

type mockBlobStore struct {
	distribution.BlobStore

	blobs map[digest.Digest][]byte

	statErr, serveErr, openErr error
}

func (r *mockBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	return distribution.Descriptor{}, r.statErr
}

func (r *mockBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst digest.Digest) error {
	return r.serveErr
}

func (r *mockBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	return nil, r.openErr
}

func (r *mockBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	b, exists := r.blobs[dgst]
	if !exists {
		return nil, distribution.ErrBlobUnknown
	}
	return b, nil
}

type mockTagService struct {
	distribution.TagService

	repo *mockRepository
}

func (r *mockTagService) Get(ctx context.Context, tag string) (distribution.Descriptor, error) {
	v, ok := r.repo.tags[tag]
	if !ok {
		return distribution.Descriptor{}, r.repo.getTagErr
	}
	dgst, err := digest.Parse(v)
	if err != nil {
		panic(err)
	}
	return distribution.Descriptor{Digest: dgst}, r.repo.getTagErr
}

func (r *mockTagService) Tag(ctx context.Context, tag string, desc distribution.Descriptor) error {
	r.repo.tags[tag] = desc.Digest.String()
	return r.repo.tagErr
}

func (r *mockTagService) Untag(ctx context.Context, tag string) error {
	if _, ok := r.repo.tags[tag]; ok {
		delete(r.repo.tags, tag)
	}
	return r.repo.untagErr
}

func (r *mockTagService) All(ctx context.Context) (res []string, err error) {
	err = r.repo.allTagErr
	for tag := range r.repo.tags {
		res = append(res, tag)
	}
	return
}

func (r *mockTagService) Lookup(ctx context.Context, digest distribution.Descriptor) ([]string, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestPing(t *testing.T) {
	retriever := NewContext(http.DefaultTransport, http.DefaultTransport).WithCredentials(NoCredentials)

	fn404 := func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }
	var fn http.HandlerFunc
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if fn != nil {
			fn(w, r)
		}
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	uri, _ := url.Parse(server.URL)

	testCases := []struct {
		name     string
		uri      url.URL
		expectV2 bool
		fn       http.HandlerFunc
	}{
		{name: "http only", uri: url.URL{Scheme: "http", Host: uri.Host}, expectV2: false, fn: fn404},
		{name: "https only", uri: url.URL{Scheme: "https", Host: uri.Host}, expectV2: false, fn: fn404},
		{
			name:     "403",
			uri:      url.URL{Scheme: "https", Host: uri.Host},
			expectV2: true,
			fn: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					w.WriteHeader(403)
					return
				}
			},
		},
		{
			name:     "401",
			uri:      url.URL{Scheme: "https", Host: uri.Host},
			expectV2: true,
			fn: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					w.WriteHeader(401)
					return
				}
			},
		},
		{
			name:     "200",
			uri:      url.URL{Scheme: "https", Host: uri.Host},
			expectV2: true,
			fn: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					w.WriteHeader(200)
					return
				}
			},
		},
		{
			name:     "has header but 500",
			uri:      url.URL{Scheme: "https", Host: uri.Host},
			expectV2: true,
			fn: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
					w.WriteHeader(500)
					return
				}
			},
		},
		{
			name:     "no header, 500",
			uri:      url.URL{Scheme: "https", Host: uri.Host},
			expectV2: false,
			fn: func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/" {
					w.WriteHeader(500)
					return
				}
			},
		},
	}

	for _, test := range testCases {
		fn = test.fn
		_, err := retriever.ping(test.uri, true, retriever.InsecureTransport)
		if (err != nil && strings.Contains(err.Error(), "does not support v2 API")) == test.expectV2 {
			t.Errorf("%s: Expected ErrNotV2Registry, got %v", test.name, err)
		}
	}
}

var unlimited = rate.NewLimiter(rate.Inf, 100)

type temporaryError struct{}

func (temporaryError) Error() string   { return "temporary" }
func (temporaryError) Timeout() bool   { return false }
func (temporaryError) Temporary() bool { return true }

func TestShouldRetry(t *testing.T) {
	r := NewLimitedRetryRepository(nil, 1, unlimited).(*retryRepository)
	sleeps := 0
	r.sleepFn = func(time.Duration) { sleeps++ }

	// nil error doesn't consume retries
	if r.shouldRetry(0, nil) {
		t.Fatal(r)
	}

	// normal error doesn't consume retries
	if r.shouldRetry(0, fmt.Errorf("error")) {
		t.Fatal(r)
	}

	// docker error doesn't consume retries
	if r.shouldRetry(0, errcode.ErrorCodeDenied) {
		t.Fatal(r)
	}
	if sleeps != 0 {
		t.Fatal(sleeps)
	}

	now := time.Unix(1, 0)
	nowFn = func() time.Time {
		return now
	}
	// should retry a temporary error
	r = NewLimitedRetryRepository(nil, 1, unlimited).(*retryRepository)
	sleeps = 0
	r.sleepFn = func(time.Duration) { sleeps++ }
	if !r.shouldRetry(0, temporaryError{}) {
		t.Fatal(r)
	}
	if r.shouldRetry(1, temporaryError{}) {
		t.Fatal(r)
	}
	if sleeps != 1 {
		t.Fatal(sleeps)
	}
}

func TestRetryFailure(t *testing.T) {
	sleeps := 0
	sleepFn := func(time.Duration) { sleeps++ }

	ctx := context.Background()
	// do not retry on Manifests()
	repo := &mockRepository{repoErr: fmt.Errorf("does not support v2 API")}
	r := NewLimitedRetryRepository(repo, 1, unlimited).(*retryRepository)
	sleeps = 0
	r.sleepFn = sleepFn
	if m, err := r.Manifests(ctx); m != nil || err != repo.repoErr || r.retries != 1 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}

	// do not retry on Manifests()
	repo = &mockRepository{repoErr: temporaryError{}}
	r = NewLimitedRetryRepository(repo, 4, unlimited).(*retryRepository)
	sleeps = 0
	r.sleepFn = sleepFn
	if m, err := r.Manifests(ctx); m != nil || err != repo.repoErr || r.retries != 4 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}

	// do not retry on non standard errors
	repo = &mockRepository{getErr: fmt.Errorf("does not support v2 API")}
	r = NewLimitedRetryRepository(repo, 4, unlimited).(*retryRepository)
	sleeps = 0
	r.sleepFn = sleepFn
	m, err := r.Manifests(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(ctx, digest.Digest("foo")); err != repo.getErr || r.retries != 4 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}

	// retry four times
	repo = &mockRepository{
		getErr: temporaryError{},
		blobs: &mockBlobStore{
			serveErr: temporaryError{},
			statErr:  temporaryError{},
			openErr:  temporaryError{},
		},
	}
	r = NewLimitedRetryRepository(repo, 4, unlimited).(*retryRepository)
	sleeps = 0
	r.sleepFn = sleepFn
	if m, err = r.Manifests(ctx); err != nil {
		t.Fatal(err)
	}
	r.retries = 2
	if _, err := m.Get(ctx, digest.Digest("foo")); err != repo.getErr {
		t.Fatalf("unexpected: %v %#v", err, r)
	}
	r.retries = 2
	if m, err := m.Exists(ctx, "foo"); m || err != repo.getErr {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}
	if sleeps != 4 {
		t.Fatal(sleeps)
	}

	r.retries = 2
	b := r.Blobs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Stat(ctx, digest.Digest("x")); err != repo.blobs.statErr {
		t.Fatalf("unexpected: %v %#v", err, r)
	}
	r.retries = 2
	if err := b.ServeBlob(ctx, nil, nil, digest.Digest("foo")); err != repo.blobs.serveErr {
		t.Fatalf("unexpected: %v %#v", err, r)
	}
	r.retries = 2
	if _, err := b.Open(ctx, digest.Digest("foo")); err != repo.blobs.openErr {
		t.Fatalf("unexpected: %v %#v", err, r)
	}
}

func Test_verifyManifest_Get(t *testing.T) {
	tests := []struct {
		name     string
		dgst     digest.Digest
		err      error
		manifest distribution.Manifest
		options  []distribution.ManifestServiceOption
		want     distribution.Manifest
		wantErr  bool
	}{
		{
			dgst:     payload1Digest,
			manifest: &fakeManifest{payload: []byte(payload1)},
			want:     &fakeManifest{payload: []byte(payload1)},
		},
		{
			dgst:     payload2Digest,
			manifest: &fakeManifest{payload: []byte(payload2)},
			want:     &fakeManifest{payload: []byte(payload2)},
		},
		{
			dgst:     payload1Digest,
			manifest: &fakeManifest{payload: []byte(payload2)},
			wantErr:  true,
		},
		{
			dgst:     payload1Digest,
			manifest: &fakeManifest{payload: []byte(payload1), err: fmt.Errorf("unknown")},
			wantErr:  true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ms := &fakeManifestService{err: tt.err, manifest: tt.manifest}
			m := manifestServiceVerifier{
				ManifestService: ms,
			}
			ctx := context.Background()
			got, err := m.Get(ctx, tt.dgst, tt.options...)
			if (err != nil) != tt.wantErr {
				t.Errorf("verifyManifest.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("verifyManifest.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

const (
	payload1 = `{"some":"content"}`
	payload2 = `{"some":"content"} `
)

var (
	payload1Digest = digest.SHA256.FromString(payload1)
	payload2Digest = digest.SHA256.FromString(payload2)
)

type fakeManifest struct {
	mediaType string
	payload   []byte
	err       error
}

func (m *fakeManifest) References() []distribution.Descriptor {
	panic("not implemented")
}

func (m *fakeManifest) Payload() (mediaType string, payload []byte, err error) {
	return m.mediaType, m.payload, m.err
}

type fakeManifestService struct {
	manifest distribution.Manifest
	err      error
}

func (s *fakeManifestService) Exists(ctx context.Context, dgst digest.Digest) (bool, error) {
	panic("not implemented")
}

func (s *fakeManifestService) Get(ctx context.Context, dgst digest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	return s.manifest, s.err
}

func (s *fakeManifestService) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (digest.Digest, error) {
	panic("not implemented")
}

func (s *fakeManifestService) Delete(ctx context.Context, dgst digest.Digest) error {
	panic("not implemented")
}

func Test_blobStoreVerifier_Get(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		err     error
		dgst    digest.Digest
		want    []byte
		wantErr bool
	}{
		{
			dgst:  payload1Digest,
			bytes: []byte(payload1),
			want:  []byte(payload1),
		},
		{
			dgst:  payload2Digest,
			bytes: []byte(payload2),
			want:  []byte(payload2),
		},
		{
			dgst:    payload1Digest,
			bytes:   []byte(payload2),
			wantErr: true,
		},
		{
			dgst:    payload1Digest,
			bytes:   []byte(payload1),
			err:     fmt.Errorf("unknown"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := &fakeBlobStore{err: tt.err, bytes: tt.bytes}
			b := blobStoreVerifier{
				BlobStore: bs,
			}
			ctx := context.Background()
			got, err := b.Get(ctx, tt.dgst)
			if (err != nil) != tt.wantErr {
				t.Errorf("blobStoreVerifier.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("blobStoreVerifier.Get() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_blobStoreVerifier_Open(t *testing.T) {
	tests := []struct {
		name    string
		bytes   []byte
		err     error
		dgst    digest.Digest
		want    func(t *testing.T, got distribution.ReadSeekCloser)
		wantErr bool
	}{
		{
			dgst:  payload1Digest,
			bytes: []byte(payload1),
			want: func(t *testing.T, got distribution.ReadSeekCloser) {
				data, err := ioutil.ReadAll(got)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal([]byte(payload1), data) {
					t.Fatalf("contents not equal: %s", hex.Dump(data))
				}
			},
		},
		{
			dgst:  payload2Digest,
			bytes: []byte(payload2),
			want: func(t *testing.T, got distribution.ReadSeekCloser) {
				data, err := ioutil.ReadAll(got)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal([]byte(payload2), data) {
					t.Fatalf("contents not equal: %s", hex.Dump(data))
				}
			},
		},
		{
			dgst:  payload1Digest,
			bytes: []byte(payload2),
			want: func(t *testing.T, got distribution.ReadSeekCloser) {
				data, err := ioutil.ReadAll(got)
				if err == nil || !strings.Contains(err.Error(), "content integrity error") || !strings.Contains(err.Error(), payload2Digest.String()) {
					t.Fatal(err)
				}
				if !bytes.Equal([]byte(payload2), data) {
					t.Fatalf("contents not equal: %s", hex.Dump(data))
				}
			},
		},
		{
			dgst:  payload1Digest,
			bytes: []byte(payload2),
			want: func(t *testing.T, got distribution.ReadSeekCloser) {
				_, err := got.Seek(0, 0)
				if err == nil || err.Error() != "invoked seek" {
					t.Fatal(err)
				}
				data, err := ioutil.ReadAll(got)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal([]byte(payload2), data) {
					t.Fatalf("contents not equal: %s", hex.Dump(data))
				}
			},
		},
		{
			dgst:    payload1Digest,
			bytes:   []byte(payload1),
			err:     fmt.Errorf("unknown"),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bs := &fakeBlobStore{err: tt.err, bytes: tt.bytes}
			b := blobStoreVerifier{
				BlobStore: bs,
			}
			ctx := context.Background()
			got, err := b.Open(ctx, tt.dgst)
			if (err != nil) != tt.wantErr {
				t.Errorf("blobStoreVerifier.Get() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			tt.want(t, got)
		})
	}
}

type fakeSeekCloser struct {
	*bytes.Buffer
}

func (f fakeSeekCloser) Seek(offset int64, whence int) (int64, error) {
	return 0, fmt.Errorf("invoked seek")
}

func (f fakeSeekCloser) Close() error {
	return fmt.Errorf("not implemented")
}

type fakeBlobStore struct {
	bytes []byte
	err   error
}

func (s *fakeBlobStore) Stat(ctx context.Context, dgst digest.Digest) (distribution.Descriptor, error) {
	panic("not implemented")
}

func (s *fakeBlobStore) Get(ctx context.Context, dgst digest.Digest) ([]byte, error) {
	return s.bytes, s.err
}

func (s *fakeBlobStore) Open(ctx context.Context, dgst digest.Digest) (distribution.ReadSeekCloser, error) {
	return fakeSeekCloser{bytes.NewBuffer(s.bytes)}, s.err
}

func (s *fakeBlobStore) Put(ctx context.Context, mediaType string, p []byte) (distribution.Descriptor, error) {
	panic("not implemented")
}

func (s *fakeBlobStore) Create(ctx context.Context, options ...distribution.BlobCreateOption) (distribution.BlobWriter, error) {
	panic("not implemented")
}

func (s *fakeBlobStore) Resume(ctx context.Context, id string) (distribution.BlobWriter, error) {
	panic("not implemented")
}

func (s *fakeBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, r *http.Request, dgst digest.Digest) error {
	panic("not implemented")
}

func (s *fakeBlobStore) Delete(ctx context.Context, dgst digest.Digest) error {
	panic("not implemented")
}
