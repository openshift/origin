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
	repoErr, getErr, getByTagErr, tagsErr, err error

	blobs *mockBlobStore

	manifest *schema1.SignedManifest
	tags     []string
}

func (r *mockRepository) Name() string { return "test" }

func (r *mockRepository) Manifests(ctx context.Context, options ...distribution.ManifestServiceOption) (distribution.ManifestService, error) {
	return r, r.repoErr
}
func (r *mockRepository) Blobs(ctx context.Context) distribution.BlobStore { return r.blobs }
func (r *mockRepository) Signatures() distribution.SignatureService        { return nil }
func (r *mockRepository) Exists(dgst digest.Digest) (bool, error) {
	return false, r.getErr
}
func (r *mockRepository) Get(dgst digest.Digest) (*schema1.SignedManifest, error) {
	return r.manifest, r.getErr
}
func (r *mockRepository) Enumerate() ([]digest.Digest, error) {
	return nil, r.getErr
}
func (r *mockRepository) Delete(dgst digest.Digest) error { return fmt.Errorf("not implemented") }
func (r *mockRepository) Put(manifest *schema1.SignedManifest) error {
	return fmt.Errorf("not implemented")
}
func (r *mockRepository) Tags() ([]string, error) { return r.tags, r.tagsErr }
func (r *mockRepository) ExistsByTag(tag string) (bool, error) {
	return false, r.tagsErr
}
func (r *mockRepository) GetByTag(tag string, options ...distribution.ManifestServiceOption) (*schema1.SignedManifest, error) {
	return r.manifest, r.getByTagErr
}

type mockBlobStore struct {
	distribution.BlobStore

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
	im := NewImageStreamImporter(retriever, 5, nil)
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
	repo = &mockRepository{getByTagErr: fmt.Errorf("does not support v2 API")}
	r = NewRetryRepository(repo, 4, 0).(*retryRepository)
	m, err := r.Manifests(nil)
	if err != nil {
		t.Fatal(err)
	}
	if m, err := m.GetByTag("test"); m != nil || err != repo.getByTagErr || r.retries != 4 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}

	// retry four times
	repo = &mockRepository{
		getByTagErr: errcode.ErrorCodeUnauthorized,
		getErr:      errcode.ErrorCodeUnauthorized,
		tagsErr:     errcode.ErrorCodeUnauthorized,
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
	if m, err := m.GetByTag("test"); m != nil || err != repo.getByTagErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}
	r.retries = 2
	if m, err := m.Get(digest.Digest("foo")); m != nil || err != repo.getErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}
	r.retries = 2
	if m, err := m.Exists("foo"); m || err != repo.getErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}
	r.retries = 2
	if m, err := m.Enumerate(); m != nil || err != repo.getErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}
	r.retries = 2
	if m, err := m.ExistsByTag("foo"); m || err != repo.getErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}
	r.retries = 2
	if m, err := m.Tags(); m != nil || err != repo.tagsErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}

	r.retries = 2
	b := r.Blobs(nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.Stat(nil, digest.Digest("x")); err != repo.getByTagErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}
	r.retries = 2
	if err := b.ServeBlob(nil, nil, nil, digest.Digest("foo")); err != repo.getErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}
	r.retries = 2
	if _, err := b.Open(nil, digest.Digest("foo")); err != repo.getErr || r.retries != 0 {
		t.Fatalf("unexpected: %v %v %#v", m, err, r)
	}
}
