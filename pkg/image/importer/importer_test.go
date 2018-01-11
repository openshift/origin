package importer

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
	"time"

	gocontext "golang.org/x/net/context"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/manifest/schema1"
	"github.com/docker/distribution/manifest/schema2"
	"github.com/docker/distribution/reference"
	godigest "github.com/opencontainers/go-digest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	dockerregistry "github.com/openshift/origin/pkg/image/importer/dockerv1client"
	"github.com/openshift/origin/pkg/image/registryclient"
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
func (r *mockRepository) Exists(ctx context.Context, dgst godigest.Digest) (bool, error) {
	return false, r.getErr
}
func (r *mockRepository) Get(ctx context.Context, dgst godigest.Digest, options ...distribution.ManifestServiceOption) (distribution.Manifest, error) {
	for _, option := range options {
		if _, ok := option.(distribution.WithTagOption); ok {
			return r.manifest, r.getByTagErr
		}
	}
	return r.manifest, r.getErr
}
func (r *mockRepository) Delete(ctx context.Context, dgst godigest.Digest) error {
	return fmt.Errorf("not implemented")
}
func (r *mockRepository) Put(ctx context.Context, manifest distribution.Manifest, options ...distribution.ManifestServiceOption) (godigest.Digest, error) {
	return "", fmt.Errorf("not implemented")
}
func (r *mockRepository) Tags(ctx context.Context) distribution.TagService {
	return &mockTagService{repo: r}
}

type mockBlobStore struct {
	distribution.BlobStore

	blobs map[godigest.Digest][]byte

	statErr, serveErr, openErr error
}

func (r *mockBlobStore) Stat(ctx context.Context, dgst godigest.Digest) (distribution.Descriptor, error) {
	return distribution.Descriptor{}, r.statErr
}

func (r *mockBlobStore) ServeBlob(ctx context.Context, w http.ResponseWriter, req *http.Request, dgst godigest.Digest) error {
	return r.serveErr
}

func (r *mockBlobStore) Open(ctx context.Context, dgst godigest.Digest) (distribution.ReadSeekCloser, error) {
	return nil, r.openErr
}

func (r *mockBlobStore) Get(ctx context.Context, dgst godigest.Digest) ([]byte, error) {
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
	dgst, err := godigest.Parse(v)
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
	image, err := schema1ToImage(m, godigest.Digest("sha256:test"))
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
	isi := &imageapi.ImageStreamImport{
		Spec: imageapi.ImageStreamImportSpec{
			Repository: &imageapi.RepositoryImportSpec{
				From:         kapi.ObjectReference{Kind: "DockerImage", Name: uri.Host + "/test:test"},
				ImportPolicy: imageapi.TagImportPolicy{Insecure: true},
			},
		},
	}

	retriever := &mockRetriever{err: fmt.Errorf("does not support v2 API")}
	im := NewImageStreamImporter(retriever, 5, nil, nil)
	if err := im.Import(ctx, isi, nil); err != nil {
		t.Fatal(err)
	}
	if images := isi.Status.Repository.Images; len(images) != 2 || images[0].Tag != "tag1" || images[1].Tag != "test" {
		t.Errorf("unexpected images: %#v", images)
	}
}

func TestImportNothing(t *testing.T) {
	ctx := registryclient.NewContext(http.DefaultTransport, http.DefaultTransport).WithCredentials(registryclient.NoCredentials)
	isi := &imageapi.ImageStreamImport{}
	i := NewImageStreamImporter(ctx, 5, nil, nil)
	if err := i.Import(nil, isi, nil); err != nil {
		t.Fatal(err)
	}
}

func expectStatusError(status metav1.Status, message string) bool {
	if status.Status != metav1.StatusFailure || status.Message != message {
		return false
	}
	return true
}

