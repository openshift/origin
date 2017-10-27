/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pretty

import (
	"fmt"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
)

// Name prints in the form `<Kind> (K8S: <K8S-Name> ExternalName: <External-Name>)`
// kind is required. k8sName and externalName are optional
func Name(kind Kind, k8sName, externalName string) string {
	s := fmt.Sprintf("%s", kind)
	if k8sName != "" && externalName != "" {
		s += fmt.Sprintf(" (K8S: %q ExternalName: %q)", k8sName, externalName)
	} else if k8sName != "" {
		s += fmt.Sprintf(" (K8S: %q)", k8sName)
	} else if externalName != "" {
		s += fmt.Sprintf(" (ExternalName: %q)", externalName)
	}
	return s
}

// ServiceInstanceName returns a string with the type, namespace and name of an instance.
func ServiceInstanceName(instance *v1beta1.ServiceInstance) string {
	return fmt.Sprintf(`%s "%s/%s"`, ServiceInstance, instance.Namespace, instance.Name)
}

// ClusterServiceBrokerName returns a string with the type and name of a broker
func ClusterServiceBrokerName(brokerName string) string {
	return fmt.Sprintf(`%s %q`, ClusterServiceBroker, brokerName)
}

// ClusterServiceClassName returns a string with the k8s name and external name if available.
func ClusterServiceClassName(serviceClass *v1beta1.ClusterServiceClass) string {
	if serviceClass != nil {
		return Name(ClusterServiceClass, serviceClass.Name, serviceClass.Spec.ExternalName)
	}
	return Name(ClusterServiceClass, "", "")
}

// ClusterServicePlanName returns a string with the k8s name and external name if available.
func ClusterServicePlanName(servicePlan *v1beta1.ClusterServicePlan) string {
	if servicePlan != nil {
		return Name(ClusterServicePlan, servicePlan.Name, servicePlan.Spec.ExternalName)
	}
	return Name(ClusterServicePlan, "", "")
}

// FromServiceInstanceOfClusterServiceClassAtBrokerName returns a string in the form of "%s of %s at %s" to help in logging the full context.
func FromServiceInstanceOfClusterServiceClassAtBrokerName(instance *v1beta1.ServiceInstance, serviceClass *v1beta1.ClusterServiceClass, brokerName string) string {
	return fmt.Sprintf(
		"%s of %s at %s",
		ServiceInstanceName(instance), ClusterServiceClassName(serviceClass), ClusterServiceBrokerName(brokerName),
	)
}
