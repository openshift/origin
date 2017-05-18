package controller

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/flowcontrol"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/controller"
	"github.com/openshift/origin/pkg/image/api"
)

// ImportControllerFactory can create an ImportController.
type ImportControllerFactory struct {
	Client               client.Interface
	ResyncInterval       time.Duration
	MinimumCheckInterval time.Duration
	ImportRateLimiter    flowcontrol.RateLimiter
	ScheduleEnabled      bool
}

// Create creates an ImportController.
func (f *ImportControllerFactory) Create() (controller.RunnableController, controller.StoppableController) {
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return f.Client.ImageStreams(metav1.NamespaceAll).List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return f.Client.ImageStreams(metav1.NamespaceAll).Watch(options)
		},
	}
	q := cache.NewResyncableFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(lw, &api.ImageStream{}, q, f.ResyncInterval).Run()

	// instantiate a scheduled importer using a number of buckets
	buckets := 4
	switch {
	case f.MinimumCheckInterval > time.Hour:
		buckets = 8
	case f.MinimumCheckInterval < 10*time.Minute:
		buckets = 2
	}
	seconds := f.MinimumCheckInterval / time.Second
	bucketQPS := 1.0 / float32(seconds) * float32(buckets)

	limiter := flowcontrol.NewTokenBucketRateLimiter(bucketQPS, 1)
	b := newScheduled(f.ScheduleEnabled, f.Client, buckets, limiter, f.ImportRateLimiter)

	// instantiate an importer for changes that happen to the image stream
	changed := &controller.RetryController{
		Queue: q,
		RetryManager: controller.NewQueueRetryManager(
			q,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				utilruntime.HandleError(err)
				return retries.Count < 5
			},
			flowcontrol.NewTokenBucketRateLimiter(1, 10),
		),
		Handle: b.Handle,
	}

	return changed, b.scheduler
}

type uniqueItem struct {
	uid             string
	resourceVersion string
}

// scheduled watches for changes to image streams and adds them to the list of streams to be
// periodically imported (later) or directly imported (now).
type scheduled struct {
	enabled     bool
	scheduler   *controller.Scheduler
	rateLimiter flowcontrol.RateLimiter
	controller  *ImportController
}

// newScheduled initializes a scheduled import object and sets its scheduler. Limiter is optional.
func newScheduled(enabled bool, client client.ImageStreamsNamespacer, buckets int, bucketLimiter, importLimiter flowcontrol.RateLimiter) *scheduled {
	b := &scheduled{
		enabled:     enabled,
		rateLimiter: importLimiter,
		controller: &ImportController{
			streams: client,
		},
	}
	b.scheduler = controller.NewScheduler(buckets, bucketLimiter, b.HandleTimed)
	return b
}

// Handle ensures an image stream is checked for scheduling and then runs a direct import
func (b *scheduled) Handle(obj interface{}) error {
	stream := obj.(*api.ImageStream)
	if b.enabled && needsScheduling(stream) {
		key, _ := cache.MetaNamespaceKeyFunc(stream)
		b.scheduler.Add(key, uniqueItem{uid: string(stream.UID), resourceVersion: stream.ResourceVersion})
	}
	return b.controller.Next(stream, b)
}

// HandleTimed is invoked when a key is ready to be processed.
func (b *scheduled) HandleTimed(key, value interface{}) {
	if !b.enabled {
		b.scheduler.Remove(key, value)
		return
	}
	if b.rateLimiter != nil && !b.rateLimiter.TryAccept() {
		return
	}
	namespace, name, _ := cache.SplitMetaNamespaceKey(key.(string))
	if err := b.controller.NextTimedByName(namespace, name); err != nil {
		// the stream cannot be imported
		if err == ErrNotImportable {
			// value must match to be removed, so we avoid races against creation by ensuring that we only
			// remove the stream if the uid and resource version in the scheduler are exactly the same.
			b.scheduler.Remove(key, value)
			return
		}
		utilruntime.HandleError(err)
		return
	}
}

// Importing is invoked when the controller decides to import a stream in order to push back
// the next schedule time.
func (b *scheduled) Importing(stream *api.ImageStream) {
	if !b.enabled {
		return
	}
	// Push the current key back to the end of the queue because it's just been imported
	key, _ := cache.MetaNamespaceKeyFunc(stream)
	b.scheduler.Delay(key)
}
