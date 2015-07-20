package cluster

// The purpose of this diagnostic is to detect nodes that are out of commission
// (which may affect the ability to schedule pods) for user awareness.

import (
	"errors"
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

const (
	clientErrorGettingNodes = `Client error while retrieving node records. Client retrieved records
during discovery, so this is likely to be a transient error. Try running
diagnostics again. If this message persists, there may be a permissions
problem with getting node records. The error was:

(%T) %[1]v`

	nodeNotReady = `Node {{.node}} is defined but is not marked as ready.
Ready status is {{.status}} because "{{.reason}}"
If the node is not intentionally disabled, check that the master can
reach the node hostname for a health check and the node is checking in
to the master with the same hostname.

While in this state, pods should not be scheduled to deploy on the node,
and any existing scheduled pods will be considered failed and removed.
`

	nodeNotSched = `Node {{.node}} is ready but is marked Unschedulable.
This is usually set manually for administrative reasons.
An administrator can mark the node schedulable with:
    oadm manage-node {{.node}} --schedulable=true

While in this state, pods should not be scheduled to deploy on the node.
Existing pods will continue to run until completed or evacuated (see
other options for 'oadm manage-node').
`
)

// NodeDefinitions
type NodeDefinitions struct {
	KubeClient *kclient.Client
}

func (d NodeDefinitions) Name() string {
	return "NodeDefinitions"
}

func (d NodeDefinitions) Description() string {
	return "Check node records on master"
}

func (d NodeDefinitions) CanRun() (bool, error) {
	if d.KubeClient == nil {
		return false, errors.New("must have kube client")
	}
	if _, err := d.KubeClient.Nodes().List(labels.LabelSelector{}, fields.Everything()); err != nil {
		// TODO check for 403 to return: "Client does not have cluster-admin access and cannot see node records"

		msg := log.Message{ID: "clGetNodesFailed", EvaluatedText: fmt.Sprintf(clientErrorGettingNodes, err)}
		return false, types.DiagnosticError{msg.ID, &msg, err}
	}

	return true, nil
}

func (d NodeDefinitions) Check() *types.DiagnosticResult {
	r := types.NewDiagnosticResult("NodeDefinition")

	nodes, err := d.KubeClient.Nodes().List(labels.LabelSelector{}, fields.Everything())
	if err != nil {
		r.Errorf("clGetNodesFailed", err, clientErrorGettingNodes, err)
		return r
	}

	anyNodesAvail := false
	for _, node := range nodes.Items {
		var ready *kapi.NodeCondition
		for i, condition := range node.Status.Conditions {
			switch condition.Type {
			// Each condition appears only once. Currently there's only one... used to be more
			case kapi.NodeReady:
				ready = &node.Status.Conditions[i]
			}
		}

		if ready == nil || ready.Status != kapi.ConditionTrue {
			templateData := log.Hash{"node": node.Name}
			if ready == nil {
				templateData["status"] = "None"
				templateData["reason"] = "There is no readiness record."
			} else {
				templateData["status"] = ready.Status
				templateData["reason"] = ready.Reason
			}
			r.Warnt("clNodeNotReady", nil, nodeNotReady, templateData)
		} else if node.Spec.Unschedulable {
			r.Warnt("clNodeNotSched", nil, nodeNotSched, log.Hash{"node": node.Name})
		} else {
			anyNodesAvail = true
		}
	}
	if !anyNodesAvail {
		r.Error("clNoAvailNodes", nil, "There were no nodes available for OpenShift to use.")
	}

	return r
}
