package controller

import (
	"fmt"
	"strings"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/golang/glog"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imageclient "github.com/openshift/origin/pkg/image/clientset/internalclientset"
)

const (
	MaxRetriesBeforeResync = 5

	ImageStreamFromIndexName = "image.openshift.io/from"
)

type UpdateTrackingTagsControllerOptions struct {
	// Resync is the time.Duration at which to fully re-list images.
	// If zero, re-list will be delayed as long as possible
	Resync time.Duration
}

type UpdateTrackingTagsController struct {
	client imageclient.Interface

	isCache      cache.Store
	isController *cache.Controller
	isIndex      cache.Indexer

	queue workqueue.RateLimitingInterface

	syncHandler func(isTagKey string) error
}

func NewUpdateTrackingTagsController(cl imageclient.Interface, options UpdateTrackingTagsControllerOptions) *UpdateTrackingTagsController {
	c := &UpdateTrackingTagsController{
		client: cl,
		queue:  workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	c.isIndex, c.isController = cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return c.client.Image().ImageStreams(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return c.client.Image().ImageStreams(kapi.NamespaceAll).Watch(options)
			},
		},
		&imageapi.ImageStream{},
		options.Resync,
		cache.ResourceEventHandlerFuncs{
			UpdateFunc: func(_, cur interface{}) {
				is := cur.(*imageapi.ImageStream)
				glog.V(4).Infof("Updating ImageStream %s/%s", is.Namespace, is.Name)
				c.enqueueImageStream(cur)
			},
			DeleteFunc: func(obj interface{}) {
				// TODO: Handle delete
				// c.enqueueImageStream(obj)
			},
		},
		cache.Indexers{
			ImageStreamFromIndexName: imageStreamFromIndex,
			cache.NamespaceIndex:     cache.MetaNamespaceIndexFunc,
		},
	)

	c.syncHandler = c.syncImageStream

	return c
}

func (c *UpdateTrackingTagsController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	glog.Infof("Starting update tracking tags controller")
	go c.isController.Run(stopCh)
	for !c.isController.HasSynced() {
		time.Sleep(100 * time.Millisecond)
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down update tracking tags controller")
	c.queue.ShutDown()
}

func (c *UpdateTrackingTagsController) worker() {
	for {
		if !c.work() {
			return
		}
	}
}

func (c *UpdateTrackingTagsController) work() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	if err := c.syncHandler(key.(string)); err == nil {
		// this means the request was successfully handled.  We should "forget" the item so that any retry
		// later on is reset
		c.queue.Forget(key)
	} else {
		// if we had an error it means that we didn't handle it, which means that we want to requeue the work
		if c.queue.NumRequeues(key) > MaxRetriesBeforeResync {
			utilruntime.HandleError(fmt.Errorf("error syncing image stream tag, it will be tried again on a resync %v: %v", key, err))
			c.queue.Forget(key)
		} else {
			glog.V(4).Infof("error syncing image stream tag, it will be retried %v: %v", key, err)
			c.queue.AddRateLimited(key)
		}
	}

	return true
}

func (c *UpdateTrackingTagsController) enqueueImageStream(obj interface{}) {
	if _, ok := obj.(*imageapi.ImageStream); !ok {
		return
	}
	key, err := controller.KeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}

	c.queue.Add(key)
}

// syncImageStream does the work
func (c *UpdateTrackingTagsController) syncImageStream(key string) error {
	obj, exists, err := c.isIndex.GetByKey(key)
	if err != nil {
		glog.V(4).Infof("Unable to retrieve image stream tag %v from store: %v", key, err)
		return err
	}
	if !exists {
		glog.V(4).Infof("ImageStream %v has been deleted", key)
		return nil
	}
	is, ok := obj.(*imageapi.ImageStream)
	if !ok {
		glog.V(4).Infof("Expected ImageStream, received %#v", obj)
		return nil
	}

	for _, tag := range is.Spec.Tags {
		// If the "source" image stream was updated, enqueue all image stream that reference
		// the update stream for tags change.
		if tag.From == nil || tag.From.Kind == "DockerImage" {
			objects, err := c.isIndex.ByIndex(ImageStreamFromIndexName, imageStreamTagName(is, tag.Name))
			if err != nil {
				glog.V(4).Infof("Error fetching image streams referencing %s/%s:%s: %v (skipping)", is.Namespace, is.Name, tag.Name, err)
				continue
			}
			// Enqueue all referenced image streams for update
			for _, obj := range objects {
				c.enqueueImageStream(obj)
			}
			continue
		}

		// Handle the image stream tags referencing another image stream.
		if tag.From.Kind == "ImageStreamTag" {
			namespace := tag.From.Namespace
			if len(namespace) == 0 {
				namespace = is.Namespace
			}
			if strings.Contains(tag.From.Name, ":") {
			}
			// FIXME: This is weird, for local tags the name of the image stream is not set,
			// but for cross-image stream tags it is. We should probably unify this.
			name, tagName, ok := imageapi.SplitImageStreamTag(tag.From.Name)
			if !ok {
				name = is.Name
				tagName = tag.From.Name
			}
			sourceStreamObj, exists, err := c.isIndex.GetByKey(namespace + "/" + name)
			if err != nil {
				glog.V(4).Infof("Error fetching source image stream %s/%s: %v", namespace, name, err)
				continue
			}
			if !exists {
				// FIXME: Should enqueue for deletion
				glog.V(4).Infof("Source image stream %s/%s was deleted", namespace, name)
				continue
			}
			sourceStream, ok := sourceStreamObj.(*imageapi.ImageStream)
			if !ok {
				glog.V(4).Infof("Error getting source image stream from %#v", sourceStreamObj)
				continue
			}
			latestSource := imageapi.LatestTaggedImage(sourceStream, tagName)
			if imageapi.DifferentTagEvent(is, tag.Name, *latestSource) {
				isCopyObj, err := kapi.Scheme.DeepCopy(is)
				if err != nil {
					glog.V(4).Infof("Unable to copy image stream %s/%s: %v", is.Namespace, is.Name, err)
					continue
				}
				isCopy := isCopyObj.(*imageapi.ImageStream)
				if imageapi.AddTagEventToImageStream(isCopy, tag.Name, *latestSource) {
					glog.V(4).Infof("Synchronized image stream %s/%s:%s -> %s/%s:%s (new %q is %s)", isCopy.Namespace, isCopy.Name, tag.Name, sourceStream.Namespace, sourceStream.Name, tagName, tagName, latestSource.Image)
					// FIXME: This is broken because the client set does not have UpdateStatus(), it
					// will be added in 1.6 rebase...
					if _, err := c.client.Image().ImageStreams(isCopy.Namespace).Update(isCopy); err != nil {
						glog.V(4).Infof("Failed to update image stream %s/%s: %v (will retry)", isCopy.Namespac, isCopy.Name, err)
						return err
					}
				}
			}
		}
	}
	return nil
}

func imageStreamTagName(is *imageapi.ImageStream, tagName string) string {
	return fmt.Sprintf("%s/%s:%s", is.Namespace, is.Name, tagName)
}

// imageStreamFromIndex indexes all image streams tags that are aliases to another image
// stream tags.
func imageStreamFromIndex(obj interface{}) ([]string, error) {
	is := obj.(*imageapi.ImageStream)
	for _, tag := range is.Spec.Tags {
		if from := tag.From; from != nil && from.Kind == "ImageStreamTag" {
			namespace := from.Namespace
			if len(namespace) == 0 {
				namespace = is.Namespace
			}
			return []string{namespace + "/" + from.Name}, nil
		}
	}
	return []string{}, nil
}
