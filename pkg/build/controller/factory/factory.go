package factory

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/watch"

	builddefaults "github.com/openshift/origin/pkg/build/admission/defaults"
	buildoverrides "github.com/openshift/origin/pkg/build/admission/overrides"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontroller "github.com/openshift/origin/pkg/build/controller"
	"github.com/openshift/origin/pkg/build/controller/policy"
	strategy "github.com/openshift/origin/pkg/build/controller/strategy"
	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	controller "github.com/openshift/origin/pkg/controller"
	imageapi "github.com/openshift/origin/pkg/image/api"
	errors "github.com/openshift/origin/pkg/util/errors"
)

const (
	// We must avoid creating processing imagestream changes until the build config store has synced.
	// If it hasn't synced, to avoid a hot loop, we'll wait this long between checks.
	storeSyncedPollPeriod = 100 * time.Millisecond
	maxRetries            = 60
)

// limitedLogAndRetry stops retrying after maxTimeout, failing the build.
func limitedLogAndRetry(buildupdater buildclient.BuildUpdater, maxTimeout time.Duration) controller.RetryFunc {
	return func(obj interface{}, err error, retries controller.Retry) bool {
		isFatal := strategy.IsFatal(err)
		build := obj.(*buildapi.Build)
		if !isFatal && time.Since(retries.StartTimestamp.Time) < maxTimeout {
			glog.V(4).Infof("Retrying Build %s/%s with error: %v", build.Namespace, build.Name, err)
			return true
		}
		build.Status.Phase = buildapi.BuildPhaseFailed
		if !isFatal {
			build.Status.Reason = buildapi.StatusReasonExceededRetryTimeout
			build.Status.Message = buildapi.StatusMessageExceededRetryTimeout
		}
		build.Status.Message = errors.ErrorToSentence(err)
		now := unversioned.Now()
		build.Status.CompletionTimestamp = &now
		glog.V(3).Infof("Giving up retrying Build %s/%s: %v", build.Namespace, build.Name, err)
		utilruntime.HandleError(err)
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
	KubeClient          kclientset.Interface
	BuildUpdater        buildclient.BuildUpdater
	BuildLister         buildclient.BuildLister
	DockerBuildStrategy *strategy.DockerBuildStrategy
	SourceBuildStrategy *strategy.SourceBuildStrategy
	CustomBuildStrategy *strategy.CustomBuildStrategy
	BuildDefaults       builddefaults.BuildDefaults
	BuildOverrides      buildoverrides.BuildOverrides

	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

// Create constructs a BuildController
func (factory *BuildControllerFactory) Create() controller.RunnableController {
	queue := cache.NewResyncableFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&buildLW{client: factory.OSClient}, &buildapi.Build{}, queue, 2*time.Minute).RunUntil(factory.Stop)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kcoreclient.EventSinkImpl{Interface: factory.KubeClient.Core().Events("")})

	client := ControllerClient{factory.KubeClient, factory.OSClient}
	buildController := &buildcontroller.BuildController{
		BuildUpdater:      factory.BuildUpdater,
		BuildLister:       factory.BuildLister,
		ImageStreamClient: client,
		PodManager:        client,
		RunPolicies:       policy.GetAllRunPolicies(factory.BuildLister, factory.BuildUpdater),
		BuildStrategy: &typeBasedFactoryStrategy{
			DockerBuildStrategy: factory.DockerBuildStrategy,
			SourceBuildStrategy: factory.SourceBuildStrategy,
			CustomBuildStrategy: factory.CustomBuildStrategy,
		},
		Recorder:       eventBroadcaster.NewRecorder(kapi.EventSource{Component: "build-controller"}),
		BuildDefaults:  factory.BuildDefaults,
		BuildOverrides: factory.BuildOverrides,
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			limitedLogAndRetry(factory.BuildUpdater, 30*time.Minute),
			flowcontrol.NewTokenBucketRateLimiter(1, 10)),
		Handle: func(obj interface{}) error {
			build := obj.(*buildapi.Build)
			err := buildController.HandleBuild(build)
			if err != nil {
				// Update the build status message only if it changed.
				if msg := errors.ErrorToSentence(err); build.Status.Message != msg {
					// Set default Reason.
					if len(build.Status.Reason) == 0 {
						build.Status.Reason = buildapi.StatusReasonError
					}
					build.Status.Message = msg
					if err := buildController.BuildUpdater.Update(build.Namespace, build); err != nil {
						glog.V(2).Infof("Failed to update status message of Build %s/%s: %v", build.Namespace, build.Name, err)
					}
					buildController.Recorder.Eventf(build, kapi.EventTypeWarning, "HandleBuildError", "Build has error: %v", err)
				}
			}
			return err
		},
	}
}

