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
	"k8s.io/client-go/pkg/api/v1"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog"
)

func TestValidateServiceBroker(t *testing.T) {
	cases := []struct {
		name   string
		broker *servicecatalog.ServiceBroker
		valid  bool
	}{
		{
			// covers the case where there is no AuthInfo field specified. the validator should
			// ignore the field and still succeed the validation
			name: "valid broker - no auth secret",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL:            "http://example.com",
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: true,
		},
		{
			name: "valid broker - basic auth - secret",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Basic: &servicecatalog.BasicAuthConfig{
							SecretRef: &v1.ObjectReference{
								Namespace: "test-ns",
								Name:      "test-secret",
							},
						},
					},
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: true,
		},
		{
			name: "valid broker - bearer auth - secret",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Bearer: &servicecatalog.BearerTokenAuthConfig{
							SecretRef: &v1.ObjectReference{
								Namespace: "test-ns",
								Name:      "test-secret",
							},
						},
					},
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: true,
		},
		{
			name: "invalid broker - broker with namespace",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-broker",
					Namespace: "oops",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL:            "http://example.com",
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - basic auth - secret missing namespace",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Basic: &servicecatalog.BasicAuthConfig{
							SecretRef: &v1.ObjectReference{
								Name: "test-secret",
							},
						},
					},
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - basic auth - secret missing name",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Basic: &servicecatalog.BasicAuthConfig{
							SecretRef: &v1.ObjectReference{
								Namespace: "test-ns",
							},
						},
					},
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - bearer auth - secret missing namespace",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Bearer: &servicecatalog.BearerTokenAuthConfig{
							SecretRef: &v1.ObjectReference{
								Name: "test-secret",
							},
						},
					},
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - bearer auth - secret missing name",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL: "http://example.com",
					AuthInfo: &servicecatalog.ServiceBrokerAuthInfo{
						Bearer: &servicecatalog.BearerTokenAuthConfig{
							SecretRef: &v1.ObjectReference{
								Namespace: "test-ns",
							},
						},
					},
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - CABundle present with InsecureSkipTLSVerify",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL: "http://example.com",
					InsecureSkipTLSVerify: true,
					CABundle:              []byte("fake CABundle"),
					RelistBehavior:        servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration:        &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: false,
		},
		{
			name: "valid broker - InsecureSkipTLSVerify without CABundle",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL: "http://example.com",
					InsecureSkipTLSVerify: true,
					RelistBehavior:        servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration:        &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: true,
		},
		{
			name: "valid broker - CABundle without InsecureSkipTLSVerify",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL:            "http://example.com",
					CABundle:       []byte("fake CABundle"),
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: true,
		},
		{
			name: "invalid broker - manual behavior with RelistDuration",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL:            "http://example.com",
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorManual,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
				},
			},
			valid: false,
		},
		{
			name: "valid broker - manual behavior without RelistDuration",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL:            "http://example.com",
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorManual,
					RelistDuration: nil,
				},
			},
			valid: true,
		},
		{
			name: "invalid broker - duration behavior without duration",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL:            "http://example.com",
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: nil,
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - relistBehavior is invalid",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL:            "http://example.com",
					RelistBehavior: "Junk",
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - relistBehavior is empty",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL:            "http://example.com",
					RelistBehavior: "",
				},
			},
			valid: false,
		},
		{
			name: "invalid broker - negative relistRequests value",
			broker: &servicecatalog.ServiceBroker{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-broker",
				},
				Spec: servicecatalog.ServiceBrokerSpec{
					URL:            "http://example.com",
					RelistBehavior: servicecatalog.ServiceBrokerRelistBehaviorDuration,
					RelistDuration: &metav1.Duration{Duration: 15 * time.Minute},
					RelistRequests: -1,
				},
			},
			valid: false,
		},
	}

	for _, tc := range cases {
		errs := ValidateServiceBroker(tc.broker)
		if len(errs) != 0 && tc.valid {
			t.Errorf("%v: unexpected error: %v", tc.name, errs)
			continue
		} else if len(errs) == 0 && !tc.valid {
			t.Errorf("%v: unexpected success", tc.name)
		}
	}
}
