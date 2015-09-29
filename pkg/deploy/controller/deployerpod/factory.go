package deployerpod

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	controller "github.com/openshift/origin/pkg/controller"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

// DeployerPodControllerFactory can create a DeployerPodController which
// handles processing deployer pods.
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

	// TODO: These should be filtered somehow to include only the primary
	// deployer pod. For now, the controller is filtering.
	// TODO: Even with the label selector, this is inefficient on the backend
	// and we should work to consolidate namespace-spanning pod watches. For
	// example, the build infra is also watching pods across namespaces.
	podLW := &deployutil.ListWatcherImpl{
		ListFunc: func() (runtime.Object, error) {
			return factory.KubeClient.Pods(kapi.NamespaceAll).List(deployutil.AnyDeployerPodSelector(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.KubeClient.Pods(kapi.NamespaceAll).Watch(deployutil.AnyDeployerPodSelector(), fields.Everything(), resourceVersion)
		},
	}
	podQueue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(podLW, &kapi.Pod{}, podQueue, 2*time.Minute).Run()

	podController := &DeployerPodController{
		deploymentClient: &deploymentClientImpl{
			getDeploymentFunc: func(namespace, name string) (*kapi.ReplicationController, error) {
				// Try to use the cache first. Trust hits and return them.
				example := &kapi.ReplicationController{ObjectMeta: kapi.ObjectMeta{Namespace: namespace, Name: name}}
				cached, exists, err := deploymentStore.Get(example)
				if err == nil && exists {
					return cached.(*kapi.ReplicationController), nil
				}
				// Double-check with the master for cache misses/errors, since those
				// are rare and API calls are expensive but more reliable.
				return factory.KubeClient.ReplicationControllers(namespace).Get(name)
			},
			updateDeploymentFunc: func(namespace string, deployment *kapi.ReplicationController) (*kapi.ReplicationController, error) {
				return factory.KubeClient.ReplicationControllers(namespace).Update(deployment)
			},
			listDeploymentsForConfigFunc: func(namespace, configName string) (*kapi.ReplicationControllerList, error) {
				return factory.KubeClient.ReplicationControllers(namespace).List(deployutil.ConfigSelector(configName))
			},
		},
		deployerPodsFor: func(namespace, name string) (*kapi.PodList, error) {
			return factory.KubeClient.Pods(namespace).List(deployutil.DeployerPodSelector(name), fields.Everything())
		},
		deletePod: func(namespace, name string) error {
			return factory.KubeClient.Pods(namespace).Delete(name, kapi.NewDeleteOptions(0))
		},
	}

	return &controller.RetryController{
		Queue: podQueue,
		RetryManager: controller.NewQueueRetryManager(
			podQueue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				kutil.HandleError(err)
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
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			pod := obj.(*kapi.Pod)
			return podController.Handle(pod)
		},
	}
}
