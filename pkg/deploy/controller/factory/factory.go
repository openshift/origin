package factory

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	controller "github.com/openshift/origin/pkg/deploy/controller"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// DeploymentConfigControllerFactory can create a DeploymentConfigController which obtains
// DeploymentConfigs from a queue populated from a watch of all DeploymentConfigs.
type DeploymentConfigControllerFactory struct {
	Client *osclient.Client
}

func (factory *DeploymentConfigControllerFactory) Create() *controller.DeploymentConfigController {
	queue := cache.NewFIFO()
	cache.NewReflector(&deploymentConfigLW{factory.Client}, &deployapi.DeploymentConfig{}, queue).Run()

	return &controller.DeploymentConfigController{
		DeploymentInterface: factory.Client,
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return queue.Pop().(*deployapi.DeploymentConfig)
		},
	}
}

// BasicDeploymentControllerFactory can create a BasicDeploymentController which obtains Deployments
// from a queue populated from a watch of Deployments whose strategy DeploymentStrategyTypeBasic.
type BasicDeploymentControllerFactory struct {
	Client     *osclient.Client
	KubeClient *kclient.Client
}

func (factory *BasicDeploymentControllerFactory) Create() *controller.BasicDeploymentController {
	field := labels.SelectorFromSet(labels.Set{"strategy.type": string(deployapi.DeploymentStrategyTypeBasic)})
	queue := cache.NewFIFO()
	cache.NewReflector(&deploymentLW{client: factory.Client, field: field}, &deployapi.Deployment{}, queue).Run()

	return &controller.BasicDeploymentController{
		DeploymentUpdater:           factory.Client,
		ReplicationControllerClient: factory.KubeClient,
		NextDeployment: func() *deployapi.Deployment {
			return queue.Pop().(*deployapi.Deployment)
		},
	}
}

// CustomPodDeploymentControllerFactory can create a CustomPodDeploymentController which obtains Deployments
// from a queue populated from a watch of Deployments whose strategy is DeploymentStrategyTypeCustomPod.
// Pods are obtained from a queue populated from a watch of all pods.
type CustomPodDeploymentControllerFactory struct {
	Client         *osclient.Client
	KubeClient     *kclient.Client
	Environment    []kapi.EnvVar
	DefaultImage   string
	UseLocalImages bool
}

func (factory *CustomPodDeploymentControllerFactory) Create() *controller.CustomPodDeploymentController {
	deploymentFieldSelector := labels.SelectorFromSet(labels.Set{"strategy.type": string(deployapi.DeploymentStrategyTypeCustomPod)})
	dQueue := cache.NewFIFO()
	cache.NewReflector(&deploymentLW{client: factory.Client, field: deploymentFieldSelector}, &deployapi.Deployment{}, dQueue).Run()
	dStore := cache.NewStore()
	cache.NewReflector(&deploymentLW{client: factory.Client, field: deploymentFieldSelector}, &deployapi.Deployment{}, dStore).Run()
	pQueue := cache.NewFIFO()
	pSelector, _ := labels.ParseSelector("deployment!=")
	cache.NewReflector(&podLW{client: factory.KubeClient, labelSelector: pSelector}, &kapi.Pod{}, pQueue).Run()

	return &controller.CustomPodDeploymentController{
		DeploymentInterface: factory.Client,
		PodInterface:        factory.KubeClient,
		Environment:         factory.Environment,
		NextDeployment: func() *deployapi.Deployment {
			return dQueue.Pop().(*deployapi.Deployment)
		},
		NextPod: func() *kapi.Pod {
			return pQueue.Pop().(*kapi.Pod)
		},
		DeploymentStore: dStore,
		DefaultImage:    factory.DefaultImage,
		UseLocalImages:  factory.UseLocalImages,
	}
}

// DeploymentConfigChangeControllerFactory can create a DeploymentConfigChangeController which obtains DeploymentConfigs
// from a queue populated from a watch of all DeploymentConfigs.
type DeploymentConfigChangeControllerFactory struct {
	Client osclient.Interface
}

