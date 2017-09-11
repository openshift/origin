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
	"k8s.io/client-go/dynamic"
)

type errUnsupportedResource struct {
	kind Kind
}

func (e errUnsupportedResource) Error() string {
	return fmt.Sprintf("unsupported resource %s", e.kind)
}

// GetResourceClient returns the *dynamic.ResourceClient for a given resource type
func GetResourceClient(cl *dynamic.Client, kind Kind, namespace string) (*dynamic.ResourceClient, error) {
	switch kind {
	case ServiceInstanceKind, ServiceInstanceListKind:
		return cl.Resource(&ServiceInstanceResource, namespace), nil
	case ServiceInstanceCredentialKind, ServiceInstanceCredentialListKind:
		return cl.Resource(&ServiceInstanceCredentialResource, namespace), nil
	case ServiceBrokerKind, ServiceBrokerListKind:
		return cl.Resource(&ServiceBrokerResource, namespace), nil
	case ServiceClassKind, ServiceClassListKind:
		return cl.Resource(&ServiceClassResource, namespace), nil
	default:
		return nil, errUnsupportedResource{kind: kind}
	}
}
