package onpremhaproxy

import (
	"fmt"

	configv1 "github.com/openshift/api/config/v1"
)

// notSupportedPlatformReason returns an empty string when the cluster runs the OpenShift-managed
// on-prem API loadbalancer (the haproxy static pods scanned by this monitor test), or a
// human-readable reason why it does not.
//
// The haproxy static pods are deployed only on the on-prem platforms scanned by this monitor test
// (BareMetal, OpenStack and vSphere), and only when an API VIP is configured (e.g. vSphere UPI has
// none) and the loadbalancer is not user-managed.
func notSupportedPlatformReason(infra *configv1.Infrastructure) string {
	if infra.Status.PlatformStatus == nil {
		return "platform status is not set, the cluster does not run the on-prem API loadbalancer"
	}

	platformType := infra.Status.PlatformStatus.Type
	hasAPIVIP := false
	loadBalancerType := configv1.LoadBalancerTypeOpenShiftManagedDefault

	switch platformType {
	case configv1.BareMetalPlatformType:
		if status := infra.Status.PlatformStatus.BareMetal; status != nil {
			hasAPIVIP = len(status.APIServerInternalIPs) > 0 || len(status.APIServerInternalIP) > 0
			if status.LoadBalancer != nil && len(status.LoadBalancer.Type) > 0 {
				loadBalancerType = status.LoadBalancer.Type
			}
		}
	case configv1.OpenStackPlatformType:
		if status := infra.Status.PlatformStatus.OpenStack; status != nil {
			hasAPIVIP = len(status.APIServerInternalIPs) > 0 || len(status.APIServerInternalIP) > 0
			if status.LoadBalancer != nil && len(status.LoadBalancer.Type) > 0 {
				loadBalancerType = status.LoadBalancer.Type
			}
		}
	case configv1.VSpherePlatformType:
		if status := infra.Status.PlatformStatus.VSphere; status != nil {
			hasAPIVIP = len(status.APIServerInternalIPs) > 0 || len(status.APIServerInternalIP) > 0
			if status.LoadBalancer != nil && len(status.LoadBalancer.Type) > 0 {
				loadBalancerType = status.LoadBalancer.Type
			}
		}
	default:
		return fmt.Sprintf("platform %q does not use the OpenShift-managed on-prem API loadbalancer", platformType)
	}

	if !hasAPIVIP {
		return fmt.Sprintf("platform %q has no API VIP configured, the OpenShift-managed on-prem API loadbalancer is not deployed", platformType)
	}
	if loadBalancerType != configv1.LoadBalancerTypeOpenShiftManagedDefault {
		return fmt.Sprintf("platform %q uses a %q API loadbalancer, the OpenShift-managed on-prem API loadbalancer is not deployed", platformType, loadBalancerType)
	}

	return ""
}
