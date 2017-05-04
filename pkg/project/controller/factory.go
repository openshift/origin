package controller

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/flowcontrol"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	controller "github.com/openshift/origin/pkg/controller"
)

type NamespaceControllerFactory struct {
	// KubeClient is a Kubernetes client.
	KubeClient kclientset.Interface
}

// Create creates a NamespaceController.
func (factory *NamespaceControllerFactory) Create() controller.RunnableController {
	namespaceLW := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return factory.KubeClient.Core().Namespaces().List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return factory.KubeClient.Core().Namespaces().Watch(options)
		},
	}
	queue := cache.NewResyncableFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(namespaceLW, &kapi.Namespace{}, queue, 1*time.Minute).Run()

	namespaceController := &NamespaceController{
		KubeClient: factory.KubeClient,
	}

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				utilruntime.HandleError(err)
				if retries.Count > 0 {
					return false
				}
				return true
			},
			flowcontrol.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			namespace := obj.(*kapi.Namespace)
			return namespaceController.Handle(namespace)
		},
	}
}
