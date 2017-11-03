package v2

import (
	"fmt"
	"net/http"
)

// internal message body types

type updateInstanceRequestBody struct {
	ServiceID  string                 `json:"service_id"`
	PlanID     *string                `json:"plan_id,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Context    map[string]interface{} `json:"context,omitempty"`

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
		ServiceID:  r.ServiceID,
		PlanID:     r.PlanID,
		Parameters: r.Parameters,
	}

	if c.APIVersion.AtLeast(Version2_12()) {
		requestBody.Context = r.Context
	}

	response, err := c.prepareAndDo(http.MethodPatch, fullURL, params, requestBody, r.OriginatingIdentity)
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
		if !r.AcceptsIncomplete {
			// If the client did not signify that it could handle asynchronous
			// operations, a '202 Accepted' response should be treated as an error.
			return nil, c.handleFailureResponse(response)
		}

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
