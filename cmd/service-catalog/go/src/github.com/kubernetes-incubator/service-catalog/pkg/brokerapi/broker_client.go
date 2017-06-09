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

package brokerapi

// BrokerClient defines the interface for interacting with a broker for catalog
// retrieval, service instance management, and service binding management.
//
// The broker client defines functions for the catalog, instance and binding APIs for the
// open service broker API (https://www.openservicebrokerapi.org/, based on the cloud foundry
// service broker API: https://docs.cloudfoundry.org/services/api.html).
//
// Each function accepts and returns parameters that are unique to this package. For example,
// the catalog API function, GetCatalog, returns a *brokerapi.Catalog data type. Most callers
// will need to translate this data type into a kubernetes-native type such as
// a series of (./pkg/apis/servicecatalog).ServiceClass data types in that specific case.
type BrokerClient interface {
	CatalogClient
	InstanceClient
	BindingClient
}

// CatalogClient defines the interface for catalog interaction with a broker.
type CatalogClient interface {
	GetCatalog() (*Catalog, error)
}

// InstanceClient defines the interface for managing service instances with a
// broker.
type InstanceClient interface {
	// TODO: these should return appropriate response objects (https://github.com/kubernetes-incubator/service-catalog/issues/116).

	// CreateServiceInstance creates a service instance in the respective broker.
	CreateServiceInstance(ID string, req *CreateServiceInstanceRequest) (*CreateServiceInstanceResponse, int, error)

	// UpdateServiceInstance updates an existing service instance in the respective
	// broker.
	UpdateServiceInstance(ID string, req *CreateServiceInstanceRequest) (*ServiceInstance, int, error)

	// DeleteServiceInstance deletes an existing service instance in the respective
	// broker.
	DeleteServiceInstance(ID string, req *DeleteServiceInstanceRequest) (*DeleteServiceInstanceResponse, int, error)

	// PollServiceInstance polls a broker for a Service Instance Last Operation
	PollServiceInstance(ID string, req *LastOperationRequest) (*LastOperationResponse, int, error)
}

// BindingClient defines the interface for managing service bindings with a
// broker.
type BindingClient interface {
	// CreateServiceBinding creates a service binding in the respective broker.
	// This method handles all asynchronous request handling.
	CreateServiceBinding(instanceID, bindingID string, req *BindingRequest) (*CreateServiceBindingResponse, error)

	// DeleteServiceBinding deletes an existing service binding in the respective
	// broker. This method handles all asynchronous request handling.
	DeleteServiceBinding(instanceID, bindingID, serviceID, planID string) error
}
