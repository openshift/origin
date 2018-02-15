package controller

import (
	"time"

	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/flowcontrol"
	"k8s.io/client-go/util/workqueue"

	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion/image/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
)

// ImageStreamControllerOptions represents a configuration for the scheduled image stream
// import controller.
type ScheduledImageStreamControllerOptions struct {
	Resync time.Duration

	// Enabled indicates that the scheduled imports for images are allowed.
	Enabled bool

	// DefaultBucketSize is the default bucket size used by QPS.
	DefaultBucketSize int

	// MaxImageImportsPerMinute sets the maximum number of simultaneous image imports per
	// minute.
	MaxImageImportsPerMinute int
}

// Buckets returns the bucket size calculated based on the resync interval of the
// scheduled image import controller. For resync interval bigger than our the bucket size
// is doubled, for resync lower then 10 minutes bucket size is set to a half of the
// default size.
func (opts ScheduledImageStreamControllerOptions) Buckets() int {
	buckets := opts.DefaultBucketSize // 4
	switch {
	case opts.Resync > time.Hour:
		return buckets * 2
	case opts.Resync < 10*time.Minute:
		return buckets / 2
	}
	return buckets
}

// BucketsToQPS converts the bucket size to QPS
func (opts ScheduledImageStreamControllerOptions) BucketsToQPS() float32 {
	seconds := float32(opts.Resync / time.Second)
	return 1.0 / seconds * float32(opts.Buckets())
}

// GetRateLimiter returns a flowcontrol rate limiter based on the maximum number of
// imports (MaxImageImportsPerMinute) setting.
func (opts ScheduledImageStreamControllerOptions) GetRateLimiter() flowcontrol.RateLimiter {
	if opts.MaxImageImportsPerMinute <= 0 {
		return flowcontrol.NewFakeAlwaysRateLimiter()
	}

	importRate := float32(opts.MaxImageImportsPerMinute) / float32(time.Minute/time.Second)
	importBurst := opts.MaxImageImportsPerMinute * 2
	return flowcontrol.NewTokenBucketRateLimiter(importRate, importBurst)
}

// NewImageStreamController returns a new image stream import controller.
func NewImageStreamController(client imageclient.Interface, informer imageinformer.ImageStreamInformer) *ImageStreamController {
	controller := &ImageStreamController{
		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		client:       client.Image(),
		lister:       informer.Lister(),
		listerSynced: informer.Informer().HasSynced,
	}
	controller.syncHandler = controller.syncImageStream

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addImageStream,
		UpdateFunc: controller.updateImageStream,
	})

	return controller
}

// NewScheduledImageStreamController returns a new scheduled image stream import
// controller.
func NewScheduledImageStreamController(client imageclient.Interface, informer imageinformer.ImageStreamInformer, opts ScheduledImageStreamControllerOptions) *ScheduledImageStreamController {
	bucketLimiter := flowcontrol.NewTokenBucketRateLimiter(opts.BucketsToQPS(), 1)

	controller := &ScheduledImageStreamController{
		enabled:      opts.Enabled,
		rateLimiter:  opts.GetRateLimiter(),
		client:       client.Image(),
		lister:       informer.Lister(),
		listerSynced: informer.Informer().HasSynced,
	}

	controller.scheduler = newScheduler(opts.Buckets(), bucketLimiter, controller.syncTimed)

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    controller.addImageStream,
		UpdateFunc: controller.updateImageStream,
		DeleteFunc: controller.deleteImageStream,
	})

	return controller
}
