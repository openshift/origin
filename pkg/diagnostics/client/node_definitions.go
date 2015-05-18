package client

import (
	"errors"
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types/diagnostic"
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
)

// NodeDefinitions
type NodeDefinition struct {
	KubeClient *kclient.Client

	Log *log.Logger
}

func (d NodeDefinition) Description() string {
	return "Check node records on master"
}
func (d NodeDefinition) CanRun() (bool, error) {
	if d.KubeClient == nil {
		// TODO make prettier?
		return false, errors.New("must have kube client")
	}
	if _, err := d.KubeClient.Nodes().List(labels.LabelSelector{}, fields.Everything()); err != nil {
		// TODO check for 403 to return: "Client does not have cluster-admin access and cannot see node records"

		return false, diagnostic.NewDiagnosticError("clGetNodesFailed", fmt.Sprintf(clientErrorGettingNodes, err), err)
	}

	return true, nil
}
func (d NodeDefinition) Check() (bool, []log.Message, []error, []error) {
	if _, err := d.CanRun(); err != nil {
		return false, nil, nil, []error{err}
	}

	nodes, err := d.KubeClient.Nodes().List(labels.LabelSelector{}, fields.Everything())
	if err != nil {
		return false, nil, nil, []error{
			diagnostic.NewDiagnosticError("clGetNodesFailed", fmt.Sprintf(clientErrorGettingNodes, err), err),
		}
	}

	for _, node := range nodes.Items {
		var ready *kapi.NodeCondition
		for i, condition := range node.Status.Conditions {
			switch condition.Type {
			// currently only one... used to be more, may be again
			case kapi.NodeReady:
				ready = &node.Status.Conditions[i]
				// TODO comment needed to explain why we do last one wins.  should this break instead?
			}
		}

		if ready == nil || ready.Status != kapi.ConditionTrue {
			// instead of building this, simply use the node object directly
			templateData := map[string]interface{}{}
			templateData["node"] = node.Name
			if ready == nil {
				templateData["status"] = "None"
				templateData["reason"] = "There is no readiness record."
			} else {
				templateData["status"] = ready.Status
				templateData["reason"] = ready.Reason
			}

			return false, nil, []error{
				diagnostic.NewDiagnosticErrorFromTemplate("clNodeBroken", nodeNotReady, templateData),
			}, nil
		}
	}

	return true, nil, nil, nil
}
