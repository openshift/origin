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

package validation

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/api/v1"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

func TestValidateBroker(t *testing.T) {
	cases := []struct {
		name   string
		broker *servicecatalog.Broker
		valid  bool
	}{
		{
			// covers the case where there is no AuthInfo field specified. the validator should
			// ignore the field and still succeed the validation
			name: "valid broker - no auth secret",
			broker: &servicecatalog.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.BrokerSpec{
					URL: "http://example.com",
				},
			},
			valid: true,
		},
		{
			name: "valid broker - basic auth - secret",
			broker: &servicecatalog.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.BrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.BrokerAuthInfo{
						Basic: &servicecatalog.BasicAuthConfig{
							SecretRef: &v1.ObjectReference{
								Namespace: "test-ns",
								Name:      "test-secret",
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "valid broker - bearer auth - secret",
			broker: &servicecatalog.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.BrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.BrokerAuthInfo{
						Bearer: &servicecatalog.BearerTokenAuthConfig{
							SecretRef: &v1.ObjectReference{
								Namespace: "test-ns",
								Name:      "test-secret",
							},
						},
					},
				},
			},
			valid: true,
		},
		{
			name: "invalid broker - broker with namespace",
			broker: &servicecatalog.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-broker",
					Namespace: "oops",
				},
				Spec: servicecatalog.BrokerSpec{
					URL: "http://example.com",
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - basic auth - secret missing namespace",
			broker: &servicecatalog.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.BrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.BrokerAuthInfo{
						Basic: &servicecatalog.BasicAuthConfig{
							SecretRef: &v1.ObjectReference{
								Name: "test-secret",
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - basic auth - secret missing name",
			broker: &servicecatalog.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.BrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.BrokerAuthInfo{
						Basic: &servicecatalog.BasicAuthConfig{
							SecretRef: &v1.ObjectReference{
								Namespace: "test-ns",
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - bearer auth - secret missing namespace",
			broker: &servicecatalog.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.BrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.BrokerAuthInfo{
						Bearer: &servicecatalog.BearerTokenAuthConfig{
							SecretRef: &v1.ObjectReference{
								Name: "test-secret",
							},
						},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - bearer auth - secret missing name",
			broker: &servicecatalog.Broker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.BrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.BrokerAuthInfo{
						Bearer: &servicecatalog.BearerTokenAuthConfig{
							SecretRef: &v1.ObjectReference{
								Namespace: "test-ns",
							},
						},
					},
				},
			},
			valid: false,
		},
	}

	for _, tc := range cases {
		errs := ValidateBroker(tc.broker)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
