package generictrigger

import (
	"reflect"
	"time"

	"github.com/golang/glog"

	kcontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	// We must avoid creating processing deployment configs until the deployment config and image
	// stream stores have synced. If it hasn't synced, to avoid a hot loop, we'll wait this long
	// between checks.
	storeSyncedPollPeriod = 100 * time.Millisecond
	// MaxRetries is the number of times a deployment config will be retried before it is dropped
	// out of the queue.
	MaxRetries = 5
)

// NewDeploymentTriggerController returns a new DeploymentTriggerController.
func NewDeploymentTriggerController(dcInformer, rcInformer, streamInformer framework.SharedIndexInformer, oc osclient.Interface, codec runtime.Codec) *DeploymentTriggerController {
	c := &DeploymentTriggerController{
		dn: oc,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		codec: codec,
	}

	dcInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addDeploymentConfig,
		UpdateFunc: c.updateDeploymentConfig,
	})
	streamInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addImageStream,
		UpdateFunc: c.updateImageStream,
	})

	c.dcLister.Indexer = dcInformer.GetIndexer()
	c.rcLister.Indexer = rcInformer.GetIndexer()
	c.dcListerSynced = dcInformer.HasSynced
	c.rcListerSynced = rcInformer.HasSynced
	return c
}

// Run begins watching and syncing.
func (c *DeploymentTriggerController) Run(workers int, stopCh <-chan struct{}) {
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
	glog.Infof("Shutting down deployment trigger controller")
	c.queue.ShutDown()
}

func (c *DeploymentTriggerController) waitForSyncedStore(ready chan<- struct{}, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	for !c.dcListerSynced() || !c.rcListerSynced() {
		glog.V(4).Infof("Waiting for the dc and rc caches to sync before starting the trigger controller workers")
		select {
		case <-time.After(storeSyncedPollPeriod):
		case <-stopCh:
			return
		}
	}
	close(ready)
}

func (c *DeploymentTriggerController) addDeploymentConfig(obj interface{}) {
	dc := obj.(*deployapi.DeploymentConfig)

	// No need to enqueue deployment configs that have no triggers or are paused.
	if len(dc.Spec.Triggers) == 0 || dc.Spec.Paused {
		return
	}
	// We don't want to compete with the main deployment config controller. Let's process this
	// config once it's synced.
	if !deployutil.HasSynced(dc, dc.Generation) {
		return
	}

	c.enqueueDeploymentConfig(dc)
}

func (c *DeploymentTriggerController) updateDeploymentConfig(old, cur interface{}) {
	newDc := cur.(*deployapi.DeploymentConfig)
	oldDc := old.(*deployapi.DeploymentConfig)

	// A periodic relist will send update events for all known deployment configs.
	if newDc.ResourceVersion == oldDc.ResourceVersion {
		return
	}
	// No need to enqueue deployment configs that have no triggers or are paused.
	if len(newDc.Spec.Triggers) == 0 || newDc.Spec.Paused {
		return
	}
	// We don't want to compete with the main deployment config controller. Let's process this
	// config once it's synced. Note that this does not eliminate conflicts between the two
	// controllers because the main controller is constantly updating deployment configs as
	// owning replication controllers and pods are updated.
	if !deployutil.HasSynced(newDc, newDc.Generation) {
		return
	}
	// Enqueue the deployment config if it hasn't been deployed yet.
	if newDc.Status.LatestVersion == 0 {
		c.enqueueDeploymentConfig(newDc)
		return
	}
	// Compare deployment config templates before enqueueing. This reduces the amount of times
	// we will try to instantiate a deployment config at the expense of duplicating some of the
	// work that the instantiate endpoint is already doing but I think this is fine.
	shouldInstantiate := true
	latestRc, err := c.rcLister.ReplicationControllers(newDc.Namespace).Get(deployutil.LatestDeploymentNameForConfig(newDc))
	if err != nil {
		// If we get an error here it may be due to the rc cache lagging behind. In such a case
		// just defer to the api server (instantiate REST) where we will retry this.
		glog.V(2).Infof("Cannot get latest rc for dc %s:%d (%v) - will defer to instantiate", deployutil.LabelForDeploymentConfig(newDc), newDc.Status.LatestVersion, err)
	} else {
		initial, err := deployutil.DecodeDeploymentConfig(latestRc, c.codec)
		if err != nil {
			glog.V(2).Infof("Cannot decode dc from replication controller %s: %v", deployutil.LabelForDeployment(latestRc), err)
			return
		}
		shouldInstantiate = !reflect.DeepEqual(newDc.Spec.Template, initial.Spec.Template)
	}
	if !shouldInstantiate {
		return
	}

	c.enqueueDeploymentConfig(newDc)
}

// addImageStream enqueues the deployment configs that point to the new image stream.
func (c *DeploymentTriggerController) addImageStream(obj interface{}) {
	stream := obj.(*imageapi.ImageStream)
	glog.V(4).Infof("Image stream %q added.", stream.Name)
	dcList, err := c.dcLister.GetConfigsForImageStream(stream)
	if err != nil {
		return
	}
	// TODO: We could check image stream tags here and enqueue only deployment configs
	// with stale lastTriggeredImages.
	for _, dc := range dcList {
		c.enqueueDeploymentConfig(dc)
	}
}

// updateImageStream enqueues the deployment configs that point to the updated image stream.
func (c *DeploymentTriggerController) updateImageStream(old, cur interface{}) {
	// A periodic relist will send update events for all known streams.
	newStream := cur.(*imageapi.ImageStream)
	oldStream := old.(*imageapi.ImageStream)
	if newStream.ResourceVersion == oldStream.ResourceVersion {
		return
	}

	glog.V(4).Infof("Image stream %q updated.", newStream.Name)
	dcList, err := c.dcLister.GetConfigsForImageStream(newStream)
	if err != nil {
		return
	}
	// TODO: We could check image stream tags here and enqueue only deployment configs
	// with stale lastTriggeredImages.
	for _, dc := range dcList {
		c.enqueueDeploymentConfig(dc)
	}
}

func (c *DeploymentTriggerController) enqueueDeploymentConfig(dc *deployapi.DeploymentConfig) {
	key, err := kcontroller.KeyFunc(dc)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", dc, err)
		return
	}
	c.queue.Add(key)
}

func (c *DeploymentTriggerController) worker() {
	for {
		if quit := c.work(); quit {
			return
		}
	}
}

func (c *DeploymentTriggerController) work() bool {
	key, quit := c.queue.Get()
	if quit {
		return true
	}
	defer c.queue.Done(key)

	dc, err := c.getByKey(key.(string))
	if err != nil {
		glog.Error(err.Error())
	}

	if dc == nil {
		return false
	}

	err = c.Handle(dc)
	c.handleErr(err, key)

	return false
}

func (c *DeploymentTriggerController) getByKey(key string) (*deployapi.DeploymentConfig, error) {
	obj, exists, err := c.dcLister.Indexer.GetByKey(key)
	if err != nil {
		glog.Infof("Unable to retrieve deployment config %q from store: %v", key, err)
		c.queue.Add(key)
		return nil, err
	}
	if !exists {
		glog.Infof("Deployment config %q has been deleted", key)
		return nil, nil
	}

	return obj.(*deployapi.DeploymentConfig), nil
}