func (factory *DeploymentConfigChangeControllerFactory) Create() *controller.DeploymentConfigChangeController {
	queue := cache.NewFIFO()
	cache.NewReflector(&deploymentConfigLW{factory.Client}, &deployapi.DeploymentConfig{}, queue).Run()

	store := cache.NewStore()
	cache.NewReflector(&deploymentLW{client: factory.Client, field: labels.Everything()}, &deployapi.Deployment{}, store).Run()

	return &controller.DeploymentConfigChangeController{
		ChangeStrategy: factory.Client,
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			return queue.Pop().(*deployapi.DeploymentConfig)
		},
		DeploymentStore: store,
	}
}

// ImageChangeControllerFactory can create an ImageChangeController which obtains ImageRepositories
// from a queue populated from a watch of all ImageRepositories.
type ImageChangeControllerFactory struct {
	Client *osclient.Client
}

func (factory *ImageChangeControllerFactory) Create() *controller.ImageChangeController {
	queue := cache.NewFIFO()
	cache.NewReflector(&imageRepositoryLW{factory.Client}, &imageapi.ImageRepository{}, queue).Run()

	store := cache.NewStore()
	cache.NewReflector(&deploymentConfigLW{factory.Client}, &deployapi.DeploymentConfig{}, store).Run()

	return &controller.ImageChangeController{
		DeploymentConfigInterface: factory.Client,
		DeploymentConfigStore:     store,
		NextImageRepository: func() *imageapi.ImageRepository {
			return queue.Pop().(*imageapi.ImageRepository)
		},
	}
}

// podLW is a ListWatcher implementation for pods.
type podLW struct {
	client        *kclient.Client
	labelSelector labels.Selector
}

// List lists all pods.
func (lw *podLW) List() (runtime.Object, error) {
	pods, err := lw.client.ListPods(kapi.NewContext(), lw.labelSelector)
	if err != nil {
		return nil, err
	}

	return pods, nil
}

// Watch watches all pods with the given selector.
func (lw *podLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.
		Get().
		Path("watch").
		Path("pods").
		SelectorParam("labels", lw.labelSelector).
		Param("resourceVersion", resourceVersion).
		Watch()
}

// deploymentLW is a ListWatcher implementation for Deployments.
type deploymentLW struct {
	client osclient.Interface
	field  labels.Selector
}

// List lists all Deployments which match the given field selector.
func (lw *deploymentLW) List() (runtime.Object, error) {
	return lw.client.ListDeployments(kapi.NewContext(), labels.Everything(), lw.field)
}

// Watch watches all Deployments matching the given field selector.
func (lw *deploymentLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.WatchDeployments(kapi.NewContext(), labels.Everything(), lw.field, "0")
}

// deploymentConfigLW is a ListWatcher implementation for DeploymentConfigs.
type deploymentConfigLW struct {
	client osclient.Interface
}

// List lists all DeploymentConfigs.
func (lw *deploymentConfigLW) List() (runtime.Object, error) {
	return lw.client.ListDeploymentConfigs(kapi.NewContext(), labels.Everything(), labels.Everything())
}

// Watch watches all DeploymentConfigs.
func (lw *deploymentConfigLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.WatchDeploymentConfigs(kapi.NewContext(), labels.Everything(), labels.Everything(), "0")
}

// imageRepositoryLW is a ListWatcher for ImageRepositories.
type imageRepositoryLW struct {
	client osclient.Interface
}

// List lists all ImageRepositories.
func (lw *imageRepositoryLW) List() (runtime.Object, error) {
	return lw.client.ListImageRepositories(kapi.NewContext(), labels.Everything())
}

// Watch watches all ImageRepositories.
func (lw *imageRepositoryLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.WatchImageRepositories(kapi.NewContext(), labels.Everything(), labels.Everything(), "0")
}
