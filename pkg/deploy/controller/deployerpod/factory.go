package deployerpod

import (
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

	controller "github.com/openshift/origin/pkg/controller"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeployerPodControllerFactory can create a DeployerPodController which gets
// pods from a queue populated from a watch of all pods filtered by a cache of
// deployments associated with pods.
type DeployerPodControllerFactory struct {
	// KubeClient is a Kubernetes client.
	KubeClient kclient.Interface
}

// Create creates a DeployerPodController.
func (factory *DeployerPodControllerFactory) Create() controller.RunnableController {
	deploymentLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).List(labels.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.KubeClient.ReplicationControllers(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	deploymentQueue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentLW, &kapi.ReplicationController{}, deploymentQueue, 2*time.Minute).Run()

	deploymentStore := cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentLW, &kapi.ReplicationController{}, deploymentStore, 2*time.Minute).Run()

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
	cache.NewPoller(pollFunc, 10*time.Second, podQueue).Run()

	podController := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Get(name)
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Update(deployment)
			},
		},
	}

	return &controller.RetryController{
		Queue: podQueue,
		RetryManager: controller.NewQueueRetryManager(
			podQueue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, count int) bool { return count < 1 },
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			pod := obj.(*kapi.Pod)
			return podController.Handle(pod)
		},
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
				glog.V(2).Infof("Unexpected state: deployment %s has no pod annotation; skipping pod polling", deployment.Name)
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
