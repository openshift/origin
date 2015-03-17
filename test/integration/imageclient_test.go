// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	imagerepositoryetcd "github.com/openshift/origin/pkg/image/registry/imagerepository/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorytag"
)

func init() {
	requireEtcd()
}

func TestImageRepositoryList(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestImageOpenShift(t)
	defer openshift.Close()

	builds, err := openshift.Client.ImageRepositories(testNamespace).List(labels.Everything(), labels.Everything())
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
	deleteAllEtcdKeys()
	openshift := NewTestImageOpenShift(t)
	defer openshift.Close()
	repo := mockImageRepository()

	if _, err := openshift.Client.ImageRepositories(testNamespace).Create(&imageapi.ImageRepository{}); err == nil || !errors.IsInvalid(err) {
		t.Fatalf("Unexpected error: %v", err)
	}

	expected, err := openshift.Client.ImageRepositories(testNamespace).Create(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty image Name %v", expected)
	}

	actual, err := openshift.Client.ImageRepositories(testNamespace).Get(repo.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("unexpected object: %s", util.ObjectDiff(expected, actual))
	}

	repos, err := openshift.Client.ImageRepositories(testNamespace).List(labels.Everything(), labels.Everything())
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(repos.Items) != 1 {
		t.Errorf("Expected one image, got %#v", repos.Items)
	}
}

func TestImageRepositoryMappingCreate(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestImageOpenShift(t)
	defer openshift.Close()
	repo := mockImageRepository()

	expected, err := openshift.Client.ImageRepositories(testNamespace).Create(repo)
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
	if err := openshift.Client.ImageRepositoryMappings(testNamespace).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify we can tag a second time with the same data, and nothing changes
	if err := openshift.Client.ImageRepositoryMappings(testNamespace).Create(mapping); err != nil {
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
	if _, err := openshift.Client.Images().Create(image); err == nil {
		t.Error("unexpected non-error")
	}
	image.DockerImageReference = "some/other/name" // can reuse references across multiple images
	actual, err := openshift.Client.Images().Create(image)
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
	if err := openshift.Client.ImageRepositoryMappings(testNamespace).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	image, err = openshift.Client.Images().Get(image.Name)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if image.DockerImageReference != "some/other/name" {
		t.Fatalf("image was unexpectedly mutated: %#v", image)
	}

	// ensure the correct tags are set
	updated, err := openshift.Client.ImageRepositories(testNamespace).Get(repo.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Tags != nil {
		t.Errorf("unexpected object: %#v", updated.Tags)
	}

	fromTag, err := openshift.Client.ImageRepositoryTags(testNamespace).Get(repo.Name, "newer")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "image1" || fromTag.UID == "" || fromTag.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = openshift.Client.ImageRepositoryTags(testNamespace).Get(repo.Name, "newest")
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
	if err := openshift.Client.ImageRepositoryMappings(testNamespace).Create(mapping); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ensure the correct tags are set
	updated, err = openshift.Client.ImageRepositories(testNamespace).Get(repo.Name)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if updated.Tags != nil {
		t.Errorf("unexpected object: %#v", updated.Tags)
	}

	fromTag, err = openshift.Client.ImageRepositoryTags(testNamespace).Get(repo.Name, "newer")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "image1" || fromTag.UID == "" || fromTag.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

	fromTag, err = openshift.Client.ImageRepositoryTags(testNamespace).Get(repo.Name, "newest")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "image2" || fromTag.UID == "" || fromTag.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}
	fromTag, err = openshift.Client.ImageRepositoryTags(testNamespace).Get(repo.Name, "anothertag")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if fromTag.Name != "image2" || fromTag.UID == "" || fromTag.DockerImageReference != "some/other/name" {
		t.Errorf("unexpected object: %#v", fromTag)
	}

}

func TestImageRepositoryDelete(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestImageOpenShift(t)
	defer openshift.Close()
	repo := mockImageRepository()

	if err := openshift.Client.ImageRepositories(testNamespace).Delete(repo.Name); err == nil || !errors.IsNotFound(err) {
		t.Fatalf("Unxpected non-error or type: %v", err)
	}
	actual, err := openshift.Client.ImageRepositories(testNamespace).Create(repo)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := openshift.Client.ImageRepositories(testNamespace).Delete(actual.Name); err != nil {
		t.Fatalf("Unxpected error: %v", err)
	}
}

type testImageOpenshift struct {
	Client       *osclient.Client
	server       *httptest.Server
	dockerServer *httptest.Server
	stop         chan struct{}
}

func (o *testImageOpenshift) Close() {
}

func NewTestImageOpenShift(t *testing.T) *testImageOpenshift {
	openshift := &testImageOpenshift{
		stop: make(chan struct{}),
	}

	etcdClient := newEtcdClient()
	etcdHelper, _ := master.NewEtcdHelper(etcdClient, klatest.Version)

	osMux := http.NewServeMux()
	openshift.server = httptest.NewServer(osMux)
	openshift.dockerServer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		t.Logf("got %s %s", req.Method, req.URL.String())
	}))

	kubeClient := client.NewOrDie(&client.Config{Host: openshift.server.URL, Version: klatest.Version})
	osClient := osclient.NewOrDie(&client.Config{Host: openshift.server.URL, Version: latest.Version})

	openshift.Client = osClient

	kubeletClient, err := kclient.NewKubeletClient(&kclient.KubeletConfig{Port: 10250})
	if err != nil {
		t.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	handlerContainer := master.NewHandlerContainer(osMux)

	_ = master.New(&master.Config{
		Client:           kubeClient,
		EtcdHelper:       etcdHelper,
		KubeletClient:    kubeletClient,
		APIPrefix:        "/api",
		RestfulContainer: handlerContainer,
	})

	interfaces, _ := latest.InterfacesFor(latest.Version)

	imageStorage := imageetcd.NewREST(etcdHelper)
	imageRegistry := image.NewRegistry(imageStorage)

	imageRepositoryStorage, imageRepositoryStatus := imagerepositoryetcd.NewREST(etcdHelper, imagerepository.DefaultRegistryFunc(func() (string, bool) { return openshift.dockerServer.URL, true }))
	imageRepositoryRegistry := imagerepository.NewRegistry(imageRepositoryStorage, imageRepositoryStatus)

	storage := map[string]apiserver.RESTStorage{
		"images":                   imageStorage,
		"imageRepositories":        imageRepositoryStorage,
		"imageRepositories/status": imageRepositoryStatus,
		"imageRepositoryMappings":  imagerepositorymapping.NewREST(imageRegistry, imageRepositoryRegistry),
		"imageRepositoryTags":      imagerepositorytag.NewREST(imageRegistry, imageRepositoryRegistry),
	}

	apiserver.NewAPIGroupVersion(storage, latest.Codec, "/osapi", "v1beta1", interfaces.MetadataAccessor, admit.NewAlwaysAdmit(), kapi.NewRequestContextMapper(), latest.RESTMapper).InstallREST(handlerContainer, "/osapi", "v1beta1")

	return openshift
}
