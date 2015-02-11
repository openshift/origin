package factory

import (
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	controller "github.com/openshift/origin/pkg/deploy/controller"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// DeploymentConfigControllerFactory can create a DeploymentConfigController which obtains
// DeploymentConfigs from a queue populated from a watch of all DeploymentConfigs.
type DeploymentConfigControllerFactory struct {
	Client     *osclient.Client
	KubeClient kclient.Interface
	Codec      runtime.Codec
	Stop       <-chan struct{}
}

func (factory *DeploymentConfigControllerFactory) Create() *controller.DeploymentConfigController {
	deploymentConfigLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), resourceVersion)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentConfigLW, &deployapi.DeploymentConfig{}, queue).RunUntil(factory.Stop)

	return &controller.DeploymentConfigController{
		DeploymentClient: &controller.DeploymentConfigControllerDeploymentClientImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Get(name)
			},
			CreateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Create(deployment)
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			config := queue.Pop().(*deployapi.DeploymentConfig)
			panicIfStopped(factory.Stop, "deployment config controller stopped")
			return config
		},
		Codec: factory.Codec,
		Stop:  factory.Stop,
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
	// Codec is used to decode DeploymentConfigs.
	Codec runtime.Codec
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

func (factory *DeploymentControllerFactory) Create() *controller.DeploymentController {
	deploymentLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), resourceVersion)
		},
	}
	deploymentQueue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentLW, &kapi.ReplicationController{}, deploymentQueue).RunUntil(factory.Stop)

	deploymentStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentLW, &kapi.ReplicationController{}, deploymentStore).RunUntil(factory.Stop)

	// Kubernetes does not currently synchronize Pod status in storage with a Pod's container
	// states. Because of this, we can't receive events related to container (and thus Pod)
	// state changes, such as Running -> Terminated. As a workaround, populate the FIFO with
	// a polling implementation which relies on client calls to list Pods - the Get/List
	// REST implementations will populate the synchronized container/pod status on-demand.
	//
	// TODO: Find a way to get watch events for Pod/container status updates. The polling
	// strategy is horribly inefficient and should be addressed upstream somehow.
	podQueue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	pollFunc := func() (cache.Enumerator, error) {
		return pollPods(deploymentStore, factory.KubeClient)
	}
	cache.NewPoller(pollFunc, 10*time.Second, podQueue).RunUntil(factory.Stop)

	return &controller.DeploymentController{
		ContainerCreator: &defaultContainerCreator{factory.RecreateStrategyImage},
		DeploymentClient: &controller.DeploymentControllerDeploymentClientImpl{
			// Since we need to use a deployment cache to support the pod poller, go ahead and use
			// it for other deployment lookups and maintain the usual REST API for not-found errors.
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				example := &kapi.ReplicationController{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: namespace,
						Name:      name,
					}}
				obj, exists, err := deploymentStore.Get(example)
				if !exists {
					return nil, kerrors.NewNotFound(example.Kind, name)
				}
				if err != nil {
					return nil, err
				}
				return obj.(*kapi.ReplicationController), nil
			},
			UpdateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Update(deployment)
			},
		},
		PodClient: &controller.DeploymentControllerPodClientImpl{
			CreatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return factory.KubeClient.Pods(namespace).Create(pod)
			},
			DeletePodFunc: func(namespace, name string) error {
				return factory.KubeClient.Pods(namespace).Delete(name)
			},
		},
		Environment: factory.Environment,
		NextDeployment: func() *kapi.ReplicationController {
			deployment := deploymentQueue.Pop().(*kapi.ReplicationController)
			panicIfStopped(factory.Stop, "deployment controller stopped")
			return deployment
		},
		NextPod: func() *kapi.Pod {
			pod := podQueue.Pop().(*kapi.Pod)
			panicIfStopped(factory.Stop, "deployment controller stopped")
			return pod
		},
		UseLocalImages: factory.UseLocalImages,
		Codec:          factory.Codec,
		Stop:           factory.Stop,
	}
}

// CreateContainer is the default DeploymentContainerCreator. It makes containers using only
// the input strategy parameters and a user defined image for the Recreate strategy.
type defaultContainerCreator struct {
	recreateStrategyImage string
}

