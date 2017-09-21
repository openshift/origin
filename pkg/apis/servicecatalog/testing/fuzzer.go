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
	"math/rand"
	"strconv"
	"testing"
	"time"

	"github.com/google/gofuzz"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/testapi"
	"github.com/satori/go.uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/apis/extensions"
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
		p.Map[c.RandString()] = c.RandString()
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

// FuzzerFor can randomly populate api objects that are destined for version.
func FuzzerFor(t *testing.T, version schema.GroupVersion, src rand.Source) *fuzz.Fuzzer {
	f := fuzz.New().NilChance(.5).NumElements(0, 1)
	if src != nil {
		f.RandSource(src)
	}
	f.Funcs(
		func(j *int, c fuzz.Continue) {
			*j = int(c.Int31())
		},
		func(j **int, c fuzz.Continue) {
			if c.RandBool() {
				i := int(c.Int31())
				*j = &i
			} else {
				*j = nil
			}
		},
		func(j *runtime.TypeMeta, c fuzz.Continue) {
			// We have to customize the randomization of TypeMetas because their
			// APIVersion and Kind must remain blank in memory.
			j.APIVersion = ""
			j.Kind = ""
		},
		func(j *metav1.TypeMeta, c fuzz.Continue) {
			// We have to customize the randomization of TypeMetas because their
			// APIVersion and Kind must remain blank in memory.
			j.APIVersion = ""
			j.Kind = ""
		},
		func(j *api.ObjectMeta, c fuzz.Continue) {
			j.Name = c.RandString()
			j.ResourceVersion = strconv.FormatUint(c.RandUint64(), 10)
			j.SelfLink = c.RandString()
			j.UID = types.UID(c.RandString())
			j.GenerateName = c.RandString()

			var sec, nsec int64
			c.Fuzz(&sec)
			c.Fuzz(&nsec)
			j.CreationTimestamp = metav1.Unix(sec, nsec).Rfc3339Copy()
		},
		func(j *api.ObjectReference, c fuzz.Continue) {
			// We have to customize the randomization of TypeMetas because their
			// APIVersion and Kind must remain blank in memory.
			j.APIVersion = c.RandString()
			j.Kind = c.RandString()
			j.Namespace = c.RandString()
			j.Name = c.RandString()
			j.ResourceVersion = strconv.FormatUint(c.RandUint64(), 10)
			j.FieldPath = c.RandString()
		},
		func(j *metav1.ListMeta, c fuzz.Continue) {
			j.ResourceVersion = strconv.FormatUint(c.RandUint64(), 10)
			j.SelfLink = c.RandString()
		},
		func(j *runtime.Object, c fuzz.Continue) {
			// TODO: uncomment when round trip starts from a versioned object
			if true { //c.RandBool() {
				*j = &runtime.Unknown{
					// We do not set TypeMeta here because it is not carried through a round trip
					Raw:         []byte(`{"apiVersion":"unknown.group/unknown","kind":"Something","someKey":"someValue"}`),
					ContentType: runtime.ContentTypeJSON,
				}
			} else {
				types := []runtime.Object{&api.Pod{}, &api.ReplicationController{}}
				t := types[c.Rand.Intn(len(types))]
				c.Fuzz(t)
				*j = t
			}
		},
		func(r *runtime.RawExtension, c fuzz.Continue) {
			// Pick an arbitrary type and fuzz it
			types := []runtime.Object{&api.Pod{}, &extensions.Deployment{}, &api.Service{}}
			obj := types[c.Rand.Intn(len(types))]
			c.Fuzz(obj)

			// Find a codec for converting the object to raw bytes.  This is necessary for the
			// api version and kind to be correctly set by serialization.
			var codec runtime.Codec
			switch obj.(type) {
			case *api.Pod:
				codec = testapi.Default.Codec()
			case *extensions.Deployment:
				codec = testapi.Extensions.Codec()
			case *api.Service:
				codec = testapi.Default.Codec()
			default:
				t.Errorf("Failed to find codec for object type: %T", obj)
				return
			}

			// Convert the object to raw bytes
			bytes, err := runtime.Encode(codec, obj)
			if err != nil {
				t.Errorf("Failed to encode object: %v", err)
				return
			}

			// Set the bytes field on the RawExtension
			r.Raw = bytes
		},
		func(bs *servicecatalog.ServiceBrokerSpec, c fuzz.Continue) {
			c.FuzzNoCustom(bs)
			bs.RelistBehavior = servicecatalog.ServiceBrokerRelistBehaviorDuration
			bs.RelistDuration = &metav1.Duration{Duration: 15 * time.Minute}
		},
		func(is *servicecatalog.ServiceInstanceSpec, c fuzz.Continue) {
			c.FuzzNoCustom(is)
			is.ExternalID = uuid.NewV4().String()
			parameters, err := createParameter(c)
			if err != nil {
				t.Errorf("Failed to create parameter object: %v", err)
				return
			}
			is.Parameters = parameters
		},
		func(bs *servicecatalog.ServiceInstanceCredentialSpec, c fuzz.Continue) {
			c.FuzzNoCustom(bs)
			bs.ExternalID = uuid.NewV4().String()
			// Don't allow the SecretName to be an empty string because
			// the defaulter for this object (on the server) will set it to
			// a non-empty string, which means the round-trip checking will
			// fail since the checker will look for an empty string.
			for bs.SecretName == "" {
				bs.SecretName = c.RandString()
			}
			parameters, err := createParameter(c)
			if err != nil {
				t.Errorf("Failed to create parameter object: %v", err)
				return
			}
			bs.Parameters = parameters
		},
		func(sc *servicecatalog.ServiceClass, c fuzz.Continue) {
			c.FuzzNoCustom(sc)
			metadata, err := createServiceMetadata(c)
			if err != nil {
				t.Errorf("Failed to create metadata object: %v", err)
				return
			}
			sc.ExternalMetadata = metadata
		},
		func(sp *servicecatalog.ServicePlan, c fuzz.Continue) {
			c.FuzzNoCustom(sp)
			metadata, err := createPlanMetadata(c)
			if err != nil {
				t.Errorf("Failed to create metadata object: %v", err)
				return
			}
			sp.ExternalMetadata = metadata
			sp.ServiceInstanceCredentialCreateParameterSchema = metadata
			sp.ServiceInstanceCreateParameterSchema = metadata
			sp.ServiceInstanceUpdateParameterSchema = metadata
		},
	)
	return f
}
