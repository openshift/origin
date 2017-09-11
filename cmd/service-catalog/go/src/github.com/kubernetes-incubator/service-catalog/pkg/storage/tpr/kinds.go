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

package tpr

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	groupName = "servicecatalog.k8s.io"
)

func withGroupName(name string) string {
	return fmt.Sprintf("%s.%s", name, groupName)
}

// Kind represents the kind of a third party resource. This type implements fmt.Stringer
type Kind string

// String is the fmt.Stringer interface implementation
func (k Kind) String() string {
	return string(k)
}

// TPRName returns the lowercase name, suitable for creating third party resources of this kind
func (k Kind) TPRName() string {
	// this code taken from code under
	// https://github.com/kubernetes/community/blob/master/contributors/design-proposals/extending-api.md#expectations-about-third-party-objects
	var result string
	for ix := range k.String() {
		current := rune(k.String()[ix])
		if unicode.IsUpper(current) && ix > 0 {
			result = result + "-"
		}
		result = result + string(unicode.ToLower(current))
	}
	return result
}

// URLName returns the URL-worthy name this TPR kind. Examples:
//
//	Kind("ServiceBroker").URLName() == "servicebrokers"
//	Kind("ServiceClass").URLName() == "serviceclasses"
//	Kind("ServiceInstance").URLName() == "serviceinstances"
//      Kind("ServiceInstanceCredntial").URLName() == "serviceinstancecredentials"
//
// Note that this function is incomplete - it is only guaranteed to properly pluralize our 4
// resource types ("ServiceBroker", "ServiceClass", "ServiceInstance", "ServiceInstanceCredential")
func (k Kind) URLName() string {
	str := k.String()
	strLen := len(str)
	lastChar := str[strLen-1]
	var ret string
	if lastChar == 's' {
		ret = str + "es"
	} else {
		ret = str + "s"
	}
	return strings.ToLower(ret)
}

const (
	// ServiceBrokerKind is the name of a Service Broker resource, a Kubernetes third party resource.
	ServiceBrokerKind Kind = "ServiceBroker"

	// ServiceBrokerListKind is the name of a list of Service Broker resources
	ServiceBrokerListKind Kind = "ServiceBrokerList"

	// ServiceInstanceCredentialKind is the name of a Service Instance
	// Credential resource, a Kubernetes third party resource.
	ServiceInstanceCredentialKind Kind = "ServiceInstanceCredential"

	// ServiceInstanceCredentialListKind is the name for lists of Service
	// Instance Credentials
	ServiceInstanceCredentialListKind Kind = "ServiceInstanceCredentialList"

	// ServiceClassKind is the name of a Service Class resource, a Kubernetes third party resource.
	ServiceClassKind Kind = "ServiceClass"

	// ServiceClassListKind is the name of a list of service class resources
	ServiceClassListKind Kind = "ServiceClassList"

	// ServiceInstanceKind is the name of a Service Instance resource, a Kubernetes third party resource.
	ServiceInstanceKind Kind = "ServiceInstance"

	// ServiceInstanceListKind is the name of a list of service instance resources
	ServiceInstanceListKind Kind = "ServiceInstanceList"
)
