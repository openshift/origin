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
	"encoding/hex"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/davecgh/go-spew/spew"
	proto "github.com/golang/protobuf/proto"

	"github.com/kubernetes-incubator/service-catalog/pkg/api"
	testapi "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/testapi"
	apitesting "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/testing"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"k8s.io/apimachinery/pkg/api/testing/fuzzer"
	"k8s.io/apimachinery/pkg/api/testing/roundtrip"

	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"
)

func init() {
	testapi.Groups[servicecatalog.GroupName] = serviceCatalogAPIGroup()
}

// BABYNETES: ripped from pkg/api/serialization_test.go

var codecsToTest = []func(version schema.GroupVersion, item runtime.Object) (runtime.Codec, bool, error){
	func(version schema.GroupVersion, item runtime.Object) (runtime.Codec, bool, error) {
		c, err := testapi.GetCodecForObject(item)
		return c, true, err
	},
}

func fuzzInternalObject(t *testing.T, forVersion schema.GroupVersion, item runtime.Object, seed int64) runtime.Object {
	fuzzer.FuzzerFor(apitesting.FuzzerFuncs, rand.NewSource(seed), api.Codecs).Fuzz(item)

	j, err := meta.TypeAccessor(item)
	if err != nil {
		t.Fatalf("Unexpected error %v for %#v", err, item)
	}
	j.SetKind("")
	j.SetAPIVersion("")

	return item
}

func dataAsString(data []byte) string {
	dataString := string(data)
	if !strings.HasPrefix(dataString, "{") {
		dataString = "\n" + hex.Dump(data)
		proto.NewBuffer(make([]byte, 0, 1024)).DebugPrint("decoded object", data)
	}
	return dataString
}

// roundTripSame verifies the same source object is tested in all API versions.
func roundTripSame(t *testing.T, group testapi.TestGroup, item runtime.Object, except ...string) {
	set := sets.NewString(except...)
	seed := rand.Int63()
	fuzzInternalObject(t, group.InternalGroupVersion(), item, seed)

	version := *group.GroupVersion()
	codecs := []runtime.Codec{}
	for _, fn := range codecsToTest {
		codec, ok, err := fn(version, item)
		if err != nil {
			t.Errorf("unable to get codec: %v", err)
			return
		}
		if !ok {
			continue
		}
		codecs = append(codecs, codec)
	}

	t.Logf("version: %v\n", version)
	t.Logf("codec: %#v\n", codecs[0])

	if !set.Has(version.String()) {
		fuzzInternalObject(t, version, item, seed)
		for _, codec := range codecs {
			roundTrip(t, codec, item)
		}
	}
}

func roundTrip(t *testing.T, codec runtime.Codec, item runtime.Object) {
	printer := spew.ConfigState{DisableMethods: true}

	original := item
	item = item.DeepCopyObject()

	name := reflect.TypeOf(item).Elem().Name()
	data, err := runtime.Encode(codec, item)
	if err != nil {
		if runtime.IsNotRegisteredError(err) {
			t.Logf("%v: not registered: %v (%s)", name, err, printer.Sprintf("%#v", item))
		} else {
			t.Errorf("%v: %v (%s)", name, err, printer.Sprintf("%#v", item))
		}
		return
	}

	if !equality.Semantic.DeepEqual(original, item) {
		t.Errorf("0: %v: encode altered the object, diff: %v", name, diff.ObjectReflectDiff(original, item))
		return
	}

	t.Logf("Codec: %+v\n", codec)
	obj2, err := runtime.Decode(codec, data)
	if err != nil {
		t.Errorf("ERROR: %v", err)
		t.Errorf("0: %v: %v\nCodec: %#v\nData: %s\nSource: %#v", name, err, codec, dataAsString(data), printer.Sprintf("%#v", item))
		panic("failed")
	}
	if !equality.Semantic.DeepEqual(original, obj2) {
		t.Errorf("\n1: %v: diff: %v\nCodec: %#v\nSource:\n\n%#v\n\nEncoded:\n\n%s\n\nFinal:\n\n%#v", name, diff.ObjectReflectDiff(item, obj2), codec, printer.Sprintf("%#v", item), dataAsString(data), printer.Sprintf("%#v", obj2))
		return
	}

	obj3 := reflect.New(reflect.TypeOf(item).Elem()).Interface().(runtime.Object)
	if err := runtime.DecodeInto(codec, data, obj3); err != nil {
		t.Errorf("2: %v: %v", name, err)
		return
	}
	if !equality.Semantic.DeepEqual(item, obj3) {
		t.Errorf("3: %v: diff: %v\nCodec: %#v", name, diff.ObjectReflectDiff(item, obj3), codec)
		return
	}
}

