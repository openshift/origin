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

package controller

import (
	"reflect"
	"testing"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	clientgofake "k8s.io/client-go/kubernetes/fake"
)

func TestBuildParameters(t *testing.T) {
	secret := &corev1.Secret{
		Data: map[string][]byte{
			"json-key":   []byte("{ \"json\": true }"),
			"string-key": []byte("textFromSecret"),
		},
	}

	cases := []struct {
		name                                  string
		parametersFrom                        []v1beta1.ParametersFromSource
		parameters                            *runtime.RawExtension
		secret                                *corev1.Secret
		expectedParameters                    map[string]interface{}
		expectedParametersWithSecretsRedacted map[string]interface{}
		shouldSucceed                         bool
	}{
		{
			name: "parameters: basic",
			parameters: &runtime.RawExtension{
				Raw: []byte(`{ "p1": "v1", "p2": "v2" }`),
			},
			expectedParameters: map[string]interface{}{
				"p1": "v1",
				"p2": "v2",
			},
			expectedParametersWithSecretsRedacted: map[string]interface{}{
				"p1": "v1",
				"p2": "v2",
			},
			shouldSucceed: true,
		},
		{
			name: "parameters: invalid JSON",
			parameters: &runtime.RawExtension{
				Raw: []byte("not a JSON"),
			},
			shouldSucceed: false,
		},
		{
			name: "parametersFrom: secretKey with blob",
			parametersFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret",
						Key:  "json-key",
					},
				},
			},
			secret: secret,
			expectedParameters: map[string]interface{}{
				"json": true,
			},
			expectedParametersWithSecretsRedacted: map[string]interface{}{
				"json": "<redacted>",
			},
			shouldSucceed: true,
		},
		{
			name: "parametersFrom: secretKey with invalid blob",
			parametersFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret",
						Key:  "string-key",
					},
				},
			},
			secret:        secret,
			shouldSucceed: false,
		},
		{
			name: "parametersFrom + parameters: normal",
			parametersFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret",
						Key:  "json-key",
					},
				},
			},
			parameters: &runtime.RawExtension{
				Raw: []byte(`{ "p1": "v1" }`),
			},
			secret: secret,
			expectedParameters: map[string]interface{}{
				"json": true,
				"p1":   "v1",
			},
			expectedParametersWithSecretsRedacted: map[string]interface{}{
				"json": "<redacted>",
				"p1":   "v1",
			},
			shouldSucceed: true,
		},
		{
			name: "parametersFrom + parameters: conflict",
			parametersFrom: []v1beta1.ParametersFromSource{
				{
					SecretKeyRef: &v1beta1.SecretKeyReference{
						Name: "secret",
						Key:  "json-key",
					},
				},
			},
			parameters: &runtime.RawExtension{
				Raw: []byte(`{ "json": "v1" }`),
			},
			secret:        secret,
			shouldSucceed: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			testBuildParameters(t, tc.parametersFrom, tc.parameters, tc.secret, tc.expectedParameters, tc.expectedParametersWithSecretsRedacted, tc.shouldSucceed)
		})
	}
}

func testBuildParameters(t *testing.T, parametersFrom []v1beta1.ParametersFromSource, parameters *runtime.RawExtension, secret *corev1.Secret, expected map[string]interface{}, expectedWithSecretsRdacted map[string]interface{}, shouldSucceed bool) {
	// create a fake kube client
	fakeKubeClient := &clientgofake.Clientset{}
	if secret != nil {
		addGetSecretReaction(fakeKubeClient, secret)
	} else {
		addGetSecretNotFoundReaction(fakeKubeClient)
	}

	actual, actualWithSecretsRedacted, err := buildParameters(fakeKubeClient, "test-ns", parametersFrom, parameters)
	if shouldSucceed {
		if err != nil {
			t.Fatalf("Failed to build parameters: %v", err)
		}
		if !reflect.DeepEqual(actual, expected) {
			t.Fatalf("incorrect result: diff \n%v", diff.ObjectGoPrintSideBySide(expected, actual))
		}
		if !reflect.DeepEqual(actualWithSecretsRedacted, expectedWithSecretsRdacted) {
			t.Fatalf("incorrect result with redacted secrets: diff \n%v", diff.ObjectGoPrintSideBySide(expectedWithSecretsRdacted, actualWithSecretsRedacted))
		}
	} else {
		if err == nil {
			t.Fatal("Expected error, but got success")
		}
	}
}

func TestGenerateChecksumOfParameters(t *testing.T) {
	cases := []struct {
		name             string
		oldParams        map[string]interface{}
		newParams        map[string]interface{}
		expectedEquality bool
	}{
		{
			name: "same",
			oldParams: map[string]interface{}{
				"a": "first",
				"b": 2,
			},
			newParams: map[string]interface{}{
				"a": "first",
				"b": 2,
			},
			expectedEquality: true,
		},
		{
			name: "different",
			oldParams: map[string]interface{}{
				"a": "first",
				"b": 2,
			},
			newParams: map[string]interface{}{
				"a": "first",
				"b": 3,
			},
			expectedEquality: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			oldChecksum, err := generateChecksumOfParameters(tc.oldParams)
			if err != nil {
				t.Fatalf("failed to generate checksum: %v", err)
			}
			newChecksum, err := generateChecksumOfParameters(tc.newParams)
			if err != nil {
				t.Fatalf("failed to generate checksum: %v", err)
			}
			actualEquality := oldChecksum == newChecksum
			if e, a := tc.expectedEquality, actualEquality; e != a {
				expectedCondition := "be equal"
				if !tc.expectedEquality {
					expectedCondition = "not be equal"
				}
				t.Fatalf("expected checksums to %s: old %q, new %q", expectedCondition, oldChecksum, newChecksum)
			}
		})
	}
}
