// +build integration,etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	klatest "k8s.io/kubernetes/pkg/api/latest"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/apiserver"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/master"
	"k8s.io/kubernetes/pkg/tools/etcdtest"
	"k8s.io/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildetcd "github.com/openshift/origin/pkg/build/registry/build/etcd"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildconfigetcd "github.com/openshift/origin/pkg/build/registry/buildconfig/etcd"
	"github.com/openshift/origin/pkg/build/webhook"
	"github.com/openshift/origin/pkg/build/webhook/generic"
	"github.com/openshift/origin/pkg/build/webhook/github"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
	testutil "github.com/openshift/origin/test/util"

	buildclonestorage "github.com/openshift/origin/pkg/build/registry/clone/generator"
	buildinstantiatestorage "github.com/openshift/origin/pkg/build/registry/instantiate/generator"
)

func init() {
	testutil.RequireEtcd()
}

type fakeSubjectAccessReviewRegistry struct {
}

var _ subjectaccessreview.Registry = &fakeSubjectAccessReviewRegistry{}

func (f *fakeSubjectAccessReviewRegistry) CreateSubjectAccessReview(ctx kapi.Context, subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	return &authorizationapi.SubjectAccessReviewResponse{Allowed: true}, nil
}

func TestListBuilds(t *testing.T) {

	testutil.DeleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()

	builds, err := openshift.Client.Builds(testutil.Namespace()).List(labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(builds.Items) != 0 {
		t.Errorf("Expected no builds, got %#v", builds.Items)
	}
}

func TestCreateBuild(t *testing.T) {

	testutil.DeleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	build := mockBuild()

	expected, err := openshift.Client.Builds(testutil.Namespace()).Create(build)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if expected.Name == "" {
		t.Errorf("Unexpected empty build Name %v", expected)
	}

	builds, err := openshift.Client.Builds(testutil.Namespace()).List(labels.Everything(), fields.Everything())
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if len(builds.Items) != 1 {
		t.Errorf("Expected one build, got %#v", builds.Items)
	}
}

func TestDeleteBuild(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	build := mockBuild()

	actual, err := openshift.Client.Builds(testutil.Namespace()).Create(build)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if err := openshift.Client.Builds(testutil.Namespace()).Delete(actual.Name); err != nil {
		t.Fatalf("Unxpected error: %v", err)
	}
}

func TestWatchBuilds(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestBuildOpenshift(t)
	defer openshift.Close()
	build := mockBuild()

	watch, err := openshift.Client.Builds(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	defer watch.Stop()

	expected, err := openshift.Client.Builds(testutil.Namespace()).Create(build)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	event := <-watch.ResultChan()
	actual := event.Object.(*buildapi.Build)

	if e, a := expected.Name, actual.Name; e != a {
		t.Errorf("Expected build Name %s, got %s", e, a)
	}
}

func mockBuild() *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: "mock-build",
			Labels: map[string]string{
				"label1": "value1",
				"label2": "value2",
			},
		},
		Spec: buildapi.BuildSpec{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://my.docker/build",
				},
				ContextDir: "context",
			},
			Strategy: buildapi.BuildStrategy{
				Type:           buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{},
			},
			Output: buildapi.BuildOutput{
				To: &kapi.ObjectReference{
					Kind: "DockerImage",
					Name: "namespace/builtimage",
				},
			},
		},
	}
}

type testBuildOpenshift struct {
	KubeClient *kclient.Client
	Client     *osclient.Client
	server     *httptest.Server
	stop       chan struct{}
	lock       sync.Mutex
}

