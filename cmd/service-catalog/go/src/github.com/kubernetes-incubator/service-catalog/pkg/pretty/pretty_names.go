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
