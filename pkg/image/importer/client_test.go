package importer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	gocontext "golang.org/x/net/context"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/reference"
	"github.com/docker/distribution/registry/api/errcode"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
)

type mockRetriever struct {
	repo     distribution.Repository
	insecure bool
	err      error
}

func (r *mockRetriever) Repository(ctx gocontext.Context, registry *url.URL, repoName string, insecure bool) (distribution.Repository, error) {
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
	dgst, err := digest.ParseDigest(v)
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

func TestSchema1ToImage(t *testing.T) {
	m := &schema1.SignedManifest{}
	if err := json.Unmarshal([]byte(etcdManifest), m); err != nil {
		t.Fatal(err)
	}
	image, err := schema1ToImage(m, digest.Digest("sha256:test"))
	if err != nil {
		t.Fatal(err)
	}
	if image.DockerImageMetadata.ID != "sha256:test" {
		t.Errorf("unexpected image: %#v", image.DockerImageMetadata.ID)
	}
}

func TestDockerV1Fallback(t *testing.T) {
	var uri *url.URL
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Docker-Endpoints", uri.Host)

		// get all tags
		if strings.HasSuffix(r.URL.Path, "/tags") {
			fmt.Fprintln(w, `{"tag1":"image1", "test":"image2"}`)
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/images") {
			fmt.Fprintln(w, `{"tag1":"image1", "test":"image2"}`)
			w.WriteHeader(http.StatusOK)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/json") {
			fmt.Fprintln(w, `{"ID":"image2"}`)
			w.WriteHeader(http.StatusOK)
			return
		}
		t.Logf("tried to access %s", r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))

	client := dockerregistry.NewClient(10*time.Second, false)
	ctx := gocontext.WithValue(gocontext.Background(), ContextKeyV1RegistryClient, client)

	uri, _ = url.Parse(server.URL)
	isi := &api.ImageStreamImport{
		Spec: api.ImageStreamImportSpec{
			Repository: &api.RepositoryImportSpec{
				From:         kapi.ObjectReference{Kind: "DockerImage", Name: uri.Host + "/test:test"},
				ImportPolicy: api.TagImportPolicy{Insecure: true},
			},
		},
	}

	retriever := &mockRetriever{err: fmt.Errorf("does not support v2 API")}
	im := NewImageStreamImporter(retriever, 5, nil, nil)
	if err := im.Import(ctx, isi); err != nil {
		t.Fatal(err)
	}
	if images := isi.Status.Repository.Images; len(images) != 2 || images[0].Tag != "tag1" || images[1].Tag != "test" {
		t.Errorf("unexpected images: %#v", images)
	}
}

func TestPing(t *testing.T) {
	retriever := NewContext(http.DefaultTransport, http.DefaultTransport).WithCredentials(NoCredentials).(*repositoryRetriever)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	uri, _ := url.Parse(server.URL)

	_, err := retriever.ping(*uri, true, retriever.context.InsecureTransport)
	if !strings.Contains(err.Error(), "does not support v2 API") {
		t.Errorf("Expected ErrNotV2Registry, got %v", err)
	}

	uri.Scheme = "https"
	_, err = retriever.ping(*uri, true, retriever.context.InsecureTransport)
	if !strings.Contains(err.Error(), "does not support v2 API") {
		t.Errorf("Expected ErrNotV2Registry, got %v", err)
	}
}

