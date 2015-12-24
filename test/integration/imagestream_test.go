// +build integration,etcd

package integration

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

func mockImageStream() *imageapi.ImageStream {
	return &imageapi.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "test"}}
}

func TestImageStreamMappingCreate(t *testing.T) {
	_, clusterAdminKubeConfig, err := testserver.StartTestMasterAPI()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err = testutil.CreateNamespace(clusterAdminKubeConfig, testutil.Namespace())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	stream := mockImageStream()

	expected, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Create(stream)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty image Name %v", expected)
	}

	// create a mapping to an image that doesn't exist
	mapping := &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: stream.Name},
		Tag:        "newer",
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "image1",
			},
			DockerImageReference: "some/other/name",
		},
	}
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify we can tag a second time with the same data, and nothing changes
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected non-error or type: %v", err)
	}

	// create an image directly
	image := &imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{Name: "image2"},
		DockerImageMetadata: imageapi.DockerImage{
			Config: &imageapi.DockerConfig{
				Env: []string{"A=B"},
			},
		},
	}
	if _, err := clusterAdminClient.Images().Create(image); err == nil {
		t.Error("unexpected non-error")
	}
	image.DockerImageReference = "some/other/name" // can reuse references across multiple images
	actual, err := clusterAdminClient.Images().Create(image)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual == nil || actual.Name != image.Name {
		t.Errorf("unexpected object: %#v", actual)
	}

	// verify that image stream mappings cannot mutate / overwrite the image (images are immutable)
	mapping = &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: stream.Name},
		Tag:        "newest",
		Image:      *image,
	}
	mapping.Image.DockerImageReference = "different"
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	image, err = clusterAdminClient.Images().Get(image.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if image.DockerImageReference != "some/other/name" {
		t.Fatalf("image was unexpectedly mutated: %#v", image)
	}

	// ensure the correct tags are set
	updated, err := clusterAdminClient.ImageStreams(testutil.Namespace()).Get(stream.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Spec.Tags != nil && len(updated.Spec.Tags) > 0 {
		t.Errorf("unexpected object: %#v", updated.Spec.Tags)
	}

	fromTag, err := clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "newer")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newer" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "newest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newest" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "different" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	// verify that image stream mappings can use the same image for different tags
	image.ResourceVersion = ""
	mapping = &imageapi.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{Name: stream.Name},
		Tag:        "anothertag",
		Image:      *image,
	}
	if err := clusterAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ensure the correct tags are set
	updated, err = clusterAdminClient.ImageStreams(testutil.Namespace()).Get(stream.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Spec.Tags != nil && len(updated.Spec.Tags) > 0 {
		t.Errorf("unexpected object: %#v", updated.Spec.Tags)
	}

	fromTag, err = clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "newer")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newer" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "newest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:newest" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "different" {
		t.Errorf("unexpected object: %#v", fromTag)
	}
	fromTag, err = clusterAdminClient.ImageStreamTags(testutil.Namespace()).Get(stream.Name, "anothertag")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "test:anothertag" || fromTag.Image.UID == "" || fromTag.Image.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

}
