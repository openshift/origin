package in_pod

import (
	"errors"
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	kcontainer "k8s.io/kubernetes/pkg/kubelet/container"
	kexec "k8s.io/utils/exec"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/network/in_pod/util"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
)

const (
	CheckNodeNetworkName = "CheckNodeNetwork"
)

// CheckNodeNetwork is a Diagnostic to check that pods in the cluster can access its own node
type CheckNodeNetwork struct {
	KubeClient kclientset.Interface
}

// Name is part of the Diagnostic interface and just returns name.
func (d CheckNodeNetwork) Name() string {
	return CheckNodeNetworkName
}

// Description is part of the Diagnostic interface and just returns the diagnostic description.
func (d CheckNodeNetwork) Description() string {
	return "Check that pods in the cluster can access its own node."
}

func (d CheckNodeNetwork) Requirements() (client bool, host bool) {
	return true, false
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

	_, localIP, err := util.GetLocalNode(d.KubeClient)
	if err != nil {
		r.Error("DNodeNet1001", err, err.Error())
		return r
	}

	localPods, _, err := util.GetLocalAndNonLocalDiagnosticPods(d.KubeClient)
	if err != nil {
		r.Error("DNodeNet1002", err, fmt.Sprintf("Getting local and nonlocal pods failed. Error: %s", err))
		return r
	}

	for _, pod := range localPods {
		checkNodeConnection(&pod, localIP, r)
	}
	return r
}

func checkNodeConnection(pod *kapi.Pod, nodeIP string, r types.DiagnosticResult) {
	if len(pod.Status.ContainerStatuses) == 0 {
		err := fmt.Errorf("ContainerID not found for pod %q", util.PrintPod(pod))
		r.Error("DNodeNet1003", err, err.Error())
		return
	}

	kexecer := kexec.New()
	containerID := kcontainer.ParseContainerID(pod.Status.ContainerStatuses[0].ContainerID).ID
	pid, err := kexecer.Command("docker", "inspect", "-f", "{{.State.Pid}}", containerID).CombinedOutput()
	if err != nil {
		r.Error("DNodeNet1004", err, fmt.Sprintf("Fetching pid for pod %q, container %q failed. Error: %s", util.PrintPod(pod), containerID, err))
		return
	}

	if _, err := kexecer.Command("nsenter", "-n", "-t", strings.Trim(fmt.Sprintf("%s", pid), "\n"), "--", "ping", "-c1", "-W2", nodeIP).CombinedOutput(); err != nil {
		r.Error("DNodeNet1005", err, fmt.Sprintf("Connectivity from pod %q to node %q failed. Error: %s", util.PrintPod(pod), nodeIP, err))
	}
}