func TestImport(t *testing.T) {
	etcdManifestSchema1 := &schema1.SignedManifest{}
	if err := json.Unmarshal([]byte(etcdManifest), etcdManifestSchema1); err != nil {
		t.Fatal(err)
	}
	t.Logf("etcd manifest schema 1 digest: %q", godigest.FromBytes([]byte(etcdManifest)))
	busyboxManifestSchema2 := &schema2.DeserializedManifest{}
	if err := busyboxManifestSchema2.UnmarshalJSON([]byte(busyboxManifest)); err != nil {
		t.Fatal(err)
	}
	busyboxConfigDigest := godigest.FromBytes([]byte(busyboxManifestConfig))
	busyboxManifestSchema2.Config = distribution.Descriptor{
		Digest:    busyboxConfigDigest,
		Size:      int64(len(busyboxManifestConfig)),
		MediaType: schema2.MediaTypeImageConfig,
	}
	t.Logf("busybox manifest schema 2 digest: %q", godigest.FromBytes([]byte(busyboxManifest)))

	insecureRetriever := &mockRetriever{
		repo: &mockRepository{
			getTagErr:   fmt.Errorf("no such tag"),
			getByTagErr: fmt.Errorf("no such manifest tag"),
			getErr:      fmt.Errorf("no such digest"),
		},
	}
	testCases := []struct {
		retriever RepositoryRetriever
		isi       imageapi.ImageStreamImport
		expect    func(*imageapi.ImageStreamImport, *testing.T)
	}{
		{
			retriever: insecureRetriever,
			isi: imageapi.ImageStreamImport{
				Spec: imageapi.ImageStreamImportSpec{
					Images: []imageapi.ImageImportSpec{
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test"}, ImportPolicy: imageapi.TagImportPolicy{Insecure: true}},
					},
				},
			},
			expect: func(isi *imageapi.ImageStreamImport, t *testing.T) {
				if !insecureRetriever.insecure {
					t.Errorf("expected retriever to beset insecure: %#v", insecureRetriever)
				}
			},
		},
		{
			retriever: &mockRetriever{
				repo: &mockRepository{
					getTagErr:   fmt.Errorf("no such tag"),
					getByTagErr: fmt.Errorf("no such manifest tag"),
					getErr:      fmt.Errorf("no such digest"),
				},
			},
			isi: imageapi.ImageStreamImport{
				Spec: imageapi.ImageStreamImportSpec{
					Images: []imageapi.ImageImportSpec{
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test"}},
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}},
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test///un/parse/able/image"}},
						{From: kapi.ObjectReference{Kind: "ImageStreamTag", Name: "test:other"}},
					},
				},
			},
			expect: func(isi *imageapi.ImageStreamImport, t *testing.T) {
				if !expectStatusError(isi.Status.Images[0].Status, "Internal error occurred: no such manifest tag") {
					t.Errorf("unexpected status: %#v", isi.Status.Images[0].Status)
				}
				if !expectStatusError(isi.Status.Images[1].Status, "Internal error occurred: no such digest") {
					t.Errorf("unexpected status: %#v", isi.Status.Images[1].Status)
				}
				if !expectStatusError(isi.Status.Images[2].Status, ` "" is invalid: from.name: Invalid value: "test///un/parse/able/image": invalid name: invalid reference format`) {
					t.Errorf("unexpected status: %s", isi.Status.Images[2].Status.Message)
				}
				// non DockerImage refs are no-ops
				if status := isi.Status.Images[3].Status; status.Status != "" {
					t.Errorf("unexpected status: %#v", isi.Status.Images[3].Status)
				}
				expectedTags := []string{"latest", "", "", ""}
				for i, image := range isi.Status.Images {
					if image.Tag != expectedTags[i] {
						t.Errorf("unexpected tag of status %d (%s != %s)", i, image.Tag, expectedTags[i])
					}
				}
			},
		},
		{
			retriever: &mockRetriever{err: fmt.Errorf("error")},
			isi: imageapi.ImageStreamImport{
				Spec: imageapi.ImageStreamImportSpec{
					Repository: &imageapi.RepositoryImportSpec{
						From: kapi.ObjectReference{Kind: "DockerImage", Name: "test"},
					},
				},
			},
			expect: func(isi *imageapi.ImageStreamImport, t *testing.T) {
				if !reflect.DeepEqual(isi.Status.Repository.AdditionalTags, []string(nil)) {
					t.Errorf("unexpected additional tags: %#v", isi.Status.Repository)
				}
				if len(isi.Status.Repository.Images) != 0 {
					t.Errorf("unexpected number of images: %#v", isi.Status.Repository.Images)
				}
				if isi.Status.Repository.Status.Status != metav1.StatusFailure || isi.Status.Repository.Status.Message != "Internal error occurred: error" {
					t.Errorf("unexpected status: %#v", isi.Status.Repository.Status)
				}
			},
		},
		{
			retriever: &mockRetriever{repo: &mockRepository{manifest: etcdManifestSchema1}},
			isi: imageapi.ImageStreamImport{
				Spec: imageapi.ImageStreamImportSpec{
					Images: []imageapi.ImageImportSpec{
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test@sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238"}},
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test:tag"}},
					},
				},
			},
			expect: func(isi *imageapi.ImageStreamImport, t *testing.T) {
				if len(isi.Status.Images) != 2 {
					t.Errorf("unexpected number of images: %#v", isi.Status.Repository.Images)
				}
				expectedTags := []string{"", "tag"}
				for i, image := range isi.Status.Images {
					if image.Status.Status != metav1.StatusSuccess {
						t.Errorf("unexpected status %d: %#v", i, image.Status)
					}
					// the image name is always the sha256, and size is calculated
					if image.Image == nil || image.Image.Name != "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238" || image.Image.DockerImageMetadata.Size != 28643712 {
						t.Errorf("unexpected image %d: %#v", i, image.Image.Name)
					}
					// the most specific reference is returned
					if image.Image.DockerImageReference != "test@sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238" {
						t.Errorf("unexpected ref %d: %#v", i, image.Image.DockerImageReference)
					}
					if image.Tag != expectedTags[i] {
						t.Errorf("unexpected tag of status %d (%s != %s)", i, image.Tag, expectedTags[i])
					}
				}
			},
		},
		{
			retriever: &mockRetriever{
				repo: &mockRepository{
					blobs: &mockBlobStore{
						blobs: map[godigest.Digest][]byte{
							busyboxConfigDigest: []byte(busyboxManifestConfig),
						},
					},
					manifest: busyboxManifestSchema2,
				},
			},
			isi: imageapi.ImageStreamImport{
				Spec: imageapi.ImageStreamImportSpec{
					Images: []imageapi.ImageImportSpec{
						{From: kapi.ObjectReference{Kind: "DockerImage", Name: "test:busybox"}},
					},
				},
			},
			expect: func(isi *imageapi.ImageStreamImport, t *testing.T) {
				if len(isi.Status.Images) != 1 {
					t.Errorf("unexpected number of images: %#v", isi.Status.Repository.Images)
				}
				image := isi.Status.Images[0]
				if image.Status.Status != metav1.StatusSuccess {
					t.Errorf("unexpected status: %#v", image.Status)
				}
				// the image name is always the sha256, and size is calculated
				if image.Image.Name != busyboxDigest {
					t.Errorf("unexpected image: %q != %q", image.Image.Name, busyboxDigest)
				}
				if image.Image.DockerImageMetadata.Size != busyboxImageSize {
					t.Errorf("unexpected image size: %d != %d", image.Image.DockerImageMetadata.Size, busyboxImageSize)
				}
				// the most specific reference is returned
				if image.Image.DockerImageReference != "test@"+busyboxDigest {
					t.Errorf("unexpected ref: %#v", image.Image.DockerImageReference)
				}
				if image.Tag != "busybox" {
					t.Errorf("unexpected tag of status: %s != busybox", image.Tag)
				}
			},
		},
		{
			retriever: &mockRetriever{
				repo: &mockRepository{
					manifest: etcdManifestSchema1,
					tags: map[string]string{
						"v1":    "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"other": "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"v2":    "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"3":     "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"3.1":   "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
						"abc":   "sha256:958608f8ecc1dc62c93b6c610f3a834dae4220c9642e6e8b4e0f2b3ad7cbd238",
					},
					getTagErr:   fmt.Errorf("no such tag"),
					getByTagErr: fmt.Errorf("no such manifest tag"),
				},
			},
			isi: imageapi.ImageStreamImport{
				Spec: imageapi.ImageStreamImportSpec{
					Repository: &imageapi.RepositoryImportSpec{
						From: kapi.ObjectReference{Kind: "DockerImage", Name: "test"},
					},
				},
			},
			expect: func(isi *imageapi.ImageStreamImport, t *testing.T) {
				if !reflect.DeepEqual(isi.Status.Repository.AdditionalTags, []string{"v2"}) {
					t.Errorf("unexpected additional tags: %#v", isi.Status.Repository)
				}
				if len(isi.Status.Repository.Images) != 5 {
					t.Errorf("unexpected number of images: %#v", isi.Status.Repository.Images)
				}
				expectedTags := []string{"3.1", "3", "abc", "other", "v1"}
				for i, image := range isi.Status.Repository.Images {
					if image.Status.Status != metav1.StatusFailure || image.Status.Message != "Internal error occurred: no such manifest tag" {
						t.Errorf("unexpected status %d: %#v", i, isi.Status.Repository.Images)
					}
					if image.Tag != expectedTags[i] {
						t.Errorf("unexpected tag of status %d (%s != %s)", i, image.Tag, expectedTags[i])
					}
				}
			},
		},
	}
	for i, test := range testCases {
		im := NewImageStreamImporter(test.retriever, 5, nil, nil)
		if err := im.Import(nil, &test.isi, &imageapi.ImageStream{}); err != nil {
			t.Errorf("%d: %v", i, err)
		}
		if test.expect != nil {
			test.expect(&test.isi, t)
		}
	}
}

