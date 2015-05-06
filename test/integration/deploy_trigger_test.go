// +build integration,!no-etcd

package integration

/*

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/rest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	watchapi "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildgenerator "github.com/openshift/origin/pkg/build/generator"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildetcd "github.com/openshift/origin/pkg/build/registry/etcd"
	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	configchangecontroller "github.com/openshift/origin/pkg/deploy/controller/configchange"
	deployconfigcontroller "github.com/openshift/origin/pkg/deploy/controller/deploymentconfig"
	imagechangecontroller "github.com/openshift/origin/pkg/deploy/controller/imagechange"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployregistry "github.com/openshift/origin/pkg/deploy/registry/deploy"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployetcd "github.com/openshift/origin/pkg/deploy/registry/etcd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	imageetcd "github.com/openshift/origin/pkg/image/registry/image/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	imagerepositoryetcd "github.com/openshift/origin/pkg/image/registry/imagerepository/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorytag"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

func TestTriggers_manual(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = testutil.Namespace()
	config.Triggers = []deployapi.DeploymentTriggerPolicy{
		{
			Type: deployapi.DeploymentTriggerManual,
		},
	}

	var err error

	dc, err := openshift.Client.DeploymentConfigs(testutil.Namespace()).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v %#v", err, config)
	}

	watch, err := openshift.KubeClient.ReplicationControllers(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), dc.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments: %v", err)
	}
	defer watch.Stop()

	config, err = openshift.Client.DeploymentConfigs(testutil.Namespace()).Generate(config.Name)
	if err != nil {
		t.Fatalf("Error generating config: %v", err)
	}
	if config.LatestVersion != 1 {
		t.Fatalf("Generated deployment should have version 1: %#v", config)
	}
	glog.Infof("config(1): %#v", config)

	new, err := openshift.Client.DeploymentConfigs(testutil.Namespace()).Update(config)
	if err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v %#v", err, config)
	}
	glog.Infof("config(2): %#v", new)

	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	deployment := event.Object.(*kapi.ReplicationController)

	if e, a := config.Name, deployment.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
	}
	if e, a := "1", deployment.Annotations[deployapi.DeploymentVersionAnnotation]; e != a {
		t.Fatalf("Deployment annotation version does not match: %#v", deployment)
	}
}

func TestTriggers_imageChange(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	imageRepo := &imageapi.ImageRepository{ObjectMeta: kapi.ObjectMeta{Name: "test-image-repo"}}

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = testutil.Namespace()
	var err error

	watch, err := openshift.KubeClient.ReplicationControllers(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}
	defer watch.Stop()

	if imageRepo, err = openshift.Client.ImageRepositories(testutil.Namespace()).Create(imageRepo); err != nil {
		t.Fatalf("Couldn't create ImageRepository: %v", err)
	}

	imageWatch, err := openshift.Client.ImageRepositories(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to ImageRepositories: %s", err)
	}
	defer imageWatch.Stop()

	// Make a function which can create a new tag event for the image repo and
	// then wait for the repo status to be asynchronously updated.
	createTagEvent := func(image string) {
		mapping := &imageapi.ImageRepositoryMapping{
			ObjectMeta: kapi.ObjectMeta{Name: imageRepo.Name},
			Tag:        "latest",
			Image: imageapi.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: image,
				},
				DockerImageReference: fmt.Sprintf("registry:8080/openshift/test-image@%s", image),
			},
		}
		if err := openshift.Client.ImageRepositoryMappings(testutil.Namespace()).Create(mapping); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		t.Log("Waiting for image repository mapping to be reflected in the IR status...")
	statusLoop:
		for {
			select {
			case event := <-imageWatch.ResultChan():
				ir := event.Object.(*imageapi.ImageRepository)
				if _, ok := ir.Status.Tags["latest"]; ok {
					t.Logf("ImageRepository now has Status with tags: %#v", ir.Status.Tags)
					break statusLoop
				} else {
					t.Log("Still waiting for latest tag status on imagerepo")
				}
			}
		}
	}

	createTagEvent("sha256:00000000000000000000000000000001")

	if config, err = openshift.Client.DeploymentConfigs(testutil.Namespace()).Create(config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	if config, err = openshift.Client.DeploymentConfigs(testutil.Namespace()).Generate(config.Name); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	if config, err = openshift.Client.DeploymentConfigs(testutil.Namespace()).Update(config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v", err)
	}

	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	deployment := event.Object.(*kapi.ReplicationController)

	if e, a := config.Name, deployment.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
	}

	createTagEvent("sha256:00000000000000000000000000000002")

	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newDeployment := event.Object.(*kapi.ReplicationController)

	if newDeployment.Name == deployment.Name {
		t.Fatalf("expected new deployment; old=%s, new=%s", deployment.Name, newDeployment.Name)
	}
}

func TestTriggers_configChange(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = testutil.Namespace()
	config.Triggers[0] = deploytest.OkConfigChangeTrigger()
	var err error

	watch, err := openshift.KubeClient.ReplicationControllers(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}
	defer watch.Stop()

	// submit the initial deployment config
	if _, err := openshift.Client.DeploymentConfigs(testutil.Namespace()).Create(config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	// verify the initial deployment exists
	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	deployment := event.Object.(*kapi.ReplicationController)

	if e, a := config.Name, deployment.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
	}

	assertEnvVarEquals("ENV1", "VAL1", deployment, t)

	// submit a new config with an updated environment variable
	if config, err = openshift.Client.DeploymentConfigs(testutil.Namespace()).Generate(config.Name); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	config.Template.ControllerTemplate.Template.Spec.Containers[0].Env[0].Value = "UPDATED"

	if _, err := openshift.Client.DeploymentConfigs(testutil.Namespace()).Update(config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v", err)
	}

	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newDeployment := event.Object.(*kapi.ReplicationController)

	assertEnvVarEquals("ENV1", "UPDATED", newDeployment, t)

	if newDeployment.Name == deployment.Name {
		t.Fatalf("expected new deployment; old=%s, new=%s", deployment.Name, newDeployment.Name)
	}
}

func assertEnvVarEquals(name string, value string, deployment *kapi.ReplicationController, t *testing.T) {
	env := deployment.Spec.Template.Spec.Containers[0].Env

	for _, e := range env {
		if e.Name == name && e.Value == value {
			return
		}
	}

	t.Fatalf("Expected env var with name %s and value %s", name, value)
}

type testOpenshift struct {
	Client     *osclient.Client
	KubeClient kclient.Interface
	Server     *httptest.Server
	stop       chan struct{}
}

func NewTestOpenshift(t *testing.T) *testOpenshift {
	t.Logf("Starting test openshift")

	openshift := &testOpenshift{
		stop: make(chan struct{}),
	}

	etcdClient := testutil.NewEtcdClient()
	etcdHelper, _ := master.NewEtcdHelper(etcdClient, klatest.Version)

	osMux := http.NewServeMux()
	openshift.Server = httptest.NewServer(osMux)

	kubeClient := client.NewOrDie(&client.Config{Host: openshift.Server.URL, Version: klatest.Version})
	osClient, _ := osclient.New(&client.Config{Host: openshift.Server.URL, Version: latest.Version})

	openshift.Client = osClient
	openshift.KubeClient = kubeClient

	kubeletClient, err := kclient.NewKubeletClient(&kclient.KubeletConfig{Port: 10250})
	if err != nil {
		t.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	handlerContainer := master.NewHandlerContainer(osMux)

	_ = master.New(&master.Config{
		EtcdHelper:       etcdHelper,
		KubeletClient:    kubeletClient,
		APIPrefix:        "/api",
		AdmissionControl: admit.NewAlwaysAdmit(),
		RestfulContainer: handlerContainer,
	})

	interfaces, _ := latest.InterfacesFor(latest.Version)

	imageStorage := imageetcd.NewREST(etcdHelper)
	imageRegistry := image.NewRegistry(imageStorage)

	imageRepositoryStorage, imageRepositoryStatus := imagerepositoryetcd.NewREST(etcdHelper, imagerepository.DefaultRegistryFunc(func() (string, bool) { return "registry:3000", true }))
	imageRepositoryRegistry := imagerepository.NewRegistry(imageRepositoryStorage, imageRepositoryStatus)
	imageRepositoryMappingStorage := imagerepositorymapping.NewREST(imageRegistry, imageRepositoryRegistry)
	imageRepositoryTagStorage := imagerepositorytag.NewREST(imageRegistry, imageRepositoryRegistry)
	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageRepositoryRegistry)

	deployEtcd := deployetcd.New(etcdHelper)
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployEtcd.GetDeploymentConfig,
			IRFn:   imageRepositoryRegistry.GetImageRepository,
			LIRFn2: imageRepositoryRegistry.ListImageRepositories,
		},
	}

	buildEtcd := buildetcd.New(etcdHelper)
	buildGenerator := &buildgenerator.BuildGenerator{
		Client: buildgenerator.Client{
			GetBuildConfigFunc:     buildEtcd.GetBuildConfig,
			UpdateBuildConfigFunc:  buildEtcd.UpdateBuildConfig,
			GetBuildFunc:           buildEtcd.GetBuild,
			CreateBuildFunc:        buildEtcd.CreateBuild,
			GetImageRepositoryFunc: imageRepositoryRegistry.GetImageRepository,
		},
	}
	buildClone, buildConfigInstantiate := buildgenerator.NewREST(buildGenerator)

	storage := map[string]rest.Storage{
		"images":                   imageStorage,
		"imageStreams":             imageRepositoryStorage,
		"imageStreamImages":        imageStreamImageStorage,
		"imageStreamMappings":      imageRepositoryMappingStorage,
		"imageStreamTags":          imageRepositoryTagStorage,
		"imageRepositories":        imageRepositoryStorage,
		"imageRepositories/status": imageRepositoryStatus,
		"imageRepositoryMappings":  imageRepositoryMappingStorage,
		"imageRepositoryTags":      imageRepositoryTagStorage,

		"deployments":               deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":         deployconfigregistry.NewREST(deployEtcd),
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, v1beta1.Codec),
		"builds":                    buildregistry.NewREST(buildEtcd),
		"builds/clone":              buildClone,
		"buildConfigs":              buildconfigregistry.NewREST(buildEtcd),
		"buildConfigs/instantiate":  buildConfigInstantiate,
	}

	version := &apiserver.APIGroupVersion{
		Root:    "/osapi",
		Version: "v1beta1",

		Storage: storage,
		Codec:   latest.Codec,

		Mapper: latest.RESTMapper,

		Creater: kapi.Scheme,
		Typer:   kapi.Scheme,
		Linker:  interfaces.MetadataAccessor,

		Admit:   admit.NewAlwaysAdmit(),
		Context: kapi.NewRequestContextMapper(),
	}
	if err := version.InstallREST(handlerContainer); err != nil {
		t.Fatalf("unable to install REST: %v", err)
	}

	dccFactory := deployconfigcontroller.DeploymentConfigControllerFactory{
		Client:     osClient,
		KubeClient: kubeClient,
		Codec:      latest.Codec,
	}
	dccFactory.Create().Run()

	cccFactory := configchangecontroller.DeploymentConfigChangeControllerFactory{
		Client:     osClient,
		KubeClient: kubeClient,
		Codec:      latest.Codec,
	}
	cccFactory.Create().Run()

	iccFactory := imagechangecontroller.ImageChangeControllerFactory{
		Client: osClient,
	}
	iccFactory.Create().Run()

	biccFactory := buildcontrollerfactory.ImageChangeControllerFactory{
		Client:                  osClient,
		BuildConfigUpdater:      buildclient.NewOSClientBuildConfigClient(osClient),
		BuildConfigInstantiator: buildclient.NewOSClientBuildConfigInstantiatorClient(osClient),
		Stop: openshift.stop,
	}
	biccFactory.Create().Run()

	bcFactory := buildcontrollerfactory.BuildControllerFactory{
		OSClient:     osClient,
		KubeClient:   kubeClient,
		BuildUpdater: buildclient.NewOSClientBuildClient(osClient),
		DockerBuildStrategy: &buildstrategy.DockerBuildStrategy{
			Image: "test-docker-builder",
			Codec: latest.Codec,
		},
		STIBuildStrategy: &buildstrategy.STIBuildStrategy{
			Image:                "test-sti-builder",
			TempDirectoryCreator: buildstrategy.STITempDirectoryCreator,
			Codec:                latest.Codec,
		},
		Stop: openshift.stop,
	}

	bcFactory.Create().Run()

	return openshift
}

func (t *testOpenshift) Close() {
}

type clientDeploymentInterface struct {
	KubeClient kclient.Interface
}

func (c *clientDeploymentInterface) GetDeployment(ctx kapi.Context, id string) (*kapi.ReplicationController, error) {
	return c.KubeClient.ReplicationControllers(kapi.NamespaceValue(ctx)).Get(id)
}

func makeRepo(name, tag, dir, image string) *imageapi.ImageRepository {
	return &imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: name},
		Status: imageapi.ImageRepositoryStatus{
			Tags: map[string]imageapi.TagEventList{
				tag: {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: dir,
							Image:                image,
						},
					},
				},
			},
		},
	}
}
*/
