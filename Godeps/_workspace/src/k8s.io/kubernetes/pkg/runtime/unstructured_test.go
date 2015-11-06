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

package runtime_test

import (
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/testapi"
	"k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/runtime"
)

func TestDecodeUnstructured(t *testing.T) {
	groupVersionString := testapi.Default.GroupVersion().String()
	rawJson := fmt.Sprintf(`{"kind":"Pod","apiVersion":"%s","metadata":{"name":"test"}}`, groupVersionString)
	pl := &api.List{
		Items: []runtime.Object{
			&api.Pod{ObjectMeta: api.ObjectMeta{Name: "1"}},
			&runtime.Unknown{TypeMeta: runtime.TypeMeta{Kind: "Pod", APIVersion: groupVersionString}, RawJSON: []byte(rawJson)},
			&runtime.Unknown{TypeMeta: runtime.TypeMeta{Kind: "", APIVersion: groupVersionString}, RawJSON: []byte(rawJson)},
			&runtime.Unstructured{TypeMeta: runtime.TypeMeta{Kind: "Foo", APIVersion: "Bar"}, Object: map[string]interface{}{"test": "value"}},
		},
	}
	if errs := runtime.DecodeList(pl.Items, runtime.UnstructuredJSONScheme); len(errs) == 1 {
		t.Fatalf("unexpected error %v", errs)
	}
	if pod, ok := pl.Items[1].(*runtime.Unstructured); !ok || pod.Object["kind"] != "Pod" || pod.Object["metadata"].(map[string]interface{})["name"] != "test" {
		t.Errorf("object not converted: %#v", pl.Items[1])
	}
	if _, ok := pl.Items[2].(*runtime.Unknown); !ok {
		t.Errorf("object should not have been converted: %#v", pl.Items[2])
	}
}

func TestDecodeNumbers(t *testing.T) {

	// Start with a valid pod
	originalJSON := []byte(`{
		"kind":"Pod",
		"apiVersion":"v1",
		"metadata":{"name":"pod","namespace":"foo"},
		"spec":{
			"containers":[{"name":"container","image":"container"}],
			"activeDeadlineSeconds":9223372036854775807
		}
	}`)

	pod := &api.Pod{}

	// Decode with structured codec
	codec, err := testapi.GetCodecForObject(pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	err = codec.DecodeInto(originalJSON, pod)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ensure pod is valid
	if errs := validation.ValidatePod(pod); len(errs) > 0 {
		t.Fatalf("pod should be valid: %v", errs)
	}

	// Round-trip with unstructured codec
	unstructuredObj, err := runtime.UnstructuredJSONScheme.Decode(originalJSON)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	roundtripJSON, err := json.Marshal(unstructuredObj.(*runtime.Unstructured).Object)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Decode with structured codec again
	pod2 := &api.Pod{}
	err = codec.DecodeInto(roundtripJSON, pod2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// ensure pod is still valid
	if errs := validation.ValidatePod(pod2); len(errs) > 0 {
		t.Fatalf("pod should be valid: %v", errs)
	}
	// ensure round-trip preserved large integers
	if !reflect.DeepEqual(pod, pod2) {
		t.Fatalf("Expected\n\t%#v, got \n\t%#v", pod, pod2)
	}
}
