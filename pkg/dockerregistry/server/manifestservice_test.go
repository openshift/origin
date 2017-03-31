package server

import (
	"fmt"
	"testing"
	"time"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"

	"github.com/openshift/origin/pkg/dockerregistry/server/configuration"
	"github.com/openshift/origin/pkg/dockerregistry/testutil"
)

func TestManifestServiceExists(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	fos, client := testutil.NewFakeOpenShiftWithClient()
	testImage := testutil.AddRandomImage(t, fos, namespace, repo, tag)

	r := newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
	})

	ms := &manifestService{
		ctx:           context.Background(),
		repo:          r,
		manifests:     nil,
		acceptschema2: r.acceptschema2,
	}

	ok, err := ms.Exists(context.Background(), digest.Digest(testImage.Name))
	if err != nil {
		t.Errorf("ms.Exists(ctx, %q): %s", testImage.Name, err)
	} else if !ok {
		t.Errorf("ms.Exists(ctx, %q): got false, want true", testImage.Name)
	}

	ok, err = ms.Exists(context.Background(), unknownBlobDigest)
	if err == nil {
		t.Errorf("ms.Exists(ctx, %q): got success, want error", unknownBlobDigest)
	}
}

func TestManifestServiceGetDoesntChangeDockerImageReference(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	fos, client := testutil.NewFakeOpenShiftWithClient()

	testImage, err := testutil.CreateRandomImage(namespace, repo)
	if err != nil {
		t.Fatal(err)
	}

	img1 := *testImage
	img1.DockerImageReference = "1"
	img1.DockerImageManifest = `{"_":"some json to start migration"}`
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

	r := newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
	})

	ms := &manifestService{
		ctx:  context.Background(),
		repo: r,
		manifests: newTestManifestService(repo, map[digest.Digest]distribution.Manifest{
			digest.Digest(testImage.Name): &schema2.DeserializedManifest{},
		}),
		acceptschema2: r.acceptschema2,
	}

	_, err = ms.Get(context.Background(), digest.Digest(testImage.Name))
	if err != nil {
		t.Fatalf("ms.Get(ctx, %q): %s", testImage.Name, err)
	}

	time.Sleep(1 * time.Second) // give it time to make the migration

	img, err = fos.GetImage(testImage.Name)
	if err != nil {
		t.Fatal(err)
	}
	if img.DockerImageManifest != "" {
		t.Errorf("image doesn't migrated, img.DockerImageManifest: want %q, got %q", "", img.DockerImageManifest)
	}
	if img.DockerImageReference != "1" {
		t.Errorf("img.DockerImageReference: want %q, got %q", "1", img.DockerImageReference)
	}
}

func TestManifestServicePut(t *testing.T) {
	namespace := "user"
	repo := "app"
	repoName := fmt.Sprintf("%s/%s", namespace, repo)

	_, client := testutil.NewFakeOpenShiftWithClient()

	bs := newTestBlobStore(map[digest.Digest][]byte{
		"test:1": []byte("{}"),
	})

	tms := newTestManifestService(repoName, nil)

	r := newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
		blobs:  bs,
	})

	ms := &manifestService{
		ctx:           context.Background(),
		repo:          r,
		manifests:     tms,
		acceptschema2: r.acceptschema2,
	}

	// TODO(dmage): eliminate global variables
	quotaEnforcing = &quotaEnforcingConfig{
		enforcementEnabled: false,
	}

	manifest := &schema2.DeserializedManifest{
		Manifest: schema2.Manifest{
			Config: distribution.Descriptor{
				Digest: "test:1",
			},
		},
	}

	ctx := context.Background()
	ctx = withAuthPerformed(ctx)
	ctx = withUserClient(ctx, client)
	ctx = WithConfiguration(ctx, &configuration.Configuration{})
	dgst, err := ms.Put(ctx, manifest)
	if err != nil {
		t.Fatalf("ms.Put(ctx, manifest): %s", err)
	}

	// recreate repository to reset cached image stream
	r = newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
		blobs:  bs,
	})

	ms = &manifestService{
		ctx:           context.Background(),
		repo:          r,
		manifests:     tms,
		acceptschema2: r.acceptschema2,
	}

	ctx = context.Background()
	_, err = ms.Get(ctx, dgst)
	if err != nil {
		t.Errorf("ms.Get(ctx, %q): %s", dgst, err)
	}
}
