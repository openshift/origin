package v2

import (
	"fmt"
	"net/http"
)

func (c *client) GetBinding(r *GetBindingRequest) (*GetBindingResponse, error) {
	if err := c.validateAlphaAPIMethodsAllowed(); err != nil {
		return nil, GetBindingNotAllowedError{
			reason: err.Error(),
		}
	}

	fullURL := fmt.Sprintf(bindingURLFmt, c.URL, r.InstanceID, r.BindingID)

	response, err := c.prepareAndDo(http.MethodGet, fullURL, nil /* params */, nil /* request body */, nil /* originating identity */)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusOK:
		userResponse := &GetBindingResponse{}
		if err := c.unmarshalResponse(response, userResponse); err != nil {
			return nil, HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
		}

		return userResponse, nil
	default:
		return nil, c.handleFailureResponse(response)
	}
}