func TestShouldRetry(t *testing.T) {
	r := NewRetryRepository(nil, 1, 0).(*retryRepository)

	// nil error doesn't consume retries
	if r.shouldRetry(nil) {
		t.Fatal(r)
	}
	if r.retries != 1 || r.initial != nil {
		t.Fatal(r)
	}

	// normal error doesn't consume retries
	if r.shouldRetry(fmt.Errorf("error")) {
		t.Fatal(r)
	}
	if r.retries != 1 || r.initial != nil {
		t.Fatal(r)
	}

	// docker error doesn't consume retries
	if r.shouldRetry(errcode.ErrorCodeDenied) {
		t.Fatal(r)
	}
	if r.retries != 1 || r.initial != nil {
		t.Fatal(r)
	}

	now := time.Unix(1, 0)
	nowFn = func() time.Time {
		return now
	}
	// should retry unauthorized
	r = NewRetryRepository(nil, 1, 0).(*retryRepository)
	if !r.shouldRetry(errcode.ErrorCodeUnauthorized) {
		t.Fatal(r)
	}
	if r.retries != 0 || r.initial == nil || !r.initial.Equal(now) {
		t.Fatal(r)
	}
	if r.shouldRetry(errcode.ErrorCodeUnauthorized) {
		t.Fatal(r)
	}

	// should not retry unauthorized after one second
	r = NewRetryRepository(nil, 2, time.Second).(*retryRepository)
	if !r.shouldRetry(errcode.ErrorCodeUnauthorized) {
		t.Fatal(r)
	}
	if r.retries != 1 || r.initial == nil || !r.initial.Equal(time.Unix(1, 0)) || r.wait != (time.Second) {
		t.Fatal(r)
	}
	now = time.Unix(3, 0)
	if !r.shouldRetry(errcode.ErrorCodeUnauthorized) {
		t.Fatal(r)
	}
	if r.retries != 0 || r.initial == nil || !r.initial.Equal(time.Unix(1, 0)) || r.wait != (time.Second) {
		t.Fatal(r)
	}
	if r.shouldRetry(errcode.ErrorCodeUnauthorized) {
		t.Fatal(r)
	}

	// should retry unauthorized within one second and preserve initial time
	now = time.Unix(0, 0)
	r = NewRetryRepository(nil, 2, time.Millisecond).(*retryRepository)
	if !r.shouldRetry(errcode.ErrorCodeUnauthorized) {
		t.Fatal(r)
	}
	if r.retries != 1 || r.initial == nil || !r.initial.Equal(time.Unix(0, 0)) {
		t.Fatal(r)
	}
	now = time.Unix(0, time.Millisecond.Nanoseconds()/2)
	if !r.shouldRetry(errcode.ErrorCodeUnauthorized) {
		t.Fatal(r)
	}
	if r.retries != 0 || r.initial == nil || !r.initial.Equal(time.Unix(0, 0)) {
		t.Fatal(r)
	}
}

func TestRetryFailure(t *testing.T) {
	if !isDockerError(errcode.ErrorCodeUnauthorized, errcode.ErrorCodeUnauthorized) {
		t.Fatal("not an error")
	}

	// do not retry on Manifests()
	repo := &mockRepository{repoErr: fmt.Errorf("does not support v2 API")}
	r := NewRetryRepository(repo, 1, 0).(*retryRepository)
	if m, err := r.Manifests(nil); m != nil || err != repo.repoErr || r.retries != 1 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}

	// do not retry on Manifests()
	repo = &mockRepository{repoErr: errcode.ErrorCodeUnauthorized}
	r = NewRetryRepository(repo, 4, 0).(*retryRepository)
	if m, err := r.Manifests(nil); m != nil || err != repo.repoErr || r.retries != 4 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}

	// do not retry on non standard errors
	repo = &mockRepository{getErr: fmt.Errorf("does not support v2 API")}
	r = NewRetryRepository(repo, 4, 0).(*retryRepository)
	m, err := r.Manifests(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Get(nil, digest.Digest("foo")); err != repo.getErr || r.retries != 4 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}

	// retry four times
	repo = &mockRepository{
		getErr: errcode.ErrorCodeUnauthorized,
		blobs: &mockBlobStore{
			serveErr: errcode.ErrorCodeUnauthorized,
			statErr:  errcode.ErrorCodeUnauthorized,
			openErr:  errcode.ErrorCodeUnauthorized,
		},
	}
	r = NewRetryRepository(repo, 4, 0).(*retryRepository)
	if m, err = r.Manifests(nil); err != nil {
		t.Fatal(err)
	}
	r.retries = 2
	if _, err := m.Get(nil, digest.Digest("foo")); err != repo.getErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %#v", err, r)
	}
	r.retries = 2
	if m, err := m.Exists(nil, "foo"); m || err != repo.getErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}

	r.retries = 2
	b := r.Blobs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Stat(nil, digest.Digest("x")); err != repo.blobs.statErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %#v", err, r)
	}
	r.retries = 2
	if err := b.ServeBlob(nil, nil, nil, digest.Digest("foo")); err != repo.blobs.serveErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %#v", err, r)
	}
	r.retries = 2
	if _, err := b.Open(nil, digest.Digest("foo")); err != repo.blobs.openErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %#v", err, r)
	}
}
