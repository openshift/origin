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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

func TestValidateClusterServiceBroker(t *testing.T) {
	cases := []struct {
		name   string
		broker *servicecatalog.ClusterServiceBroker
		valid  bool
	}{
		{
			// covers the case where there is no AuthInfo field specified. the validator should
			// ignore the field and still succeed the validation
			name: "valid broker - no auth secret",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: true,
		},
		{
			name: "valid broker - basic auth - secret",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Basic: &servicecatalog.ClusterBasicAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Namespace: "test-ns",
								Name:      "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
						URL:            "http://example.com",
					},
				},
			},
			valid: true,
		},
		{
			name: "valid broker - bearer auth - secret",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Bearer: &servicecatalog.ClusterBearerTokenAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Namespace: "test-ns",
								Name:      "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: true,
		},
		{
			name: "invalid broker - broker with namespace",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-broker",
					Namespace: "oops",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - basic auth - secret missing namespace",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Basic: &servicecatalog.ClusterBasicAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Name: "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - basic auth - secret missing name",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Basic: &servicecatalog.ClusterBasicAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Namespace: "test-ns",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - bearer auth - secret missing namespace",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Bearer: &servicecatalog.ClusterBearerTokenAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Name: "test-secret",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - bearer auth - secret missing name",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					AuthInfo: &servicecatalog.ClusterServiceBrokerAuthInfo{
						Bearer: &servicecatalog.ClusterBearerTokenAuthConfig{
							SecretRef: &servicecatalog.ObjectReference{
								Namespace: "test-ns",
							},
						},
					},
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - CABundle present with InsecureSkipTLSVerify",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL: "http://example.com",
						InsecureSkipTLSVerify: true,
						CABundle:              []byte("fake CABundle"),
						RelistBehavior:        servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration:        &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: false,
		},
		{
			name: "valid broker - InsecureSkipTLSVerify without CABundle",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL: "http://example.com",
						InsecureSkipTLSVerify: true,
						RelistBehavior:        servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration:        &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: true,
		},
		{
			name: "valid broker - CABundle without InsecureSkipTLSVerify",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						CABundle:       []byte("fake CABundle"),
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: true,
		},
		{
			name: "invalid broker - manual behavior with RelistDuration",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorManual,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					},
				},
			},
			valid: false,
		},
		{
			name: "valid broker - manual behavior without RelistDuration",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorManual,
						RelistDuration: nil,
					},
				},
			},
			valid: true,
		},
		{
			name: "invalid broker - duration behavior without duration",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: nil,
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - relistBehavior is invalid",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "Junk",
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - relistBehavior is empty",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: "",
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - negative relistRequests value",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
						RelistRequests: -1,
					},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - negative relistDuration value",
			broker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: -15 * time.Minute},
					},
				},
			},
			valid: false,
		},
	}

	for _, tc := range cases {
		errs := ValidateClusterServiceBroker(tc.broker)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}

	updateCases := []struct {
		name      string
		newBroker *servicecatalog.ClusterServiceBroker
		oldBroker *servicecatalog.ClusterServiceBroker
		valid     bool
	}{
		{
			name: "valid broker update - equal relistRequests value",
			newBroker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
						RelistRequests: 1,
					},
				},
			},
			oldBroker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
						RelistRequests: 1,
					},
				},
			},
			valid: true,
		},
		{
			name: "valid broker update - increasing relistRequests value",
			newBroker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
						RelistRequests: 2,
					},
				},
			},
			oldBroker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
						RelistRequests: 1,
					},
				},
			},
			valid: true,
		},
		{
			name: "invalid broker update - nonincreasing relistRequests value",
			newBroker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
						RelistRequests: 1,
					},
				},
			},
			oldBroker: &servicecatalog.ClusterServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ClusterServiceBrokerSpec{
					CommonServiceBrokerSpec: servicecatalog.CommonServiceBrokerSpec{
						URL:            "http://example.com",
						RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
						RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
						RelistRequests: 2,
					},
				},
			},
			valid: false,
		},
	}
	for _, tc := range updateCases {
		errs := ValidateClusterServiceBrokerUpdate(tc.newBroker, tc.oldBroker)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
