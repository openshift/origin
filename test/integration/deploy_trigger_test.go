// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	osclient "github.com/openshift/origin/pkg/client"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploycontroller "github.com/openshift/origin/pkg/deploy/controller"
	deploycontrollerfactory "github.com/openshift/origin/pkg/deploy/controller/factory"
	deployconfiggenerator "github.com/openshift/origin/pkg/deploy/generator"
	deployregistry "github.com/openshift/origin/pkg/deploy/registry/deploy"
	deployconfigregistry "github.com/openshift/origin/pkg/deploy/registry/deployconfig"
	deployetcd "github.com/openshift/origin/pkg/deploy/registry/etcd"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imageetcd "github.com/openshift/origin/pkg/image/registry/etcd"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
	"github.com/openshift/origin/pkg/image/registry/imagerepositorymapping"
)

func init() {
	requireEtcd()
}

func TestSuccessfulManualDeployment(t *testing.T) {
	ctx := kapi.NewContext()
	openshift := NewTestOpenshift(t)
	defer openshift.Shutdown()

	config := manualDeploymentConfig()
	var err error

	if _, err := openshift.Client.CreateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v %#v", err, config)
	}

	if config, err = openshift.Client.GenerateDeploymentConfig(ctx, config.ID); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	if _, err := openshift.Client.UpdateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v %#v", err, config)
	}

	watch, err := openshift.Client.WatchDeployments(ctx, labels.Everything(),
		labels.Set{deployapi.DeploymentConfigIDLabel: config.ID}.AsSelector(), 0)
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments: %v", err)
	}

	openshift.DeploymentConfigController.HandleDeploymentConfig()

	event := <-watch.ResultChan()

	deployment := event.Object.(*deployapi.Deployment)

	if e, a := config.ID, deployment.Labels[deployapi.DeploymentConfigIDLabel]; e != a {
		t.Fatalf("Expected deployment DeploymentConfigIDLabel label '%s', got '%s'", e, a)
	}
}

func TestSimpleImageChangeTrigger(t *testing.T) {
	ctx := kapi.NewContext()
	openshift := NewTestOpenshift(t)
	defer openshift.Shutdown()

	imageRepo := &imageapi.ImageRepository{
		JSONBase:              kapi.JSONBase{ID: "test-image-repo"},
		DockerImageRepository: "registry:8080/openshift/test-image",
		Tags: map[string]string{
			"latest": "ref-1",
		},
	}

	config := imageChangeDeploymentConfig()
	var err error

	if imageRepo, err = openshift.Client.CreateImageRepository(ctx, imageRepo); err != nil {
		t.Fatalf("Couldn't create ImageRepository: %v", err)
	}

	if _, err := openshift.Client.CreateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	if config, err = openshift.Client.GenerateDeploymentConfig(ctx, config.ID); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	if _, err := openshift.Client.UpdateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v", err)
	}

	watch, err := openshift.Client.WatchDeployments(ctx, labels.Everything(),
		labels.Set{deployapi.DeploymentConfigIDLabel: config.ID}.AsSelector(), 0)
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}

	openshift.DeploymentConfigController.HandleDeploymentConfig()

	event := <-watch.ResultChan()

	deployment := event.Object.(*deployapi.Deployment)

	if e, a := config.ID, deployment.Labels[deployapi.DeploymentConfigIDLabel]; e != a {
		t.Fatalf("Expected deployment DeploymentConfigIDLabel label '%s', got '%s'", e, a)
	}

	imageRepo.Tags["latest"] = "ref-2"

	if _, err = openshift.Client.UpdateImageRepository(ctx, imageRepo); err != nil {
		t.Fatalf("Error updating imageRepo: %v", err)
	}

	openshift.ImageChangeController.OneImageRepo()
	openshift.DeploymentConfigController.HandleDeploymentConfig()

	event = <-watch.ResultChan()

	deployment = event.Object.(*deployapi.Deployment)

	if e, a := config.ID, deployment.Labels[deployapi.DeploymentConfigIDLabel]; e != a {
		t.Fatalf("Expected deployment DeploymentConfigIDLabel label '%s', got '%s'", e, a)
	}

	if deployment.ID != config.ID+"-2" {
		t.Fatalf("Unexpected deployment ID: %v", deployment.ID)
	}
}

