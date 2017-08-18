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

package unversioned

import (
	"crypto/sha256"
	"fmt"

	"github.com/golang/glog"
	"github.com/kubernetes-incubator/service-catalog/pkg/apis/servicecatalog/v1alpha1"
)

// InstanceSpecChecksum calculates a checksum of the given InstanceSpec based
// on the following fields;
//
// - ServiceClassName
// - PlanName
// - Parameters
// - ParametersFrom
// - ExternalID
func InstanceSpecChecksum(spec v1alpha1.InstanceSpec) string {
	specString := ""
	specString += fmt.Sprintf("serviceClassName: %v\n", spec.ServiceClassName)
	specString += fmt.Sprintf("planName: %v\n", spec.PlanName)

	if spec.Parameters != nil {
		specString += fmt.Sprintf("parameters:\n\n%v\n\n", string(spec.Parameters.Raw))
	}
	if spec.ParametersFrom != nil {
		specString += "parametersFrom: \n"
		for _, p := range spec.ParametersFrom {
			specString += fmt.Sprintf("%v\n", parametersFromChecksum(p))
		}
	}

	specString += fmt.Sprintf("externalID: %v\n", spec.ExternalID)

	sum := sha256.Sum256([]byte(specString))
	return fmt.Sprintf("%x", sum)
}

// BindingSpecChecksum calculates a checksum of the given BindingSpec based on
// the following fields:
//
// - InstanceRef.Name
// - Parameters
// - ExternalID
func BindingSpecChecksum(spec v1alpha1.BindingSpec) string {
	specString := ""
	specString += fmt.Sprintf("instanceRef: %v\n", spec.InstanceRef.Name)

	if spec.Parameters != nil {
		specString += fmt.Sprintf("parameters:\n\n%v\n\n", string(spec.Parameters.Raw))
	}
	if spec.ParametersFrom != nil {
		specString += "parametersFrom: \n"
		for _, p := range spec.ParametersFrom {
			specString += fmt.Sprintf("%v\n", parametersFromChecksum(p))
		}
	}

	specString += fmt.Sprintf("externalID: %v\n", spec.ExternalID)

	sum := sha256.Sum256([]byte(specString))
	return fmt.Sprintf("%x", sum)
}

// BrokerSpecChecksum calculates a sha256 hash for the given BrokerSpec based on
// the following fields:
// - URL
// - AuthInfo (may be nil, but special handling is unnecessary with %v)
func BrokerSpecChecksum(spec v1alpha1.BrokerSpec) string {
	specString := fmt.Sprintf("URL: %v\n", spec.URL)
	specString += fmt.Sprintf("AuthInfo: %v\n", spec.AuthInfo)
	glog.V(5).Infof("specString: %v", specString)
	sum := sha256.Sum256([]byte(specString))
	return fmt.Sprintf("%x", sum)
}

func parametersFromChecksum(parameters v1alpha1.ParametersFromSource) string {
	specString := ""

	if parameters.SecretKeyRef != nil {
		specString += fmt.Sprintf("secretKeyRef: %v[%v]\n", parameters.SecretKeyRef.Name, parameters.SecretKeyRef.Key)
	}

	sum := sha256.Sum256([]byte(specString))
	return fmt.Sprintf("%x", sum)
}
