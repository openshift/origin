/*
Copyright 2016 The Kubernetes Authors.

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

package v1beta1

import (
	"github.com/satori/go.uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"time"
)

func addDefaultingFuncs(scheme *runtime.Scheme) error {
	return RegisterDefaults(scheme)
}

func SetDefaults_ClusterServiceBrokerSpec(spec *ClusterServiceBrokerSpec) {
	if spec.RelistBehavior == "" {
		spec.RelistBehavior = ServiceBrokerRelistBehaviorDuration
	}

	if spec.RelistBehavior == ServiceBrokerRelistBehaviorDuration && spec.RelistDuration == nil {
		spec.RelistDuration = &metav1.Duration{Duration: 15 * time.Minute}
	}
}

func SetDefaults_ServiceInstanceSpec(spec *ServiceInstanceSpec) {
	if spec.ExternalID == "" {
		spec.ExternalID = uuid.NewV4().String()
	}
}

func SetDefaults_ServiceBindingSpec(spec *ServiceBindingSpec) {
	if spec.ExternalID == "" {
		spec.ExternalID = uuid.NewV4().String()
	}
}

func SetDefaults_ServiceBinding(binding *ServiceBinding) {
	// If not specified, make the SecretName default to the binding name
	if binding.Spec.SecretName == "" {
		binding.Spec.SecretName = binding.Name
	}
}
