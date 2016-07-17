package network

import (
	"errors"
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kexec "k8s.io/kubernetes/pkg/util/exec"

	"github.com/openshift/origin/pkg/diagnostics/types"
	"github.com/openshift/origin/pkg/diagnostics/util"
	"github.com/openshift/origin/pkg/util/netutils"
)

const (
	CheckNodeNetworkName = "CheckNodeNetwork"
)

// CheckNodeNetwork is a Diagnostic to check that all nodes in the cluster are accessible within a pod
type CheckNodeNetwork struct {
	KubeClient *kclient.Client
}

// Name is part of the Diagnostic interface and just returns name.
func (d CheckNodeNetwork) Name() string {
	return CheckNodeNetworkName
}

// Description is part of the Diagnostic interface and just returns the diagnostic description.
func (d CheckNodeNetwork) Description() string {
	return "Check that all nodes in the cluster are accessible within a pod"
}

// CanRun is part of the Diagnostic interface; it determines if the conditions are right to run this diagnostic.
func (d CheckNodeNetwork) CanRun() (bool, error) {
	if d.KubeClient == nil {
		return false, errors.New("must have kube client")
	}
	return true, nil
}

// Check is part of the Diagnostic interface; it runs the actual diagnostic logic
func (d CheckNodeNetwork) Check() types.DiagnosticResult {
	r := types.NewDiagnosticResult(CheckNodeNetworkName)

	nodes, err := util.GetNodes(d.KubeClient)
	if err != nil {
		r.Error("DNodeNet1001", err, err.Error())
		return r
	}

	for _, node := range nodes {
		checkNodeConnection(&node, r)
	}
	return r
}

func checkNodeConnection(node *kapi.Node, r types.DiagnosticResult) {
	nodeIP, err := getNodeIP(node)
	if err != nil {
		r.Error("DNodeNet1002", err, fmt.Sprintf("Getting IP for node %q failed. Error: %s", node.Name, err))
		return
	}

	kexecer := kexec.New()
	if _, err = kexecer.Command("ping", "-c1", "-W1", nodeIP).CombinedOutput(); err != nil {
		r.Error("DNodeNet1003", err, fmt.Sprintf("Pinging node %q with IP %q failed. Error: %s", node.Name, nodeIP, err))
		return
	}
}

func getNodeIP(node *kapi.Node) (string, error) {
	if len(node.Status.Addresses) > 0 && node.Status.Addresses[0].Address != "" {
		return node.Status.Addresses[0].Address, nil
	} else {
		return netutils.GetNodeIP(node.Name)
	}
}
