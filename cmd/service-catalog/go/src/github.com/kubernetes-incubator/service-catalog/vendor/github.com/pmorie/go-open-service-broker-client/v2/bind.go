package v2

import (
	"fmt"
	"net/http"
)

// internal message body types

type bindRequestBody struct {
	ServiceID    string                 `json:"service_id"`
	PlanID       string                 `json:"plan_id"`
	Parameters   map[string]interface{} `json:"parameters,omitempty"`
	BindResource map[string]interface{} `json:"bind_resource,omitempty"`
}

const (
	bindResourceAppGUIDKey = "app_guid"
	bindResourceRouteKey   = "route"
)

func (c *client) Bind(r *BindRequest) (*BindResponse, error) {
	if err := validateBindRequest(r); err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf(bindingURLFmt, c.URL, r.InstanceID, r.BindingID)

	requestBody := &bindRequestBody{
		ServiceID:  r.ServiceID,
		PlanID:     r.PlanID,
		Parameters: r.Parameters,
	}

	if r.BindResource != nil {
		requestBody.BindResource = map[string]interface{}{}
		if r.BindResource.AppGUID != nil {
			requestBody.BindResource[bindResourceAppGUIDKey] = *r.BindResource.AppGUID
		}
		if r.BindResource.Route != nil {
			requestBody.BindResource[bindResourceRouteKey] = *r.BindResource.AppGUID
		}
	}

	response, err := c.prepareAndDo(http.MethodPut, fullURL, nil /* params */, requestBody, r.OriginatingIdentity)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusOK, http.StatusCreated:
		userResponse := &BindResponse{}
		if err := c.unmarshalResponse(response, userResponse); err != nil {
			return nil, HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
		}

		return userResponse, nil
	default:
		return nil, c.handleFailureResponse(response)
	}

	return nil, nil
}

func validateBindRequest(request *BindRequest) error {
	if request.BindingID == "" {
		return required("bindingID")
	}

	if request.InstanceID == "" {
		return required("instanceID")
	}

	if request.ServiceID == "" {
		return required("serviceID")
	}

	if request.PlanID == "" {
		return required("planID")
	}

	return nil
}
