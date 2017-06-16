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

package server

import (
	"context"

	"github.com/pivotal-cf/brokerapi"
)

// Handler is a fake implementation oif a brokerapi.ServiceBroker. It's useful as a mock
// because it has pre-canned response values for use in testing, and also keeps track of calls
// made to it. Handler is not concurrency-safe
type Handler struct {
	Catalog []brokerapi.Service
	// Since there are no data passed to catalog calls, this is just the number of calls
	// that were made to the catalog endpoint
	CatalogRequests int

	ProvisionResp      brokerapi.ProvisionedServiceSpec
	ProvisionRespError error
	ProvisionRequests  []ProvisionRequest

	DeprovisionResp     brokerapi.DeprovisionServiceSpec
	DeprovisonRespErr   error
	DeprovisionRequests []DeprovisionRequest

	BindResp     brokerapi.Binding
	BindRespErr  error
	BindRequests []BindRequest

	UnbindRespErr  error
	UnbindRequests []UnbindRequest

	UpdateResp     brokerapi.UpdateServiceSpec
	UpdateRespErr  error
	UpdateRequests []UpdateRequest

	LastOperationResp     brokerapi.LastOperation
	LastOperationRespErr  error
	LastOperationRequests []LastOperationRequest
}

// NewHandler creates a new fake server handler
func NewHandler() *Handler {
	return &Handler{}
}

// Services increments h.CatalogRequests and returns h.Catalog
func (h *Handler) Services(ctx context.Context) []brokerapi.Service {
	h.CatalogRequests++
	return h.Catalog
}

// Provision adds an element to h.ProvisionRequests and returns
// h.ProvisionResp, h.ProvisionRespError
func (h *Handler) Provision(
	ctx context.Context,
	instanceID string,
	details brokerapi.ProvisionDetails,
	asyncAllowed bool,
) (brokerapi.ProvisionedServiceSpec, error) {
	h.ProvisionRequests = append(h.ProvisionRequests, ProvisionRequest{
		InstanceID:   instanceID,
		Details:      details,
		AsyncAllowed: asyncAllowed,
	})
	return h.ProvisionResp, h.ProvisionRespError
}

// Deprovision adds an element to h.DeprovisionRequests and returns
// h.DeprovisionResp, h.DeprovisionRespErr
func (h *Handler) Deprovision(context context.Context, instanceID string, details brokerapi.DeprovisionDetails, asyncAllowed bool) (brokerapi.DeprovisionServiceSpec, error) {
	h.DeprovisionRequests = append(h.DeprovisionRequests, DeprovisionRequest{
		InstanceID: instanceID,
		Details:    details,
	})
	return h.DeprovisionResp, h.DeprovisonRespErr
}

// Bind adds an element to h.BindRequqests and returns h.BindResp, h.BindRespErr
func (h *Handler) Bind(context context.Context, instanceID, bindingID string, details brokerapi.BindDetails) (brokerapi.Binding, error) {
	h.BindRequests = append(h.BindRequests, BindRequest{
		InstanceID: instanceID,
		BindingID:  bindingID,
		Details:    details,
	})
	return h.BindResp, h.BindRespErr
}

// Unbind adds an element to h.UnbindRequests and returns h.UnbindRespErr
func (h *Handler) Unbind(context context.Context, instanceID, bindingID string, details brokerapi.UnbindDetails) error {
	h.UnbindRequests = append(h.UnbindRequests, UnbindRequest{
		InstanceID: instanceID,
		BindingID:  bindingID,
		Details:    details,
	})
	return h.UnbindRespErr
}

// Update adds an element to h.UpdateRequests and returns h.UpdateResp, h.UpdateRespErr
func (h *Handler) Update(context context.Context, instanceID string, details brokerapi.UpdateDetails, asyncAllowed bool) (brokerapi.UpdateServiceSpec, error) {
	h.UpdateRequests = append(h.UpdateRequests, UpdateRequest{
		InstanceID:   instanceID,
		Details:      details,
		AsyncAllowed: asyncAllowed,
	})
	return h.UpdateResp, h.UpdateRespErr
}

// LastOperation adds an element to h.LastOperationRequests and returns
// h.LastOperationResp, h.LastOperationRespErr
func (h *Handler) LastOperation(context context.Context, instanceID, operationData string) (brokerapi.LastOperation, error) {
	h.LastOperationRequests = append(h.LastOperationRequests, LastOperationRequest{
		InstanceID:    instanceID,
		OperationData: operationData,
	})
	return h.LastOperationResp, h.LastOperationRespErr
}
