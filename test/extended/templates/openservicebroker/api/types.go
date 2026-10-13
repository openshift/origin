package api

// from https://github.com/openservicebrokerapi/servicebroker/blob/b33070e07cea53b9540b1d5ab8d1f62ddc556eb5/spec.md
// and https://github.com/openservicebrokerapi/servicebroker/blob/b33070e07cea53b9540b1d5ab8d1f62ddc556eb5/profile.md
// and https://github.com/avade/servicebroker/blob/9165f14ce6c54c3f81fba760cca53bd781febd6f/spec.md

import (
	jsschema "github.com/lestrrat/go-jsschema"
	"k8s.io/apiserver/pkg/authentication/user"
)

const (
	XBrokerAPIVersion                   = "X-Broker-Api-Version"
	APIVersion                          = "2.11"
	XBrokerAPIOriginatingIdentity       = "X-Broker-API-Originating-Identity"
	OriginatingIdentitySchemeKubernetes = "kubernetes"
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
	ServiceInstance ServiceInstances `json:"service_instance,omitempty"`
	ServiceBinding  ServiceBindings  `json:"service_binding,omitempty"`
}

type ServiceInstances struct {
	Create map[string]*jsschema.Schema `json:"create,omitempty"`
	Update map[string]*jsschema.Schema `json:"update,omitempty"`
}

type ServiceBindings struct {
	Create map[string]*jsschema.Schema `json:"create,omitempty"`
}

// ParameterSchemas is an OpenShift extension to allow for form building
// for provision/bind parameters.
type ParameterSchemas struct {
	ServiceInstance ParameterSchema `json:"service_instance,omitempty"`
}

type ParameterSchema struct {
	Create OpenShiftMetadata `json:"create,omitempty"`
}

type OpenShiftMetadata struct {
	OpenShiftFormDefinition []string `json:"openshift_form_definition,omitempty"`
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
	ServiceID      string            `json:"service_id"`
	PlanID         string            `json:"plan_id"`
	Context        KubernetesContext `json:"context,omitempty"`
	OrganizationID string            `json:"organization_guid"`
	SpaceID        string            `json:"space_guid"`
	Parameters     map[string]string `json:"parameters,omitempty"`
}

type KubernetesContext struct {
	Platform  string `json:"platform"`
	Namespace string `json:"namespace"`
}

const ContextPlatformKubernetes = "kubernetes"

type ProvisionResponse struct {
	DashboardURL string    `json:"dashboard_url,omitempty"`
	Operation    Operation `json:"operation,omitempty"`
}

type Operation string

type UpdateRequest struct {
	Context        KubernetesContext `json:"context,omitempty"`
	ServiceID      string            `json:"service_id"`
	PlanID         string            `json:"plan_id,omitempty"`
	Parameters     map[string]string `json:"parameters,omitempty"`
	PreviousValues struct {
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
	Error       string `json:"error,omitempty"`
	Description string `json:"description"`
}

var AsyncRequired = ErrorResponse{
	Error:       "AsyncRequired",
	Description: "This request requires client support for asynchronous service operations.",
}

var ConcurrencyError = ErrorResponse{
	Error:       "ConcurrencyError",
	Description: "Another operation for this Service Instance is in progress.",
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
	Provision(u user.Info, instanceID string, preq *ProvisionRequest) *Response
	Deprovision(u user.Info, instanceID string) *Response
	Bind(u user.Info, instanceID string, bindingID string, breq *BindRequest) *Response
	Unbind(u user.Info, instanceID string, bindingID string) *Response
	LastOperation(u user.Info, instanceID string, operation Operation) *Response
}

const (
	OperationProvisioning   Operation = "provisioning"
	OperationUpdating       Operation = "updating"
	OperationDeprovisioning Operation = "deprovisioning"
)

// OpenServiceBrokerInstanceExternalID is a common, optional annotation that stores the OSBAPI instance (UU)ID associated with an object.
const OpenServiceBrokerInstanceExternalID = "openservicebroker.openshift.io/instance-external-id"