func NewTestBuildOpenshift(t *testing.T) *testBuildOpenshift {
	openshift := &testBuildOpenshift{
		stop: make(chan struct{}),
	}

	openshift.lock.Lock()
	defer openshift.lock.Unlock()
	etcdClient := testutil.NewEtcdClient()
	etcdHelper, _ := master.NewEtcdStorage(etcdClient, latest.InterfacesFor, latest.Version, etcdtest.PathPrefix())

	osMux := http.NewServeMux()
	openshift.server = httptest.NewServer(osMux)

	kubeClient := kclient.NewOrDie(&kclient.Config{Host: openshift.server.URL, Version: klatest.DefaultVersionForLegacyGroup()})
	osClient := osclient.NewOrDie(&kclient.Config{Host: openshift.server.URL, Version: latest.Version})

	openshift.Client = osClient
	openshift.KubeClient = kubeClient

	kubeletClient, err := kclient.NewKubeletClient(&kclient.KubeletConfig{Port: 10250})
	if err != nil {
		t.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	handlerContainer := master.NewHandlerContainer(osMux)

	storageDestinations := master.NewStorageDestinations()
	storageDestinations.AddAPIGroup("", etcdHelper)

	_ = master.New(&master.Config{
		StorageDestinations: storageDestinations,
		KubeletClient:       kubeletClient,
		APIPrefix:           "/api",
		AdmissionControl:    admit.NewAlwaysAdmit(),
		RestfulContainer:    handlerContainer,
		DisableV1:           false,
	})

	interfaces, _ := latest.InterfacesFor(latest.Version)

	buildStorage := buildetcd.NewStorage(etcdHelper)
	buildRegistry := buildregistry.NewRegistry(buildStorage)
	buildConfigStorage := buildconfigetcd.NewStorage(etcdHelper)
	buildConfigRegistry := buildconfigregistry.NewRegistry(buildConfigStorage)

	imageStorage := imageetcd.NewREST(etcdHelper)
	imageRegistry := image.NewRegistry(imageStorage)

	imageStreamStorage, imageStreamStatus := imagestreametcd.NewREST(
		etcdHelper,
		imagestream.DefaultRegistryFunc(func() (string, bool) {
			return "registry:3000", true
		}),
		&fakeSubjectAccessReviewRegistry{},
	)
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatus)

	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamImageRegistry := imagestreamimage.NewRegistry(imageStreamImageStorage)

	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	imageStreamTagRegistry := imagestreamtag.NewRegistry(imageStreamTagStorage)

	buildGenerator := &buildgenerator.BuildGenerator{
		Client: buildgenerator.Client{
			GetBuildConfigFunc:      buildConfigRegistry.GetBuildConfig,
			UpdateBuildConfigFunc:   buildConfigRegistry.UpdateBuildConfig,
			GetBuildFunc:            buildRegistry.GetBuild,
			CreateBuildFunc:         buildRegistry.CreateBuild,
			GetImageStreamFunc:      imageStreamRegistry.GetImageStream,
			GetImageStreamImageFunc: imageStreamImageRegistry.GetImageStreamImage,
			GetImageStreamTagFunc:   imageStreamTagRegistry.GetImageStreamTag,
		},
	}

	buildConfigWebHooks := buildconfigregistry.NewWebHookREST(
		buildConfigRegistry,
		buildclient.NewOSClientBuildConfigInstantiatorClient(osClient),
		map[string]webhook.Plugin{
			"generic": generic.New(),
			"github":  github.New(),
		},
	)

	storage := map[string]rest.Storage{
		"builds":                   buildStorage,
		"buildConfigs":             buildConfigStorage,
		"buildConfigs/webhooks":    buildConfigWebHooks,
		"builds/clone":             buildclonestorage.NewStorage(buildGenerator),
		"buildConfigs/instantiate": buildinstantiatestorage.NewStorage(buildGenerator),
		"imageStreams":             imageStreamStorage,
		"imageStreams/status":      imageStreamStatus,
		"imageStreamTags":          imageStreamTagStorage,
		"imageStreamImages":        imageStreamImageStorage,
	}
	for k, v := range storage {
		storage[strings.ToLower(k)] = v
	}

	version := &apiserver.APIGroupVersion{
		Root:    "/oapi",
		Version: "v1",

		Storage: storage,
		Codec:   latest.Codec,

		Mapper: latest.RESTMapper,

		Creater:   kapi.Scheme,
		Typer:     kapi.Scheme,
		Convertor: kapi.Scheme,
		Linker:    interfaces.MetadataAccessor,

		Admit:   admit.NewAlwaysAdmit(),
		Context: kapi.NewRequestContextMapper(),
	}
	if err := version.InstallREST(handlerContainer); err != nil {
		t.Fatalf("unable to install REST: %v", err)
	}

	bcFactory := buildcontrollerfactory.BuildControllerFactory{
		OSClient:     osClient,
		KubeClient:   kubeClient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osClient),
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image: "test-docker-builder",
			Codec: latest.Codec,
		},
		SourceBuildStrategy: &buildstrategy.SourceBuildStrategy{
			Image:                "test-sti-builder",
			TempDirectoryCreator: buildstrategy.STITempDirectoryCreator,
			Codec:                latest.Codec,
		},
		Stop: openshift.stop,
	}

	bcFactory.Create().Run()

	bpcFactory := buildcontrollerfactory.BuildPodControllerFactory{
		OSClient:     osClient,
		KubeClient:   kubeClient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osClient),
		Stop:         openshift.stop,
	}

	bpcFactory.Create().Run()

	return openshift
}

func (t *testBuildOpenshift) Close() {
}
