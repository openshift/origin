package factory

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
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

const maxRetries = 60

// limitedLogAndRetry stops retrying after maxTimeout, failing the build.
func limitedLogAndRetry(buildupdater buildclient.BuildUpdater, maxTimeout time.Duration) controller.RetryFunc {
	return func(obj interface{}, err error, retries controller.Retry) bool {
		kutil.HandleError(err)
		if time.Since(retries.StartTimestamp.Time) < maxTimeout {
			return true
		}
		build := obj.(*buildapi.Build)
		build.Status = buildapi.BuildStatusFailed
		build.Message = err.Error()
		now := kutil.Now()
		build.CompletionTimestamp = &now
		glog.V(3).Infof("Giving up retrying Build %s/%s: %v", build.Namespace, build.Name, err)
		if err := buildupdater.Update(build.Namespace, build); err != nil {
			// retry update, but only on error other than NotFound
			return !kerrors.IsNotFound(err)
		}
		return false
	}
}

// BuildControllerFactory constructs BuildController objects
type BuildControllerFactory struct {
	OSClient            osclient.Interface
	KubeClient          kclient.Interface
	BuildUpdater        buildclient.BuildUpdater
	DockerBuildStrategy *strategy.DockerBuildStrategy
	SourceBuildStrategy *strategy.SourceBuildStrategy
	CustomBuildStrategy *strategy.CustomBuildStrategy
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

// Create constructs a BuildController
func (factory *BuildControllerFactory) Create() controller.RunnableController {
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&buildLW{client: factory.OSClient}, &buildapi.Build{}, queue, 2*time.Minute).Run()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(factory.KubeClient.Events(""))

	client := ControllerClient{factory.KubeClient, factory.OSClient}
	buildController := &buildcontroller.BuildController{
		BuildUpdater:      factory.BuildUpdater,
		ImageStreamClient: client,
		PodManager:        client,
		BuildStrategy: &typeBasedFactoryStrategy{
			DockerBuildStrategy: factory.DockerBuildStrategy,
			SourceBuildStrategy: factory.SourceBuildStrategy,
			CustomBuildStrategy: factory.CustomBuildStrategy,
		},
		Recorder: eventBroadcaster.NewRecorder(kapi.EventSource{Component: "build-controller"}),
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			limitedLogAndRetry(factory.BuildUpdater, 30*time.Minute),
			kutil.NewTokenBucketRateLimiter(1, 10)),
		Handle: func(obj interface{}) error {
			build := obj.(*buildapi.Build)
			return buildController.HandleBuild(build)
		},
	}
}

// CreateDeleteController constructs a BuildDeleteController
func (factory *BuildControllerFactory) CreateDeleteController() controller.RunnableController {
	client := ControllerClient{factory.KubeClient, factory.OSClient}
	queue := cache.NewDeltaFIFO(cache.MetaNamespaceKeyFunc, nil, nil)
	cache.NewReflector(&buildDeleteLW{client, queue}, &buildapi.Build{}, queue, 5*time.Minute).Run()

	buildDeleteController := &buildcontroller.BuildDeleteController{
		PodManager: client,
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			controller.RetryNever,
			kutil.NewTokenBucketRateLimiter(1, 10)),
		Handle: func(obj interface{}) error {
			deltas := obj.(cache.Deltas)
			for _, delta := range deltas {
				if delta.Type == cache.Deleted {
					return buildDeleteController.HandleBuildDeletion(delta.Object.(*buildapi.Build))
				}
			}
			return nil
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

	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&podLW{client: factory.KubeClient}, &kapi.Pod{}, queue, 2*time.Minute).Run()

	client := ControllerClient{factory.KubeClient, factory.OSClient}
	buildPodController := &buildcontroller.BuildPodController{
		BuildStore:   factory.buildStore,
		BuildUpdater: factory.BuildUpdater,
		PodManager:   client,
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				kutil.HandleError(err)
				return retries.Count < maxRetries
			},
			kutil.NewTokenBucketRateLimiter(1, 10)),
		Handle: func(obj interface{}) error {
			pod := obj.(*kapi.Pod)
			return buildPodController.HandlePod(pod)
		},
	}
}

