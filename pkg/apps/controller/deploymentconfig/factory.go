package deploymentconfig

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kcoreinformers "k8s.io/client-go/informers/core/v1"
	kclientset "k8s.io/client-go/kubernetes"
	kv1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kcontroller "k8s.io/kubernetes/pkg/controller"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsinformer "github.com/openshift/origin/pkg/apps/generated/informers/internalversion/apps/internalversion"
	appsclient "github.com/openshift/origin/pkg/apps/generated/internalclientset"
	metrics "github.com/openshift/origin/pkg/apps/metrics/prometheus"
)

// NewDeploymentConfigController creates a new DeploymentConfigController.
func NewDeploymentConfigController(
	dcInformer appsinformer.DeploymentConfigInformer,
	rcInformer kcoreinformers.ReplicationControllerInformer,
	appsClientset appsclient.Interface,
	kubeClientset kclientset.Interface,
	codec runtime.Codec,
) *DeploymentConfigController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&kv1core.EventSinkImpl{Interface: kv1core.New(kubeClientset.CoreV1().RESTClient()).Events("")})
	recorder := eventBroadcaster.NewRecorder(legacyscheme.Scheme, v1.EventSource{Component: "deploymentconfig-controller"})

	c := &DeploymentConfigController{
		dn: appsClientset.Apps(),
		rn: kubeClientset.CoreV1(),

		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		rcLister:       rcInformer.Lister(),
		rcListerSynced: rcInformer.Informer().HasSynced,
		rcControl: RealRCControl{
			KubeClient: kubeClientset,
			Recorder:   recorder,
		},

		recorder: recorder,
		codec:    codec,
	}

	c.dcLister = dcInformer.Lister()
	dcInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addDeploymentConfig,
		UpdateFunc: c.updateDeploymentConfig,
		DeleteFunc: c.deleteDeploymentConfig,
	})
	c.dcStoreSynced = dcInformer.Informer().HasSynced

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

	metrics.InitializeMetricsCollector(c.rcLister)

	<-stopCh

	glog.Infof("Shutting down deploymentconfig controller")
}

func (c *DeploymentConfigController) addDeploymentConfig(obj interface{}) {
	dc := obj.(*appsapi.DeploymentConfig)
	glog.V(4).Infof("Adding deployment config %s/%s", dc.Namespace, dc.Name)
	c.enqueueDeploymentConfig(dc)
}

func (c *DeploymentConfigController) updateDeploymentConfig(old, cur interface{}) {
	newDc := cur.(*appsapi.DeploymentConfig)
	oldDc := old.(*appsapi.DeploymentConfig)

	glog.V(4).Infof("Updating deployment config %s/%s", oldDc.Namespace, oldDc.Name)
	c.enqueueDeploymentConfig(newDc)
}

func (c *DeploymentConfigController) deleteDeploymentConfig(obj interface{}) {
	dc, ok := obj.(*appsapi.DeploymentConfig)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %+v", obj))
			return
		}
		dc, ok = tombstone.Obj.(*appsapi.DeploymentConfig)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a deployment config: %+v", obj))
			return
		}
	}
	glog.V(4).Infof("Deleting deployment config %s/%s", dc.Namespace, dc.Name)
	c.enqueueDeploymentConfig(dc)
}

// updateReplicationController figures out which deploymentconfig is managing this replication
// controller and requeues the deployment config.
func (c *DeploymentConfigController) updateReplicationController(old, cur interface{}) {
	curRC := cur.(*v1.ReplicationController)
	oldRC := old.(*v1.ReplicationController)

	// We can safely ignore periodic re-lists on RCs as we react to periodic re-lists of DCs
	if curRC.ResourceVersion == oldRC.ResourceVersion {
		return
	}

	if dc, err := c.dcLister.GetConfigForController(curRC); err == nil && dc != nil {
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
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		rc, ok = tombstone.Obj.(*v1.ReplicationController)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a replication controller %#v", obj))
			return
		}
	}
	if dc, err := c.dcLister.GetConfigForController(rc); err == nil && dc != nil {
		c.enqueueDeploymentConfig(dc)
	}
}

func (c *DeploymentConfigController) enqueueDeploymentConfig(dc *appsapi.DeploymentConfig) {
	key, err := kcontroller.KeyFunc(dc)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %#v: %v", dc, err))
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

	namespace, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		utilruntime.HandleError(err)
		return false
	}
	dc, err := c.dcLister.DeploymentConfigs(namespace).Get(name)
	if errors.IsNotFound(err) {
		return false
	}
	if err != nil {
		utilruntime.HandleError(err)
		return false
	}

	err = c.Handle(dc)
	c.handleErr(err, key)

	return false
}
