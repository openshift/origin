package v2

import (
	"fmt"
	"net/http"
)

func (c *client) PollLastOperation(r *LastOperationRequest) (*LastOperationResponse, error) {
	if err := validateLastOperationRequest(r); err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf(lastOperationURLFmt, c.URL, r.InstanceID)
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

func validateLastOperationRequest(request *LastOperationRequest) error {
	if request.InstanceID == "" {
		return required("instanceID")
	}

	return nil
}
