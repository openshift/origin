package factory

import (
	"time"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	kclientv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/flowcontrol"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientsetexternal "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildcontroller "github.com/openshift/origin/pkg/build/controller"
	osclient "github.com/openshift/origin/pkg/client"
	controller "github.com/openshift/origin/pkg/controller"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	// We must avoid creating processing imagestream changes until the build config store has synced.
	// If it hasn't synced, to avoid a hot loop, we'll wait this long between checks.
	storeSyncedPollPeriod = 100 * time.Millisecond
	maxRetries            = 60
)

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

type BuildConfigControllerFactory struct {
	Client                  osclient.Interface
	KubeClient              kclientset.Interface
	ExternalKubeClient      kclientsetexternal.Interface
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
	BuildConfigGetter       buildclient.BuildConfigGetter
	BuildLister             buildclient.BuildLister
	BuildDeleter            buildclient.BuildDeleter
	// Stop may be set to allow controllers created by this factory to be terminated.
	Stop <-chan struct{}
}

// Create creates a new ConfigChangeController which is used to trigger builds on creation
func (factory *BuildConfigControllerFactory) Create() controller.RunnableController {
	queue := cache.NewResyncableFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(newBuildConfigLW(factory.Client), &buildapi.BuildConfig{}, queue, 2*time.Minute).RunUntil(factory.Stop)

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: kv1core.New(factory.ExternalKubeClient.CoreV1().RESTClient()).Events("")})

	bcController := &buildcontroller.BuildConfigController{
		BuildConfigInstantiator: factory.BuildConfigInstantiator,
		BuildConfigGetter:       factory.BuildConfigGetter,
		BuildLister:             factory.BuildLister,
		BuildDeleter:            factory.BuildDeleter,
		Recorder:                eventBroadcaster.NewRecorder(kapi.Scheme, kclientv1.EventSource{Component: "build-config-controller"}),
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

func newBuildConfigLW(client osclient.Interface) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return client.BuildConfigs(metav1.NamespaceAll).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return client.BuildConfigs(metav1.NamespaceAll).Watch(options)
		},
	}
}

func newImageStreamLW(client osclient.Interface) cache.ListerWatcher {
	return &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return client.ImageStreams(metav1.NamespaceAll).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return client.ImageStreams(metav1.NamespaceAll).Watch(options)
		},
	}
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
	return c.KubeClient.Core().Pods(namespace).Get(name, metav1.GetOptions{})
}

// GetImageStream retrieves an image repository by namespace and name
func (c ControllerClient) GetImageStream(namespace, name string) (*imageapi.ImageStream, error) {
	return c.Client.ImageStreams(namespace).Get(name, metav1.GetOptions{})
}