// CreateDeleteController constructs a BuildPodDeleteController
func (factory *BuildPodControllerFactory) CreateDeleteController() controller.RunnableController {

	client := ControllerClient{factory.KubeClient, factory.OSClient}
	queue := cache.NewDeltaFIFO(cache.MetaNamespaceKeyFunc, nil, nil)
	cache.NewReflector(&buildPodDeleteLW{client, queue}, &kapi.Pod{}, queue, 5*time.Minute).Run()

	buildPodDeleteController := &buildcontroller.BuildPodDeleteController{
		BuildStore:   factory.buildStore,
		BuildUpdater: factory.BuildUpdater,
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			controller.RetryNever,
			kutil.NewTokenBucketRateLimiter(1, 10)),
		Handle: func(obj interface{}) error {
			deltas := obj.(cache.Deltas)
			for _, delta := range deltas {
				if delta.Type == cache.Deleted {
					return buildPodDeleteController.HandleBuildPodDeletion(delta.Object.(*kapi.Pod))
				}
			}
			return nil
		},
	}
}

// ImageChangeControllerFactory can create an ImageChangeController which obtains ImageStreams
// from a queue populated from a watch of all ImageStreams.
type ImageChangeControllerFactory struct {
	Client                  osclient.Interface
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
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
		BuildConfigInstantiator: factory.BuildConfigInstantiator,
		Stop: factory.Stop,
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				kutil.HandleError(err)
				if _, isFatal := err.(buildcontroller.ImageChangeControllerFatalError); isFatal {
					return false
				}
				return retries.Count < maxRetries
			},
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			imageRepo := obj.(*imageapi.ImageStream)
			return imageChangeController.HandleImageRepo(imageRepo)
		},
	}
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
	SourceBuildStrategy *strategy.SourceBuildStrategy
	CustomBuildStrategy *strategy.CustomBuildStrategy
}

func (f *typeBasedFactoryStrategy) CreateBuildPod(build *buildapi.Build) (*kapi.Pod, error) {
	var pod *kapi.Pod
	var err error
	switch build.Parameters.Strategy.Type {
	case buildapi.DockerBuildStrategyType:
		pod, err = f.DockerBuildStrategy.CreateBuildPod(build)
	case buildapi.SourceBuildStrategyType:
		pod, err = f.SourceBuildStrategy.CreateBuildPod(build)
	case buildapi.CustomBuildStrategyType:
		pod, err = f.CustomBuildStrategy.CreateBuildPod(build)
	default:
		return nil, fmt.Errorf("no supported build strategy defined for Build %s/%s with type %s", build.Namespace, build.Name, build.Parameters.Strategy.Type)
	}
	if pod != nil {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[buildapi.BuildAnnotation] = build.Name
	}
	return pod, err
}

// panicIfStopped panics with the provided object if the channel is closed
func panicIfStopped(ch <-chan struct{}, message interface{}) {
	select {
	case <-ch:
		panic(message)
	default:
	}
}

// podLW is a ListWatcher implementation for Pods.
type podLW struct {
	client kclient.Interface
}

