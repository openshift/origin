package controller

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/controller"
	"github.com/openshift/origin/pkg/image/api"
)

// ImageStreamControllerFactory can create an ImageStreamController.
type ImageStreamControllerFactory struct {
	Client client.Interface
}

// Create creates an ImageStreamController.
func (f *ImageStreamControllerFactory) Create() controller.RunnableController {
	lw := &cache.ListWatch{
		ListFunc: func() (runtime.Object, error) {
			return f.Client.ImageStreams(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
		},
		WatchFunc: func(resourceVersion string) (watch.Interface, error) {
			return f.Client.ImageStreams(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
		},
	}
	q := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(lw, &api.ImageStream{}, q, 2*time.Minute).Run()

	c := &ImageStreamController{
		streams:         f.Client,
		streamDeletions: f.Client,
		streamImages:    f.Client,
		images:          f.Client,
		mappings:        f.Client,
	}

	return &controller.RetryController{
		Queue: q,
		RetryManager: controller.NewQueueRetryManager(
			q,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				util.HandleError(err)
				return retries.Count < 5
			},
			kutil.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: func(obj interface{}) error {
			r := obj.(*api.ImageStream)
			return c.Next(r)
		},
	}
}
