package network

import (
	"fmt"
	"net"

	configv1 "github.com/openshift/api/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift/library-go/pkg/operator/events"
)

// GetClusterCIDRs reads the cluster CIDRs from the global network configuration resource. Emits events if CIDRs are not found.
func GetClusterCIDRs(lister configlistersv1.NetworkLister, recorder events.Recorder) ([]string, error) {
	network, err := lister.Get("cluster")
	if errors.IsNotFound(err) {
		recorder.Warningf("GetClusterCIDRsFailed", "Required networks.%s/cluster not found", configv1.GroupName)
		return nil, nil
	}
	if err != nil {
		recorder.Warningf("GetClusterCIDRsFailed", "error getting networks.%s/cluster: %v", configv1.GroupName, err)
		return nil, err
	}

	if len(network.Status.ClusterNetwork) == 0 {
		recorder.Warningf("GetClusterCIDRsFailed", "Required status.clusterNetwork field is not set in networks.%s/cluster", configv1.GroupName)
		return nil, fmt.Errorf("networks.%s/cluster: status.clusterNetwork not found", configv1.GroupName)
	}

	var clusterCIDRs []string
	for i, clusterNetwork := range network.Status.ClusterNetwork {
		if len(clusterNetwork.CIDR) == 0 {
			recorder.Warningf("GetClusterCIDRsFailed", "Required status.clusterNetwork[%d].cidr field is not set in networks.%s/cluster", i, configv1.GroupName)
			return nil, fmt.Errorf("networks.%s/cluster: status.clusterNetwork[%d].cidr not found", configv1.GroupName, i)
		}
		clusterCIDRs = append(clusterCIDRs, clusterNetwork.CIDR)
	}

	return clusterCIDRs, nil
}

// GetServiceCIDR reads the service IP range from the global network configuration resource. Emits events if CIDRs are not found.
func GetServiceCIDR(lister configlistersv1.NetworkLister, recorder events.Recorder) (string, error) {
	network, err := lister.Get("cluster")
	if errors.IsNotFound(err) {
		recorder.Warningf("GetServiceCIDRFailed", "Required networks.%s/cluster not found", configv1.GroupName)
		return "", nil
	}
	if err != nil {
		recorder.Warningf("GetServiceCIDRFailed", "error getting networks.%s/cluster: %v", configv1.GroupName, err)
		return "", err
	}

	if len(network.Status.ServiceNetwork) == 0 || len(network.Status.ServiceNetwork[0]) == 0 {
		recorder.Warningf("GetServiceCIDRFailed", "Required status.serviceNetwork field is not set in networks.%s/cluster", configv1.GroupName)
		return "", fmt.Errorf("networks.%s/cluster: status.serviceNetwork not found", configv1.GroupName)
	}

	return network.Status.ServiceNetwork[0], nil
}

// GetExternalIPPolicy retrieves the ExternalIPPolicy for the cluster.
// The policy may be null.
func GetExternalIPPolicy(lister configlistersv1.NetworkLister, recorder events.Recorder) (*configv1.ExternalIPPolicy, error) {
	network, err := lister.Get("cluster")
	if errors.IsNotFound(err) {
		recorder.Warningf("GetExternalIPPolicyFailed", "Required networks.%s/cluster not found", configv1.GroupName)
		return nil, nil
	}
	if err != nil {
		recorder.Warningf("GetExternalIPPolicyFailed", "error getting networks.%s/cluster: %v", configv1.GroupName, err)
		return nil, err
	}

	if network.Spec.ExternalIP == nil {
		return nil, nil
	}

	pol := network.Spec.ExternalIP.Policy
	if pol != nil {
		if err := validateCIDRs(pol.AllowedCIDRs); err != nil {
			recorder.Warningf("GetExternalIPPolicyFailed", "error parsing networks.%s/cluster Spec.ExternalIP.Policy.AllowedCIDRs: invalid cidr: %v", configv1.GroupName, err)
			return nil, err
		}
		if err := validateCIDRs(pol.RejectedCIDRs); err != nil {
			recorder.Warningf("GetExternalIPPolicyFailed", "error parsing networks.%s/cluster Spec.ExternalIP.Policy.RejectedCIDRs: invalid cidr: %v", configv1.GroupName, err)
			return nil, err
		}
	}

	return network.Spec.ExternalIP.Policy, nil
}

// GetExternalIPAutoAssignCIDRs retrieves the ExternalIPAutoAssignCIDRs, if configured.
func GetExternalIPAutoAssignCIDRs(lister configlistersv1.NetworkLister, recorder events.Recorder) ([]string, error) {
	network, err := lister.Get("cluster")
	if errors.IsNotFound(err) {
		recorder.Warningf("GetExternalIPAutoAssignCIDRsFailed", "Required networks.%s/cluster not found", configv1.GroupName)
		return nil, nil
	}
	if err != nil {
		recorder.Warningf("GetExternalIPAutoAssignCIDRsFailed", "error getting networks.%s/cluster: %v", configv1.GroupName, err)
		return nil, err
	}

	if network.Spec.ExternalIP == nil {
		return nil, nil
	}

	// ensure all ips are valid
	if err := validateCIDRs(network.Spec.ExternalIP.AutoAssignCIDRs); err != nil {
		recorder.Warningf("GetExternalIPAutoAssignCIDRsFailed", "error parsing networks.%s/cluster Spec.ExternalIP.AutoAssignCIDRs: invalid cidr: %v", configv1.GroupName, err)
		return nil, err
	}

	return network.Spec.ExternalIP.AutoAssignCIDRs, nil
}

// validateCIDRs returns an err if any cidr in the list is invalid
func validateCIDRs(in []string) error {
	for _, cidr := range in {
		_, _, err := net.ParseCIDR(cidr)
		if err != nil {
			return err
		}
	}
	return nil
}
