package trigger

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/controller"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion/image/internalversion"
	imageinternalversion "github.com/openshift/origin/pkg/image/generated/listers/image/internalversion"
	"github.com/openshift/origin/pkg/image/trigger"
)

const (
	// maxRetries is the number of times an image stream will be retried before it is dropped out of the queue.
	maxRetries          = 5
	maxResourceInterval = 30 * time.Second
)

// ErrUnresolvedTag is used to indicate a resource is not ready to be triggered
var ErrUnresolvedTag = fmt.Errorf("one or more triggers on this object cannot be resolved")

// TriggerSource defines the behavior for given resource type that can be triggered
// by image stream tag changes.
type TriggerSource struct {
	// Resource is a value guaranteed to be unique across all TriggerSources passed
	// to a given controller - used to separate keys processed by this controller.
	Resource schema.GroupResource
	// Informer is the source of changes for the resource type.
	Informer cache.SharedInformer
	// Store is the latest in memory cache for the resource type.
	Store cache.Store
	// TriggerFn must return a function that converts an object returned by the
	// informer into a trigger.CacheEntry and add prefix to the trigger.CacheEntry key.
	TriggerFn func(prefix string) trigger.Indexer
	// Reactor is invoked when an image stream tag change is detected on one or
	// more of the triggers defined on the object.
	Reactor trigger.ImageReactor
}

// tagRetriever implements trigger.TagRetriever over an image stream lister.
type tagRetriever struct {
	lister imageinternalversion.ImageStreamLister
}

var _ trigger.TagRetriever = tagRetriever{}

// NewTagRetriever will return a tag retriever that can look up image stream tag
// references from an image stream.
func NewTagRetriever(lister imageinternalversion.ImageStreamLister) trigger.TagRetriever {
	return tagRetriever{lister}
}

// ImageStreamTag returns a valid image reference for the provided image stream tag name and namespace,
// or returns false. rv is the resource version of the underlying image stream.
func (r tagRetriever) ImageStreamTag(namespace, name string) (ref string, rv int64, ok bool) {
	streamName, tag, ok := imageapi.SplitImageStreamTag(name)
	if !ok {
		return "", 0, false
	}
	is, err := r.lister.ImageStreams(namespace).Get(streamName)
	if err != nil {
		return "", 0, false
	}
	rv, err = strconv.ParseInt(is.ResourceVersion, 10, 64)
	if err != nil {
		return "", 0, false
	}
	ref, ok = imageapi.ResolveLatestTaggedImage(is, tag)
	return ref, rv, ok
}

// defaultResourceFailureDelay will retry failures forever, but implements an exponential
// capped backoff after a certain limit.
func defaultResourceFailureDelay(requeue int) (time.Duration, bool) {
	if requeue > 5 {
		return maxResourceInterval, true
	}
	t := time.Duration(math.Pow(2.0, float64(requeue)) * float64(time.Second))
	if t > maxResourceInterval {
		t = maxResourceInterval
	}
	return t, true
}

// TriggerController updates fields on resources to the docker image reference that
// an ImageStreamTag points to. Different resource types may have different mechanisms
// for describing triggers (DeploymentConfigs and BuildConfigs have fields, other
// resource types use annotations) and different mechanisms for updating the objects.
// The provided TriggerSources map objects into a set of triggers (trigger.CacheEntry)
// watches for either object changes or image stream tag changes, and then calls a
// type specific ImageReactor function with the latest cached copy of the object.
// Because image triggers are explicitly level driven, the controller avoids pre-
// calculating the state of images, preferring to let the image reaction itself
// retrieve the latest tag. The controller will try forever to apply image changes but
// a backoff will be used.
//
// Trigger indexers must handle logical failures themselves (for types that don't do
// validation up front), and ImageReactors are responsible for applying type specific
// business logic (DeploymentConfigs do not fire for the first time until all potential
// images have resolved at least once).
type TriggerController struct {
	eventRecorder record.EventRecorder

	triggerCache   cache.ThreadSafeStore
	triggerSources map[string]TriggerSource

	// To allow injection of syncs for testing.
	syncImageStreamFn func(key string) error
	// To allow injection of syncs for testing.
	syncResourceFn func(key string) error
	// used for unit testing
	enqueueImageStreamFn func(is *imageapi.ImageStream)
	// Allows injection for testing, controls requeues on image errors
	resourceFailureDelayFn func(requeue int) (time.Duration, bool)

	// lister can list/get image streams from the shared informer's store
	lister imageinternalversion.ImageStreamLister
	// tagRetriever helps get the latest value of a tag
	tagRetriever trigger.TagRetriever

	// queue is the list of image stream keys that must be synced.
	queue workqueue.RateLimitingInterface
	// imageChangeQueue tracks the pending changes to objects
	imageChangeQueue workqueue.RateLimitingInterface

	// syncs are the items that must return true before the queue can be processed
	syncs []cache.InformerSynced
}

func NewTriggerEventBroadcaster(client kv1core.CoreV1Interface) record.EventBroadcaster {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	// TODO: remove the wrapper when every client has moved to use the clientset.
	eventBroadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: client.Events("")})
	return eventBroadcaster
}

