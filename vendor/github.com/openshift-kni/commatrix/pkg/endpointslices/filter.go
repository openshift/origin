package endpointslices

import (
	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
)

// Holds a mapping between a container port and it's corresponding endpointslice Port.
type EndpointPortInfo struct {
	EndpointPort  discoveryv1.EndpointPort
	ContainerPort corev1.ContainerPort
}

// getEndpointSlicePortsFromPod returns the corresponding slice of EndpointPortInfo for the given pod.
func getEndpointSlicePortsFromPod(pod corev1.Pod, endpointPorts []discoveryv1.EndpointPort) []EndpointPortInfo {
	ports := []EndpointPortInfo{}
	for _, endpointPort := range endpointPorts {
		if endpointPort.Port == nil {
			continue
		}

		for _, container := range pod.Spec.Containers {
			for _, containerPort := range container.Ports {
				if containerPort.ContainerPort == *endpointPort.Port {
					ports = append(ports, EndpointPortInfo{EndpointPort: endpointPort, ContainerPort: containerPort})
				}
			}
		}
	}
	return ports
}

// filterHostNetwork checks if the pods behind the endpointSlice are using host ports.
func filterEndpointPortsByPodHostPort(portsInfo []EndpointPortInfo) []discoveryv1.EndpointPort {
	// Assuming all pods in an EndpointSlice are uniformly on host ports or not, we only check the first one.
	filteredPorts := []discoveryv1.EndpointPort{}
	for _, port := range portsInfo {
		if port.ContainerPort.HostPort != 0 {
			filteredPorts = append(filteredPorts, port.EndpointPort)
		}
	}
	return filteredPorts
}

// filterHostNetwork checks if the pods behind the endpointSlice are host network.
func isHostNetworked(pod corev1.Pod) bool {
	// Assuming all pods in an EndpointSlice are uniformly on host network or not, we only check the first one.
	return pod.Spec.HostNetwork
}

// FilterServiceTypes checks if the service behind the endpointSlice is of type LoadBalancer or NodePort.
func isExposedService(service corev1.Service) bool {
	if service.Spec.Type != corev1.ServiceTypeLoadBalancer &&
		service.Spec.Type != corev1.ServiceTypeNodePort {
		return false
	}

	return true
}
