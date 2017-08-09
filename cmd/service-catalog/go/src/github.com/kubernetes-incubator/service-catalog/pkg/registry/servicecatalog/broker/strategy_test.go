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
	checksum "github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/checksum/unversioned"
)

func brokerWithFalseReadyCondition() *sc.Broker {
	return &sc.Broker{
		Spec: sc.BrokerSpec{
			URL: "https://kubernetes.default.svc:443/brokers/template.k8s.io",
		},
		Status: sc.BrokerStatus{
			Conditions: []sc.BrokerCondition{
				{
					Type:   sc.BrokerConditionReady,
					Status: sc.ConditionFalse,
				},
			},
		},
	}
}

func brokerWithTrueReadyCondition() *sc.Broker {
	return &sc.Broker{
		Spec: sc.BrokerSpec{
			URL: "https://kubernetes.default.svc:443/brokers/template.k8s.io",
		},
		Status: sc.BrokerStatus{
			Conditions: []sc.BrokerCondition{
				{
					Type:   sc.BrokerConditionReady,
					Status: sc.ConditionTrue,
				},
			},
		},
	}
}

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

func TestValidateUpdateStatusPrepareForUpdate(t *testing.T) {
	// The test cases below test PrepareForUpdate, which takes a new broker
	// and an older broker. In PrepareForUpdate if the new broker ready
	// condition is set to true, the status checksum field is set to return
	// the checksum for the spec associated with that broker.
	// The test cases are proving:
	// - if ready condition isn't set, nothing changes
	// - if ready condition isn't set and checksum was previously set,
	//   ensure that the checksum is being copied properly
	// - if ready condition is set (and checksum wasn't previously set),
	//   ensure that the checksum is being calculated properly
	// Note that when transitioning to the ready condition, it does not
	// matter if the checksum was previously set.
	//
	// Anonymous struct fields:
	// name: short description of the test
	// old: the old broker to compare against new
	// newer: the new broker to compare against old
	// shouldChecksum: whether or not the checksum should be calculated and verified
	// checksumShouldBeSet: whether or not checksum field in newer broker should be set
	cases := []struct {
		name                string
		old                 *sc.Broker
		newer               *sc.Broker
		shouldChecksum      bool
		checksumShouldBeSet bool
	}{
		{
			name:                "not ready -> not ready",
			old:                 brokerWithFalseReadyCondition(),
			newer:               brokerWithFalseReadyCondition(),
			shouldChecksum:      false,
			checksumShouldBeSet: false,
		},
		{
			name: "not ready -> not ready, checksum already set",
			old: func() *sc.Broker {
				b := brokerWithFalseReadyCondition()
				cs := "22081-9471-471"
				b.Status.Checksum = &cs
				return b
			}(),
			newer:               brokerWithFalseReadyCondition(),
			shouldChecksum:      false,
			checksumShouldBeSet: true,
		},
		{
			name:           "not ready -> ready",
			old:            brokerWithFalseReadyCondition(),
			newer:          brokerWithTrueReadyCondition(),
			shouldChecksum: true,
		},
	}

	for _, tc := range cases {
		strategy := brokerStatusUpdateStrategy
		strategy.PrepareForUpdate(nil /* api context */, tc.newer, tc.old)

		if tc.shouldChecksum {
			if tc.newer.Status.Checksum == nil {
				t.Errorf("%v: Checksum should have been set", tc.name)
				continue
			}

			if e, a := checksum.BrokerSpecChecksum(tc.newer.Spec), *tc.newer.Status.Checksum; e != a {
				t.Errorf("%v: Checksum was incorrect; expected %v got %v", tc.name, e, a)
			}
		} else if tc.checksumShouldBeSet != (tc.newer.Status.Checksum != nil) {
			t.Errorf("%v: expected checksum to be populated, but was nil", tc.name)
		}
	}
}
