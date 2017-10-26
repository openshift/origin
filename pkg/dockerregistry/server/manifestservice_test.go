package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"

	registryclient "github.com/openshift/origin/pkg/dockerregistry/server/client"
	"github.com/openshift/origin/pkg/dockerregistry/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func TestManifestServiceExists(t *testing.T) {
	ctx := context.Background()
	ctx = testutil.WithTestLogger(ctx, t)

	namespace := "user"
	repo := "app"
	tag := "latest"

	fos, imageClient := testutil.NewFakeOpenShiftWithClient(ctx)
	testImage := testutil.AddRandomImage(t, fos, namespace, repo, tag)

	r := newTestRepository(ctx, t, namespace, repo, testRepositoryOptions{
		client: registryclient.NewFakeRegistryAPIClient(nil, imageClient),
	})

	ms := &manifestService{
		ctx:           ctx,
		repo:          r,
		manifests:     nil,
		acceptschema2: r.config.acceptSchema2,
	}

	ok, err := ms.Exists(ctx, digest.Digest(testImage.Name))
	if err != nil {
		t.Errorf("ms.Exists(ctx, %q): %s", testImage.Name, err)
	} else if !ok {
		t.Errorf("ms.Exists(ctx, %q): got false, want true", testImage.Name)
	}

	ok, err = ms.Exists(ctx, unknownBlobDigest)
	if err == nil {
		t.Errorf("ms.Exists(ctx, %q): got success, want error", unknownBlobDigest)
	}
}

func TestManifestServiceGetDoesntChangeDockerImageReference(t *testing.T) {
	ctx := context.Background()
	ctx = testutil.WithTestLogger(ctx, t)

	namespace := "user"
	repo := "app"
	tag := "latest"
	const img1Manifest = `{"_":"some json to start migration"}`

	fos, imageClient := testutil.NewFakeOpenShiftWithClient(ctx)

	testImage, err := testutil.CreateRandomImage(namespace, repo)
	if err != nil {
		t.Fatal(err)
	}

	img1 := *testImage
	img1.DockerImageReference = "1"
	img1.DockerImageManifest = img1Manifest
	testutil.AddUntaggedImage(t, fos, &img1)

	img2 := *testImage
	img2.DockerImageReference = "2"
	testutil.AddImageStream(t, fos, namespace, repo, nil)
	testutil.AddImage(t, fos, &img2, namespace, repo, tag)

	img, err := fos.GetImage(testImage.Name)
	if err != nil {
		t.Fatal(err)
	}
	if img.DockerImageReference != "1" {
		t.Fatalf("img.DockerImageReference: want %q, got %q", "1", img.DockerImageReference)
	}

	r := newTestRepository(ctx, t, namespace, repo, testRepositoryOptions{
		client: registryclient.NewFakeRegistryAPIClient(nil, imageClient),
	})

	ms := &manifestService{
		ctx:  ctx,
		repo: r,
		manifests: newTestManifestService(repo, map[digest.Digest]distribution.Manifest{
			digest.Digest(testImage.Name): &schema2.DeserializedManifest{},
		}),
		acceptschema2: r.config.acceptSchema2,
	}

	_, err = ms.Get(ctx, digest.Digest(testImage.Name))
	if err != nil {
		t.Fatalf("ms.Get(ctx, %q): %s", testImage.Name, err)
	}

	time.Sleep(1 * time.Second) // give it time to make the migration

	img, err = fos.GetImage(testImage.Name)
	if err != nil {
		t.Fatal(err)
	}
	if img.Annotations[imageapi.ImageManifestBlobStoredAnnotation] != "true" {
		t.Errorf("missing %q annotation on image", imageapi.ImageManifestBlobStoredAnnotation)
	}
	if img.DockerImageManifest != img1Manifest {
		t.Errorf("image doesn't migrated, img.DockerImageManifest: want %q, got %q", "", img.DockerImageManifest)
	}
	if img.DockerImageReference != "1" {
		t.Errorf("img.DockerImageReference: want %q, got %q", "1", img.DockerImageReference)
	}
}

func TestManifestServicePut(t *testing.T) {
	ctx := context.Background()
	ctx = testutil.WithTestLogger(ctx, t)

	namespace := "user"
	repo := "app"
	repoName := fmt.Sprintf("%s/%s", namespace, repo)

	_, imageClient := testutil.NewFakeOpenShiftWithClient(ctx)

	bs := newTestBlobStore(nil, blobContents{
		"test:1": []byte("{}"),
	})

	tms := newTestManifestService(repoName, nil)

	r := newTestRepository(ctx, t, namespace, repo, testRepositoryOptions{
		client: registryclient.NewFakeRegistryAPIClient(nil, imageClient),
		blobs:  bs,
	})

	ms := &manifestService{
		ctx:           ctx,
		repo:          r,
		manifests:     tms,
		acceptschema2: r.config.acceptSchema2,
	}

	manifest := &schema2.DeserializedManifest{
		Manifest: schema2.Manifest{
			Config: distribution.Descriptor{
				Digest: "test:1",
				Size:   2,
			},
		},
	}

	osclient, err := registryclient.NewFakeRegistryClient(imageClient).Client()
	if err != nil {
		t.Fatal(err)
	}

	putCtx := withAuthPerformed(ctx)
	putCtx = withUserClient(putCtx, osclient)
	dgst, err := ms.Put(putCtx, manifest)
	if err != nil {
		t.Fatalf("ms.Put(ctx, manifest): %s", err)
	}

	// recreate repository to reset cached image stream
	r = newTestRepository(ctx, t, namespace, repo, testRepositoryOptions{
		client: registryclient.NewFakeRegistryAPIClient(nil, imageClient),
		blobs:  bs,
	})

	ms = &manifestService{
		ctx:           ctx,
		repo:          r,
		manifests:     tms,
		acceptschema2: r.config.acceptSchema2,
	}

	_, err = ms.Get(ctx, dgst)
	if err != nil {
		t.Errorf("ms.Get(ctx, %q): %s", dgst, err)
	}
}