// NewTriggerController instantiates a trigger controller from the provided sources.
func NewTriggerController(eventBroadcaster record.EventBroadcaster, isInformer imageinformer.ImageStreamInformer, sources ...TriggerSource) *TriggerController {
	lister := isInformer.Lister()
	c := &TriggerController{
		eventRecorder:    eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "image-trigger-controller"}),
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "image-trigger"),
		imageChangeQueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "image-trigger-reactions"),
		lister:           lister,
		tagRetriever:     NewTagRetriever(lister),
		triggerCache:     NewTriggerCache(),

		resourceFailureDelayFn: defaultResourceFailureDelay,
	}

	c.syncImageStreamFn = c.syncImageStream
	c.syncResourceFn = c.syncResource
	c.enqueueImageStreamFn = c.enqueueImageStream

	isInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addImageStreamNotification,
		UpdateFunc: c.updateImageStreamNotification,
	})
	c.syncs = []cache.InformerSynced{isInformer.Informer().HasSynced}

	triggers, syncs, err := setupTriggerSources(c.triggerCache, c.tagRetriever, sources, c.imageChangeQueue)
	if err != nil {
		panic(err)
	}
	c.triggerSources = triggers
	c.syncs = append(c.syncs, syncs...)

	return c
}

// setupTriggerSources is used by test code to simulate a trigger controller.
func setupTriggerSources(triggerCache cache.ThreadSafeStore, tagRetriever trigger.TagRetriever, sources []TriggerSource, imageChangeQueue workqueue.RateLimitingInterface) (map[string]TriggerSource, []cache.InformerSynced, error) {
	var syncs []cache.InformerSynced
	triggerSources := make(map[string]TriggerSource)
	for _, source := range sources {
		if source.Store == nil {
			source.Store = source.Informer.GetStore()
		}
		prefix := source.Resource.String() + "/"
		if _, ok := triggerSources[source.Resource.String()]; ok {
			return nil, nil, fmt.Errorf("duplicate resource names registered in %#v", sources)
		}
		triggerSources[source.Resource.String()] = source

		handler := ProcessEvents(triggerCache, source.TriggerFn(prefix), imageChangeQueue, tagRetriever)
		source.Informer.AddEventHandler(handler)
		syncs = append(syncs, source.Informer.HasSynced)
	}
	return triggerSources, syncs, nil
}

// Run begins watching and syncing.
func (c *TriggerController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting trigger controller")

	if !cache.WaitForCacheSync(stopCh, c.syncs...) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.imageStreamWorker, time.Second, stopCh)
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.resourceWorker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down trigger controller")
}

func (c *TriggerController) addImageStreamNotification(obj interface{}) {
	is := obj.(*imageapi.ImageStream)
	c.enqueueImageStreamFn(is)
}

func (c *TriggerController) updateImageStreamNotification(old, cur interface{}) {
	c.enqueueImageStreamFn(cur.(*imageapi.ImageStream))
}

func (c *TriggerController) enqueueImageStream(is *imageapi.ImageStream) {
	key, err := controller.KeyFunc(is)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", is, err))
		return
	}
	c.queue.Add(key)
}

// imageStreamWorker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *TriggerController) imageStreamWorker() {
	for c.processNextImageStream() {
	}
	glog.V(4).Infof("Image stream worker stopped")
}

func (c *TriggerController) processNextImageStream() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncImageStreamFn(key.(string))
	c.handleImageStreamErr(err, key)

	return true
}

func (c *TriggerController) handleImageStreamErr(err error, key interface{}) {
	if err == nil {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < maxRetries {
		glog.V(4).Infof("Error syncing image stream %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	glog.V(4).Infof("Dropping image stream %q out of the queue: %v", key, err)
	c.queue.Forget(key)
}

// resourceWorker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *TriggerController) resourceWorker() {
	for c.processNextResource() {
	}
	glog.V(4).Infof("Resource worker stopped")
}

func (c *TriggerController) processNextResource() bool {
	key, quit := c.imageChangeQueue.Get()
	if quit {
		return false
	}
	defer c.imageChangeQueue.Done(key.(string))

	err := c.syncResourceFn(key.(string))
	c.handleResourceErr(err, key.(string))

	return true
}

func (c *TriggerController) handleResourceErr(err error, key string) {
	if err == nil {
		c.imageChangeQueue.Forget(key)
		return
	}

	if delay, ok := c.resourceFailureDelayFn(c.imageChangeQueue.NumRequeues(key)); ok {
		glog.V(4).Infof("Error syncing resource %s: %v", key, err)
		c.imageChangeQueue.AddAfter(key, delay)
		return
	}

	utilruntime.HandleError(err)
	glog.V(4).Infof("Dropping resource %q out of the queue: %v", key, err)
	c.imageChangeQueue.Forget(key)
}

// syncImageStream will sync the image stream with the given key.
// This function is not meant to be invoked concurrently with the same key.
func (c *TriggerController) syncImageStream(key string) error {
	if glog.V(4) {
		startTime := time.Now()
		glog.Infof("Started syncing image stream %q", key)
		defer func() {
			glog.Infof("Finished syncing image stream %q (%v)", key, time.Since(startTime))
		}()
	}

	// find the set of triggers to act on
	triggered, err := c.triggerCache.ByIndex("images", key)
	if err != nil {
		return err
	}
	if len(triggered) == 0 {
		return nil
	}

	// queue every trigger
	// TODO: possibly filter for impossible triggers
	for _, t := range triggered {
		entry := t.(*trigger.CacheEntry)
		c.imageChangeQueue.Add(entry.Key)
	}
	return nil
}

// syncResource handles a set of changes against one of the possible resources generically.
func (c *TriggerController) syncResource(key string) error {
	if glog.V(4) {
		startTime := time.Now()
		glog.Infof("Started syncing resource %q", key)
		defer func() {
			glog.Infof("Finished syncing resource %q (%v)", key, time.Since(startTime))
		}()
	}

	parts := strings.SplitN(key, "/", 2)
	source := c.triggerSources[parts[0]]
	obj, exists, err := source.Store.GetByKey(parts[1])
	if err != nil {
		return fmt.Errorf("unable to retrieve %s %s from store: %v", parts[0], parts[1], err)
	}
	if !exists {
		return nil
	}

	return source.Reactor.ImageChanged(obj.(runtime.Object), c.tagRetriever)
}
