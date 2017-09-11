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

package v1alpha1_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/testapi"
	versioned "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/pkg/api"
)

func init() {
	groupVersion, err := schema.ParseGroupVersion("servicecatalog.k8s.io/v1alpha1")
	if err != nil {
		panic(fmt.Sprintf("Error parsing groupversion: %v", err))
	}

	externalGroupVersion := schema.GroupVersion{Group: servicecatalog.GroupName,
		Version: api.Registry.GroupOrDie(servicecatalog.GroupName).GroupVersion.Version}

	testapi.Groups[servicecatalog.GroupName] = testapi.NewTestGroup(
		groupVersion,
		servicecatalog.SchemeGroupVersion,
		api.Scheme.KnownTypes(servicecatalog.SchemeGroupVersion),
		api.Scheme.KnownTypes(externalGroupVersion),
	)
}

func roundTrip(t *testing.T, obj runtime.Object) runtime.Object {
	codec, err := testapi.GetCodecForObject(obj)
	if err != nil {
		t.Fatalf("%v\n %#v", err, obj)
	}
	data, err := runtime.Encode(codec, obj)
	if err != nil {
		t.Fatalf("%v\n %#v", err, obj)
	}
	obj2, err := runtime.Decode(codec, data)
	if err != nil {
		t.Fatalf("%v\nData: %s\nSource: %#v", err, string(data), obj)
	}
	obj3 := reflect.New(reflect.TypeOf(obj).Elem()).Interface().(runtime.Object)
	err = api.Scheme.Convert(obj2, obj3, nil)
	if err != nil {
		t.Fatalf("%v\nSource: %#v", err, obj2)
	}
	return obj3
}

func TestSetDefaultServiceInstance(t *testing.T) {
	i := &versioned.ServiceInstance{}
	obj2 := roundTrip(t, runtime.Object(i))
	i2 := obj2.(*versioned.ServiceInstance)

	if i2.Spec.ExternalID == "" {
		t.Error("Expected a default ExternalID, but got none")
	}
}

func TestSetDefaultServiceInstanceCredential(t *testing.T) {
	b := &versioned.ServiceInstanceCredential{}
	obj2 := roundTrip(t, runtime.Object(b))
	b2 := obj2.(*versioned.ServiceInstanceCredential)

	if b2.Spec.ExternalID == "" {
		t.Error("Expected a default ExternalID, but got none")
	}
}
