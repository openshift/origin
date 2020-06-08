package node

import (
	"context"
	"fmt"
	"strings"

	coreapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/informers"
	corelisterv1 "k8s.io/client-go/listers/core/v1"

	operatorv1 "github.com/openshift/api/operator/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

// NodeController watches for new master nodes and adds them to the node status list in the operator config status.
type NodeController struct {
	operatorClient v1helpers.StaticPodOperatorClient
	nodeLister     corelisterv1.NodeLister
}

// NewNodeController creates a new node controller.
func NewNodeController(
	operatorClient v1helpers.StaticPodOperatorClient,
	kubeInformersClusterScoped informers.SharedInformerFactory,
	eventRecorder events.Recorder,
) factory.Controller {
	c := &NodeController{
		operatorClient: operatorClient,
		nodeLister:     kubeInformersClusterScoped.Core().V1().Nodes().Lister(),
	}
	return factory.New().WithInformers(operatorClient.Informer(), kubeInformersClusterScoped.Core().V1().Nodes().Informer()).WithSync(c.sync).ToController("NodeController", eventRecorder)
}

func (c NodeController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
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
			syncCtx.Recorder().Warningf("MasterNodeRemoved", "Observed removal of master node %s", nodeState.NodeName)
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

		syncCtx.Recorder().Eventf("MasterNodeObserved", "Observed new master node %s", node.Name)
		newTargetNodeStates = append(newTargetNodeStates, operatorv1.NodeStatus{NodeName: node.Name})
	}

	// detect and report master nodes that are not ready
	notReadyNodes := []string{}
	for _, node := range nodes {
		nodeReadyCondition := nodeConditionFinder(&node.Status, coreapiv1.NodeReady)

		// If a "Ready" condition is not found, that node should be deemed as not Ready by default.
		if nodeReadyCondition == nil {
			notReadyNodes = append(notReadyNodes, fmt.Sprintf("node %q not ready, no Ready condition found in status block", node.Name))
			continue
		}

		if nodeReadyCondition.Status != coreapiv1.ConditionTrue {
			notReadyNodes = append(notReadyNodes, fmt.Sprintf("node %q not ready since %s because %s (%s)", node.Name, nodeReadyCondition.LastTransitionTime, nodeReadyCondition.Reason, nodeReadyCondition.Message))
		}
	}

	newCondition := operatorv1.OperatorCondition{
		Type: condition.NodeControllerDegradedConditionType,
	}
	if len(notReadyNodes) > 0 {
		newCondition.Status = operatorv1.ConditionTrue
		newCondition.Reason = "MasterNodesReady"
		newCondition.Message = fmt.Sprintf("The master nodes not ready: %s", strings.Join(notReadyNodes, ", "))
	} else {
		newCondition.Status = operatorv1.ConditionFalse
		newCondition.Reason = "MasterNodesReady"
		newCondition.Message = "All master nodes are ready"
	}

	oldStatus := &operatorv1.StaticPodOperatorStatus{}
	_, updated, updateError := v1helpers.UpdateStaticPodStatus(c.operatorClient, func(status *operatorv1.StaticPodOperatorStatus) error {
		//a hack for storing the old status (before we mutate it)
		oldStatus = status
		return nil
	}, v1helpers.UpdateStaticPodConditionFn(newCondition), func(status *operatorv1.StaticPodOperatorStatus) error {
		status.NodeStatuses = newTargetNodeStates
		return nil
	})

	if updateError != nil {
		return updateError
	}

	if !updated {
		return nil
	}

	oldNodeDegradedCondition := v1helpers.FindOperatorCondition(oldStatus.Conditions, condition.NodeControllerDegradedConditionType)
	if oldNodeDegradedCondition == nil || oldNodeDegradedCondition.Message != newCondition.Message {
		syncCtx.Recorder().Eventf("MasterNodesReadyChanged", newCondition.Message)
	}

	return nil
}

func nodeConditionFinder(status *coreapiv1.NodeStatus, condType coreapiv1.NodeConditionType) *coreapiv1.NodeCondition {
	for i := range status.Conditions {
		if status.Conditions[i].Type == condType {
			return &status.Conditions[i]
		}
	}

	return nil
}
