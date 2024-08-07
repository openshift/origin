package endpointslices

import (
	corev1 "k8s.io/api/core/v1"
)

// filterHostNetwork checks if the pods behind the endpointSlice are host network.
func filterHostNetwork(pod corev1.Pod) bool {
	// Assuming all pods in an EndpointSlice are uniformly on host network or not, we only check the first one.
	return pod.Spec.HostNetwork
}

// FilterServiceTypes checks if the service behind the endpointSlice is of type LoadBalancer or NodePort.
func filterServiceTypes(service corev1.Service) bool {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer &&
		service.Spec.Type != corev1.ServiceTypeNodePort {
		return false
	}

	return true
}
