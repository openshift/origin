package factory

import (
	"time"

	"github.com/golang/glog"

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

// DeploymentControllerFactory can create a DeploymentController which obtains Deployments
// from a queue populated from a watch of Deployments.
// Pods are obtained from a queue populated from a watch of all pods.
type DeploymentControllerFactory struct {
	// Client satisfies DeploymentInterface.
	Client *osclient.Client
	// KubeClient satisfies PodInterface.
	KubeClient *kclient.Client
	// Environment is a set of environment which should be injected into all deployment pod containers.
	Environment []kapi.EnvVar
	// UseLocalImages configures the ImagePullPolicy for containers deployment pods.
	UseLocalImages bool
	// RecreateStrategyImage specifies which Docker image which should implement the Recreate strategy.
	RecreateStrategyImage string

	// deploymentStore is maintained on the factory to support narrowing of the pod polling scope.
	deploymentStore cache.Store
}

func (factory *DeploymentControllerFactory) Create() *controller.DeploymentController {
	deploymentQueue := cache.NewFIFO()
	cache.NewReflector(&deploymentLW{client: factory.Client, field: labels.Everything()}, &deployapi.Deployment{}, deploymentQueue).Run()

	factory.deploymentStore = cache.NewStore()
	cache.NewReflector(&deploymentLW{client: factory.Client, field: labels.Everything()}, &deployapi.Deployment{}, factory.deploymentStore).Run()

	// Kubernetes does not currently synchronize Pod status in storage with a Pod's container
	// states. Because of this, we can't receive events related to container (and thus Pod)
	// state changes, such as Running -> Terminated. As a workaround, populate the FIFO with
	// a polling implementation which relies on client calls to list Pods - the Get/List
	// REST implementations will populate the synchronized container/pod status on-demand.
	//
	// TODO: Find a way to get watch events for Pod/container status updates. The polling
	// strategy is horribly inefficient and should be addressed upstream somehow.
	podQueue := cache.NewFIFO()
	cache.NewPoller(factory.pollPods, 10*time.Second, podQueue).Run()

	return &controller.DeploymentController{
		ContainerCreator:    factory,
		DeploymentInterface: factory.Client,
		PodInterface:        &DeploymentControllerPodInterface{factory.KubeClient},
		Environment:         factory.Environment,
		NextDeployment: func() *deployapi.Deployment {
			return deploymentQueue.Pop().(*deployapi.Deployment)
		},
		NextPod: func() *kapi.Pod {
			return podQueue.Pop().(*kapi.Pod)
		},
		DeploymentStore: factory.deploymentStore,
		UseLocalImages:  factory.UseLocalImages,
	}
}

// CreateContainer lets DeploymentControllerFactory satisfy the DeploymentContainerCreator interface
// and makes a container using the configuration of the factory.
func (factory *DeploymentControllerFactory) CreateContainer(strategy *deployapi.DeploymentStrategy) *kapi.Container {
	// Every strategy type should be handled here.
	switch strategy.Type {
	case deployapi.DeploymentStrategyTypeRecreate:
		// Use the factory-configured image.
		return &kapi.Container{
			Image: factory.RecreateStrategyImage,
		}
	case deployapi.DeploymentStrategyTypeCustom:
		// Use user-defined values from the strategy input.
		return &kapi.Container{
			Image: strategy.CustomParams.Image,
			Env:   strategy.CustomParams.Environment,
		}
	default:
		// TODO: This shouldn't be reachable. Improve error handling.
		glog.Errorf("Unsupported deployment strategy type %s", strategy.Type)
		return nil
	}
}

// pollPods lists all pods associated with pending or running deployments and returns
// a cache.Enumerator suitable for use with a cache.Poller.
func (factory *DeploymentControllerFactory) pollPods() (cache.Enumerator, error) {
	list := &kapi.PodList{}

	for _, obj := range factory.deploymentStore.List() {
		deployment := obj.(*deployapi.Deployment)

		switch deployment.Status {
		case deployapi.DeploymentStatusPending, deployapi.DeploymentStatusRunning:
			// Validate the correlating pod annotation
			podID, hasPodID := deployment.Annotations[deployapi.DeploymentPodAnnotation]
			if !hasPodID {
				glog.V(2).Infof("Unexpected state: Deployment %s has no pod annotation; skipping pod polling", deployment.Name)
				continue
			}

			pod, err := factory.KubeClient.Pods(deployment.Namespace).Get(podID)
			if err != nil {
				glog.V(2).Infof("Couldn't find pod %s for deployment %s: %#v", podID, deployment.Name, err)
				continue
			}

			list.Items = append(list.Items, *pod)
		}
	}

	return &podEnumerator{list}, nil
}

type DeploymentControllerPodInterface struct {
	KubeClient kclient.Interface
}

func (i DeploymentControllerPodInterface) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return i.KubeClient.Pods(namespace).Create(pod)
}

func (i DeploymentControllerPodInterface) DeletePod(namespace, id string) error {
	return i.KubeClient.Pods(namespace).Delete(id)
}

// podEnumerator allows a cache.Poller to enumerate items in an api.PodList
type podEnumerator struct {
	*kapi.PodList
}

// Len returns the number of items in the pod list.
func (pe *podEnumerator) Len() int {
	if pe.PodList == nil {
		return 0
	}
	return len(pe.Items)
}

// Get returns the item (and ID) with the particular index.
func (pe *podEnumerator) Get(index int) (string, interface{}) {
	return pe.Items[index].Name, &pe.Items[index]
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
