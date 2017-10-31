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

package pretty

import (
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPrettyNames(t *testing.T) {
	e := `ServiceInstance (K8S: "k8s" ExternalName: "extern")`
	g := Name(ServiceInstance, "k8s", "extern")
	if g != e {
		t.Fatalf("Unexpected value of PrettyName String; expected %v, got %v", e, g)
	}
}

func TestServiceInstanceName(t *testing.T) {
	e := `ServiceInstance "namespace/name"`
	serviceInstance := &v1beta1.ServiceInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "name", Namespace: "namespace"},
	}
	g := ServiceInstanceName(serviceInstance)
	if g != e {
		t.Fatalf("Unexpected value of PrettyName String; expected %v, got %v", e, g)
	}
}

func TestClusterServiceBrokerName(t *testing.T) {
	e := `ClusterServiceBroker "brokerName"`
	g := ClusterServiceBrokerName("brokerName")
	if g != e {
		t.Fatalf("Unexpected value of PrettyName String; expected %v, got %v", e, g)
	}
}

func TestClusterServiceClassName(t *testing.T) {
	serviceClass := &v1beta1.ClusterServiceClass{
		ObjectMeta: metav1.ObjectMeta{Name: "service-class"},
		Spec: v1beta1.ClusterServiceClassSpec{
			ExternalName: "external-class-name",
		},
	}
	e := `ClusterServiceClass (K8S: "service-class" ExternalName: "external-class-name")`
	g := ClusterServiceClassName(serviceClass)
	if g != e {
		t.Fatalf("Unexpected value of PrettyName String; expected %v, got %v", e, g)
	}
}

func TestClusterServicePlanName(t *testing.T) {
	servicePlan := &v1beta1.ClusterServicePlan{
		ObjectMeta: metav1.ObjectMeta{Name: "service-plan"},
		Spec: v1beta1.ClusterServicePlanSpec{
			ExternalName: "external-plan-name",
		},
	}

	e := `ClusterServicePlan (K8S: "service-plan" ExternalName: "external-plan-name")`
	g := ClusterServicePlanName(servicePlan)
	if g != e {
		t.Fatalf("Unexpected value of PrettyName String; expected %v, got %v", e, g)
	}
}
