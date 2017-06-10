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

package tpr

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

const (
	tprKind    = "ThirdPartyResource"
	tprVersion = "v1alpha1"
)

// ServiceInstanceResource represents the API resource for the service instance third
// party resource
var ServiceInstanceResource = metav1.APIResource{
	Name:       ServiceInstanceKind.TPRName(),
	Namespaced: true,
}

// ServiceBindingResource represents the API resource for the service binding third
// party resource
var ServiceBindingResource = metav1.APIResource{
	Name:       ServiceBindingKind.TPRName(),
	Namespaced: true,
}

// ServiceBrokerResource represents the API resource for the service broker third
// party resource
var ServiceBrokerResource = metav1.APIResource{
	Name:       ServiceBrokerKind.TPRName(),
	Namespaced: true,
}

// ServiceClassResource represents the API resource for the service class third
// party resource
var ServiceClassResource = metav1.APIResource{
	// ServiceClass is the kind, but TPRName converts it to 'serviceclass'. For now, just hard-code
	// it here
	Name:       "service-class",
	Namespaced: true,
}

// ServiceInstanceResource represents the API resource for the service instance third
// party resource
var serviceInstanceTPR = v1beta1.ThirdPartyResource{
	TypeMeta: metav1.TypeMeta{
		Kind:       tprKind,
		APIVersion: tprVersion,
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: withGroupName(ServiceInstanceKind.TPRName()),
	},
	Versions: []v1beta1.APIVersion{
		{Name: tprVersion},
	},
}

// ServiceBindingResource represents the API resource for the service binding third
// party resource
var serviceBindingTPR = v1beta1.ThirdPartyResource{
	TypeMeta: metav1.TypeMeta{
		Kind:       tprKind,
		APIVersion: tprVersion,
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: withGroupName(ServiceBindingKind.TPRName()),
	},
	Versions: []v1beta1.APIVersion{
		{Name: tprVersion},
	},
}

// ServiceBrokerResource represents the API resource for the service broker third
// party resource
var serviceBrokerTPR = v1beta1.ThirdPartyResource{
	TypeMeta: metav1.TypeMeta{
		Kind:       tprKind,
		APIVersion: tprVersion,
	},
	ObjectMeta: metav1.ObjectMeta{
		Name: withGroupName(ServiceBrokerKind.TPRName()),
	},
	Versions: []v1beta1.APIVersion{
		{Name: tprVersion},
	},
}

// ServiceClassResource represents the API resource for the service class third
// party resource
var serviceClassTPR = v1beta1.ThirdPartyResource{
	TypeMeta: metav1.TypeMeta{
		Kind:       tprKind,
		APIVersion: tprVersion,
	},
	// ServiceClass is the kind, but TPRName converts it to 'serviceclass'. For now, just hard-code
	// it here
	ObjectMeta: metav1.ObjectMeta{
		Name: withGroupName(ServiceClassKind.TPRName()),
	},
	Versions: []v1beta1.APIVersion{
		{Name: tprVersion},
	},
}
