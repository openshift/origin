package node

import (
	"fmt"
	"strings"
	"time"

	coreapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	corelisterv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const nodeControllerWorkQueueKey = "key"

// NodeController watches for new master nodes and adds them to the node status list in the operator config status.
type NodeController struct {
	operatorClient v1helpers.StaticPodOperatorClient

	nodeLister corelisterv1.NodeLister

	cachesToSync  []cache.InformerSynced
	queue         workqueue.RateLimitingInterface
	eventRecorder events.Recorder
}

// NewNodeController creates a new node controller.
func NewNodeController(
	operatorClient v1helpers.StaticPodOperatorClient,
	kubeInformersClusterScoped informers.SharedInformerFactory,
	eventRecorder events.Recorder,
) *NodeController {
	c := &NodeController{
		operatorClient: operatorClient,
		eventRecorder:  eventRecorder.WithComponentSuffix("node-controller"),
		nodeLister:     kubeInformersClusterScoped.Core().V1().Nodes().Lister(),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "NodeController"),
	}

	operatorClient.Informer().AddEventHandler(c.eventHandler())
	kubeInformersClusterScoped.Core().V1().Nodes().Informer().AddEventHandler(c.eventHandler())

	c.cachesToSync = append(c.cachesToSync, operatorClient.Informer().HasSynced)
	c.cachesToSync = append(c.cachesToSync, kubeInformersClusterScoped.Core().V1().Nodes().Informer().HasSynced)

	return c
}

func (c NodeController) sync() error {
	_, originalOperatorStatus, _, err := c.operatorClient.GetStaticPodOperatorState()
	if err != nil {
		return err
	}

	selector, err := labels.NewRequirement("node-role.kubernetes.io/master", selection.Equals, []string{""})
	if err != nil {
		panic(err)
	}
	nodes, err := c.nodeLister.List(labels.NewSelector().Add(*selector))
	if err != nil {
		return err
	}

	newTargetNodeStates := []operatorv1.NodeStatus{}
	// remove entries for missing nodes
	for i, nodeState := range originalOperatorStatus.NodeStatuses {
		found := false
		for _, node := range nodes {
			if nodeState.NodeName == node.Name {
				found = true
			}
		}
		if found {
			newTargetNodeStates = append(newTargetNodeStates, originalOperatorStatus.NodeStatuses[i])
		} else {
			c.eventRecorder.Warningf("MasterNodeRemoved", "Observed removal of master node %s", nodeState.NodeName)
		}
	}

	// add entries for new nodes
	for _, node := range nodes {
		found := false
		for _, nodeState := range originalOperatorStatus.NodeStatuses {
			if nodeState.NodeName == node.Name {
				found = true
			}
		}
		if found {
			continue
		}

		c.eventRecorder.Eventf("MasterNodeObserved", "Observed new master node %s", node.Name)
		newTargetNodeStates = append(newTargetNodeStates, operatorv1.NodeStatus{NodeName: node.Name})
	}

	// detect and report master nodes that are not ready
	notReadyNodes := []string{}
	for _, node := range nodes {
		for _, con := range node.Status.Conditions {
			if con.Type == coreapiv1.NodeReady && con.Status != coreapiv1.ConditionTrue {
				notReadyNodes = append(notReadyNodes, node.Name)
			}
		}
	}
	newCondition := operatorv1.OperatorCondition{
		Type: condition.NodeControllerDegradedConditionType,
	}
	if len(notReadyNodes) > 0 {
		newCondition.Status = operatorv1.ConditionTrue
		newCondition.Reason = "MasterNodesReady"
		newCondition.Message = fmt.Sprintf("The master node(s) %q not ready", strings.Join(notReadyNodes, ","))
	} else {
		newCondition.Status = operatorv1.ConditionFalse
		newCondition.Reason = "MasterNodesReady"
		newCondition.Message = "All master node(s) are ready"
	}

	oldStatus := &operatorv1.StaticPodOperatorStatus{}
	_, updated, updateError := v1helpers.UpdateStaticPodStatus(c.operatorClient, v1helpers.UpdateStaticPodConditionFn(newCondition), func(status *operatorv1.StaticPodOperatorStatus) error {
		status.NodeStatuses = newTargetNodeStates
		return nil
	}, func(status *operatorv1.StaticPodOperatorStatus) error {
		//a hack for storing the old status (before the update)
		oldStatus = status
		return nil
	})

	if updateError != nil {
		return updateError
	}

	if !updated {
		return nil
	}

	for _, oldCondition := range oldStatus.Conditions {
		if oldCondition.Type == condition.NodeControllerDegradedConditionType && oldCondition.Message != newCondition.Message {
			c.eventRecorder.Eventf("MasterNodesReadyChanged", newCondition.Message)
			break
		}
	}
	return nil
}

// Run starts the kube-apiserver and blocks until stopCh is closed.
func (c *NodeController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting NodeController")
	defer klog.Infof("Shutting down NodeController")
	if !cache.WaitForCacheSync(stopCh, c.cachesToSync...) {
		return
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *NodeController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *NodeController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

// eventHandler queues the operator to check spec and status
func (c *NodeController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(nodeControllerWorkQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(nodeControllerWorkQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(nodeControllerWorkQueueKey) },
	}
}
