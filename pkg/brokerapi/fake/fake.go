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

package fake

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/kubernetes-incubator/service-catalog/pkg/brokerapi"
	uuid "github.com/satori/go.uuid"
)

// Client implements a fake (./pkg/brokerapi).BrokerClient. The implementation is 100% in-memory
// and is useful for unit testing. None of the methods in this client are concurrency-safe.
//
// See the (./pkg/controller).TestCreateServiceInstanceHelper unit test for example usage of this
// client
type Client struct {
	*CatalogClient
	*InstanceClient
	*BindingClient
}

// NewClientFunc returns a function suitable for creating a new BrokerClient from a given
// Broker object. The returned function is suitable for passing as a callback to code that
// needs to create clients on-demand
func NewClientFunc(
	catCl *CatalogClient,
	instCl *InstanceClient,
	bindCl *BindingClient,
) func(name, url, username, password string) brokerapi.BrokerClient {
	return func(name, url, username, password string) brokerapi.BrokerClient {
		return &Client{
			CatalogClient:  catCl,
			InstanceClient: instCl,
			BindingClient:  bindCl,
		}
	}
}

// CatalogClient implements a fake CF catalog API client
type CatalogClient struct {
	RetCatalog *brokerapi.Catalog
	RetErr     error
}

// GetCatalog just returns c.RetCatalog and c.RetErr
func (c *CatalogClient) GetCatalog() (*brokerapi.Catalog, error) {
	return c.RetCatalog, c.RetErr
}

// InstanceClient implements a fake CF instance API client
type InstanceClient struct {
	Instances                     map[string]*brokerapi.ServiceInstance
	ResponseCode                  int
	Operation                     string
	DashboardURL                  string
	LastOperationResponse         *brokerapi.LastOperationResponse
	DeleteServiceInstanceResponse *brokerapi.DeleteServiceInstanceResponse
	CreateErr                     error
	UpdateErr                     error
	DeleteErr                     error
	PollErr                       error
}

// NewInstanceClient creates a new empty instance client ready for use
func NewInstanceClient() *InstanceClient {
	return &InstanceClient{
		Instances:    make(map[string]*brokerapi.ServiceInstance),
		ResponseCode: http.StatusOK,
	}
}

// CreateServiceInstance returns i.CreateErr if non-nil. If it is nil, checks if id already exists
// in i.Instances and returns http.StatusConfict and ErrInstanceAlreadyExists if so. If not,
// converts req to a ServiceInstance and adds it to i.Instances
func (i *InstanceClient) CreateServiceInstance(
	id string,
	req *brokerapi.CreateServiceInstanceRequest,
) (*brokerapi.CreateServiceInstanceResponse, int, error) {

	if i.CreateErr != nil {
		return nil, i.ResponseCode, i.CreateErr
	}
	if i.exists(id) {
		return nil, http.StatusConflict, ErrInstanceAlreadyExists
	}
	// context profile and contents should always (optionally) exist.
	if req.ContextProfile.Platform != brokerapi.ContextProfilePlatformKubernetes {
		return nil, i.ResponseCode, errors.New("OSB context profile not set to " + brokerapi.ContextProfilePlatformKubernetes)
	}
	if req.ContextProfile.Namespace == "" {
		return nil, i.ResponseCode, errors.New("missing valid OSB context profile namespace")
	}

	i.Instances[id] = convertInstanceRequest(req)
	resp := &brokerapi.CreateServiceInstanceResponse{}
	if i.Operation != "" {
		resp.Operation = i.Operation
	}
	if i.DashboardURL != "" {
		resp.DashboardURL = i.DashboardURL
	}
	return resp, i.ResponseCode, nil
}

// UpdateServiceInstance returns i.UpdateErr if it was non-nil. Otherwise, returns
// ErrInstanceNotFound if id already exists in i.Instances. If it didn't exist, converts req into
// a ServiceInstance, adds it to i.Instances and returns it
func (i *InstanceClient) UpdateServiceInstance(
	id string,
	req *brokerapi.CreateServiceInstanceRequest,
) (*brokerapi.ServiceInstance, int, error) {

	if i.UpdateErr != nil {
		return nil, i.ResponseCode, i.UpdateErr
	}
	if !i.exists(id) {
		return nil, i.ResponseCode, ErrInstanceNotFound
	}

	i.Instances[id] = convertInstanceRequest(req)
	return i.Instances[id], i.ResponseCode, nil
}

