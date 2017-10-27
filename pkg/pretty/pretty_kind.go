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

package pretty

// Kind is used for the enum of the Type of object we are building context for.
type Kind int

// Names of Types to use when creating pretty messages
const (
	Unknown Kind = iota
	ClusterServiceBroker
	ClusterServiceClass
	ClusterServicePlan
	ServiceBinding
	ServiceInstance
)

func (k Kind) String() string {
	switch k {
	case ClusterServiceBroker:
		return "ClusterServiceBroker"
	case ClusterServiceClass:
		return "ClusterServiceClass"
	case ClusterServicePlan:
		return "ClusterServicePlan"
	case ServiceBinding:
		return "ServiceBinding"
	case ServiceInstance:
		return "ServiceInstance"
	default:
		return ""
	}
}
