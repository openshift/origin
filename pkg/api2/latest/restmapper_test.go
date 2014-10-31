/*
Copyright 2014 Google Inc. All rights reserved.

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

package latest

import (
	"testing"
)

func TestDefaultRESTMapperVersionAndKindForResource(t *testing.T) {
	mapper := NewDefaultRESTMapper()
	testCases := map[string]struct {
		Kind, APIVersion string
		Err              bool
	}{
		"po":   {Err: true},
		"pod":  {Kind: "Pod", APIVersion: Version},
		"pods": {Kind: "Pod", APIVersion: Version},

		"replicationcontroller":  {Kind: "ReplicationController", APIVersion: Version},
		"replicationcontrollers": {Kind: "ReplicationController", APIVersion: Version},
		"replicationControllers": {Kind: "ReplicationController", APIVersion: Version},
	}
	for resource, testCase := range testCases {
		v, k, err := mapper.VersionAndKindForResource(resource)
		hasErr := err != nil
		if hasErr != testCase.Err {
			t.Errorf("%s: unexpected error behavior %f: %v", resource, testCase.Err, err)
			continue
		}
		if v != testCase.APIVersion || k != testCase.Kind {
			t.Errorf("%s: unexpected version and kind: %s %s", resource, v, k)
		}
	}
}

func TestDefaultRESTMapperRESTMapping(t *testing.T) {
	mapper := NewDefaultRESTMapper()
	testCases := []struct {
		Kind, APIVersion string

		Resource string
		Version  string
		Err      bool
	}{
		{Kind: "Unknown", APIVersion: "", Err: true},

		{Kind: "Pod", APIVersion: "v1beta1", Resource: "pods"},
		{Kind: "Pod", APIVersion: "v1beta2", Resource: "pods"},
		{Kind: "Pod", APIVersion: "", Resource: "pods", Version: Version},

		{Kind: "ReplicationController", APIVersion: "v1beta1", Resource: "replicationControllers"},

		// TODO: add test for a resource that exists in one version but not another
	}
	for i, testCase := range testCases {
		mapping, err := mapper.RESTMapping(testCase.APIVersion, testCase.Kind)
		hasErr := err != nil
		if hasErr != testCase.Err {
			t.Errorf("%d: unexpected error behavior %f: %v", i, testCase.Err, err)
		}
		if hasErr {
			continue
		}
		if mapping.Resource != testCase.Resource {
			t.Errorf("%d: unexpected resource: %#v", i, mapping)
		}
		version := testCase.Version
		if version == "" {
			version = testCase.APIVersion
		}
		if mapping.APIVersion != version {
			t.Errorf("%d: unexpected version: %#v", i, mapping)
		}
		if mapping.Codec == nil || mapping.MetadataAccessor == nil {
			t.Errorf("%d: missing codec and accessor: %#v", i, mapping)
		}
	}
}
