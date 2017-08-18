package v2

import (
	"fmt"
	"net/http"
)

// internal message body types

type updateInstanceRequestBody struct {
	serviceID  string                 `json:"service_id"`
	planID     *string                `json:"plan_id,omitempty"`
	parameters map[string]interface{} `json:"parameters,omitempty"`

	// Note: this client does not currently support the 'previous_values'
	// field of this request body.
}

func (c *client) UpdateInstance(r *UpdateInstanceRequest) (*UpdateInstanceResponse, error) {
	if err := validateUpdateInstanceRequest(r); err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf(serviceInstanceURLFmt, c.URL, r.InstanceID)
	params := map[string]string{}
	if r.AcceptsIncomplete {
		params[asyncQueryParamKey] = "true"
	}

	requestBody := &updateInstanceRequestBody{
		serviceID:  r.ServiceID,
		planID:     r.PlanID,
		parameters: r.Parameters,
	}

	response, err := c.prepareAndDo(http.MethodPatch, fullURL, params, requestBody)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusOK:
		if err := c.unmarshalResponse(response, &struct{}{}); err != nil {
			return nil, HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
		}

		return &UpdateInstanceResponse{}, nil
	case http.StatusAccepted:
		responseBodyObj := &asyncSuccessResponseBody{}
		if err := c.unmarshalResponse(response, responseBodyObj); err != nil {
			return nil, HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
		}

		var opPtr *OperationKey
		if responseBodyObj.Operation != nil {
			opStr := *responseBodyObj.Operation
			op := OperationKey(opStr)
			opPtr = &op
		}

		userResponse := &UpdateInstanceResponse{
			Async:        true,
			OperationKey: opPtr,
		}

		// TODO: fix op key handling

		return userResponse, nil
	default:
		return nil, c.handleFailureResponse(response)
	}

	return nil, nil
}

func validateUpdateInstanceRequest(request *UpdateInstanceRequest) error {
	if request.InstanceID == "" {
		return required("instanceID")
	}

	if request.ServiceID == "" {
		return required("serviceID")
	}

	return nil
}