// List lists all Pods.
func (lw *podLW) List() (runtime.Object, error) {
	return lw.client.Pods(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
}

// Watch watches all Pods.
func (lw *podLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.client.Pods(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
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

// buildDeleteLW is a ListWatcher implementation that watches for builds being deleted
type buildDeleteLW struct {
	ControllerClient
	store cache.Store
}

// List returns an empty list but adds delete events to the store for all Builds that have been deleted but still have pods.
func (lw *buildDeleteLW) List() (runtime.Object, error) {
	glog.V(5).Info("Checking for deleted builds")
	podList, err := lw.KubeClient.Pods(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		glog.V(4).Infof("Failed to find any pods due to error %v", err)
		return nil, err
	}
	for _, pod := range podList.Items {
		if len(pod.Labels[buildapi.BuildLabel]) == 0 {
			continue
		}
		glog.V(5).Infof("Found build pod %s/%s", pod.Namespace, pod.Name)

		build, err := lw.Client.Builds(pod.Namespace).Get(pod.Labels[buildapi.BuildLabel])
		if err != nil && !kerrors.IsNotFound(err) {
			glog.V(4).Infof("Error getting build for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			return nil, err
		}
		if err != nil && kerrors.IsNotFound(err) {
			build = nil
		}

		if build == nil {
			deletedBuild := &buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{
					Name:      pod.Labels[buildapi.BuildLabel],
					Namespace: pod.Namespace,
				},
			}
			glog.V(4).Infof("No build found for build pod %s/%s, deleting pod", pod.Namespace, pod.Name)
			err := lw.store.Delete(deletedBuild)
			if err != nil {
				glog.V(4).Infof("Error queuing delete event: %v", err)
			}
		} else {
			glog.V(5).Infof("Found build %s/%s for pod %s", build.Namespace, build.Name, pod.Name)
		}
	}
	return &buildapi.BuildList{}, nil
}

// Watch watches all Builds.
func (lw *buildDeleteLW) Watch(resourceVersion string) (watch.Interface, error) {
	//return lw.client.Client.Builds(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
	return lw.Client.Builds(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
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

// buildPodDeleteLW is a ListWatcher implementation that watches for Pods(that are associated with a Build) being deleted
type buildPodDeleteLW struct {
	ControllerClient
	store cache.Store
}

// List lists all Pods associated with a Build.
func (lw *buildPodDeleteLW) List() (runtime.Object, error) {
	glog.V(5).Info("Checking for deleted build pods")
	buildList, err := lw.Client.Builds(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
	if err != nil {
		glog.V(4).Infof("Failed to find any builds due to error %v", err)
		return nil, err
	}
	for _, build := range buildList.Items {
		glog.V(5).Infof("Found build %s/%s", build.Namespace, build.Name)
		if buildutil.IsBuildComplete(&build) {
			glog.V(5).Infof("Ignoring build %s/%s because it is complete", build.Namespace, build.Name)
			continue
		}
		pod, err := lw.KubeClient.Pods(build.Namespace).Get(buildutil.GetBuildPodName(&build))
		if err != nil && !kerrors.IsNotFound(err) {
			glog.V(4).Infof("Error getting pod for build %s/%s: %v", build.Namespace, build.Name, err)
			return nil, err
		}
		if (err != nil && kerrors.IsNotFound(err)) || pod.Labels[buildapi.BuildLabel] != build.Name {
			pod = nil
		}
		if pod == nil {
			deletedPod := &kapi.Pod{
				ObjectMeta: kapi.ObjectMeta{
					Name:      buildutil.GetBuildPodName(&build),
					Namespace: build.Namespace,
				},
			}
			glog.V(4).Infof("No build pod found for build %s/%s, sending delete event for build pod", build.Namespace, build.Name)
			err := lw.store.Delete(deletedPod)
			if err != nil {
				glog.V(4).Infof("Error queuing delete event: %v", err)
			}
		} else {
			glog.V(5).Infof("Found build pod %s/%s for build %s", pod.Namespace, pod.Name, build.Name)
		}
	}
	return &kapi.PodList{}, nil
}

// Watch watches all Pods for deletion
func (lw *buildPodDeleteLW) Watch(resourceVersion string) (watch.Interface, error) {
	return lw.KubeClient.Pods(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
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
	return c.KubeClient.Pods(namespace).Delete(pod.Name, nil)
}

// GetPod gets a pod using the Kubernetes client.
func (c ControllerClient) GetPod(namespace, name string) (*kapi.Pod, error) {
	return c.KubeClient.Pods(namespace).Get(name)
}

// GetImageStream retrieves an image repository by namespace and name
func (c ControllerClient) GetImageStream(namespace, name string) (*imageapi.ImageStream, error) {
	return c.Client.ImageStreams(namespace).Get(name)
}