func TestSimpleConfigChangeTrigger(t *testing.T) {
	ctx := kapi.NewContext()
	openshift := NewTestOpenshift(t)
	defer openshift.Shutdown()

	config := changeDeploymentConfig()
	var err error

	// submit the initial deployment config
	if _, err := openshift.Client.CreateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	// submit the initial generated config, which will cause an initial deployment
	if config, err = openshift.Client.GenerateDeploymentConfig(ctx, config.ID); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	if _, err := openshift.Client.UpdateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v", err)
	}

	watch, err := openshift.Client.WatchDeployments(ctx, labels.Everything(),
		labels.Set{deployapi.DeploymentConfigIDLabel: config.ID}.AsSelector(), 0)
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}

	openshift.DeploymentConfigController.HandleDeploymentConfig()

	event := <-watch.ResultChan()

	// verify the initial deployment exists
	deployment := event.Object.(*deployapi.Deployment)

	if e, a := config.ID, deployment.Labels[deployapi.DeploymentConfigIDLabel]; e != a {
		t.Fatalf("Expected deployment deployapi.DeploymentConfigIDLabel label '%s', got '%s'", e, a)
	}

	assertEnvVarEquals("ENV_TEST", "ENV_VALUE1", deployment, t)

	// submit a new config with an updated environment variable
	if config, err = openshift.Client.GenerateDeploymentConfig(ctx, config.ID); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	config.Template.ControllerTemplate.PodTemplate.DesiredState.Manifest.Containers[0].Env[0].Value = "UPDATED"

	if _, err := openshift.Client.UpdateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v", err)
	}

	openshift.ConfigChangeController.HandleDeploymentConfig()
	openshift.DeploymentConfigController.HandleDeploymentConfig()

	event = <-watch.ResultChan()
	deployment = event.Object.(*deployapi.Deployment)

	assertEnvVarEquals("ENV_TEST", "UPDATED", deployment, t)
}

func assertEnvVarEquals(name string, value string, deployment *deployapi.Deployment, t *testing.T) {
	env := deployment.ControllerTemplate.PodTemplate.DesiredState.Manifest.Containers[0].Env

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

func (p *podInfoGetter) GetPodInfo(host, podID string) (kapi.PodInfo, error) {
	return p.PodInfo, p.Error
}

type testOpenshift struct {
	Client                     *osclient.Client
	server                     *httptest.Server
	DeploymentConfigController *deploycontroller.DeploymentConfigController
	ConfigChangeController     *deploycontroller.ConfigChangeController
	ImageChangeController      *deploycontroller.ImageChangeController
}

func (o *testOpenshift) Shutdown() {
	deleteAllEtcdKeys()
	glog.Info("Destroyed test openshift")
}

func NewTestOpenshift(t *testing.T) *testOpenshift {
	glog.Info("Starting test openshift")

	openshift := &testOpenshift{}

	etcdClient := newEtcdClient()
	etcdHelper, _ := master.NewEtcdHelper(etcdClient.GetCluster(), klatest.Version)

	osMux := http.NewServeMux()
	openshift.server = httptest.NewServer(osMux)

	kubeClient := client.NewOrDie(&client.Config{Host: openshift.server.URL, Version: klatest.Version})
	osClient, _ := osclient.New(&client.Config{Host: openshift.server.URL, Version: latest.Version})

	openshift.Client = osClient

	kmaster := master.New(&master.Config{
		Client:             kubeClient,
		EtcdHelper:         etcdHelper,
		PodInfoGetter:      &podInfoGetter{},
		HealthCheckMinions: false,
		Minions:            []string{"127.0.0.1"},
	})

	interfaces, _ := latest.InterfacesFor(latest.Version)

	imageEtcd := imageetcd.New(etcdHelper)
	deployEtcd := deployetcd.New(etcdHelper)
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		DeploymentInterface:       deployEtcd,
		DeploymentConfigInterface: deployEtcd,
		ImageRepositoryInterface:  imageEtcd,
	}

	storage := map[string]apiserver.RESTStorage{
		"images":                    image.NewREST(imageEtcd),
		"imageRepositories":         imagerepository.NewREST(imageEtcd),
		"imageRepositoryMappings":   imagerepositorymapping.NewREST(imageEtcd, imageEtcd),
		"deployments":               deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":         deployconfigregistry.NewREST(deployEtcd),
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, v1beta1.Codec),
	}

	apiserver.NewAPIGroup(kmaster.API_v1beta1()).InstallREST(osMux, "/api/v1beta1")
	osPrefix := "/osapi/v1beta1"
	apiserver.NewAPIGroup(storage, v1beta1.Codec, osPrefix, interfaces.SelfLinker).InstallREST(osMux, osPrefix)
	apiserver.InstallSupport(osMux)

	info, err := kubeClient.ServerVersion()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if e, a := version.Get(), *info; !reflect.DeepEqual(e, a) {
		t.Errorf("Expected %#v, got %#v", e, a)
	}

	deployControllerFactory := deploycontrollerfactory.DeploymentConfigControllerFactory{osClient}
	openshift.DeploymentConfigController = deployControllerFactory.Create()

	configChangeControllerFactory := deploycontrollerfactory.ConfigChangeControllerFactory{osClient}
	openshift.ConfigChangeController = configChangeControllerFactory.Create()

	imageChangeControllerFactory := deploycontrollerfactory.ImageChangeControllerFactory{osClient}
	openshift.ImageChangeController = imageChangeControllerFactory.Create()

	return openshift
}

func imageChangeDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		JSONBase: kapi.JSONBase{ID: "image-deploy-config"},
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
				Type: "customPod",
				CustomPod: &deployapi.CustomPodDeploymentStrategy{
					Image: "registry:8080/openshift/kube-deploy",
				},
			},
			ControllerTemplate: kapi.ReplicationControllerState{
				Replicas: 1,
				ReplicaSelector: map[string]string{
					"name": "test-pod",
				},
				PodTemplate: kapi.PodTemplate{
					Labels: map[string]string{
						"name": "test-pod",
					},
					DesiredState: kapi.PodState{
						Manifest: kapi.ContainerManifest{
							Version: "v1beta1",
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
		},
	}
}

func manualDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		JSONBase: kapi.JSONBase{ID: "manual-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerManual,
			},
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: "customPod",
				CustomPod: &deployapi.CustomPodDeploymentStrategy{
					Image: "registry:8080/openshift/kube-deploy",
				},
			},
			ControllerTemplate: kapi.ReplicationControllerState{
				Replicas: 1,
				ReplicaSelector: map[string]string{
					"name": "test-pod",
				},
				PodTemplate: kapi.PodTemplate{
					Labels: map[string]string{
						"name": "test-pod",
					},
					DesiredState: kapi.PodState{
						Manifest: kapi.ContainerManifest{
							Version: "v1beta1",
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
		},
	}
}

func changeDeploymentConfig() *deployapi.DeploymentConfig {
	return &deployapi.DeploymentConfig{
		JSONBase: kapi.JSONBase{ID: "change-deploy-config"},
		Triggers: []deployapi.DeploymentTriggerPolicy{
			{
				Type: deployapi.DeploymentTriggerManual,
			},
			{
				Type: deployapi.DeploymentTriggerOnConfigChange,
			},
		},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: "customPod",
				CustomPod: &deployapi.CustomPodDeploymentStrategy{
					Image: "registry:8080/openshift/kube-deploy",
				},
			},
			ControllerTemplate: kapi.ReplicationControllerState{
				Replicas: 1,
				ReplicaSelector: map[string]string{
					"name": "test-pod",
				},
				PodTemplate: kapi.PodTemplate{
					Labels: map[string]string{
						"name": "test-pod",
					},
					DesiredState: kapi.PodState{
						Manifest: kapi.ContainerManifest{
							Version: "v1beta1",
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
		},
	}
}
