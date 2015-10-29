/*
Copyright 2015 The Kubernetes Authors All rights reserved.

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

package api_test

import (
	"io/ioutil"
	"strings"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/registry/namespace"
	"k8s.io/kubernetes/pkg/util/sets"
)

func BenchmarkPodConversion(b *testing.B) {
	data, err := ioutil.ReadFile("pod_example.json")
	if err != nil {
		b.Fatalf("Unexpected error while reading file: %v", err)
	}
	var pod api.Pod
	if err := api.Scheme.DecodeInto(data, &pod); err != nil {
		b.Fatalf("Unexpected error decoding pod: %v", err)
	}

	scheme := api.Scheme.Raw()
	var result *api.Pod
	for i := 0; i < b.N; i++ {
		versionedObj, err := scheme.ConvertToVersion(&pod, testapi.Default.Version())
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		obj, err := scheme.ConvertToVersion(versionedObj, scheme.InternalVersion)
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		result = obj.(*api.Pod)
	}
	if !api.Semantic.DeepDerivative(pod, *result) {
		b.Fatalf("Incorrect conversion: expected %v, got %v", pod, *result)
	}
}

func BenchmarkNodeConversion(b *testing.B) {
	data, err := ioutil.ReadFile("node_example.json")
	if err != nil {
		b.Fatalf("Unexpected error while reading file: %v", err)
	}
	var node api.Node
	if err := api.Scheme.DecodeInto(data, &node); err != nil {
		b.Fatalf("Unexpected error decoding node: %v", err)
	}

	scheme := api.Scheme.Raw()
	var result *api.Node
	for i := 0; i < b.N; i++ {
		versionedObj, err := scheme.ConvertToVersion(&node, testapi.Default.Version())
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		obj, err := scheme.ConvertToVersion(versionedObj, scheme.InternalVersion)
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		result = obj.(*api.Node)
	}
	if !api.Semantic.DeepDerivative(node, *result) {
		b.Fatalf("Incorrect conversion: expected %v, got %v", node, *result)
	}
}

func BenchmarkReplicationControllerConversion(b *testing.B) {
	data, err := ioutil.ReadFile("replication_controller_example.json")
	if err != nil {
		b.Fatalf("Unexpected error while reading file: %v", err)
	}
	var replicationController api.ReplicationController
	if err := api.Scheme.DecodeInto(data, &replicationController); err != nil {
		b.Fatalf("Unexpected error decoding node: %v", err)
	}

	scheme := api.Scheme.Raw()
	var result *api.ReplicationController
	for i := 0; i < b.N; i++ {
		versionedObj, err := scheme.ConvertToVersion(&replicationController, testapi.Default.Version())
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		obj, err := scheme.ConvertToVersion(versionedObj, scheme.InternalVersion)
		if err != nil {
			b.Fatalf("Conversion error: %v", err)
		}
		result = obj.(*api.ReplicationController)
	}
	if !api.Semantic.DeepDerivative(replicationController, *result) {
		b.Fatalf("Incorrect conversion: expected %v, got %v", replicationController, *result)
	}
}

func TestNamespaceFieldLabelConversion(t *testing.T) {
	ns := &api.Namespace{
		ObjectMeta: api.ObjectMeta{
			Name: "ns",
		},
		Status: api.NamespaceStatus{
			Phase: api.NamespaceActive,
		},
	}

	testCases := []struct {
		label      string
		value      string
		shouldFail bool
	}{
		// good cases
		{"metadata.name", "ns", false},
		{"name", "ns", false},
		{"status.phase", string(api.NamespaceActive), false},
		// bad cases
		{".name", "", true},
		{"metadata", "", true},
		{"phase", "", true},
		{"status", "", true},
	}

	labels := namespace.NamespaceToSelectableFields(ns)
	lSet := sets.NewString()
	for l := range labels {
		lSet.Insert(l)
	}

	for _, apiVersion := range []string{"v1", "v1beta3"} {
		for _, ts := range testCases {
			newLabel, newValue, err := api.Scheme.ConvertFieldLabel(apiVersion, "Namespace", ts.label, ts.value)
			if ts.shouldFail && err == nil {
				t.Errorf("%s %s: got unexpected non-error", apiVersion, ts.label)
			} else if !ts.shouldFail && err != nil {
				t.Errorf("%s %s: got unexpected error: %v", apiVersion, ts.label, err)
			} else if !ts.shouldFail {
				if newLabel != ts.label {
					t.Errorf("%s %s: got unexpected label name %q", apiVersion, ts.label, newLabel)
				}
				if newValue != ts.value {
					t.Errorf("%s %s: got unexpected new value (%q != %q)", apiVersion, ts.label, newValue, ts.value)
				}
			}
			lSet.Delete(ts.label)
		}

		if len(lSet) > 0 {
			t.Errorf("%s untested fields: %s", apiVersion, strings.Join(lSet.List(), ", "))
		}
	}
}
