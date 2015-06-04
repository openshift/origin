package deployerpod

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
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
			func(obj interface{}, err error, retries controller.Retry) bool { return retries.Count < 1 },
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
func pollPods(deploymentStore cache.Store, kClient kclient.Interface) (cache.Enumerator, error) {
	list := &kapi.PodList{}

	for _, obj := range deploymentStore.List() {
		deployment := obj.(*kapi.ReplicationController)
		currentStatus := deployutil.DeploymentStatusFor(deployment)

		switch currentStatus {
		case deployapi.DeploymentStatusPending, deployapi.DeploymentStatusRunning:
			// Validate the correlating pod annotation
			podID := deployutil.DeployerPodNameFor(deployment)
			if len(podID) == 0 {
				glog.V(2).Infof("Unexpected state: deployment %s has no pod annotation; skipping pod polling", deployment.Name)
				continue
			}

			pod, err := kClient.Pods(deployment.Namespace).Get(podID)
			if err != nil {
				glog.V(2).Infof("Couldn't find pod %s for deployment %s: %#v", podID, deployment.Name, err)

				// if the deployer pod doesn't exist, update the deployment status to failed
				// TODO: This update should be moved the controller
				// once this poll is changed in favor of pod status updates.
				if kerrors.IsNotFound(err) {
					nextStatus := deployapi.DeploymentStatusFailed
					deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(nextStatus)
					deployment.Annotations[deployapi.DeploymentStatusReasonAnnotation] = deployapi.DeploymentFailedDeployerPodNoLongerExists

					if _, err := kClient.ReplicationControllers(deployment.Namespace).Update(deployment); err != nil {
						kutil.HandleError(fmt.Errorf("couldn't update deployment %s to status %s: %v", deployutil.LabelForDeployment(deployment), nextStatus, err))
					}
					glog.V(2).Infof("Updated deployment %s status from %s to %s", deployutil.LabelForDeployment(deployment), currentStatus, nextStatus)
				}
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
