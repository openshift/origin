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

package testing

import (
	"encoding/json"
	"time"

	"fmt"
	"github.com/google/gofuzz"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"

	"k8s.io/apimachinery/pkg/api/testing/fuzzer"
	genericfuzzer "k8s.io/apimachinery/pkg/apis/meta/fuzzer"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	runtimeserializer "k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/uuid"
)

type serviceMetadata struct {
	DisplayName string `json:"displayName"`
}

type planCost struct {
	Unit string `json:"unit"`
}

type planMetadata struct {
	Costs []planCost `json:"costs"`
}

type parameter struct {
	Value string            `json:"value"`
	Map   map[string]string `json:"map"`
}

func createParameter(c fuzz.Continue) (*runtime.RawExtension, error) {
	p := parameter{Value: c.RandString()}
	p.Map = make(map[string]string)
	for i := 0; i < c.Rand.Intn(10); i++ {
		// Non-random key since JSON key cannot contain special characters
		key := fmt.Sprintf("key%d", i+1)
		p.Map[key] = c.RandString()
	}

	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{Raw: b}, nil
}

func createServiceMetadata(c fuzz.Continue) (*runtime.RawExtension, error) {
	m := serviceMetadata{DisplayName: c.RandString()}

	// TODO: Add more fields once OSB spec materialized
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{Raw: b}, nil
}

func createPlanMetadata(c fuzz.Continue) (*runtime.RawExtension, error) {
	m := planMetadata{}
	for i := 0; i < c.Rand.Intn(10); i++ {
		m.Costs = append(m.Costs, planCost{Unit: c.RandString()})
	}

	// TODO: Add more fields once OSB spec materialized
	b, err := json.Marshal(m)
	if err != nil {
		return nil, err
	}
	return &runtime.RawExtension{Raw: b}, nil
}

// servicecatalogFuncs defines fuzzer funcs for Service Catalog types
func servicecatalogFuncs(codecs runtimeserializer.CodecFactory) []interface{} {
	return []interface{}{
		func(bs *servicecatalog.ClusterServiceBrokerSpec, c fuzz.Continue) {
			c.FuzzNoCustom(bs)
			bs.RelistBehavior = servicecatalog.ServiceBrokerRelistBehaviorDuration
			bs.RelistDuration = &metav1.Duration{Duration: 15 * time.Minute}
		},
		func(is *servicecatalog.ServiceInstanceSpec, c fuzz.Continue) {
			c.FuzzNoCustom(is)
			is.ExternalID = string(uuid.NewUUID())
			parameters, err := createParameter(c)
			if err != nil {
				panic(fmt.Sprintf("Failed to create parameter object: %v", err))
			}
			is.Parameters = parameters
		},
		func(bs *servicecatalog.ServiceBindingSpec, c fuzz.Continue) {
			c.FuzzNoCustom(bs)
			bs.ExternalID = string(uuid.NewUUID())
			// Don't allow the SecretName to be an empty string because
			// the defaulter for this object (on the server) will set it to
			// a non-empty string, which means the round-trip checking will
			// fail since the checker will look for an empty string.
			for bs.SecretName == "" {
				bs.SecretName = c.RandString()
			}
			parameters, err := createParameter(c)
			if err != nil {
				panic(fmt.Sprintf("Failed to create parameter object: %v", err))
			}
			bs.Parameters = parameters
		},
		func(bs *servicecatalog.ServiceInstancePropertiesState, c fuzz.Continue) {
			c.FuzzNoCustom(bs)
			parameters, err := createParameter(c)
			if err != nil {
				panic(fmt.Sprintf("Failed to create parameter object: %v", err))
			}
			bs.Parameters = parameters
		},
		func(bs *servicecatalog.ServiceBindingPropertiesState, c fuzz.Continue) {
			c.FuzzNoCustom(bs)
			parameters, err := createParameter(c)
			if err != nil {
				panic(fmt.Sprintf("Failed to create parameter object: %v", err))
			}
			bs.Parameters = parameters
		},
		func(sc *servicecatalog.ClusterServiceClass, c fuzz.Continue) {
			c.FuzzNoCustom(sc)
			metadata, err := createServiceMetadata(c)
			if err != nil {
				panic(fmt.Sprintf("Failed to create metadata object: %v", err))
			}
			sc.Spec.ExternalMetadata = metadata
		},
		func(sp *servicecatalog.ClusterServicePlan, c fuzz.Continue) {
			c.FuzzNoCustom(sp)
			metadata, err := createPlanMetadata(c)
			if err != nil {
				panic(fmt.Sprintf("Failed to create metadata object: %v", err))
			}
			sp.Spec.ExternalMetadata = metadata
			sp.Spec.ServiceBindingCreateParameterSchema = metadata
			sp.Spec.ServiceInstanceCreateParameterSchema = metadata
			sp.Spec.ServiceInstanceUpdateParameterSchema = metadata
		},
	}
}

// FuzzerFuncs provides merged set of fuzzers from different sources
var FuzzerFuncs = fuzzer.MergeFuzzerFuncs(
	genericfuzzer.Funcs,
	servicecatalogFuncs,
)
