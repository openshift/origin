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

// ServiceInstance represents an instance of a service
type ServiceInstance struct {
	ID               string `json:"id"`
	DashboardURL     string `json:"dashboard_url"`
	InternalID       string `json:"internal_id,omitempty"`
	ServiceID        string `json:"service_id"`
	PlanID           string `json:"plan_id"`
	OrganizationGUID string `json:"organization_guid"`
	SpaceGUID        string `json:"space_guid"`

	LastOperation *LastOperationResponse `json:"last_operation,omitempty"`

	Parameters map[string]interface{} `json:"parameters,omitempty"`
}

// CreateServiceInstanceRequest represents a request to a broker to provision an
// instance of a service
type CreateServiceInstanceRequest struct {
	OrgID             string                 `json:"organization_guid,omitempty"`
	PlanID            string                 `json:"plan_id,omitempty"`
	ServiceID         string                 `json:"service_id,omitempty"`
	SpaceID           string                 `json:"space_guid,omitempty"`
	Parameters        map[string]interface{} `json:"parameters,omitempty"`
	AcceptsIncomplete bool                   `json:"accepts_incomplete,omitempty"`
	ContextProfile    ContextProfile         `json:"context,omitempty"`
}

// ContextProfilePlatformKubernetes is a constant to send when the
// client is representing a kubernetes style ecosystem.
const ContextProfilePlatformKubernetes string = "kubernetes"

// ContextProfile implements the optional OSB field
// https://github.com/duglin/servicebroker/blob/CFisms/context-profiles.md#kubernetes
type ContextProfile struct {
	// Platform is always `kubernetes`
	Platform string `json:"platform,omitempty"`
	// Namespace is the Kubernetes namespace in which the service instance will be visible.
	Namespace string `json:"namespace,omitempty"`
}

// CreateServiceInstanceResponse represents the response from a broker after a
// request to provision an instance of a service
type CreateServiceInstanceResponse struct {
	DashboardURL string `json:"dashboard_url,omitempty"`
	Operation    string `json:"operation,omitempty"`
}

// DeleteServiceInstanceRequest represents a request to a broker to deprovision an
// instance of a service
type DeleteServiceInstanceRequest struct {
	ServiceID         string `json:"service_id"`
	PlanID            string `json:"plan_id"`
	AcceptsIncomplete bool   `json:"accepts_incomplete,omitempty"`
}

// DeleteServiceInstanceResponse represents the response from a broker after a request
// to deprovision an instance of a service
type DeleteServiceInstanceResponse struct {
	Operation string `json:"operation,omitempty"`
}

// LastOperationRequest represents a request to a broker to give the state of the action
// it is completing asynchronously
type LastOperationRequest struct {
	ServiceID string `json:"service_id,omitempty"`
	PlanID    string `json:"plan_id,omitempty"`
	Operation string `json:"operation,omitempty"`
}

// LastOperationResponse represents the broker response with the state of a discrete action
// that the broker is completing asynchronously
type LastOperationResponse struct {
	State       string `json:"state"`
	Description string `json:"description,omitempty"`
}

// Defines the possible states of an asynchronous request to a broker
const (
	StateInProgress = "in progress"
	StateSucceeded  = "succeeded"
	StateFailed     = "failed"
)
