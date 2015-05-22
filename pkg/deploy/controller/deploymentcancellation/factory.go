package deploymentcancellation

import (
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/record"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	controller "github.com/openshift/origin/pkg/controller"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeploymentCancellationControllerFactory can create a DeploymentCancellationController
// that cancels deployments that have the cancellation annotation set.
type DeploymentCancellationControllerFactory struct {
	// KubeClient is a Kubernetes client.
	KubeClient kclient.Interface
	// Codec is used for encoding/decoding.
	Codec runtime.Codec
}

// Create creates a DeploymentCancellationController.
func (factory *DeploymentCancellationControllerFactory) Create() controller.RunnableController {
	deploymentLW := &deployutil.ListWatcherImpl{
		// TODO: Investigate specifying annotation field selectors to fetch only 'deployments'
		// Currently field selectors are not supported for replication controllers
		ListFunc: func() (runtime.Object, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	deploymentQueue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentLW, &kapi.ReplicationController{}, deploymentQueue, 2*time.Minute).Run()

	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(factory.KubeClient.Events(""))

	deployController := &DeploymentCancellationController{
		podClient: &podClientImpl{
			getPodFunc: func(namespace, name string) (*kapi.Pod, error) {
				return factory.KubeClient.Pods(namespace).Get(name)
			},
			updatePodFunc: func(namespace string, pod *kapi.Pod) (*kapi.Pod, error) {
				return factory.KubeClient.Pods(namespace).Update(pod)
			},
		},
		recorder: eventBroadcaster.NewRecorder(kapi.EventSource{Component: "deployer"}),
	}

	return &controller.RetryController{
		Queue: deploymentQueue,
		RetryManager: controller.NewQueueRetryManager(
			deploymentQueue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, count int) bool { return count < 1 },
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			deployment := obj.(*kapi.ReplicationController)
			return deployController.Handle(deployment)
		},
	}
}
