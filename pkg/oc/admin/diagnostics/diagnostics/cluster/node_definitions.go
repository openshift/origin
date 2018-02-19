package cluster

// The purpose of this diagnostic is to detect nodes that are out of commission
// (which may affect the ability to schedule pods) for user awareness.

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/apis/authorization"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/util"
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
    oc adm manage-node {{.node}} --schedulable=true

While in this state, pods should not be scheduled to deploy on the node.
Existing pods will continue to run until completed or evacuated (see
other options for 'oc adm manage-node').
`
)

// NodeDefinitions is a Diagnostic for analyzing the nodes in a cluster.
type NodeDefinitions struct {
	KubeClient kclientset.Interface
}

const NodeDefinitionsName = "NodeDefinitions"

func (d *NodeDefinitions) Name() string {
	return NodeDefinitionsName
}

func (d *NodeDefinitions) Description() string {
	return "Check node records on master"
}

func (d *NodeDefinitions) Requirements() (client bool, host bool) {
	return true, false
}

func (d *NodeDefinitions) CanRun() (bool, error) {
	if d.KubeClient == nil {
		return false, errors.New("must have kube client")
	}
	can, err := util.UserCan(d.KubeClient.Authorization(), &authorization.ResourceAttributes{
		Verb:     "list",
		Group:    kapi.GroupName,
		Resource: "nodes",
	})
	if err != nil {
		return false, types.DiagnosticError{ID: "DClu0005", LogMessage: fmt.Sprintf(clientErrorGettingNodes, err), Cause: err}
	} else if !can {
		return false, types.DiagnosticError{ID: "DClu0006", LogMessage: "Client does not have access to see node status", Cause: err}
	}
	return true, nil
}

func (d *NodeDefinitions) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult("NodeDefinition")

	nodes, err := d.KubeClient.Core().Nodes().List(metav1.ListOptions{})
	if err != nil {
		r.Error("DClu0001", err, fmt.Sprintf(clientErrorGettingNodes, err))
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
			r.Warn("DClu0002", nil, log.EvalTemplate("DClu0002", nodeNotReady, templateData))
		} else if node.Spec.Unschedulable {
			r.Warn("DClu0003", nil, log.EvalTemplate("DClu0003", nodeNotSched, log.Hash{"node": node.Name}))
		} else {
			anyNodesAvail = true
		}
	}
	if !anyNodesAvail {
		r.Error("DClu0004", nil, "There were no nodes available to use. No new pods can be scheduled.")
	}

	return r
}
