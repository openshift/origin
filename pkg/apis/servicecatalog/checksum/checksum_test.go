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

package checksum

import (
	"testing"

	"k8s.io/client-go/pkg/api/v1"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/unversioned"
	checksumv1alpha1 "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/versioned/v1alpha1"
	_ "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/install"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
)

func TestInstanceSpecChecksum(t *testing.T) {
	spec := servicecatalog.InstanceSpec{
		ServiceClassName: "blorb",
		PlanName:         "plumbus",
		ExternalID:       "ea6d2fc8-0bb8-11e7-af5d-0242ac110005",
	}

	unversionedChecksum := unversioned.InstanceSpecChecksum(spec)

	versionedSpec := v1alpha1.InstanceSpec{}
	v1alpha1.Convert_servicecatalog_InstanceSpec_To_v1alpha1_InstanceSpec(&spec, &versionedSpec, nil /* conversionScope */)
	versionedChecksum := checksumv1alpha1.InstanceSpecChecksum(versionedSpec)

	if e, a := unversionedChecksum, versionedChecksum; e != a {
		t.Fatalf("versioned and unversioned checksums should match; expected %v, got %v", e, a)
	}
}

// TestBindingChecksum tests that an internal and v1alpha1 checksum of the same object are equivalent
func TestBindingSpecChecksum(t *testing.T) {
	spec := servicecatalog.BindingSpec{
		InstanceRef: v1.LocalObjectReference{Name: "test-instance"},
		SecretName:  "test-secret",
		ExternalID:  "1995a7e6-d078-4ce6-9057-bcefd793634e",
	}

	unversionedChecksum := unversioned.BindingSpecChecksum(spec)

	versionedSpec := v1alpha1.BindingSpec{}
	v1alpha1.Convert_servicecatalog_BindingSpec_To_v1alpha1_BindingSpec(&spec, &versionedSpec, nil /* conversionScope */)
	versionedChecksum := checksumv1alpha1.BindingSpecChecksum(versionedSpec)

	if e, a := unversionedChecksum, versionedChecksum; e != a {
		t.Fatalf("versioned and unversioned checksums should match; expected %v, got %v", e, a)
	}
}

// TestBrokerSpecChecksum tests than an internal and v1alpha1 checksum of the same object are equivalent
func TestBrokerSpecChecksum(t *testing.T) {
	spec := servicecatalog.BrokerSpec{
		URL: "https://kubernetes.default.svc:443/brokers/template.k8s.io",
		AuthInfo: &servicecatalog.BrokerAuthInfo{
			Basic: &servicecatalog.BasicAuthConfig{
				SecretRef: &v1.ObjectReference{
					Namespace: "test-ns",
					Name:      "test-secret",
				},
			},
		},
	}

	unversionedChecksum := unversioned.BrokerSpecChecksum(spec)
	versionedSpec := v1alpha1.BrokerSpec{}
	v1alpha1.Convert_servicecatalog_BrokerSpec_To_v1alpha1_BrokerSpec(&spec, &versionedSpec, nil /* conversionScope */)
	versionedChecksum := checksumv1alpha1.BrokerSpecChecksum(versionedSpec)

	if e, a := unversionedChecksum, versionedChecksum; e != a {
		t.Fatalf("versioned and unversioned checksums should match; expected %v, got %v", e, a)
	}
}
