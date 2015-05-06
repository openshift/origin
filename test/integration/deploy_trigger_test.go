// +build integration,!no-etcd

package integration

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
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
	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools/etcdtest"
	watchapi "github.com/GoogleCloudPlatform/kubernetes/pkg/watch"
	"github.com/GoogleCloudPlatform/kubernetes/plugin/pkg/admission/admit"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/api/v1beta1"
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
	"github.com/openshift/origin/pkg/image/registry/imagestream"
	imagestreametcd "github.com/openshift/origin/pkg/image/registry/imagestream/etcd"
	"github.com/openshift/origin/pkg/image/registry/imagestreamimage"
	"github.com/openshift/origin/pkg/image/registry/imagestreammapping"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
	testutil "github.com/openshift/origin/test/util"
)

func init() {
	testutil.RequireEtcd()
}

func TestTriggers_manual(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestDeployOpenshift(t)
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
	_, clusterAdminKubeConfig, err := testutil.StartTestMaster()
	if err != nil {
		t.Fatalf("error starting master: %v", err)
	}
	openshiftClusterAdminClient, err := testutil.GetClusterAdminClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting OpenShift cluster admin client: %v", err)
	}
	kubeClusterAdminClient, err := testutil.GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting OpenShift cluster admin client: %v", err)
	}
	openshiftClusterAdminClientConfig, err := testutil.GetClusterAdminClientConfig(clusterAdminKubeConfig)
	if err != nil {
		t.Fatalf("error getting cluster admin client config: %v", err)
	}
	openshiftProjectAdminClient, err := testutil.CreateNewProject(openshiftClusterAdminClient, *openshiftClusterAdminClientConfig, testutil.Namespace(), "bob")
	if err != nil {
		t.Fatalf("error creating project: %v", err)
	}

	imageStream := &imageapi.ImageStream{ObjectMeta: kapi.ObjectMeta{Name: "test-image-stream"}}

	config := deploytest.OkDeploymentConfig(0)
	config.Namespace = testutil.Namespace()

	watch, err := kubeClusterAdminClient.ReplicationControllers(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to Deployments %v", err)
	}
	defer watch.Stop()

	if imageStream, err = openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Create(imageStream); err != nil {
		t.Fatalf("Couldn't create ImageStream: %v", err)
	}

	imageWatch, err := openshiftProjectAdminClient.ImageStreams(testutil.Namespace()).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		t.Fatalf("Couldn't subscribe to ImageStreams: %s", err)
	}
	defer imageWatch.Stop()

	// Make a function which can create a new tag event for the image stream and
	// then wait for the stream status to be asynchronously updated.
	createTagEvent := func(image string) {
		mapping := &imageapi.ImageStreamMapping{
			ObjectMeta: kapi.ObjectMeta{Name: imageStream.Name},
			Tag:        "latest",
			Image: imageapi.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: image,
				},
				DockerImageReference: fmt.Sprintf("registry:8080/openshift/test-image@%s", image),
			},
		}
		if err := openshiftProjectAdminClient.ImageStreamMappings(testutil.Namespace()).Create(mapping); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		t.Log("Waiting for image stream mapping to be reflected in the IS status...")
	statusLoop:
		for {
			select {
			case event := <-imageWatch.ResultChan():
				ir := event.Object.(*imageapi.ImageStream)
				if _, ok := ir.Status.Tags["latest"]; ok {
					t.Logf("ImageStream now has Status with tags: %#v", ir.Status.Tags)
					break statusLoop
				} else {
					t.Log("Still waiting for latest tag status on imagestream")
				}
			}
		}
	}

	createTagEvent("sha256:00000000000000000000000000000001")

	if config, err = openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Create(config); err != nil {
		t.Fatalf("Couldn't create DeploymentConfig: %v", err)
	}

	if config, err = openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Generate(config.Name); err != nil {
		t.Fatalf("Error generating config: %v", err)
	}

	if config, err = openshiftProjectAdminClient.DeploymentConfigs(testutil.Namespace()).Update(config); err != nil {
		t.Fatalf("Couldn't create updated DeploymentConfig: %v", err)
	}

	var deployment *kapi.ReplicationController
	// expecting a new RC
	t.Log("Waiting for a new replication controller")
waitForNewRC:
	for {
		select {
		case event := <-watch.ResultChan():
			if event.Type == watchapi.Added {
				deployment = event.Object.(*kapi.ReplicationController)
				break waitForNewRC
			}
		}
	}

	if e, a := config.Name, deployment.Annotations[deployapi.DeploymentConfigAnnotation]; e != a {
		t.Fatalf("Expected deployment annotated with deploymentConfig '%s', got '%s'", e, a)
	}

	// expecting RC update from new to pending
	t.Log("Waiting for deployment status to be set to pending")
