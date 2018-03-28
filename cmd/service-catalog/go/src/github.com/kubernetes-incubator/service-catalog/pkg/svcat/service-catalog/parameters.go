/*
Copyright 2018 The Kubernetes Authors.

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

package servicecatalog

import (
	"encoding/json"
	"fmt"

	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
)

// BuildParameters converts a map of variable assignments to a byte encoded json document,
// which is what the ServiceCatalog API consumes.
func BuildParameters(params interface{}) *runtime.RawExtension {
	paramsJSON, err := json.Marshal(params)
	if err != nil {
		// This should never be hit because marshalling a map[string]string is pretty safe
		// I'd rather throw a panic then force handling of an error that I don't think is possible.
		panic(fmt.Errorf("unable to marshal the request parameters %v (%s)", params, err))
	}

	return &runtime.RawExtension{Raw: paramsJSON}
}

// BuildParametersFrom converts a map of secrets names to secret keys to the
// type consumed by the ServiceCatalog API.
func BuildParametersFrom(secrets map[string]string) []v1beta1.ParametersFromSource {
	params := make([]v1beta1.ParametersFromSource, 0, len(secrets))

	for secret, key := range secrets {
		param := v1beta1.ParametersFromSource{
			SecretKeyRef: &v1beta1.SecretKeyReference{
				Name: secret,
				Key:  key,
			},
		}

		params = append(params, param)
	}

	return params
}
