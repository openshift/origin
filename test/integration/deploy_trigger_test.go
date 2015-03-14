// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	watchapi "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontrollerfactory "github.com/openshift/origin/pkg/build/controller/factory"
	buildstrategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildregistry "github.com/openshift/origin/pkg/build/registry/build"
	buildconfigregistry "github.com/openshift/origin/pkg/build/registry/buildconfig"
	buildetcd "github.com/openshift/origin/pkg/build/registry/etcd"
	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
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
)

func init() {
	requireEtcd()
}

func TestSuccessfulManualDeployment(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	config := manualDeploymentConfig()
	var err error

	dc, err := openshift.Client.DeploymentConfigs(testNamespace).Create(config)
	if err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v %#v", err, config)
	}

	watch, err := openshift.KubeClient.ReplicationControllers(testNamespace).Watch(labels.Everything(), labels.Everything(), dc.ResourceVersion)
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments: %v", err)
	}
	defer watch.Stop()

	config, err = openshift.Client.DeploymentConfigs(testNamespace).Generate(config.Name)
	if err != nil {
		t.Fatalf("Error generating config: %v", err)
	}
	if config.LatestVersion != 1 {
		t.Fatalf("Generated deployment should have version 1: %#v", config)
	}
	glog.Infof("config(1): %#v", config)

	new, err := openshift.Client.DeploymentConfigs(testNamespace).Update(config)
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