waitForPendingStatus:
	for {
		select {
		case event := <-watch.ResultChan():
			if event.Type == watchapi.Modified {
				deployment = event.Object.(*kapi.ReplicationController)
				break waitForPendingStatus
			}
		}
	}
	if e, a := string(deployapi.DeploymentStatusPending), deployment.Annotations[deployapi.DeploymentStatusAnnotation]; e != a {
		t.Fatalf("expected deployment status %q, got %q", e, a)
	}

	createTagEvent("sha256:00000000000000000000000000000002")

	var newDeployment *kapi.ReplicationController
	// expecting new RC for new deployment
	t.Log("Waiting for a new replication controller for the new deployment")
waitForNewDeployment:
	for {
		select {
		case event := <-watch.ResultChan():
			newDeployment = event.Object.(*kapi.ReplicationController)
			break waitForNewDeployment
		}
	}

	if newDeployment.Name == deployment.Name {
		t.Fatalf("expected new deployment; old=%s, new=%s", deployment.Name, newDeployment.Name)
	}
}

func TestTriggers_configChange(t *testing.T) {
	testutil.DeleteAllEtcdKeys()
	openshift := NewTestDeployOpenshift(t)
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

type testDeployOpenshift struct {
	Client     *osclient.Client
	KubeClient *kclient.Client
	server     *httptest.Server
	stop       chan struct{}
	lock       sync.Mutex
}

func NewTestDeployOpenshift(t *testing.T) *testDeployOpenshift {
	t.Logf("Starting test openshift")

	openshift := &testDeployOpenshift{
		stop: make(chan struct{}),
	}

	openshift.lock.Lock()
	defer openshift.lock.Unlock()

	etcdClient := testutil.NewEtcdClient()
	etcdHelper, _ := master.NewEtcdHelper(etcdClient, latest.Version, etcdtest.PathPrefix())

	osMux := http.NewServeMux()
	openshift.server = httptest.NewServer(osMux)

	kubeClient := client.NewOrDie(&client.Config{Host: openshift.server.URL, Version: klatest.Version})
	osClient := osclient.NewOrDie(&client.Config{Host: openshift.server.URL, Version: latest.Version})

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

	imageStreamStorage, imageStreamStatus := imagestreametcd.NewREST(
		etcdHelper,
		imagestream.DefaultRegistryFunc(func() (string, bool) {
			return "registry:3000", true
		}),
		&fakeSubjectAccessReviewRegistry{},
	)
	imageStreamRegistry := imagestream.NewRegistry(imageStreamStorage, imageStreamStatus)

	imageStreamMappingStorage := imagestreammapping.NewREST(imageRegistry, imageStreamRegistry)

	imageStreamImageStorage := imagestreamimage.NewREST(imageRegistry, imageStreamRegistry)
	//imageStreamImageRegistry := imagestreamimage.NewRegistry(imageStreamImageStorage)

	imageStreamTagStorage := imagestreamtag.NewREST(imageRegistry, imageStreamRegistry)
	//imageStreamTagRegistry := imagestreamtag.NewRegistry(imageStreamTagStorage)

	deployEtcd := deployetcd.New(etcdHelper)
	deployConfigGenerator := &deployconfiggenerator.DeploymentConfigGenerator{
		Client: deployconfiggenerator.Client{
			DCFn:   deployEtcd.GetDeploymentConfig,
			ISFn:   imageStreamRegistry.GetImageStream,
			LISFn2: imageStreamRegistry.ListImageStreams,
		},
	}

	storage := map[string]rest.Storage{
		"images":                    imageStorage,
		"imageStreams":              imageStreamStorage,
		"imageStreamImages":         imageStreamImageStorage,
		"imageStreamMappings":       imageStreamMappingStorage,
		"imageStreamTags":           imageStreamTagStorage,
		"deployments":               deployregistry.NewREST(deployEtcd),
		"deploymentConfigs":         deployconfigregistry.NewREST(deployEtcd),
		"generateDeploymentConfigs": deployconfiggenerator.NewREST(deployConfigGenerator, v1beta1.Codec),
	}

	version := &apiserver.APIGroupVersion{
		Root:    "/osapi",
		Version: "v1beta1",

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

	return openshift
}

func (t *testDeployOpenshift) Close() {
}

type clientDeploymentInterface struct {
	KubeClient kclient.Interface
}

func (c *clientDeploymentInterface) GetDeployment(ctx kapi.Context, id string) (*kapi.ReplicationController, error) {
	return c.KubeClient.ReplicationControllers(kapi.NamespaceValue(ctx)).Get(id)
}

func makeStream(name, tag, dir, image string) *imageapi.ImageStream {
	return &imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{Name: name},
		Status: imageapi.ImageStreamStatus{
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
