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

package unversioned

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
)

func TestPodLogsGet(t *testing.T) {
	ns := api.NamespaceDefault
	opts := &api.PodLogOptions{
		Follow:     true,
		Timestamps: true,
	}
	c := &testClient{}

	request, err := c.Setup(t).PodLogs(ns).Get("podName", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if request.verb != "GET" {
		t.Fatalf("unexpected verb %q, expected %q", request.verb, "GET")
	}
	if request.resource != "pods" {
		t.Fatalf("unexpected resource %q, expected %q", request.subresource, "pods")
	}
	if request.subresource != "log" {
		t.Fatalf("unexpected subresource %q, expected %q", request.subresource, "log")
	}
	expected := map[string]string{"container": "", "follow": "true", "previous": "false", "timestamps": "true"}
	for gotKey, gotValue := range request.params {
		if gotValue[0] != expected[gotKey] {
			t.Fatalf("unexpected key-value %s=%s, expected %s=%s", gotKey, gotValue[0], gotKey, expected[gotKey])
		}
	}
}
