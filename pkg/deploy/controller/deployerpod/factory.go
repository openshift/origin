package deployerpod

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/watch"

	osclient "github.com/openshift/origin/pkg/client"
	controller "github.com/openshift/origin/pkg/controller"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeployerPodControllerFactory can create a DeployerPodController which
// handles processing deployer pods.
type DeployerPodControllerFactory struct {
	// Client is an OpenShift client.
	Client osclient.Interface
	// KubeClient is a Kubernetes client.
	KubeClient kclient.Interface
	// Codec is used for encoding/decoding.
	Codec runtime.Codec
}

// Create creates a DeployerPodController.
func (factory *DeployerPodControllerFactory) Create() controller.RunnableController {
	deploymentLW := &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).List(options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).Watch(options)
		},
	}
	deploymentStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentLW, &kapi.ReplicationController{}, deploymentStore, 2*time.Minute).Run()

	// TODO: These should be filtered somehow to include only the primary
	// deployer pod. For now, the controller is filtering.
	// TODO: Even with the label selector, this is inefficient on the backend
	// and we should work to consolidate namespace-spanning pod watches. For
	// example, the build infra is also watching pods across namespaces.
	podLW := &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			opts := kapi.ListOptions{LabelSelector: deployutil.AnyDeployerPodSelector()}
			return factory.KubeClient.Pods(kapi.NamespaceAll).List(opts)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			opts := kapi.ListOptions{LabelSelector: deployutil.AnyDeployerPodSelector(), ResourceVersion: options.ResourceVersion}
			return factory.KubeClient.Pods(kapi.NamespaceAll).Watch(opts)
		},
	}
	podQueue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(podLW, &kapi.Pod{}, podQueue, 2*time.Minute).Run()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(factory.KubeClient.Events(""))

	podController := &DeployerPodController{
		store:   deploymentStore,
		client:  factory.Client,
		kClient: factory.KubeClient,
		decodeConfig: func(deployment *kapi.ReplicationController) (*deployapi.DeploymentConfig, error) {
			return deployutil.DecodeDeploymentConfig(deployment, factory.Codec)
		},
		recorder: eventBroadcaster.NewRecorder(kapi.EventSource{Component: "deployerpod-controller"}),
	}

	return &controller.RetryController{
		Queue: podQueue,
		RetryManager: controller.NewQueueRetryManager(
			podQueue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				utilruntime.HandleError(err)
				// infinite retries for a transient error
				if _, isTransient := err.(transientError); isTransient {
					return true
				}
				// no retries for anything else
				if retries.Count > 0 {
					return false
				}
				return true
			},
			flowcontrol.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			pod := obj.(*kapi.Pod)
			return podController.Handle(pod)
		},
	}
}
