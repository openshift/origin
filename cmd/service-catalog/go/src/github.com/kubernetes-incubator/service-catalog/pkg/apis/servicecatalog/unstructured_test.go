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

package servicecatalog_test

import (
	"math/rand"
	"reflect"
	"testing"

	"github.com/google/gofuzz"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/testapi"
	sctesting "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/testing"

	"github.com/satori/go.uuid"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/testing/fuzzer"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/json"
)

// doUnstructuredRoundTrip performs the following round-tripping on a fuzzed
// internal api object:
//
// 1. converts the internal object to versioned
// 2. marshals the versioned object
// 3. unmarshals the serialization of (2) into a map[string]interface{}
// 4. marshals the unstructured object in (3)
// 5. unmarshals (4) back into a versioned type
// 6. does semantic equal on (5) and (1)
// 7. uses the default converter to convert (1) directly into a map[string]interface{}
// 8. converts (7) back into a versioned object
// 9. does semantic equal on (1) and (8)
func doUnstructuredRoundTrip(t *testing.T, group testapi.TestGroup, kind string) {
	// We do fuzzing on the internal version of the object, and only then
	// convert to the external version. This is because custom fuzzing
	// functions are only supported for internal objects.
	internalObj, err := api.Scheme.New(group.InternalGroupVersion().WithKind(kind))
	if err != nil {
		t.Fatalf("Couldn't create internal object %v: %v", kind, err)
	}
	seed := rand.Int63()
	fuzzer.FuzzerFor(sctesting.FuzzerFuncs, rand.NewSource(seed), api.Codecs).Funcs(
		// custom fuzzer funcs because RawExtension fields seem to
		// experience some reordering during unstructured roundtripping.
		func(is *servicecatalog.ServiceInstanceSpec, c fuzz.Continue) {
			c.FuzzNoCustom(is)
			is.ExternalID = uuid.NewV4().String()
			is.Parameters = nil
		},
		func(bs *servicecatalog.ServiceBindingSpec, c fuzz.Continue) {
			c.FuzzNoCustom(bs)
			bs.ExternalID = uuid.NewV4().String()
			// Don't allow the SecretName to be an empty string because
			// the defaulter for this object (on the server) will set it to
			// a non-empty string, which means the round-trip checking will
			// fail since the checker will look for an empty string.
			for bs.SecretName == "" {
				bs.SecretName = c.RandString()
			}
			bs.Parameters = nil
		},
		func(ps *servicecatalog.ServiceInstancePropertiesState, c fuzz.Continue) {
			c.FuzzNoCustom(ps)
			ps.Parameters = nil
		},
		func(ps *servicecatalog.ServiceBindingPropertiesState, c fuzz.Continue) {
			c.FuzzNoCustom(ps)
			ps.Parameters = nil
		},
	).Fuzz(internalObj)

	item, err := api.Scheme.New(group.GroupVersion().WithKind(kind))
	if err != nil {
		t.Fatalf("Couldn't create external object %v: %v", kind, err)
	}
	if err := api.Scheme.Convert(internalObj, item, nil); err != nil {
		t.Fatalf("Conversion for %v failed: %v", kind, err)
	}

	data, err := json.Marshal(item)
	if err != nil {
		t.Errorf("Error when marshaling object: %v", err)
		return
	}
	unstr := make(map[string]interface{})
	err = json.Unmarshal(data, &unstr)
	if err != nil {
		t.Errorf("Error when unmarshaling to unstructured: %v", err)
		return
	}

	data, err = json.Marshal(unstr)
	if err != nil {
		t.Errorf("Error when marshaling unstructured: %v", err)
		return
	}
	unmarshalledObj := reflect.New(reflect.TypeOf(item).Elem()).Interface()
	err = json.Unmarshal(data, &unmarshalledObj)
	if err != nil {
		t.Errorf("Error when unmarshaling to object: %v", err)
		return
	}
	if !apiequality.Semantic.DeepEqual(item, unmarshalledObj) {
		t.Errorf("Object changed during JSON operations, diff: %v", diff.ObjectReflectDiff(item, unmarshalledObj))
		return
	}

	newUnstr, err := runtime.DefaultUnstructuredConverter.ToUnstructured(item)
	if err != nil {
		t.Errorf("ToUnstructured failed: %v", err)
		return
	}

	newObj := reflect.New(reflect.TypeOf(item).Elem()).Interface().(runtime.Object)
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newUnstr, newObj)
	if err != nil {
		t.Errorf("FromUnstructured failed: %v", err)
		return
	}

	if !apiequality.Semantic.DeepEqual(item, newObj) {
		t.Errorf("%v: Object changed, diff: %v", kind, diff.ObjectReflectDiff(item, newObj))
	}
}

func TestRoundTripTypesToUnstructured(t *testing.T) {
	for groupKey, group := range catalogGroups {
		for kind := range group.InternalTypes() {
			t.Logf("Testing: %v in %v", kind, groupKey)
			for i := 0; i < 50; i++ {
				doUnstructuredRoundTrip(t, group, kind)
				if t.Failed() {
					break
				}
			}
		}
	}
}
