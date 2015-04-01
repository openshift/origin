package factory

import (
	"errors"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontroller "github.com/openshift/origin/pkg/build/controller"
	strategy "github.com/openshift/origin/pkg/build/controller/strategy"
	buildutil "github.com/openshift/origin/pkg/build/util"
	osclient "github.com/openshift/origin/pkg/client"
	controller "github.com/openshift/origin/pkg/controller"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// limitedLogAndRetry stops retrying after 60 attempts.  Given the throttler configuration,
// this means this event has been retried over a period of at least 50 seconds.
func limitedLogAndRetry(obj interface{}, err error, count int) bool {
	kutil.HandleError(err)
	return count < 60
}

// BuildControllerFactory constructs BuildController objects
type BuildControllerFactory struct {
	OSClient            osclient.Interface
	KubeClient          kclient.Interface
	BuildUpdater        buildclient.BuildUpdater
	DockerBuildStrategy *strategy.DockerBuildStrategy
	STIBuildStrategy    *strategy.STIBuildStrategy
	CustomBuildStrategy *strategy.CustomBuildStrategy
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

// Create constructs a BuildController
func (factory *BuildControllerFactory) Create() controller.RunnableController {
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&buildLW{client: factory.OSClient}, &buildapi.Build{}, queue, 2*time.Minute).Run()

	client := ControllerClient{factory.KubeClient, factory.OSClient}
	buildController := &buildcontroller.BuildController{
		BuildUpdater:      factory.BuildUpdater,
		ImageStreamClient: client,
		PodManager:        client,
		BuildStrategy: &typeBasedFactoryStrategy{
			DockerBuildStrategy: factory.DockerBuildStrategy,
			STIBuildStrategy:    factory.STIBuildStrategy,
			CustomBuildStrategy: factory.CustomBuildStrategy,
		},
	}

	return &controller.RetryController{
		Queue:        queue,
		RetryManager: controller.NewQueueRetryManager(queue, cache.MetaNamespaceKeyFunc, limitedLogAndRetry, kutil.NewTokenBucketRateLimiter(1, 10)),
		Handle: func(obj interface{}) error {
			build := obj.(*buildapi.Build)
			return buildController.HandleBuild(build)
		},
	}
}

// BuildPodControllerFactory construct BuildPodController objects
type BuildPodControllerFactory struct {
	OSClient     osclient.Interface
	KubeClient   kclient.Interface
	BuildUpdater buildclient.BuildUpdater
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}

	buildStore cache.Store
}

// Create constructs a BuildPodController
func (factory *BuildPodControllerFactory) Create() controller.RunnableController {
	factory.buildStore = cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&buildLW{client: factory.OSClient}, &buildapi.Build{}, factory.buildStore, 2*time.Minute).Run()

	// Kubernetes does not currently synchronize Pod status in storage with a Pod's container
	// states. Because of this, we can't receive events related to container (and thus Pod)
	// state changes, such as Running -> Terminated. As a workaround, populate the FIFO with
	// a polling implementation which relies on client calls to list Pods - the Get/List
	// REST implementations will populate the synchronized container/pod status on-demand.
	//
	// TODO: Find a way to get watch events for Pod/container status updates. The polling
	// strategy is horribly inefficient and should be addressed upstream somehow.
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewPoller(factory.pollPods, 10*time.Second, queue).RunUntil(factory.Stop)

	client := ControllerClient{factory.KubeClient, factory.OSClient}
	buildPodController := &buildcontroller.BuildPodController{
		BuildStore:   factory.buildStore,
		BuildUpdater: factory.BuildUpdater,
		PodManager:   client,
	}

	return &controller.RetryController{
		Queue:        queue,
		RetryManager: controller.NewQueueRetryManager(queue, cache.MetaNamespaceKeyFunc, limitedLogAndRetry, kutil.NewTokenBucketRateLimiter(1, 10)),
		Handle: func(obj interface{}) error {
			pod := obj.(*kapi.Pod)
			return buildPodController.HandlePod(pod)
		},
	}
}