func TestSimpleImageChangeTrigger(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	imageRepo := &imageapi.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test-image-repo"},
		DockerImageRepository: "registry:8080/openshift/test-image",
		Tags: map[string]string{
			"latest": "ref-1",
		},
	}

	config := imageChangeDeploymentConfig()
	var err error

	watch, err := openshift.KubeClient.ReplicationControllers(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}
	defer watch.Stop()

	if imageRepo, err = openshift.Client.ImageRepositories(testNamespace).Create(imageRepo); err != nil {
		t.Fatalf("Couldn't create ImageRepository: %v", err)
	}

	if _, err := openshift.Client.DeploymentConfigs(testNamespace).Create(config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	if config, err = openshift.Client.DeploymentConfigs(testNamespace).Generate(config.Name); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	if _, err := openshift.Client.DeploymentConfigs(testNamespace).Update(config); err != nil {
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

	imageRepo.Tags["latest"] = "ref-2"

	if _, err = openshift.Client.ImageRepositories(testNamespace).Update(imageRepo); err != nil {
		t.Fatalf("Error updating imageRepo: %v", err)
	}

	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newDeployment := event.Object.(*kapi.ReplicationController)

	if newDeployment.Name == deployment.Name {
		t.Fatalf("expected new deployment; old=%s, new=%s", deployment.Name, newDeployment.Name)
	}
}

func TestSimpleImageChangeTriggerFrom(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	imageRepo := &imageapi.ImageRepository{
		ObjectMeta: kapi.ObjectMeta{Name: "test-image-repo"},
		Tags: map[string]string{
			"latest": "ref-1",
		},
	}

	config := imageChangeDeploymentConfig()
	config.Triggers[0].ImageChangeParams.RepositoryName = ""
	config.Triggers[0].ImageChangeParams.From = kapi.ObjectReference{
		Name: "test-image-repo",
	}
	var err error

	watch, err := openshift.KubeClient.ReplicationControllers(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}
	defer watch.Stop()

	if imageRepo, err = openshift.Client.ImageRepositories(testNamespace).Create(imageRepo); err != nil {
		t.Fatalf("Couldn't create ImageRepository: %v", err)
	}

	if _, err := openshift.Client.DeploymentConfigs(testNamespace).Create(config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	if config, err = openshift.Client.DeploymentConfigs(testNamespace).Generate(config.Name); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	if _, err := openshift.Client.DeploymentConfigs(testNamespace).Update(config); err != nil {
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

	imageRepo.Tags["latest"] = "ref-2"

	if _, err = openshift.Client.ImageRepositories(testNamespace).Update(imageRepo); err != nil {
		t.Fatalf("Error updating imageRepo: %v", err)
	}

	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newDeployment := event.Object.(*kapi.ReplicationController)

	if newDeployment.Name == deployment.Name {
		t.Fatalf("expected new deployment; old=%s, new=%s", deployment.Name, newDeployment.Name)
	}
	if a, e := newDeployment.Spec.Template.Spec.Containers[0].Image, "registry:3000/integration-test/test-image-repo:ref-2"; e != a {
		t.Fatalf("new deployment isn't pointing to the right image: %s %s", e, a)
	}
}

func TestSimpleConfigChangeTrigger(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)
	defer openshift.Close()

	config := changeDeploymentConfig()
	var err error

	watch, err := openshift.KubeClient.ReplicationControllers(testNamespace).Watch(labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}
	defer watch.Stop()

	// submit the initial deployment config
	if _, err := openshift.Client.DeploymentConfigs(testNamespace).Create(config); err != nil {
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

	assertEnvVarEquals("ENV_TEST", "ENV_VALUE1", deployment, t)

	// submit a new config with an updated environment variable
	if config, err = openshift.Client.DeploymentConfigs(testNamespace).Generate(config.Name); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	config.Template.ControllerTemplate.Template.Spec.Containers[0].Env[0].Value = "UPDATED"

	if _, err := openshift.Client.DeploymentConfigs(testNamespace).Update(config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v", err)
	}

	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newDeployment := event.Object.(*kapi.ReplicationController)

	assertEnvVarEquals("ENV_TEST", "UPDATED", newDeployment, t)

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

type podInfoGetter struct {
	PodInfo kapi.PodInfo
	Error   error
}

func (p *podInfoGetter) GetPodInfo(host, namespace, podID string) (kapi.PodInfo, error) {
	return p.PodInfo, p.Error
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

	etcdClient := newEtcdClient()
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
		Client:           kubeClient,
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

	deployEtcd := deployetcd.New(etcdHelper)
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployEtcd.GetDeploymentConfig,
			IRFn:   imageRepositoryRegistry.GetImageRepository,
			LIRFn2: imageRepositoryRegistry.ListImageRepositories,
		},
		Codec: latest.Codec,
	}

	buildEtcd := buildetcd.New(etcdHelper)

	storage := map[string]apiserver.RESTStorage{
		"images":                    imageStorage,
		"imageRepositories":         imageRepositoryStorage,
		"imageRepositories/status":  imageRepositoryStatus,
		"imageRepositoryMappings":   imagerepositorymapping.NewREST(imageRegistry, imageRepositoryRegistry),
		"deployments":               deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":         deployconfigregistry.NewREST(deployEtcd),
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, v1beta1.Codec),
		"builds":                    buildregistry.NewREST(buildEtcd),
		"buildConfigs":              buildconfigregistry.NewREST(buildEtcd),
	}

	apiserver.NewAPIGroupVersion(storage, v1beta1.Codec, "/osapi", "v1beta1", interfaces.MetadataAccessor, admit.NewAlwaysAdmit(), kapi.NewRequestContextMapper(), latest.RESTMapper).InstallREST(handlerContainer, "/osapi", "v1beta1")

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
		Client:             osClient,
		BuildConfigUpdater: buildclient.NewOSClientBuildConfigClient(osClient),
		BuildCreator:       buildclient.NewOSClientBuildClient(osClient),
		Stop:               openshift.stop,
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

func imageChangeDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "image-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnImageChange,
				ImageChangeParams: &deployapi.DeploymentTriggerImageChangeParams{
					Automatic: true,
					ContainerNames: []string{
						"container-1",
					},
					RepositoryName: "registry:8080/openshift/test-image",
					Tag:            "latest",
				},
			},
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{
					"name": "test-pod",
				},
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{
							"name": "test-pod",
						},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:  "container-1",
								Image: "registry:8080/openshift/test-image:ref-1",
							},
							{
								Name:  "container-2",
								Image: "registry:8080/openshift/another-test-image:ref-1",
							},
						},
					},
				},
			},
		},
	}
}

func manualDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "manual-deploy-config"},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{
					"name": "test-pod",
				},
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{
							"name": "test-pod",
						},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:  "container-1",
								Image: "registry:8080/openshift/test-image:ref-1",
							},
						},
					},
				},
			},
		},
	}
}

func changeDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "change-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerOnConfigChange,
			},
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
			},
			ControllerTemplate: kapi.ReplicationControllerSpec{
				Replicas: 1,
				Selector: map[string]string{
					"name": "test-pod",
				},
				Template: &kapi.PodTemplateSpec{
					ObjectMeta: kapi.ObjectMeta{
						Labels: map[string]string{
							"name": "test-pod",
						},
					},
					Spec: kapi.PodSpec{
						Containers: []kapi.Container{
							{
								Name:  "container-1",
								Image: "registry:8080/openshift/test-image:ref-1",
								Env: []kapi.EnvVar{
									{
										Name:  "ENV_TEST",
										Value: "ENV_VALUE1",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
