// +build integration,!no-etcd

package integration

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	klatest "github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/master"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/version"
	watchapi "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
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
	deleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)

	config := manualDeploymentConfig()
	ctx := kapi.NewContext()
	var err error

	watch, err := openshift.Client.WatchDeployments(ctx, labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments: %v", err)
	}

	if _, err := openshift.Client.CreateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v %#v", err, config)
	}

	if config, err = openshift.Client.GenerateDeploymentConfig(ctx, config.Name); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	if _, err := openshift.Client.UpdateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v %#v", err, config)
	}

	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	deployment := event.Object.(*deployapi.Deployment)

	if e, a := config.Name, deployment.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
	}
}

func TestSimpleImageChangeTrigger(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)

	imageRepo := &imageapi.ImageRepository{
		ObjectMeta:            kapi.ObjectMeta{Name: "test-image-repo"},
		DockerImageRepository: "registry:8080/openshift/test-image",
		Tags: map[string]string{
			"latest": "ref-1",
		},
	}

	config := imageChangeDeploymentConfig()
	ctx := kapi.NewContext()
	var err error

	watch, err := openshift.Client.WatchDeployments(ctx, labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}

	if imageRepo, err = openshift.Client.CreateImageRepository(ctx, imageRepo); err != nil {
		t.Fatalf("Couldn't create ImageRepository: %v", err)
	}

	if _, err := openshift.Client.CreateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	if config, err = openshift.Client.GenerateDeploymentConfig(ctx, config.Name); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	if _, err := openshift.Client.UpdateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v", err)
	}

	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	deployment := event.Object.(*deployapi.Deployment)

	if e, a := config.Name, deployment.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
	}

	imageRepo.Tags["latest"] = "ref-2"

	if _, err = openshift.Client.UpdateImageRepository(ctx, imageRepo); err != nil {
		t.Fatalf("Error updating imageRepo: %v", err)
	}

	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newDeployment := event.Object.(*deployapi.Deployment)

	if newDeployment.Name == deployment.Name {
		t.Fatalf("expected new deployment; old=%s, new=%s", deployment.Name, newDeployment.Name)
	}
}

func TestSimpleConfigChangeTrigger(t *testing.T) {
	deleteAllEtcdKeys()
	openshift := NewTestOpenshift(t)

	config := changeDeploymentConfig()
	ctx := kapi.NewContext()
	var err error

	watch, err := openshift.Client.WatchDeployments(ctx, labels.Everything(), labels.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}

	// submit the initial deployment config
	if _, err := openshift.Client.CreateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	// verify the initial deployment exists
	event := <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}

	deployment := event.Object.(*deployapi.Deployment)

	if e, a := config.Name, deployment.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
	}

	assertEnvVarEquals("ENV_TEST", "ENV_VALUE1", deployment, t)

	// submit a new config with an updated environment variable
	if config, err = openshift.Client.GenerateDeploymentConfig(ctx, config.Name); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	config.Template.ControllerTemplate.PodTemplate.DesiredState.Manifest.Containers[0].Env[0].Value = "UPDATED"

	if _, err := openshift.Client.UpdateDeploymentConfig(ctx, config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v", err)
	}

	event = <-watch.ResultChan()
	if e, a := watchapi.Added, event.Type; e != a {
		t.Fatalf("expected watch event type %s, got %s", e, a)
	}
	newDeployment := event.Object.(*deployapi.Deployment)

	assertEnvVarEquals("ENV_TEST", "UPDATED", newDeployment, t)

	if newDeployment.Name == deployment.Name {
		t.Fatalf("expected new deployment; old=%s, new=%s", deployment.Name, newDeployment.Name)
	}
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

func (p *podInfoGetter) GetPodInfo(host, namespace, podID string) (kapi.PodInfo, error) {
	return p.PodInfo, p.Error
}

type testOpenshift struct {
	Client *osclient.Client
	Server *httptest.Server
}

func NewTestOpenshift(t *testing.T) *testOpenshift {
	glog.Info("Starting test openshift")

	openshift := &testOpenshift{}

	etcdClient := newEtcdClient()
	etcdHelper, _ := master.NewEtcdHelper(etcdClient, klatest.Version)

	osMux := http.NewServeMux()
	openshift.Server = httptest.NewServer(osMux)

	kubeClient := client.NewOrDie(&client.Config{Host: openshift.Server.URL, Version: klatest.Version})
	osClient, _ := osclient.New(&client.Config{Host: openshift.Server.URL, Version: latest.Version})

	openshift.Client = osClient

	kubeletClient, err := kclient.NewKubeletClient(&kclient.KubeletConfig{Port: 10250})
	if err != nil {
		glog.Fatalf("Unable to configure Kubelet client: %v", err)
	}

	kmaster := master.New(&master.Config{
		Client:             kubeClient,
		EtcdHelper:         etcdHelper,
		HealthCheckMinions: false,
		KubeletClient:      kubeletClient,
		APIPrefix:          "/api/v1beta1",
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
	apiserver.NewAPIGroup(storage, v1beta1.Codec, osPrefix, interfaces.MetadataAccessor).InstallREST(osMux, osPrefix)
	apiserver.InstallSupport(osMux)

	info, err := kubeClient.ServerVersion()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if e, a := version.Get(), *info; !reflect.DeepEqual(e, a) {
		t.Errorf("Expected %#v, got %#v", e, a)
	}

	dccFactory := deploycontrollerfactory.DeploymentConfigControllerFactory{Client: osClient}
	dccFactory.Create().Run()

	cccFactory := deploycontrollerfactory.DeploymentConfigChangeControllerFactory{osClient}
	cccFactory.Create().Run()

	iccFactory := deploycontrollerfactory.ImageChangeControllerFactory{osClient}
	iccFactory.Create().Run()

	return openshift
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
		ObjectMeta: kapi.ObjectMeta{Name: "manual-deploy-config"},
		Template: deployapi.DeploymentTemplate{
			Strategy: deployapi.DeploymentStrategy{
				Type: deployapi.DeploymentStrategyTypeRecreate,
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
