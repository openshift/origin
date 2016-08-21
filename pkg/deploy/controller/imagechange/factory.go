package imagechange

import (
	"time"

	"github.com/golang/glog"

	kcontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	// We must avoid creating processing image stream until the deployment config and image
	// stream stores have synced.
	StoreSyncedPollPeriod = 100 * time.Millisecond
	// MaxRetries is the number of times an image stream will be retried before it is dropped
	// out of the queue.
	MaxRetries = 5
)

// NewImageChangeController returns a new ImageChangeController.
func NewImageChangeController(dcInformer, streamInformer framework.SharedIndexInformer, oc osclient.Interface) *ImageChangeController {
	c := &ImageChangeController{
		dn: oc,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	c.streamLister.Indexer = streamInformer.GetIndexer()
	streamInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addImageStream,
		UpdateFunc: c.updateImageStream,
	})
	c.streamStoreSynced = streamInformer.HasSynced

	c.dcLister.Indexer = dcInformer.GetIndexer()
	dcInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addDeploymentConfig,
		UpdateFunc: c.updateDeploymentConfig,
	})
	c.dcStoreSynced = dcInformer.HasSynced

	return c
}

// Run begins watching and syncing.
func (c *ImageChangeController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the dc store to sync before starting any work in this controller.
	ready := make(chan struct{})
	go c.waitForSyncedStore(ready, stopCh)
	select {
	case <-ready:
	case <-stopCh:
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.Infof("Shutting down image change controller")
	c.queue.ShutDown()
}

func (c *ImageChangeController) waitForSyncedStore(ready chan<- struct{}, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	for !c.streamStoreSynced() || !c.dcStoreSynced() {
		glog.V(4).Infof("Waiting for the image stream and deployment config caches to sync before starting the image change controller workers")
		select {
		case <-time.After(StoreSyncedPollPeriod):
		case <-stopCh:
			return
		}
	}
	close(ready)
}

func (c *ImageChangeController) addImageStream(obj interface{}) {
	stream := obj.(*imageapi.ImageStream)
	c.enqueueImageStream(stream)
}

func (c *ImageChangeController) updateImageStream(old, cur interface{}) {
	// A periodic relist will send update events for all known streams.
	newStream := cur.(*imageapi.ImageStream)
	oldStream := old.(*imageapi.ImageStream)
	if newStream.ResourceVersion == oldStream.ResourceVersion {
		return
	}

	c.enqueueImageStream(newStream)
}

// addDeploymentConfig is used for making sure that new deployment configs with triggers pointing
// to existing images will be deployed as soon as they are created.
func (c *ImageChangeController) addDeploymentConfig(obj interface{}) {
	dc := obj.(*deployapi.DeploymentConfig)
	for _, stream := range c.streamLister.GetStreamsForConfig(dc) {
		glog.V(4).Infof("Reconciling stream %q for config %q\n", imageapi.LabelForStream(stream), deployutil.LabelForDeploymentConfig(dc))
		c.enqueueImageStream(stream)
	}
}

// updateDeploymentConfig is used for making sure that deployment configs with new triggers
// pointing to existing images will be deployed as soon as they are updated.
func (c *ImageChangeController) updateDeploymentConfig(old, cur interface{}) {
	// A periodic relist will send update events for all known configs.
	newDc := cur.(*deployapi.DeploymentConfig)
	oldDc := old.(*deployapi.DeploymentConfig)
	if newDc.ResourceVersion == oldDc.ResourceVersion {
		return
	}

	for _, stream := range c.streamLister.GetStreamsForConfig(newDc) {
		glog.V(4).Infof("Reconciling stream %q for config %q\n", imageapi.LabelForStream(stream), deployutil.LabelForDeploymentConfig(newDc))
		c.enqueueImageStream(stream)
	}
}

func (c *ImageChangeController) enqueueImageStream(stream *imageapi.ImageStream) {
	key, err := kcontroller.KeyFunc(stream)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", stream, err)
		return
	}
	c.queue.Add(key)
}

func (c *ImageChangeController) worker() {
	for {
		if quit := c.work(); quit {
			return
		}
	}
}

func (c *ImageChangeController) work() bool {
	key, quit := c.queue.Get()
	if quit {
		return true
	}
	defer c.queue.Done(key)

	stream, err := c.getByKey(key.(string))
	if err != nil {
		glog.Error(err.Error())
	}

	if stream == nil {
		return false
	}

	if err := c.Handle(stream); err != nil {
		utilruntime.HandleError(err)
	}

	return false
}

func (c *ImageChangeController) getByKey(key string) (*imageapi.ImageStream, error) {
	obj, exists, err := c.streamLister.Indexer.GetByKey(key)
	if err != nil {
		glog.Infof("Unable to retrieve image stream %q from store: %v", key, err)
		c.queue.Add(key)
		return nil, err
	}
	if !exists {
		glog.Infof("Image stream %q has been deleted", key)
		return nil, nil
	}

	return obj.(*imageapi.ImageStream), nil
}