// CreateDeleteController constructs a BuildDeleteController
func (factory *BuildControllerFactory) CreateDeleteController() controller.RunnableController {
	client := ControllerClient{factory.KubeClient, factory.OSClient}
	queue := cache.NewDeltaFIFO(cache.MetaNamespaceKeyFunc, nil, keyListerGetter{})
	cache.NewReflector(&buildDeleteLW{client, queue}, &buildapi.Build{}, queue, 5*time.Minute).RunUntil(factory.Stop)

	buildDeleteController := &buildcontroller.BuildDeleteController{
		PodManager: client,
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			queue.KeyOf,
			controller.RetryNever,
			flowcontrol.NewTokenBucketRateLimiter(1, 10)),
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

// retryFunc returns a function to retry a controller event
func retryFunc(kind string, isFatal func(err error) bool) controller.RetryFunc {
	return func(obj interface{}, err error, retries controller.Retry) bool {
		name, keyErr := cache.MetaNamespaceKeyFunc(obj)
		if keyErr != nil {
			name = "Unknown"
		}
		if isFatal != nil && isFatal(err) {
			glog.V(3).Infof("Will not retry fatal error for %s %s: %v", kind, name, err)
			utilruntime.HandleError(err)
			return false
		}
		if retries.Count > maxRetries {
			glog.V(3).Infof("Giving up retrying %s %s: %v", kind, name, err)
			utilruntime.HandleError(err)
			return false
		}
		glog.V(4).Infof("Retrying %s %s: %v", kind, name, err)
		return true
	}
}

// keyListerGetter is a dummy implementation of a KeyListerGetter
// which always returns a fake object and true for gets, and
// returns no items for list.  This forces the DeltaFIFO queue
// to always queue delete events it receives from etcd.  Our
// client will properly handle duplicated events and this is more
// efficient than maintaining a local cache of all the build pods
// so the DeltaFIFO can perform a proper diff.
type keyListerGetter struct {
	client osclient.Interface
}

// ListKeys is a dummy implementation of a KeyListerGetter interface returning
// empty string array; used only to force DeltaFIFO to always queue delete events.
func (keyListerGetter) ListKeys() []string {
	return []string{}
}

// GetByKey is a dummy implementation of a KeyListerGetter interface returning
// always "", true, nil; used only to force DeltaFIFO to always queue delete events.
func (keyListerGetter) GetByKey(key string) (interface{}, bool, error) {
	return "", true, nil
}

// ImageChangeControllerFactory can create an ImageChangeController which obtains ImageStreams
// from a queue populated from a watch of all ImageStreams.
type ImageChangeControllerFactory struct {
	Client                  osclient.Interface
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
	BuildConfigIndex        oscache.StoreToBuildConfigLister
	BuildConfigIndexSynced  func() bool
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

// Create creates a new ImageChangeController which is used to trigger builds when a new
// image is available
func (factory *ImageChangeControllerFactory) Create() controller.RunnableController {
	queue := cache.NewResyncableFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&imageStreamLW{factory.Client}, &imageapi.ImageStream{}, queue, 2*time.Minute).RunUntil(factory.Stop)

	imageChangeController := &buildcontroller.ImageChangeController{
		BuildConfigIndex:        factory.BuildConfigIndex,
		BuildConfigInstantiator: factory.BuildConfigInstantiator,
	}

	// Wait for the bc store to sync before starting any work in this controller.
	factory.waitForSyncedStores()

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			retryFunc("ImageStream update", nil),
			flowcontrol.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			imageRepo := obj.(*imageapi.ImageStream)
			return imageChangeController.HandleImageStream(imageRepo)
		},
	}
}

func (factory *ImageChangeControllerFactory) waitForSyncedStores() {
	for !factory.BuildConfigIndexSynced() {
		glog.V(4).Infof("Waiting for the bc caches to sync before starting the imagechange buildconfig controller worker")
		select {
		case <-time.After(storeSyncedPollPeriod):
		case <-factory.Stop:
			return
		}

	}
}

