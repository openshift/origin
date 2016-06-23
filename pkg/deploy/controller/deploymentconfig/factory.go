package deploymentconfig

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"

	osclient "github.com/openshift/origin/pkg/client"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

const (
	// We must avoid creating new replication controllers until the {replication controller,pods} store
	// has synced. If it hasn't synced, to avoid a hot loop, we'll wait this long between checks.
	StoreSyncedPollPeriod = 100 * time.Millisecond
)

// NewDeploymentConfigController creates a new DeploymentConfigController.
func NewDeploymentConfigController(dcInformer, rcInformer, podInformer framework.SharedIndexInformer, oc osclient.Interface, kc kclient.Interface, codec runtime.Codec) *DeploymentConfigController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(kc.Events(""))
	recorder := eventBroadcaster.NewRecorder(kapi.EventSource{Component: "deploymentconfig-controller"})

	c := &DeploymentConfigController{
		dn: oc,
		rn: kc,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		recorder: recorder,
		codec:    codec,
	}

	c.dcStore.Indexer = dcInformer.GetIndexer()
	dcInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addDeploymentConfig,
		UpdateFunc: c.updateDeploymentConfig,
		DeleteFunc: c.deleteDeploymentConfig,
	})
	c.rcStore.Indexer = rcInformer.GetIndexer()
	rcInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addReplicationController,
		UpdateFunc: c.updateReplicationController,
		DeleteFunc: c.deleteReplicationController,
	})
	c.podStore.Indexer = podInformer.GetIndexer()

	c.dcStoreSynced = dcInformer.HasSynced
	c.rcStoreSynced = rcInformer.HasSynced
	c.podStoreSynced = podInformer.HasSynced

	return c
}

// Run begins watching and syncing.
func (c *DeploymentConfigController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the rc and dc stores to sync before starting any work in this controller.
	ready := make(chan struct{})
	go c.waitForSyncedStores(ready)
	select {
	case <-ready:
	case <-stopCh:
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down deploymentconfig controller")
	c.queue.ShutDown()
}

func (c *DeploymentConfigController) waitForSyncedStores(ready chan struct{}) {
	defer utilruntime.HandleCrash()

	for !c.dcStoreSynced() || !c.rcStoreSynced() || !c.podStoreSynced() {
		glog.V(4).Infof("Waiting for the dc, rc, and pod caches to sync before starting the deployment config controller workers")
		time.Sleep(StoreSyncedPollPeriod)
	}
	close(ready)
}

func (c *DeploymentConfigController) addDeploymentConfig(obj interface{}) {
	dc := obj.(*deployapi.DeploymentConfig)
	glog.V(4).Infof("Adding deployment config %q", dc.Name)
	c.enqueueDeploymentConfig(dc)
}

func (c *DeploymentConfigController) updateDeploymentConfig(old, cur interface{}) {
	oldDc := old.(*deployapi.DeploymentConfig)
	glog.V(4).Infof("Updating deployment config %q", oldDc.Name)
	c.enqueueDeploymentConfig(cur.(*deployapi.DeploymentConfig))
}

func (c *DeploymentConfigController) deleteDeploymentConfig(obj interface{}) {
	dc, ok := obj.(*deployapi.DeploymentConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Errorf("Couldn't get object from tombstone %+v", obj)
			return
		}
		dc, ok = tombstone.Obj.(*deployapi.DeploymentConfig)
		if !ok {
			glog.Errorf("Tombstone contained object that is not a deployment config: %+v", obj)
			return
		}
	}
	glog.V(4).Infof("Deleting deployment config %q", dc.Name)
	c.enqueueDeploymentConfig(dc)
}

// addReplicationController enqueues the deployment that manages a replicationcontroller when the replicationcontroller is created.
func (c *DeploymentConfigController) addReplicationController(obj interface{}) {
	rc := obj.(*kapi.ReplicationController)
	glog.V(4).Infof("Replication controller %q added.", rc.Name)
	// We are waiting for the deployment config store to sync but still there are pathological
	// cases of highly latent watches.
	if dc, err := c.dcStore.GetConfigForController(rc); err == nil && dc != nil {
		c.enqueueDeploymentConfig(dc)
	}
}

// updateReplicationController figures out which deploymentconfig is managing this replication
// controller and requeues the deployment config.
func (c *DeploymentConfigController) updateReplicationController(old, cur interface{}) {
	// A periodic relist will send update events for all known controllers.
	if kapi.Semantic.DeepEqual(old, cur) {
		return
	}

	curRC := cur.(*kapi.ReplicationController)
	glog.V(4).Infof("Replication controller %q updated.", curRC.Name)
	if dc, err := c.dcStore.GetConfigForController(curRC); err == nil && dc != nil {
		c.enqueueDeploymentConfig(dc)
	}
}

// deleteReplicationController enqueues the deployment that manages a replicationcontroller when
// the replicationcontroller is deleted. obj could be an *kapi.ReplicationController, or
// a DeletionFinalStateUnknown marker item.
func (c *DeploymentConfigController) deleteReplicationController(obj interface{}) {
	rc, ok := obj.(*kapi.ReplicationController)

	// When a delete is dropped, the relist will notice a pod in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Errorf("Couldn't get object from tombstone %#v", obj)
			return
		}
		rc, ok = tombstone.Obj.(*kapi.ReplicationController)
		if !ok {
			glog.Errorf("Tombstone contained object that is not a replication controller %#v", obj)
			return
		}
	}
	glog.V(4).Infof("Replication controller %q deleted.", rc.Name)
	if dc, err := c.dcStore.GetConfigForController(rc); err == nil && dc != nil {
		c.enqueueDeploymentConfig(dc)
	}
}

func (c *DeploymentConfigController) enqueueDeploymentConfig(dc *deployapi.DeploymentConfig) {
	key, err := kcontroller.KeyFunc(dc)
	if err != nil {
		glog.Errorf("Couldn't get key for object %#v: %v", dc, err)
		return
	}
	c.queue.Add(key)
}

func (c *DeploymentConfigController) worker() {
	for {
		if quit := c.work(); quit {
			return
		}
	}
}

func (c *DeploymentConfigController) work() bool {
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

	copied, err := dcCopy(dc)
	if err != nil {
		glog.Error(err.Error())
		return false
	}

	err = c.Handle(copied)
	c.handleErr(err, key)

	return false
}

func (c *DeploymentConfigController) getByKey(key string) (*deployapi.DeploymentConfig, error) {
	obj, exists, err := c.dcStore.Indexer.GetByKey(key)
	if err != nil {
		glog.V(2).Infof("Unable to retrieve deployment config %q from store: %v", key, err)
		c.queue.AddRateLimited(key)
		return nil, err
	}
	if !exists {
		glog.V(4).Infof("Deployment config %q has been deleted", key)
		return nil, nil
	}

	return obj.(*deployapi.DeploymentConfig), nil
}

func dcCopy(dc *deployapi.DeploymentConfig) (*deployapi.DeploymentConfig, error) {
	objCopy, err := kapi.Scheme.DeepCopy(dc)
	if err != nil {
		return nil, err
	}
	copied, ok := objCopy.(*deployapi.DeploymentConfig)
	if !ok {
		return nil, fmt.Errorf("expected DeploymentConfig, got %#v", objCopy)
	}
	return copied, nil
}
