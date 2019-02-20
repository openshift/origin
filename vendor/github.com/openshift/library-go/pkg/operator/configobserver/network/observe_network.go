package network

import (
	"fmt"

	configv1 "github.com/openshift/api/config"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/library-go/pkg/operator/events"
)

// GetClusterCIDRs reads the cluster CIDRs from the global network configuration resource. Emits events if CIDRs are not found.
func GetClusterCIDRs(lister configlistersv1.NetworkLister, recorder events.Recorder) ([]string, error) {
	network, err := lister.Get("cluster")
	if errors.IsNotFound(err) {
		recorder.Warningf("ObserveRestrictedCIDRFailed", "Required networks.%s/cluster not found", configv1.GroupName)
		return nil, nil
	}
	if err != nil {
		recorder.Warningf("ObserveRestrictedCIDRFailed", "error getting networks.%s/cluster: %v", configv1.GroupName, err)
		return nil, err
	}

	if len(network.Status.ClusterNetwork) == 0 {
		recorder.Warningf("ObserveClusterCIDRFailed", "Required status.clusterNetwork field is not set in networks.%s/cluster", configv1.GroupName)
		return nil, fmt.Errorf("networks.%s/cluster: status.clusterNetwork not found", configv1.GroupName)
	}

	var clusterCIDRs []string
	for i, clusterNetwork := range network.Status.ClusterNetwork {
		if len(clusterNetwork.CIDR) == 0 {
			recorder.Warningf("ObserveRestrictedCIDRFailed", "Required status.clusterNetwork[%d].cidr field is not set in networks.%s/cluster", i, configv1.GroupName)
			return nil, fmt.Errorf("networks.%s/cluster: status.clusterNetwork[%d].cidr not found", configv1.GroupName, i)
		}
		clusterCIDRs = append(clusterCIDRs, clusterNetwork.CIDR)
	}
	// TODO fallback to podCIDR? is that still a thing?
	return clusterCIDRs, nil
}

// GetServiceCIDR reads the service IP range from the global network configuration resource. Emits events if CIDRs are not found.
func GetServiceCIDR(lister configlistersv1.NetworkLister, recorder events.Recorder) (string, error) {
	network, err := lister.Get("cluster")
	if errors.IsNotFound(err) {
		recorder.Warningf("ObserveServiceClusterIPRangesFailed", "Required networks.%s/cluster not found", configv1.GroupName)
		return "", nil
	}
	if err != nil {
		recorder.Warningf("ObserveServiceClusterIPRangesFailed", "error getting networks.%s/cluster: %v", configv1.GroupName, err)
		return "", err
	}

	if len(network.Status.ServiceNetwork) == 0 || len(network.Status.ServiceNetwork[0]) == 0 {
		recorder.Warningf("ObserveServiceClusterIPRangesFailed", "Required status.serviceNetwork field is not set in networks.%s/cluster", configv1.GroupName)
		return "", fmt.Errorf("networks.%s/cluster: status.serviceNetwork not found", configv1.GroupName)
	}
	return network.Status.ServiceNetwork[0], nil
}
