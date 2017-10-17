package v2

import (
	"fmt"
	"net/http"

	"github.com/golang/glog"
)

// internal message body types

type provisionRequestBody struct {
	ServiceID        string                 `json:"service_id"`
	PlanID           string                 `json:"plan_id"`
	OrganizationGUID string                 `json:"organization_guid"`
	SpaceGUID        string                 `json:"space_guid"`
	Parameters       map[string]interface{} `json:"parameters,omitempty"`
	Context          map[string]interface{} `json:"context,omitempty"`
}

type provisionSuccessResponseBody struct {
	DashboardURL *string `json:"dashboard_url"`
	Operation    *string `json:"operation"`
}

func (c *client) ProvisionInstance(r *ProvisionRequest) (*ProvisionResponse, error) {
	if err := validateProvisionRequest(r); err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf(serviceInstanceURLFmt, c.URL, r.InstanceID)

	params := map[string]string{}
	if r.AcceptsIncomplete {
		params[asyncQueryParamKey] = "true"
	}

	requestBody := &provisionRequestBody{
		ServiceID:        r.ServiceID,
		PlanID:           r.PlanID,
		OrganizationGUID: r.OrganizationGUID,
		SpaceGUID:        r.SpaceGUID,
		Parameters:       r.Parameters,
	}

	if c.APIVersion.AtLeast(Version2_12()) {
		requestBody.Context = r.Context
	}

	response, err := c.prepareAndDo(http.MethodPut, fullURL, params, requestBody, r.OriginatingIdentity)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusCreated, http.StatusOK:
		userResponse := &ProvisionResponse{}
		if err := c.unmarshalResponse(response, userResponse); err != nil {
			return nil, HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
		}

		return userResponse, nil
	case http.StatusAccepted:
		if !r.AcceptsIncomplete {
			// If the client did not signify that it could handle asynchronous
			// operations, a '202 Accepted' response should be treated as an error.
			return nil, c.handleFailureResponse(response)
		}

		responseBodyObj := &provisionSuccessResponseBody{}
		if err := c.unmarshalResponse(response, responseBodyObj); err != nil {
			return nil, HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
		}

		var opPtr *OperationKey
		if responseBodyObj.Operation != nil {
			opStr := *responseBodyObj.Operation
			op := OperationKey(opStr)
			opPtr = &op
		}

		userResponse := &ProvisionResponse{
			Async:        true,
			DashboardURL: responseBodyObj.DashboardURL,
			OperationKey: opPtr,
		}

		if c.Verbose {
			glog.Infof("broker %q: received asynchronous response", c.Name)
		}

		return userResponse, nil
	default:
		return nil, c.handleFailureResponse(response)
	}
}

func required(name string) error {
	return fmt.Errorf("%v is required", name)
}

func validateProvisionRequest(request *ProvisionRequest) error {
	if request.InstanceID == "" {
		return required("instanceID")
	}

	if request.ServiceID == "" {
		return required("serviceID")
	}

	if request.PlanID == "" {
		return required("planID")
	}

	if request.OrganizationGUID == "" {
		return required("organizationGUID")
	}

	if request.SpaceGUID == "" {
		return required("spaceGUID")
	}

	return nil
}
