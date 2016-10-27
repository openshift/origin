package deployment

import (
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

	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

const (
	// We must avoid creating processing deployment configs until the deployment config and image
	// stream stores have synced. If it hasn't synced, to avoid a hot loop, we'll wait this long
	// between checks.
	storeSyncedPollPeriod = 100 * time.Millisecond
)

// NewDeploymentController creates a new DeploymentController.
func NewDeploymentController(rcInformer, podInformer framework.SharedIndexInformer, kc kclient.Interface, sa, image string, env []kapi.EnvVar, codec runtime.Codec) *DeploymentController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(kc.Events(""))
	recorder := eventBroadcaster.NewRecorder(kapi.EventSource{Component: "deployments-controller"})

	c := &DeploymentController{
		rn: kc,
		pn: kc,

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		serviceAccount: sa,
		deployerImage:  image,
		environment:    env,
		recorder:       recorder,
		codec:          codec,
	}

	c.rcStore.Indexer = rcInformer.GetIndexer()
	rcInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		AddFunc:    c.addReplicationController,
		UpdateFunc: c.updateReplicationController,
	})

	c.podStore.Indexer = podInformer.GetIndexer()
	podInformer.AddEventHandler(framework.ResourceEventHandlerFuncs{
		UpdateFunc: c.updatePod,
		DeleteFunc: c.deletePod,
	})

	c.rcStoreSynced = rcInformer.HasSynced
	c.podStoreSynced = podInformer.HasSynced

	return c
}

// Run begins watching and syncing.
func (c *DeploymentController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	// Wait for the dc store to sync before starting any work in this controller.
	ready := make(chan struct{})
	go c.waitForSyncedStores(ready, stopCh)
	select {
	case <-ready:
	case <-stopCh:
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.Infof("Shutting down deployment controller")
	c.queue.ShutDown()
}

func (c *DeploymentController) waitForSyncedStores(ready chan<- struct{}, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()

	for !c.rcStoreSynced() || !c.podStoreSynced() {
		glog.V(4).Infof("Waiting for the rc and pod caches to sync before starting the deployment controller workers")
		select {
		case <-time.After(storeSyncedPollPeriod):
		case <-stopCh:
			return
		}
	}
	close(ready)
}

func (c *DeploymentController) addReplicationController(obj interface{}) {
	rc := obj.(*kapi.ReplicationController)
	// Filter out all unrelated replication controllers.
	if !deployutil.IsOwnedByConfig(rc) {
		return
	}

	c.enqueueReplicationController(rc)
}

func (c *DeploymentController) updateReplicationController(old, cur interface{}) {
	// A periodic relist will send update events for all known controllers.
	curRC := cur.(*kapi.ReplicationController)
	oldRC := old.(*kapi.ReplicationController)
	if curRC.ResourceVersion == oldRC.ResourceVersion {
		return
	}

	// Filter out all unrelated replication controllers.
	if !deployutil.IsOwnedByConfig(curRC) {
		return
	}

	c.enqueueReplicationController(curRC)
}

func (c *DeploymentController) updatePod(old, cur interface{}) {
	// A periodic relist will send update events for all known pods.
	curPod := cur.(*kapi.Pod)
	oldPod := old.(*kapi.Pod)
	if curPod.ResourceVersion == oldPod.ResourceVersion {
		return
	}

	if rc, err := c.rcForDeployerPod(curPod); err == nil && rc != nil {
		c.enqueueReplicationController(rc)
	}
}

func (c *DeploymentController) deletePod(obj interface{}) {
	pod, ok := obj.(*kapi.Pod)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Errorf("Couldn't get object from tombstone: %+v", obj)
			return
		}
		pod, ok = tombstone.Obj.(*kapi.Pod)
		if !ok {
			glog.Errorf("Tombstone contained object that is not a pod: %+v", obj)
			return
		}
	}

	if rc, err := c.rcForDeployerPod(pod); err == nil && rc != nil {
		c.enqueueReplicationController(rc)
	}
}

func (c *DeploymentController) enqueueReplicationController(rc *kapi.ReplicationController) {
	key, err := kcontroller.KeyFunc(rc)
	if err != nil {
		glog.Errorf("Couldn't get key for object %#v: %v", rc, err)
		return
	}
	c.queue.Add(key)
}

func (c *DeploymentController) rcForDeployerPod(pod *kapi.Pod) (*kapi.ReplicationController, error) {
	key := pod.Namespace + "/" + deployutil.DeploymentNameFor(pod)
	return c.getByKey(key)
}

func (c *DeploymentController) worker() {
	for {
		if quit := c.work(); quit {
			return
		}
	}
}

func (c *DeploymentController) work() bool {
	key, quit := c.queue.Get()
	if quit {
		return true
	}
	defer c.queue.Done(key)

	rc, err := c.getByKey(key.(string))
	if err != nil {
		glog.Error(err.Error())
	}

	if rc == nil {
		return false
	}

	err = c.Handle(rc)
	c.handleErr(err, key, rc)

	return false
}

func (c *DeploymentController) getByKey(key string) (*kapi.ReplicationController, error) {
	obj, exists, err := c.rcStore.Indexer.GetByKey(key)
	if err != nil {
		glog.Infof("Unable to retrieve replication controller %q from store: %v", key, err)
		c.queue.Add(key)
		return nil, err
	}
	if !exists {
		glog.Infof("Replication controller %q has been deleted", key)
		return nil, nil
	}

	return obj.(*kapi.ReplicationController), nil
}