func (c *defaultContainerCreator) CreateContainer(strategy *deployapi.DeploymentStrategy) *kapi.Container {
	// Every strategy type should be handled here.
	switch strategy.Type {
	case deployapi.DeploymentStrategyTypeRecreate:
		// Use the factory-configured image.
		return &kapi.Container{
			Image: c.recreateStrategyImage,
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
func pollPods(deploymentStore cache.Store, kClient kclient.PodsNamespacer) (cache.Enumerator, error) {
	list := &kapi.PodList{}

	for _, obj := range deploymentStore.List() {
		deployment := obj.(*kapi.ReplicationController)

		switch deployapi.DeploymentStatus(deployment.Annotations[deployapi.DeploymentStatusAnnotation]) {
		case deployapi.DeploymentStatusPending, deployapi.DeploymentStatusRunning:
			// Validate the correlating pod annotation
			podID, hasPodID := deployment.Annotations[deployapi.DeploymentPodAnnotation]
			if !hasPodID {
				glog.V(2).Infof("Unexpected state: Deployment %s has no pod annotation; skipping pod polling", deployment.Name)
				continue
			}

			pod, err := kClient.Pods(deployment.Namespace).Get(podID)
			if err != nil {
				glog.V(2).Infof("Couldn't find pod %s for deployment %s: %#v", podID, deployment.Name, err)
				continue
			}

			list.Items = append(list.Items, *pod)
		}
	}

	return &podEnumerator{list}, nil
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
func (pe *podEnumerator) Get(index int) interface{} {
	return &pe.Items[index]
}

// DeploymentConfigChangeControllerFactory can create a DeploymentConfigChangeController which obtains DeploymentConfigs
// from a queue populated from a watch of all DeploymentConfigs.
type DeploymentConfigChangeControllerFactory struct {
	Client     osclient.Interface
	KubeClient kclient.Interface
	Codec      runtime.Codec
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

func (factory *DeploymentConfigChangeControllerFactory) Create() *controller.DeploymentConfigChangeController {
	deploymentConfigLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), resourceVersion)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentConfigLW, &deployapi.DeploymentConfig{}, queue).RunUntil(factory.Stop)

	return &controller.DeploymentConfigChangeController{
		ChangeStrategy: &controller.ChangeStrategyImpl{
			GetDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Get(name)
			},
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return factory.Client.DeploymentConfigs(namespace).Generate(name)
			},
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				return factory.Client.DeploymentConfigs(namespace).Update(config)
			},
		},
		NextDeploymentConfig: func() *deployapi.DeploymentConfig {
			config := queue.Pop().(*deployapi.DeploymentConfig)
			panicIfStopped(factory.Stop, "deployment config change controller stopped")
			return config
		},
		Codec: factory.Codec,
		Stop:  factory.Stop,
	}
}

// ImageChangeControllerFactory can create an ImageChangeController which obtains ImageRepositories
// from a queue populated from a watch of all ImageRepositories.
type ImageChangeControllerFactory struct {
	Client *osclient.Client
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

func (factory *ImageChangeControllerFactory) Create() *controller.ImageChangeController {
	imageRepositoryLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.Client.ImageRepositories(kapi.NamespaceAll).List(labels.Everything(), labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.Client.ImageRepositories(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), resourceVersion)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(imageRepositoryLW, &imageapi.ImageRepository{}, queue).RunUntil(factory.Stop)

	deploymentConfigLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).List(labels.Everything(), labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).Watch(labels.Everything(), labels.Everything(), resourceVersion)
		},
	}
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentConfigLW, &deployapi.DeploymentConfig{}, store).RunUntil(factory.Stop)

	return &controller.ImageChangeController{
		DeploymentConfigClient: &controller.ImageChangeControllerDeploymentConfigClientImpl{
			ListDeploymentConfigsFunc: func() ([]*deployapi.DeploymentConfig, error) {
				configs := []*deployapi.DeploymentConfig{}
				objs := store.List()
				for _, obj := range objs {
					configs = append(configs, obj.(*deployapi.DeploymentConfig))
				}
				return configs, nil
			},
			GenerateDeploymentConfigFunc: func(namespace, name string) (*deployapi.DeploymentConfig, error) {
				return factory.Client.DeploymentConfigs(namespace).Generate(name)
			},
			UpdateDeploymentConfigFunc: func(namespace string, config *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
				return factory.Client.DeploymentConfigs(namespace).Update(config)
			},
		},
		NextImageRepository: func() *imageapi.ImageRepository {
			repo := queue.Pop().(*imageapi.ImageRepository)
			panicIfStopped(factory.Stop, "deployment config change controller stopped")
			return repo
		},
		Stop: factory.Stop,
	}
}

// panicIfStopped panics with the provided object if the channel is closed
func panicIfStopped(ch <-chan struct{}, message interface{}) {
	select {
	case <-ch:
		panic(message)
	default:
	}
}