func serviceCatalogAPIGroup() testapi.TestGroup {
	// OOPS: didn't register the right group version
	groupVersion, err := schema.ParseGroupVersion("servicecatalog.k8s.io/v1beta1")
	if err != nil {
		panic(fmt.Sprintf("Error parsing groupversion: %v", err))
	}

	externalGroupVersion := schema.GroupVersion{Group: servicecatalog.GroupName,
		Version: api.Registry.GroupOrDie(servicecatalog.GroupName).GroupVersion.Version}

	return testapi.NewTestGroup(
		groupVersion,
		servicecatalog.SchemeGroupVersion,
		api.Scheme.KnownTypes(servicecatalog.SchemeGroupVersion),
		api.Scheme.KnownTypes(externalGroupVersion),
	)
}

// TestSpecificKind round-trips a single specific kind and is intended to help
// debug issues that arise while adding a new API type.
func TestSpecificKind(t *testing.T) {
	group := serviceCatalogAPIGroup()

	for _, kind := range group.InternalTypes() {
		t.Log(kind)
	}

	internalGVK := schema.GroupVersionKind{Group: group.GroupVersion().Group, Version: group.GroupVersion().Version, Kind: "ClusterServiceClass"}
	seed := rand.Int63()
	fuzzer := fuzzer.FuzzerFor(apitesting.FuzzerFuncs, rand.NewSource(seed), api.Codecs)

	roundtrip.RoundTripSpecificKindWithoutProtobuf(t, internalGVK, api.Scheme, api.Codecs, fuzzer, nil)
}

func TestClusterServiceBrokerList(t *testing.T) {
	kind := "ClusterServiceBrokerList"
	item, err := api.Scheme.New(serviceCatalogAPIGroup().InternalGroupVersion().WithKind(kind))
	if err != nil {
		t.Errorf("Couldn't make a %v? %v", kind, err)
		return
	}
	roundTripSame(t, serviceCatalogAPIGroup(), item)
}

// based on pkg/api/testapi
var catalogGroups = map[string]testapi.TestGroup{
	"servicecatalog": serviceCatalogAPIGroup(),
}

// TestRoundTripTypes applies the round-trip test to all round-trippable Kinds
// in all of the API groups registered for test in the testapi package.
func TestRoundTripTypes(t *testing.T) {
	seed := rand.Int63()
	fuzzer := fuzzer.FuzzerFor(apitesting.FuzzerFuncs, rand.NewSource(seed), api.Codecs)
	nonRoundTrippableTypes := map[schema.GroupVersionKind]bool{}
	roundtrip.RoundTripTypesWithoutProtobuf(t, api.Scheme, api.Codecs, fuzzer, nonRoundTrippableTypes)
}

func TestBadJSONRejection(t *testing.T) {
	badJSONMissingKind := []byte(`{ }`)
	if _, err := runtime.Decode(testapi.ServiceCatalog.Codec(), badJSONMissingKind); err == nil {
		t.Errorf("Did not reject despite lack of kind field: %s", badJSONMissingKind)
	}
	badJSONUnknownType := []byte(`{"kind": "bar"}`)
	if _, err1 := runtime.Decode(testapi.ServiceCatalog.Codec(), badJSONUnknownType); err1 == nil {
		t.Errorf("Did not reject despite use of unknown type: %s", badJSONUnknownType)
	}
}
