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

package tpr

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTPRName(t *testing.T) {
	testCases := []struct {
		before string
		after  string
	}{
		{before: "ServiceClass", after: "service-class"},
		{before: "ThisIsAThing", after: "this-is-a-thing"},
		{before: "thisIsAThing", after: "this-is-a-thing"},
		{before: "ServiceInstanceCredential", after: "service-instance-credential"},
		{before: "ThisIsAAThing", after: "this-is-a-a-thing"},
	}
	for _, testCase := range testCases {
		kind := Kind(testCase.before)
		if kind.TPRName() != testCase.after {
			t.Errorf("expected %s, got %s", testCase.after, kind.TPRName())
		}
	}
}

func TestURLName(t *testing.T) {
	testCases := []struct {
		before string
		after  string
	}{
		{before: "ServiceClass", after: "serviceclasses"},
		{before: "ThisIsAThing", after: "thisisathings"},
		{before: "thisIsAThing", after: "thisisathings"},
		{before: "ServiceInstanceCredential", after: "serviceinstancecredentials"},
	}

	for _, testCase := range testCases {
		kind := Kind(testCase.before)
		if kind.URLName() != testCase.after {
			t.Errorf("expected %s, got %s", testCase.after, kind.URLName())
		}
	}
}

func newTypeMeta(kind Kind) metav1.TypeMeta {
	return metav1.TypeMeta{Kind: kind.TPRName(), APIVersion: groupName + "/v1alpha1'"}
}
