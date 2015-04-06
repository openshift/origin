// +build integration,!no-etcd

package integration

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	imageapi "github.com/openshift/origin/pkg/image/api"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

func TestImageRepositoryList(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	builds, err := clusterAdminClient.ImageRepositories(testutil.Namespace()).List(labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(builds.Items) != 0 {
		t.Errorf("Expected no builds, got %#v", builds.Items)
	}
}

func mockImageRepository() *imageapi.ImageRepository {
	return &imageapi.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "test"}}
}

func TestImageRepositoryCreate(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	repo := mockImageRepository()

	if _, err := clusterAdminClient.ImageRepositories(testutil.Namespace()).Create(&imageapi.ImageRepository{}); err == nil || !errors.IsInvalid(err) {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected, err := clusterAdminClient.ImageRepositories(testutil.Namespace()).Create(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty image Name %v", expected)
	}

	actual, err := clusterAdminClient.ImageRepositories(testutil.Namespace()).Get(repo.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("unexpected object: %s", util.ObjectDiff(expected, actual))
	}

	repos, err := clusterAdminClient.ImageRepositories(testutil.Namespace()).List(labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(repos.Items) != 1 {
		t.Errorf("Expected one image, got %#v", repos.Items)
	}
}

func TestImageRepositoryMappingCreate(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	repo := mockImageRepository()

	expected, err := clusterAdminClient.ImageRepositories(testutil.Namespace()).Create(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty image Name %v", expected)
	}

	// create a mapping to an image that doesn't exist
	mapping := &imageapi.ImageRepositoryMapping{
		ObjectMeta: kapi.ObjectMeta{Name: repo.Name},
		Tag:        "newer",
		Image: imageapi.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: "image1",
			},
			DockerImageReference: "some/other/name",
		},
	}
	if err := clusterAdminClient.ImageRepositoryMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify we can tag a second time with the same data, and nothing changes
	if err := clusterAdminClient.ImageRepositoryMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected non-error or type: %v", err)
	}

	// create an image directly
	image := &imageapi.Image{
		ObjectMeta: kapi.ObjectMeta{Name: "image2"},
		DockerImageMetadata: imageapi.DockerImage{
			Config: imageapi.DockerConfig{
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

	// verify that image repository mappings cannot mutate / overwrite the image (images are immutable)
	mapping = &imageapi.ImageRepositoryMapping{
		ObjectMeta: kapi.ObjectMeta{Name: repo.Name},
		Tag:        "newest",
		Image:      *image,
	}
	mapping.Image.DockerImageReference = "different"
	if err := clusterAdminClient.ImageRepositoryMappings(testutil.Namespace()).Create(mapping); err != nil {
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
	updated, err := clusterAdminClient.ImageRepositories(testutil.Namespace()).Get(repo.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Tags != nil {
		t.Errorf("unexpected object: %#v", updated.Tags)
	}

	fromTag, err := clusterAdminClient.ImageRepositoryTags(testutil.Namespace()).Get(repo.Name, "newer")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "image1" || fromTag.UID == "" || fromTag.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminClient.ImageRepositoryTags(testutil.Namespace()).Get(repo.Name, "newest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "image2" || fromTag.UID == "" || fromTag.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	// verify that image repository mappings can use the same image for different tags
	image.ResourceVersion = ""
	mapping = &imageapi.ImageRepositoryMapping{
		ObjectMeta: kapi.ObjectMeta{Name: repo.Name},
		Tag:        "anothertag",
		Image:      *image,
	}
	if err := clusterAdminClient.ImageRepositoryMappings(testutil.Namespace()).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ensure the correct tags are set
	updated, err = clusterAdminClient.ImageRepositories(testutil.Namespace()).Get(repo.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Tags != nil {
		t.Errorf("unexpected object: %#v", updated.Tags)
	}

	fromTag, err = clusterAdminClient.ImageRepositoryTags(testutil.Namespace()).Get(repo.Name, "newer")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "image1" || fromTag.UID == "" || fromTag.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = clusterAdminClient.ImageRepositoryTags(testutil.Namespace()).Get(repo.Name, "newest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "image2" || fromTag.UID == "" || fromTag.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}
	fromTag, err = clusterAdminClient.ImageRepositoryTags(testutil.Namespace()).Get(repo.Name, "anothertag")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "image2" || fromTag.UID == "" || fromTag.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

}

func TestImageRepositoryDelete(t *testing.T) {
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	clusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	repo := mockImageRepository()

	if err := clusterAdminClient.ImageRepositories(testutil.Namespace()).Delete(repo.Name); err == nil || !errors.IsNotFound(err) {
		t.Fatalf("Unxpected non-error or type: %v", err)
	}
	actual, err := clusterAdminClient.ImageRepositories(testutil.Namespace()).Create(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := clusterAdminClient.ImageRepositories(testutil.Namespace()).Delete(actual.Name); err != nil {
		t.Fatalf("Unxpected error: %v", err)
	}
}