const etcdManifest = `
{
   "schemaVersion": 1, 
   "tag": "latest", 
   "name": "coreos/etcd", 
   "architecture": "amd64", 
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }, 
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }, 
      {
         "blobSum": "sha256:2560187847cadddef806eaf244b7755af247a9dbabb90ca953dd2703cf423766"
      }, 
      {
         "blobSum": "sha256:744b46d0ac8636c45870a03830d8d82c20b75fbfb9bc937d5e61005d23ad4cfe"
      }
   ], 
   "history": [
      {
         "v1Compatibility": "{\"id\":\"fe50ac14986497fa6b5d2cc24feb4a561d01767bc64413752c0988cb70b0b8b9\",\"parent\":\"a5a18474fa96a3c6e240bc88e41de2afd236520caf904356ad9d5f8d875c3481\",\"created\":\"2015-12-30T22:29:13.967754365Z\",\"container\":\"c8d0f1a274b5f52fa5beb280775ef07cf18ec0f95e5ae42fbad01157e2614d42\",\"container_config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"2379/tcp\":{},\"2380/tcp\":{},\"4001/tcp\":{},\"7001/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ENTRYPOINT \\u0026{[\\\"/etcd\\\"]}\"],\"Image\":\"a5a18474fa96a3c6e240bc88e41de2afd236520caf904356ad9d5f8d875c3481\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":[\"/etcd\"],\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"2379/tcp\":{},\"2380/tcp\":{},\"4001/tcp\":{},\"7001/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"a5a18474fa96a3c6e240bc88e41de2afd236520caf904356ad9d5f8d875c3481\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":[\"/etcd\"],\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      }, 
      {
         "v1Compatibility": "{\"id\":\"a5a18474fa96a3c6e240bc88e41de2afd236520caf904356ad9d5f8d875c3481\",\"parent\":\"796d581500e960cc02095dcdeccf55db215b8e54c57e3a0b11392145ffe60cf6\",\"created\":\"2015-12-30T22:29:13.504159783Z\",\"container\":\"080708d544f85052a46fab72e701b4358c1b96cb4b805a5b2d66276fc2aaf85d\",\"container_config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"2379/tcp\":{},\"2380/tcp\":{},\"4001/tcp\":{},\"7001/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) EXPOSE 2379/tcp 2380/tcp 4001/tcp 7001/tcp\"],\"Image\":\"796d581500e960cc02095dcdeccf55db215b8e54c57e3a0b11392145ffe60cf6\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"ExposedPorts\":{\"2379/tcp\":{},\"2380/tcp\":{},\"4001/tcp\":{},\"7001/tcp\":{}},\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"796d581500e960cc02095dcdeccf55db215b8e54c57e3a0b11392145ffe60cf6\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      }, 
      {
         "v1Compatibility": "{\"id\":\"796d581500e960cc02095dcdeccf55db215b8e54c57e3a0b11392145ffe60cf6\",\"parent\":\"309c960c7f875411ae2ee2bfb97b86eee5058f3dad77206dd0df4f97df8a77fa\",\"created\":\"2015-12-30T22:29:12.912813629Z\",\"container\":\"f28be899c9b8680d4cf8585e663ad20b35019db062526844e7cfef117ce9037f\",\"container_config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:e330b1da49d993059975e46560b3bd360691498b0f2f6e00f39fc160cf8d4ec3 in /\"],\"Image\":\"309c960c7f875411ae2ee2bfb97b86eee5058f3dad77206dd0df4f97df8a77fa\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"309c960c7f875411ae2ee2bfb97b86eee5058f3dad77206dd0df4f97df8a77fa\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":13502144}"
      }, 
      {
         "v1Compatibility": "{\"id\":\"309c960c7f875411ae2ee2bfb97b86eee5058f3dad77206dd0df4f97df8a77fa\",\"created\":\"2015-12-30T22:29:12.346834862Z\",\"container\":\"1b97abade59e4b5b935aede236980a54fb500cd9ee5bd4323c832c6d7b3ffc6e\",\"container_config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:74912593c6783292c4520514f5cc9313acbd1da0f46edee0fdbed2a24a264d6f in /\"],\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.9.1\",\"config\":{\"Hostname\":\"1b97abade59e\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":15141568}"
      }
   ], 
   "signatures": [
      {
         "header": {
            "alg": "RS256", 
            "jwk": {
               "e": "AQAB", 
               "kty": "RSA", 
               "n": "yB40ou1GMvIxYs1jhxWaeoDiw3oa0_Q2UJThUPtArvO0tRzaun9FnSphhOEHIGcezfq95jy-3MN-FIjmsWgbPHY8lVDS38fF75aCw6qkholwqjmMtUIgPNYoMrg0rLUE5RRyJ84-hKf9Fk7V3fItp1mvCTGKaS3ze-y5dTTrfbNGE7qG638Dla2Fuz-9CNgRQj0JH54o547WkKJC-pG-j0jTDr8lzsXhrZC7lJas4yc-vpt3D60iG4cW_mkdtIj52ZFEgHZ56sUj7AhnNVly0ZP9W1hmw4xEHDn9WLjlt7ivwARVeb2qzsNdguUitcI5hUQNwpOVZ_O3f1rUIL_kRw"
            }
         }, 
         "protected": "eyJmb3JtYXRUYWlsIjogIkNuMCIsICJmb3JtYXRMZW5ndGgiOiA1OTI2LCAidGltZSI6ICIyMDE2LTAxLTAyVDAyOjAxOjMzWiJ9", 
         "signature": "DrQ43UWeit-thDoRGTCP0Gd2wL5K2ecyPhHo_au0FoXwuKODja0tfwHexB9ypvFWngk-ijXuwO02x3aRIZqkWpvKLxxzxwkrZnPSje4o_VrFU4z5zwmN8sJw52ODkQlW38PURIVksOxCrb0zRl87yTAAsUAJ_4UUPNltZSLnhwy-qPb2NQ8ghgsONcBxRQrhPFiWNkxDKZ3kjvzYyrXDxTcvwK3Kk_YagZ4rCOhH1B7mAdVSiSHIvvNV5grPshw_ipAoqL2iNMsxWxLjYZl9xSJQI2asaq3fvh8G8cZ7T-OahDUos_GyhnIj39C-9ouqdJqMUYFETqbzRCR6d36CpQ"
      }
   ]
}`

