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
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/extensions/v1beta1"
)

//make sure each of Third Party Resource kinds are built with the correct structure
func TestTPRKinds(t *testing.T) {
	var serviceInstanceSample = v1beta1.ThirdPartyResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ThirdPartyResource",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: withGroupName("instance"),
		},
		Versions: []v1beta1.APIVersion{
			{Name: "v1alpha1"},
		},
	}

	var serviceBrokerSample = v1beta1.ThirdPartyResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ThirdPartyResource",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: withGroupName("broker"),
		},
		Versions: []v1beta1.APIVersion{
			{Name: "v1alpha1"},
		},
	}

	var serviceClassSample = v1beta1.ThirdPartyResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ThirdPartyResource",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: withGroupName("service-class"),
		},
		Versions: []v1beta1.APIVersion{
			{Name: "v1alpha1"},
		},
	}

	var serviceBindingSample = v1beta1.ThirdPartyResource{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ThirdPartyResource",
			APIVersion: "v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: withGroupName("binding"),
		},
		Versions: []v1beta1.APIVersion{
			{Name: "v1alpha1"},
		},
	}

	if !reflect.DeepEqual(serviceInstanceSample, serviceInstanceTPR) {
		t.Errorf("Unexpected Instance TPR structure")
	}

	if !reflect.DeepEqual(serviceBindingSample, serviceBindingTPR) {
		t.Errorf("Unexpected Broker TPR structure")
	}

	if !reflect.DeepEqual(serviceBrokerSample, serviceBrokerTPR) {
		t.Errorf("Unexpected Binding TPR structure")
	}

	if !reflect.DeepEqual(serviceClassSample, serviceClassTPR) {
		t.Errorf("Unexpected Service Class TPR structure")
	}
}
