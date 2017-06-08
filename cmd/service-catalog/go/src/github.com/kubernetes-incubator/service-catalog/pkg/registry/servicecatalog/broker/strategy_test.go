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

package broker

import (
	"testing"

	sc "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

// TestBrokerStrategyTrivial is the testing of the trivial hardcoded
// boolean flags.
func TestBrokerStrategyTrivial(t *testing.T) {
	if brokerRESTStrategies.NamespaceScoped() {
		t.Errorf("broker create must not be namespace scoped")
	}
	if brokerRESTStrategies.NamespaceScoped() {
		t.Errorf("broker update must not be namespace scoped")
	}
	if brokerRESTStrategies.AllowCreateOnUpdate() {
		t.Errorf("broker should not allow create on update")
	}
	if brokerRESTStrategies.AllowUnconditionalUpdate() {
		t.Errorf("broker should not allow unconditional update")
	}
}

// TestBrokerCreate
func TestBroker(t *testing.T) {
	// Create a broker or brokers
	broker := &sc.Broker{
		Spec: sc.BrokerSpec{
			URL: "abcd",
		},
		Status: sc.BrokerStatus{
			Conditions: nil,
		},
	}

	// Canonicalize the broker
	brokerRESTStrategies.PrepareForCreate(nil, broker)

	if broker.Status.Conditions == nil {
		t.Fatalf("Fresh broker should have empty status")
	}
	if len(broker.Status.Conditions) != 0 {
		t.Fatalf("Fresh broker should have empty status")
	}
}