const busyboxDigest = "sha256:a59906e33509d14c036c8678d687bd4eec81ed7c4b8ce907b888c607f6a1e0e6"

const busyboxManifest = `{
   "schemaVersion": 2,
   "mediaType": "application/vnd.docker.distribution.manifest.v2+json",
   "config": {
      "mediaType": "application/octet-stream",
      "size": 1459,
      "digest": "sha256:2b8fd9751c4c0f5dd266fcae00707e67a2545ef34f9a29354585f93dac906749"
   },
   "layers": [
      {
         "mediaType": "application/vnd.docker.image.rootfs.diff.tar.gzip",
         "size": 667590,
         "digest": "sha256:8ddc19f16526912237dd8af81971d5e4dd0587907234be2b83e249518d5b673f"
      }
   ]
}`

const busyboxManifestConfig = `{"architecture":"amd64","config":{"Hostname":"55cd1f8f6e5b","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["sh"],"Image":"sha256:e732471cb81a564575aad46b9510161c5945deaf18e9be3db344333d72f0b4b2","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":{}},"container":"764ef4448baa9a1ce19e4ae95f8cdd4eda7a1186c512773e56dc634dff208a59","container_config":{"Hostname":"55cd1f8f6e5b","Domainname":"","User":"","AttachStdin":false,"AttachStdout":false,"AttachStderr":false,"Tty":false,"OpenStdin":false,"StdinOnce":false,"Env":["PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"],"Cmd":["/bin/sh","-c","#(nop) CMD [\"sh\"]"],"Image":"sha256:e732471cb81a564575aad46b9510161c5945deaf18e9be3db344333d72f0b4b2","Volumes":null,"WorkingDir":"","Entrypoint":null,"OnBuild":null,"Labels":{}},"created":"2016-06-23T23:23:37.198943461Z","docker_version":"1.10.3","history":[{"created":"2016-06-23T23:23:36.73131105Z","created_by":"/bin/sh -c #(nop) ADD file:9ca60502d646bdd815bb51e612c458e2d447b597b95cf435f9673f0966d41c1a in /"},{"created":"2016-06-23T23:23:37.198943461Z","created_by":"/bin/sh -c #(nop) CMD [\"sh\"]","empty_layer":true}],"os":"linux","rootfs":{"type":"layers","diff_ids":["sha256:8ac8bfaff55af948c796026ee867448c5b5b5d9dd3549f4006d9759b25d4a893"]}}`

const busyboxImageSize int64 = int64(1459 + 667590)
