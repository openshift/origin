package v2

import (
	"fmt"
	"net/http"
)

func (c *client) PollBindingLastOperation(r *BindingLastOperationRequest) (*LastOperationResponse, error) {
	if err := c.validateAlphaAPIMethodsAllowed(); err != nil {
		return nil, AsyncBindingOperationsNotAllowedError{
			reason: err.Error(),
		}
	}

	if err := validateBindingLastOperationRequest(r); err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf(bindingLastOperationURLFmt, c.URL, r.InstanceID, r.BindingID)
	params := map[string]string{}

	if r.ServiceID != nil {
		params[VarKeyServiceID] = *r.ServiceID
	}
	if r.PlanID != nil {
		params[VarKeyPlanID] = *r.PlanID
	}
	if r.OperationKey != nil {
		op := *r.OperationKey
		opStr := string(op)
		params[VarKeyOperation] = opStr
	}

	response, err := c.prepareAndDo(http.MethodGet, fullURL, params, nil /* request body */, r.OriginatingIdentity)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusOK:
		userResponse := &LastOperationResponse{}
		if err := c.unmarshalResponse(response, userResponse); err != nil {
			return nil, HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
		}

		return userResponse, nil
	default:
		return nil, c.handleFailureResponse(response)
	}
}

func validateBindingLastOperationRequest(request *BindingLastOperationRequest) error {
	if request.InstanceID == "" {
		return required("instanceID")
	}

	if request.BindingID == "" {
		return required("bindingID")
	}

	return nil
}
