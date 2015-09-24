package controller

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

	osclient "github.com/openshift/origin/pkg/client"
	controller "github.com/openshift/origin/pkg/controller"
)

type NamespaceControllerFactory struct {
	// Client is an OpenShift client.
	Client osclient.Interface
	// KubeClient is a Kubernetes client.
	KubeClient kclient.Interface
}

// Create creates a NamespaceController.
func (factory *NamespaceControllerFactory) Create() controller.RunnableController {
	namespaceLW := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return factory.KubeClient.Namespaces().List(labels.Everything(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return factory.KubeClient.Namespaces().Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(namespaceLW, &kapi.Namespace{}, queue, 1*time.Minute).Run()

	namespaceController := &NamespaceController{
		Client:     factory.Client,
		KubeClient: factory.KubeClient,
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				kutil.HandleError(err)
				if _, isFatal := err.(fatalError); isFatal {
					return false
				}
				if retries.Count > 0 {
					return false
				}
				return true
			},
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			namespace := obj.(*kapi.Namespace)
			return namespaceController.Handle(namespace)
		},
	}
}
