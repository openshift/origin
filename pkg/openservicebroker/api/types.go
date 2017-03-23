package api

// from https://github.com/openservicebrokerapi/servicebroker/blob/1d301105c66187b5aa2e061a1264ecf3cbc3d2a0/_spec.md
// and https://github.com/avade/servicebroker/blob/9ef94ce96ca65bfc9ce482d5ea5be0ad62643a84/_spec.md

import (
	jsschema "github.com/lestrrat/go-jsschema"
)

const (
	XBrokerAPIVersion = "X-Broker-Api-Version"
	APIVersion        = "2.11"
)

type Service struct {
	Name            string                 `json:"name"`
	ID              string                 `json:"id"`
	Description     string                 `json:"description"`
	Tags            []string               `json:"tags,omitempty"`
	Requires        []string               `json:"requires,omitempty"`
	Bindable        bool                   `json:"bindable"`
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
	DashboardClient *DashboardClient       `json:"dashboard_client,omitempty"`
	PlanUpdatable   bool                   `json:"plan_updateable,omitempty"`
	Plans           []Plan                 `json:"plans"`
}

type DashboardClient struct {
	ID          string `json:"id"`
	Secret      string `json:"secret"`
	RedirectURI string `json:"redirect_uri"`
}

type Plan struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	Free        bool                   `json:"free,omitempty"`
	Bindable    bool                   `json:"bindable,omitempty"`
	Schemas     Schema                 `json:"schemas,omitempty"`
}

type Schema struct {
	ServiceInstances ServiceInstances `json:"service-instances,omitempty"`
	ServiceBindings  ServiceBindings  `json:"service-bindings,omitempty"`
}

type ServiceInstances struct {
	Create map[string]*jsschema.Schema `json:"create,omitempty"`
	Update map[string]*jsschema.Schema `json:"update,omitempty"`
}

type ServiceBindings struct {
	Create map[string]*jsschema.Schema `json:"create,omitempty"`
}

type CatalogResponse struct {
	Services []*Service `json:"services"`
}

type LastOperationResponse struct {
	State       LastOperationState `json:"state"`
	Description string             `json:"description,omitempty"`
}

type LastOperationState string

const (
	LastOperationStateInProgress LastOperationState = "in progress"
	LastOperationStateSucceeded  LastOperationState = "succeeded"
	LastOperationStateFailed     LastOperationState = "failed"
)

type ProvisionRequest struct {
	ServiceID         string            `json:"service_id"`
	PlanID            string            `json:"plan_id"`
	Parameters        map[string]string `json:"parameters,omitempty"`
	AcceptsIncomplete bool              `json:"accepts_incomplete,omitempty"`
	OrganizationID    string            `json:"organization_guid"`
	SpaceID           string            `json:"space_guid"`
}

type ProvisionResponse struct {
	DashboardURL string    `json:"dashboard_url,omitempty"`
	Operation    Operation `json:"operation,omitempty"`
}

type Operation string

type UpdateRequest struct {
	ServiceID         string            `json:"service_id"`
	PlanID            string            `json:"plan_id,omitempty"`
	Parameters        map[string]string `json:"parameters,omitempty"`
	AcceptsIncomplete bool              `json:"accepts_incomplete,omitempty"`
	PreviousValues    struct {
		ServiceID      string `json:"service_id,omitempty"`
		PlanID         string `json:"plan_id,omitempty"`
		OrganizationID string `json:"organization_id,omitempty"`
		SpaceID        string `json:"space_id,omitempty"`
	} `json:"previous_values,omitempty"`
}

type UpdateResponse struct {
	Operation Operation `json:"operation,omitempty"`
}

type BindRequest struct {
	ServiceID    string `json:"service_id"`
	PlanID       string `json:"plan_id"`
	AppGUID      string `json:"app_guid,omitempty"`
	BindResource struct {
		AppGUID string `json:"app_guid,omitempty"`
		Route   string `json:"route,omitempty"`
	} `json:"bind_resource,omitempty"`
	Parameters map[string]string `json:"parameters,omitempty"`
}

type BindResponse struct {
	Credentials     map[string]interface{} `json:"credentials,omitempty"`
	SyslogDrainURL  string                 `json:"syslog_drain_url,omitempty"`
	RouteServiceURL string                 `json:"route_service_url,omitempty"`
	VolumeMounts    []interface{}          `json:"volume_mounts,omitempty"`
}

type UnbindResponse struct {
}

type DeprovisionResponse struct {
	Operation Operation `json:"operation,omitempty"`
}

type ErrorResponse struct {
	Description string `json:"description"`
}

// asyncRequiredResponse type is not formally defined in the spec
type asyncRequiredResponse struct {
	Error       string `json:"error,omitempty"`
	Description string `json:"description"`
}

var AsyncRequired = asyncRequiredResponse{
	Error:       "AsyncRequired",
	Description: "This service plan requires client support for asynchronous service operations.",
}

// from http://docs.cloudfoundry.org/services/catalog-metadata.html#services-metadata-fields

const (
	ServiceMetadataDisplayName         = "displayName"
	ServiceMetadataImageURL            = "imageUrl"
	ServiceMetadataLongDescription     = "longDescription"
	ServiceMetadataProviderDisplayName = "providerDisplayName"
	ServiceMetadataDocumentationURL    = "documentationUrl"
	ServiceMetadataSupportURL          = "supportUrl"
)

// the types below are not specified in the openservicebrokerapi spec

type Response struct {
	Code int
	Body interface{}
	Err  error
}

type Broker interface {
	Catalog() *Response
	Provision(instanceID string, preq *ProvisionRequest) *Response
	Deprovision(instanceID string) *Response
	Bind(instanceID string, bindingID string, breq *BindRequest) *Response
	Unbind(instanceID string, bindingID string) *Response
	LastOperation(instanceID string, operation Operation) *Response
}

const (
	OperationProvisioning   Operation = "provisioning"
	OperationUpdating       Operation = "updating"
	OperationDeprovisioning Operation = "deprovisioning"
)
