package tls

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// gvr creates a schema.GroupVersionResource with the given fields.
// All fields must be non-empty.
func gvr(group, version, resource string) schema.GroupVersionResource {
	if group == "" {
		panic("gvr: group cannot be empty")
	}
	if version == "" {
		panic("gvr: version cannot be empty")
	}
	if resource == "" {
		panic("gvr: resource cannot be empty")
	}
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
}

// newObservedConfigTarget creates an observedConfigTarget with all required fields.
// This constructor ensures no fields are accidentally omitted when adding new entries.
// All string parameters and servingInfoPath elements must be non-empty.
func newObservedConfigTarget(
	namespace string,
	operatorConfigGVR schema.GroupVersionResource,
	operatorConfigName string,
	servingInfoPath []string,
) observedConfigTarget {
	// Validate all string fields are non-empty
	if namespace == "" {
		panic("observedConfigTarget: namespace cannot be empty")
	}
	if operatorConfigGVR.Group == "" {
		panic("observedConfigTarget: operatorConfigGVR.Group cannot be empty")
	}
	if operatorConfigGVR.Version == "" {
		panic("observedConfigTarget: operatorConfigGVR.Version cannot be empty")
	}
	if operatorConfigGVR.Resource == "" {
		panic("observedConfigTarget: operatorConfigGVR.Resource cannot be empty")
	}
	if operatorConfigName == "" {
		panic("observedConfigTarget: operatorConfigName cannot be empty")
	}
	if len(servingInfoPath) == 0 {
		panic("observedConfigTarget: servingInfoPath cannot be empty")
	}
	for i, segment := range servingInfoPath {
		if segment == "" {
			panic(fmt.Sprintf("observedConfigTarget: servingInfoPath[%d] cannot be empty", i))
		}
	}

	return observedConfigTarget{
		namespace:                  namespace,
		operatorConfigGVR:          operatorConfigGVR,
		operatorConfigName:         operatorConfigName,
		servingInfoPath:            servingInfoPath,
	}
}

// newConfigMapTarget creates a configMapTarget with all required fields.
// This constructor ensures no fields are accidentally omitted when adding new entries.
// All string parameters must be non-empty.
func newConfigMapTarget(
	namespace string,
	configMapName string,
	configMapNamespace string,
	configMapKey string,
) configMapTarget {
	// Validate all string fields are non-empty
	if namespace == "" {
		panic("configMapTarget: namespace cannot be empty")
	}
	if configMapName == "" {
		panic("configMapTarget: configMapName cannot be empty")
	}
	if configMapNamespace == "" {
		panic("configMapTarget: configMapNamespace cannot be empty")
	}
	if configMapKey == "" {
		panic("configMapTarget: configMapKey cannot be empty")
	}

	return configMapTarget{
		namespace:                  namespace,
		configMapName:              configMapName,
		configMapNamespace:         configMapNamespace,
		configMapKey:               configMapKey,
	}
}

// newDeploymentEnvVarTarget creates a deploymentEnvVarTarget with all required fields.
// This constructor ensures no fields are accidentally omitted when adding new entries.
// All string parameters must be non-empty.
func newDeploymentEnvVarTarget(
	namespace string,
	deploymentName string,
	tlsMinVersionEnvVar string,
	cipherSuitesEnvVar string,
) deploymentEnvVarTarget {
	// Validate all string fields are non-empty
	if namespace == "" {
		panic("deploymentEnvVarTarget: namespace cannot be empty")
	}
	if deploymentName == "" {
		panic("deploymentEnvVarTarget: deploymentName cannot be empty")
	}
	if tlsMinVersionEnvVar == "" {
		panic("deploymentEnvVarTarget: tlsMinVersionEnvVar cannot be empty")
	}
	if cipherSuitesEnvVar == "" {
		panic("deploymentEnvVarTarget: cipherSuitesEnvVar cannot be empty")
	}

	return deploymentEnvVarTarget{
		namespace:                  namespace,
		deploymentName:             deploymentName,
		tlsMinVersionEnvVar:        tlsMinVersionEnvVar,
		cipherSuitesEnvVar:         cipherSuitesEnvVar,
	}
}

// newEndpointTarget creates a new endpointTarget with validation.
// Exactly one of deploymentName or podSelector must be provided (mutually exclusive).
// namespace and ports must be non-empty.
func newEndpointTarget(namespace, deploymentName string, podSelector map[string]string, ports []string) endpointTarget {
	// Validate namespace
	if namespace == "" {
		panic("endpointTarget: namespace cannot be empty")
	}

	// Validate ports
	if len(ports) == 0 {
		panic("endpointTarget: ports cannot be empty")
	}
	for i, port := range ports {
		if port == "" {
			panic(fmt.Sprintf("endpointTarget: ports[%d] cannot be empty", i))
		}
	}

	// Validate mutual exclusivity of deploymentName and podSelector
	hasDeployment := deploymentName != ""
	hasSelector := len(podSelector) > 0

	if hasDeployment && hasSelector {
		panic(fmt.Sprintf("endpointTarget: both deploymentName and podSelector provided for namespace %s - they are mutually exclusive", namespace))
	}
	if !hasDeployment && !hasSelector {
		panic(fmt.Sprintf("endpointTarget: neither deploymentName nor podSelector provided for namespace %s - exactly one is required", namespace))
	}

	return endpointTarget{
		namespace:      namespace,
		deploymentName: deploymentName,
		podSelector:    podSelector,
		ports:          ports,
	}
}
