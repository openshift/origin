package brokerapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

type ServiceBroker interface {
	Services(ctx context.Context) []Service

	Provision(ctx context.Context, instanceID string, details ProvisionDetails, asyncAllowed bool) (ProvisionedServiceSpec, error)
	Deprovision(ctx context.Context, instanceID string, details DeprovisionDetails, asyncAllowed bool) (DeprovisionServiceSpec, error)

	Bind(ctx context.Context, instanceID, bindingID string, details BindDetails) (Binding, error)
	Unbind(ctx context.Context, instanceID, bindingID string, details UnbindDetails) error

	Update(ctx context.Context, instanceID string, details UpdateDetails, asyncAllowed bool) (UpdateServiceSpec, error)

	LastOperation(ctx context.Context, instanceID, operationData string) (LastOperation, error)
}

type DetailsWithRawParameters interface {
	GetRawParameters() json.RawMessage
}

type DetailsWithRawContext interface {
	GetRawContext() json.RawMessage
}

func (d ProvisionDetails) GetRawContext() json.RawMessage {
	return d.RawContext
}

func (d ProvisionDetails) GetRawParameters() json.RawMessage {
	return d.RawParameters
}

func (d BindDetails) GetRawContext() json.RawMessage {
	return d.RawContext
}

func (d BindDetails) GetRawParameters() json.RawMessage {
	return d.RawParameters
}

func (d UpdateDetails) GetRawParameters() json.RawMessage {
	return d.RawParameters
}

type ProvisionDetails struct {
	ServiceID        string          `json:"service_id"`
	PlanID           string          `json:"plan_id"`
	OrganizationGUID string          `json:"organization_guid"`
	SpaceGUID        string          `json:"space_guid"`
	RawContext       json.RawMessage `json:"context,omitempty"`
	RawParameters    json.RawMessage `json:"parameters,omitempty"`
}

type ProvisionedServiceSpec struct {
	IsAsync       bool
	DashboardURL  string
	OperationData string
}

type BindDetails struct {
	AppGUID       string          `json:"app_guid"`
	PlanID        string          `json:"plan_id"`
	ServiceID     string          `json:"service_id"`
	BindResource  *BindResource   `json:"bind_resource,omitempty"`
	RawContext    json.RawMessage `json:"context,omitempty"`
	RawParameters json.RawMessage `json:"parameters,omitempty"`
}

type BindResource struct {
	AppGuid            string `json:"app_guid,omitempty"`
	Route              string `json:"route,omitempty"`
	CredentialClientID string `json:"credential_client_id,omitempty"`
}

type UnbindDetails struct {
	PlanID    string `json:"plan_id"`
	ServiceID string `json:"service_id"`
}

type UpdateServiceSpec struct {
	IsAsync       bool
	OperationData string
}

type DeprovisionServiceSpec struct {
	IsAsync       bool
	OperationData string
}

type DeprovisionDetails struct {
	PlanID    string `json:"plan_id"`
	ServiceID string `json:"service_id"`
}

type UpdateDetails struct {
	ServiceID      string          `json:"service_id"`
	PlanID         string          `json:"plan_id"`
	RawParameters  json.RawMessage `json:"parameters,omitempty"`
	PreviousValues PreviousValues  `json:"previous_values"`
	RawContext     json.RawMessage `json:"context,omitempty"`
}

type PreviousValues struct {
	PlanID    string `json:"plan_id"`
	ServiceID string `json:"service_id"`
	OrgID     string `json:"organization_id"`
	SpaceID   string `json:"space_id"`
}

type LastOperation struct {
	State       LastOperationState
	Description string
}

type LastOperationState string

const (
	InProgress LastOperationState = "in progress"
	Succeeded  LastOperationState = "succeeded"
	Failed     LastOperationState = "failed"
)

type Binding struct {
	Credentials     interface{}   `json:"credentials"`
	SyslogDrainURL  string        `json:"syslog_drain_url,omitempty"`
	RouteServiceURL string        `json:"route_service_url,omitempty"`
	VolumeMounts    []VolumeMount `json:"volume_mounts,omitempty"`
}

type VolumeMount struct {
	Driver       string       `json:"driver"`
	ContainerDir string       `json:"container_dir"`
	Mode         string       `json:"mode"`
	DeviceType   string       `json:"device_type"`
	Device       SharedDevice `json:"device"`
}

type SharedDevice struct {
	VolumeId    string                 `json:"volume_id"`
	MountConfig map[string]interface{} `json:"mount_config"`
}

const (
	instanceExistsMsg           = "instance already exists"
	instanceDoesntExistMsg      = "instance does not exist"
	serviceLimitReachedMsg      = "instance limit for this service has been reached"
	servicePlanQuotaExceededMsg = "The quota for this service plan has been exceeded. Please contact your Operator for help."
	serviceQuotaExceededMsg     = "The quota for this service has been exceeded. Please contact your Operator for help."
	bindingExistsMsg            = "binding already exists"
	bindingDoesntExistMsg       = "binding does not exist"
	asyncRequiredMsg            = "This service plan requires client support for asynchronous service operations."
	planChangeUnsupportedMsg    = "The requested plan migration cannot be performed"
	rawInvalidParamsMsg         = "The format of the parameters is not valid JSON"
	appGuidMissingMsg           = "app_guid is a required field but was not provided"
)

var (
	ErrInstanceAlreadyExists = NewFailureResponseBuilder(
		errors.New(instanceExistsMsg), http.StatusConflict, instanceAlreadyExistsErrorKey,
	).WithEmptyResponse().Build()

	ErrInstanceDoesNotExist = NewFailureResponseBuilder(
		errors.New(instanceDoesntExistMsg), http.StatusGone, instanceMissingErrorKey,
	).WithEmptyResponse().Build()

	ErrInstanceLimitMet = NewFailureResponse(
		errors.New(serviceLimitReachedMsg), http.StatusInternalServerError, instanceLimitReachedErrorKey,
	)

	ErrBindingAlreadyExists = NewFailureResponse(
		errors.New(bindingExistsMsg), http.StatusConflict, bindingAlreadyExistsErrorKey,
	)

	ErrBindingDoesNotExist = NewFailureResponseBuilder(
		errors.New(bindingDoesntExistMsg), http.StatusGone, bindingMissingErrorKey,
	).WithEmptyResponse().Build()

	ErrAsyncRequired = NewFailureResponseBuilder(
		errors.New(asyncRequiredMsg), http.StatusUnprocessableEntity, asyncRequiredKey,
	).WithErrorKey("AsyncRequired").Build()

	ErrPlanChangeNotSupported = NewFailureResponseBuilder(
		errors.New(planChangeUnsupportedMsg), http.StatusUnprocessableEntity, planChangeNotSupportedKey,
	).WithErrorKey("PlanChangeNotSupported").Build()

	ErrRawParamsInvalid = NewFailureResponse(
		errors.New(rawInvalidParamsMsg), http.StatusUnprocessableEntity, invalidRawParamsKey,
	)

	ErrAppGuidNotProvided = NewFailureResponse(
		errors.New(appGuidMissingMsg), http.StatusUnprocessableEntity, appGuidNotProvidedErrorKey,
	)

	ErrPlanQuotaExceeded    = errors.New(servicePlanQuotaExceededMsg)
	ErrServiceQuotaExceeded = errors.New(serviceQuotaExceededMsg)
)
