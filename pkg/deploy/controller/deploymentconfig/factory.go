package deploymentconfig

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	kclientv1 "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/clientset"
	kcoreinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/externalversions/core/v1"
	kcontroller "k8s.io/kubernetes/pkg/controller"

	osclient "github.com/openshift/origin/pkg/client"
	oscontroller "github.com/openshift/origin/pkg/controller"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
)

const (
	// We must avoid creating new replication controllers until the deployment config and replication
	// controller stores have synced. If it hasn't synced, to avoid a hot loop, we'll wait this long
	// between checks.
	storeSyncedPollPeriod = 100 * time.Millisecond
)

// NewDeploymentConfigController creates a new DeploymentConfigController.
func NewDeploymentConfigController(
	dcInformer cache.SharedIndexInformer,
	rcInformer kcoreinformers.ReplicationControllerInformer,
	oc osclient.Interface,
	kubeClientset kclientset.Interface,
	codec runtime.Codec,
) *DeploymentConfigController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: kv1core.New(kubeClientset.CoreV1().RESTClient()).Events("")})
	recorder := eventBroadcaster.NewRecorder(kapi.Scheme, kclientv1.EventSource{Component: "deploymentconfig-controller"})

	c := &DeploymentConfigController{
		dn: oc,
		rn: kubeClientset.Core(),

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		rcLister:       rcInformer.Lister(),
		rcListerSynced: rcInformer.Informer().HasSynced,
		rcControl: oscontroller.RealRCControl{
			KubeClient: kubeClientset,
			Recorder:   recorder,
		},

		recorder: recorder,
		codec:    codec,
	}

	c.dcStore.Indexer = dcInformer.GetIndexer()
	dcInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addDeploymentConfig,
		UpdateFunc: c.updateDeploymentConfig,
		DeleteFunc: c.deleteDeploymentConfig,
	})
	c.dcStoreSynced = dcInformer.HasSynced

	rcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: c.updateReplicationController,
		DeleteFunc: c.deleteReplicationController,
	})

	return c
}

// Run begins watching and syncing.
func (c *DeploymentConfigController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting deploymentconfig controller")

	// Wait for the rc and dc stores to sync before starting any work in this controller.
	if !cache.WaitForCacheSync(stopCh, c.dcStoreSynced, c.rcListerSynced) {
		return
	}

	glog.Info("deploymentconfig controller caches are synced. Starting workers.")

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh

	glog.Infof("Shutting down deploymentconfig controller")
}

func (c *DeploymentConfigController) addDeploymentConfig(obj interface{}) {
	dc := obj.(*deployapi.DeploymentConfig)
	glog.V(4).Infof("Adding deployment config %q", dc.Name)
	c.enqueueDeploymentConfig(dc)
}

func (c *DeploymentConfigController) updateDeploymentConfig(old, cur interface{}) {
	// A periodic relist will send update events for all known configs.
	newDc := cur.(*deployapi.DeploymentConfig)
	oldDc := old.(*deployapi.DeploymentConfig)
	if newDc.ResourceVersion == oldDc.ResourceVersion {
		return
	}

	glog.V(4).Infof("Updating deployment config %q", newDc.Name)
	c.enqueueDeploymentConfig(newDc)
}

func (c *DeploymentConfigController) deleteDeploymentConfig(obj interface{}) {
	dc, ok := obj.(*deployapi.DeploymentConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %+v", obj))
			return
		}
		dc, ok = tombstone.Obj.(*deployapi.DeploymentConfig)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a deployment config: %+v", obj))
			return
		}
	}
	glog.V(4).Infof("Deleting deployment config %q", dc.Name)
	c.enqueueDeploymentConfig(dc)
}

// updateReplicationController figures out which deploymentconfig is managing this replication
// controller and requeues the deployment config.
func (c *DeploymentConfigController) updateReplicationController(old, cur interface{}) {
	// A periodic relist will send update events for all known controllers.
	curRC := cur.(*v1.ReplicationController)
	oldRC := old.(*v1.ReplicationController)
	if curRC.ResourceVersion == oldRC.ResourceVersion {
		return
	}

	if dc, err := c.dcStore.GetConfigForController(curRC); err == nil && dc != nil {
		c.enqueueDeploymentConfig(dc)
	}
}

// deleteReplicationController enqueues the deployment that manages a replicationcontroller when
// the replicationcontroller is deleted. obj could be an *v1.ReplicationController, or
// a DeletionFinalStateUnknown marker item.
func (c *DeploymentConfigController) deleteReplicationController(obj interface{}) {
	rc, ok := obj.(*v1.ReplicationController)

	// When a delete is dropped, the relist will notice a pod in the store not
	// in the list, leading to the insertion of a tombstone object which contains
	// the deleted key/value.
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Couldn't get object from tombstone %#v", obj))
			return
		}
		rc, ok = tombstone.Obj.(*v1.ReplicationController)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("Tombstone contained object that is not a replication controller %#v", obj))
			return
		}
	}
	if dc, err := c.dcStore.GetConfigForController(rc); err == nil && dc != nil {
		c.enqueueDeploymentConfig(dc)
	}
}

func (c *DeploymentConfigController) enqueueDeploymentConfig(dc *deployapi.DeploymentConfig) {
	key, err := kcontroller.KeyFunc(dc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for object %#v: %v", dc, err))
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
		utilruntime.HandleError(err)
	}

	if dc == nil {
		return false
	}

	err = c.Handle(dc)
	c.handleErr(err, key)

	return false
}

func (c *DeploymentConfigController) getByKey(key string) (*deployapi.DeploymentConfig, error) {
	obj, exists, err := c.dcStore.Indexer.GetByKey(key)
	if err != nil {
		c.queue.AddRateLimited(key)
		return nil, err
	}
	if !exists {
		glog.V(4).Infof("Deployment config %q has been deleted", key)
		return nil, nil
	}

	return obj.(*deployapi.DeploymentConfig), nil
}
