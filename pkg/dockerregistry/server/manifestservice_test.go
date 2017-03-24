package server

import (
	"fmt"
	"testing"

	"github.com/docker/distribution"
	"github.com/docker/distribution/context"
	"github.com/docker/distribution/digest"
	"github.com/docker/distribution/manifest/schema2"

	"github.com/openshift/origin/pkg/dockerregistry/testutil"
)

func TestManifestServiceExists(t *testing.T) {
	namespace := "user"
	repo := "app"
	tag := "latest"

	os, client := testutil.NewFakeOpenShiftWithClient()

	testImage, err := testutil.RegisterRandomImage(os, namespace, repo, tag)
	if err != nil {
		t.Fatal(err)
	}

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

func TestManifestServicePut(t *testing.T) {
	namespace := "user"
	repo := "app"
	repoName := fmt.Sprintf("%s/%s", namespace, repo)

	_, client := testutil.NewFakeOpenShiftWithClient()

	r := newTestRepository(t, namespace, repo, testRepositoryOptions{
		client: client,
		blobs: newTestBlobStore(map[digest.Digest][]byte{
			"test:1": []byte("{}"),
		}),
	})

	ms := &manifestService{
		ctx:           context.Background(),
		repo:          r,
		manifests:     newTestManifestService(repoName, nil),
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
	dgst, err := ms.Put(ctx, manifest)
	if err != nil {
		t.Fatalf("ms.Put(ctx, manifest): %s", err)
	}

	ctx = context.Background()
	_, err = ms.Get(ctx, dgst)
	if err != nil {
		t.Errorf("ms.Get(ctx, %q): %s", dgst, err)
	}
}
