// +build !linux

package plugin

import (
	"fmt"

	"github.com/openshift/origin/pkg/sdn/plugin/cniserver"

	kubehostport "k8s.io/kubernetes/pkg/kubelet/network/hostport"

	cnitypes "github.com/containernetworking/cni/pkg/types"
)

func (m *podManager) setup(req *cniserver.PodRequest) (*cnitypes.Result, *runningPod, error) {
	return nil, nil, fmt.Errorf("openshift-sdn is unsupported on this OS!")
}

func (m *podManager) update(req *cniserver.PodRequest) (*runningPod, error) {
	return nil, fmt.Errorf("openshift-sdn is unsupported on this OS!")
}

// Clean up all pod networking (clear OVS flows, release IPAM lease, remove host/container veth)
func (m *podManager) teardown(req *cniserver.PodRequest) error {
	return fmt.Errorf("openshift-sdn is unsupported on this OS!")
}