type BuildConfigControllerFactory struct {
	Client                  osclient.Interface
	KubeClient              kclientset.Interface
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

// Create creates a new ConfigChangeController which is used to trigger builds on creation
func (factory *BuildConfigControllerFactory) Create() controller.RunnableController {
	queue := cache.NewResyncableFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(&buildConfigLW{client: factory.Client}, &buildapi.BuildConfig{}, queue, 2*time.Minute).RunUntil(factory.Stop)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kcoreclient.EventSinkImpl{Interface: factory.KubeClient.Core().Events("")})

	bcController := &buildcontroller.BuildConfigController{
		BuildConfigInstantiator: factory.BuildConfigInstantiator,
		Recorder:                eventBroadcaster.NewRecorder(kapi.EventSource{Component: "build-config-controller"}),
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			retryFunc("BuildConfig", buildcontroller.IsFatal),
			flowcontrol.NewTokenBucketRateLimiter(1, 10)),
		Handle: func(obj interface{}) error {
			bc := obj.(*buildapi.BuildConfig)
			return bcController.HandleBuildConfig(bc)
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
	switch {
	case build.Spec.Strategy.DockerStrategy != nil:
		pod, err = f.DockerBuildStrategy.CreateBuildPod(build)
	case build.Spec.Strategy.SourceStrategy != nil:
		pod, err = f.SourceBuildStrategy.CreateBuildPod(build)
	case build.Spec.Strategy.CustomStrategy != nil:
		pod, err = f.CustomBuildStrategy.CreateBuildPod(build)
	case build.Spec.Strategy.JenkinsPipelineStrategy != nil:
		return nil, nil
	default:
		return nil, fmt.Errorf("no supported build strategy defined for Build %s/%s", build.Namespace, build.Name)
	}

	if pod != nil {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[buildapi.BuildAnnotation] = build.Name
	}
	return pod, err
}

// podLW is a ListWatcher implementation for Pods.
type podLW struct {
	client kclientset.Interface
}

// List lists all Pods that have a build label.
func (lw *podLW) List(options kapi.ListOptions) (runtime.Object, error) {
	return listPods(lw.client)
}

func listPods(client kclientset.Interface) (*kapi.PodList, error) {
	// get builds with new label
	sel, err := labels.Parse(buildapi.BuildLabel)
	if err != nil {
		return nil, err
	}
	listNew, err := client.Core().Pods(kapi.NamespaceAll).List(kapi.ListOptions{LabelSelector: sel})
	if err != nil {
		return nil, err
	}
	return listNew, nil
}

// Watch watches all Pods that have a build label.
func (lw *podLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	// FIXME: since we cannot have OR on label name we'll just get builds with new label
	sel, err := labels.Parse(buildapi.BuildLabel)
	if err != nil {
		return nil, err
	}
	opts := kapi.ListOptions{
		LabelSelector:   sel,
		ResourceVersion: options.ResourceVersion,
	}
	return lw.client.Core().Pods(kapi.NamespaceAll).Watch(opts)
}

// buildLW is a ListWatcher implementation for Builds.
type buildLW struct {
	client osclient.Interface
}

// List lists all Builds.
func (lw *buildLW) List(options kapi.ListOptions) (runtime.Object, error) {
	return lw.client.Builds(kapi.NamespaceAll).List(options)
}

// Watch watches all Builds.
func (lw *buildLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	return lw.client.Builds(kapi.NamespaceAll).Watch(options)
}

// buildDeleteLW is a ListWatcher implementation that watches for builds being deleted
type buildDeleteLW struct {
	ControllerClient
	store cache.Store
}

// List returns an empty list but adds delete events to the store for all Builds that have been deleted but still have pods.
func (lw *buildDeleteLW) List(options kapi.ListOptions) (runtime.Object, error) {
	glog.V(5).Info("Checking for deleted builds")
	podList, err := listPods(lw.KubeClient)
	if err != nil {
		glog.V(4).Infof("Failed to find any pods due to error %v", err)
		return nil, err
	}

	for _, pod := range podList.Items {
		buildName := buildapi.GetBuildName(&pod)
		if len(buildName) == 0 {
			continue
		}
		glog.V(5).Infof("Found build pod %s/%s", pod.Namespace, pod.Name)

		build, err := lw.Client.Builds(pod.Namespace).Get(buildName)
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
					Name:      buildName,
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
func (lw *buildDeleteLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	return lw.Client.Builds(kapi.NamespaceAll).Watch(options)
}

// buildConfigLW is a ListWatcher implementation for BuildConfigs.
type buildConfigLW struct {
	client osclient.Interface
}

// List lists all BuildConfigs.
func (lw *buildConfigLW) List(options kapi.ListOptions) (runtime.Object, error) {
	return lw.client.BuildConfigs(kapi.NamespaceAll).List(options)
}

// Watch watches all BuildConfigs.
func (lw *buildConfigLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	return lw.client.BuildConfigs(kapi.NamespaceAll).Watch(options)
}

// imageStreamLW is a ListWatcher for ImageStreams.
type imageStreamLW struct {
	client osclient.Interface
}

// List lists all ImageStreams.
func (lw *imageStreamLW) List(options kapi.ListOptions) (runtime.Object, error) {
	return lw.client.ImageStreams(kapi.NamespaceAll).List(options)
}

// Watch watches all ImageStreams.
func (lw *imageStreamLW) Watch(options kapi.ListOptions) (watch.Interface, error) {
	return lw.client.ImageStreams(kapi.NamespaceAll).Watch(options)
}

// ControllerClient implements the common interfaces needed for build controllers
type ControllerClient struct {
	KubeClient kclientset.Interface
	Client     osclient.Interface
}

// CreatePod creates a pod using the Kubernetes client.
func (c ControllerClient) CreatePod(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
	return c.KubeClient.Core().Pods(namespace).Create(pod)
}

// DeletePod destroys a pod using the Kubernetes client.
func (c ControllerClient) DeletePod(namespace string, pod *kapi.Pod) error {
	return c.KubeClient.Core().Pods(namespace).Delete(pod.Name, nil)
}

// GetPod gets a pod using the Kubernetes client.
func (c ControllerClient) GetPod(namespace, name string) (*kapi.Pod, error) {
	return c.KubeClient.Core().Pods(namespace).Get(name)
}

// GetImageStream retrieves an image repository by namespace and name
func (c ControllerClient) GetImageStream(namespace, name string) (*imageapi.ImageStream, error) {
	return c.Client.ImageStreams(namespace).Get(name)
}