// ImageChangeControllerFactory can create an ImageChangeController which obtains ImageStreams
// from a queue populated from a watch of all ImageStreams.
type ImageChangeControllerFactory struct {
	Client                  osclient.Interface
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
	BuildConfigUpdater      buildclient.BuildConfigUpdater
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

// Create creates a new ImageChangeController which is used to trigger builds when a new
// image is available
func (factory *ImageChangeControllerFactory) Create() controller.RunnableController {
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&imageStreamLW{factory.Client}, &imageapi.ImageStream{}, queue, 2*time.Minute).Run()

	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&buildConfigLW{client: factory.Client}, &buildapi.BuildConfig{}, store, 2*time.Minute).Run()

	imageChangeController := &buildcontroller.ImageChangeController{
		BuildConfigStore:        store,
		BuildConfigUpdater:      factory.BuildConfigUpdater,
		BuildConfigInstantiator: factory.BuildConfigInstantiator,
		Stop: factory.Stop,
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, count int) bool {
				kutil.HandleError(err)
				if _, isFatal := err.(buildcontroller.ImageChangeControllerFatalError); isFatal {
					return false
				}
				if count > 60 {
					return false
				}
				return true
			},
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			imageRepo := obj.(*imageapi.ImageStream)
			return imageChangeController.HandleImageRepo(imageRepo)
		},
	}
}

// pollPods lists pods for all builds in the buildStore which are pending or running and
// returns an enumerator for cache.Poller. The poll scope is narrowed for efficiency.
func (factory *BuildPodControllerFactory) pollPods() (cache.Enumerator, error) {
	list := &kapi.PodList{}

	for _, obj := range factory.buildStore.List() {
		build := obj.(*buildapi.Build)

		switch build.Status {
		case buildapi.BuildStatusPending, buildapi.BuildStatusRunning:
			podName := buildutil.GetBuildPodName(build)
			pod, err := factory.KubeClient.Pods(build.Namespace).Get(podName)
			if err != nil {
				glog.V(2).Infof("Couldn't find pod %s for build %s: %#v", podName, build.Name, err)
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

type typeBasedFactoryStrategy struct {
	DockerBuildStrategy *strategy.DockerBuildStrategy
	STIBuildStrategy    *strategy.STIBuildStrategy
	CustomBuildStrategy *strategy.CustomBuildStrategy
}

func (f *typeBasedFactoryStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	switch build.Parameters.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		return f.DockerBuildStrategy.CreateBuildPod(build)
	case buildapi.STIBuildStrategyType:
		return f.STIBuildStrategy.CreateBuildPod(build)
	case buildapi.CustomBuildStrategyType:
		return f.CustomBuildStrategy.CreateBuildPod(build)
	default:
		return nil, errors.New("No strategy defined for type")
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

// buildLW is a ListWatcher implementation for Builds.
type buildLW struct {
	client osclient.Interface
}

// List lists all Builds.
func (lw *buildLW) List() (runtime.Object, error) {
	return lw.client.Builds(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
}

// Watch watches all Builds.
func (lw *buildLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.Builds(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
}

// buildConfigLW is a ListWatcher implementation for BuildConfigs.
type buildConfigLW struct {
	client osclient.Interface
}

// List lists all BuildConfigs.
func (lw *buildConfigLW) List() (runtime.Object, error) {
	return lw.client.BuildConfigs(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
}

// Watch watches all BuildConfigs.
func (lw *buildConfigLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.BuildConfigs(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
}

// imageStreamLW is a ListWatcher for ImageStreams.
type imageStreamLW struct {
	client osclient.Interface
}

// List lists all ImageStreams.
func (lw *imageStreamLW) List() (runtime.Object, error) {
	return lw.client.ImageStreams(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
}

// Watch watches all ImageStreams.
func (lw *imageStreamLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.ImageStreams(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
}

// ControllerClient implements the common interfaces needed for build controllers
type ControllerClient struct {
	KubeClient kclient.Interface
	Client     osclient.Interface
}

// CreatePod creates a pod using the Kubernetes client.
func (c ControllerClient) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return c.KubeClient.Pods(namespace).Create(pod)
}

// DeletePod destroys a pod using the Kubernetes client.
func (c ControllerClient) DeletePod(namespace string, pod *kapi.Pod) error {
	return c.KubeClient.Pods(namespace).Delete(pod.Name)
}

// GetImageStream retrieves an image repository by namespace and name
func (c ControllerClient) GetImageStream(namespace, name string) (*imageapi.ImageStream, error) {
	return c.Client.ImageStreams(namespace).Get(name)
}
