package v2

import (
	"fmt"
	"net/http"
)

func (c *client) Unbind(r *UnbindRequest) (*UnbindResponse, error) {
	if err := validateUnbindRequest(r); err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf(bindingURLFmt, c.URL, r.InstanceID, r.BindingID)
	params := map[string]string{}
	params[serviceIDKey] = r.ServiceID
	params[planIDKey] = r.PlanID

	response, err := c.prepareAndDo(http.MethodDelete, fullURL, params, nil)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusOK, http.StatusGone:
		userResponse := &UnbindResponse{}
		if err := c.unmarshalResponse(response, userResponse); err != nil {
			return nil, HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
		}

		return userResponse, nil
	default:
		return nil, c.handleFailureResponse(response)
	}

	return nil, nil
}

func validateUnbindRequest(request *UnbindRequest) error {
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