// DeleteServiceInstance returns i.DeleteErr if it was non-nil. Otherwise returns
// ErrInstanceNotFound if id didn't already exist in i.Instances. If it it did already exist,
// removes i.Instances[id] from the map and returns nil
func (i *InstanceClient) DeleteServiceInstance(id string, req *brokerapi.DeleteServiceInstanceRequest) (*brokerapi.DeleteServiceInstanceResponse, int, error) {
	resp := &brokerapi.DeleteServiceInstanceResponse{}
	if i.DeleteServiceInstanceResponse != nil {
		resp = i.DeleteServiceInstanceResponse
	}

	if i.DeleteErr != nil {
		return resp, i.ResponseCode, i.DeleteErr
	}
	if !i.exists(id) {
		return resp, i.ResponseCode, ErrInstanceNotFound
	}
	delete(i.Instances, id)
	return resp, i.ResponseCode, nil
}

// PollServiceInstance returns i.PollErr if it was non-nil. Otherwise returns i.LastOperationResponse
func (i *InstanceClient) PollServiceInstance(ID string, req *brokerapi.LastOperationRequest) (*brokerapi.LastOperationResponse, int, error) {
	if i.PollErr != nil {
		return nil, i.ResponseCode, i.PollErr
	}
	resp := &brokerapi.LastOperationResponse{}
	if i.LastOperationResponse != nil {
		resp = i.LastOperationResponse
	}
	return resp, i.ResponseCode, nil
}

func (i *InstanceClient) exists(id string) bool {
	_, ok := i.Instances[id]
	return ok
}

func convertInstanceRequest(req *brokerapi.CreateServiceInstanceRequest) *brokerapi.ServiceInstance {
	return &brokerapi.ServiceInstance{
		ID:               uuid.NewV4().String(),
		DashboardURL:     "https://github.com/kubernetes-incubator/service-catalog",
		InternalID:       uuid.NewV4().String(),
		ServiceID:        req.ServiceID,
		PlanID:           req.PlanID,
		OrganizationGUID: req.OrgID,
		SpaceGUID:        req.SpaceID,
		LastOperation:    nil,
		Parameters:       req.Parameters,
	}
}

// BindingClient implements a fake CF binding API client
type BindingClient struct {
	Bindings        map[string]*brokerapi.ServiceBinding
	BindingRequests map[string]*brokerapi.BindingRequest
	CreateCreds     brokerapi.Credential
	CreateErr       error
	DeleteErr       error
}

// NewBindingClient creates a new empty binding client, ready for use
func NewBindingClient() *BindingClient {
	return &BindingClient{
		Bindings:        make(map[string]*brokerapi.ServiceBinding),
		BindingRequests: make(map[string]*brokerapi.BindingRequest),
	}
}

// CreateServiceBinding returns b.CreateErr if it was non-nil. Otherwise, returns
// ErrBindingAlreadyExists if the IDs already existed in b.Bindings. If they didn't already exist,
// adds the IDs to b.Bindings and returns a new CreateServiceBindingResponse with b.CreateCreds in
// it
func (b *BindingClient) CreateServiceBinding(
	instanceID,
	bindingID string,
	req *brokerapi.BindingRequest,
) (*brokerapi.CreateServiceBindingResponse, error) {

	if b.CreateErr != nil {
		return nil, b.CreateErr
	}
	if b.exists(instanceID, bindingID) {
		return nil, ErrBindingAlreadyExists
	}

	b.Bindings[BindingsMapKey(instanceID, bindingID)] = convertBindingRequest(req)
	b.BindingRequests[BindingsMapKey(instanceID, bindingID)] = req
	return &brokerapi.CreateServiceBindingResponse{Credentials: b.CreateCreds}, nil
}

// DeleteServiceBinding returns b.DeleteErr if it was non-nil. Otherwise, if the binding associated
// with the given IDs didn't exist, returns ErrBindingNotFound. If it did exist, removes it and
// returns nil
func (b *BindingClient) DeleteServiceBinding(instanceID, bindingID, serviceID, planID string) error {
	if b.DeleteErr != nil {
		return b.DeleteErr
	}
	if !b.exists(instanceID, bindingID) {
		return ErrBindingNotFound
	}

	delete(b.Bindings, BindingsMapKey(instanceID, bindingID))
	return nil
}

func (b *BindingClient) exists(instanceID, bindingID string) bool {
	_, ok := b.Bindings[BindingsMapKey(instanceID, bindingID)]
	return ok
}

func convertBindingRequest(req *brokerapi.BindingRequest) *brokerapi.ServiceBinding {
	return &brokerapi.ServiceBinding{
		ID:            uuid.NewV4().String(),
		ServiceID:     req.ServiceID,
		ServicePlanID: req.PlanID,
		Parameters:    req.Parameters,
		AppID:         req.AppGUID,
	}
}

// BindingsMapKey constructs the key used for bindings given a Service ID and
// a Binding ID
func BindingsMapKey(instanceID, bindingID string) string {
	return fmt.Sprintf("%s:%s", instanceID, bindingID)
}
