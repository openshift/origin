package v2

import (
	"fmt"
	"net/http"

	"github.com/golang/glog"
)

type unbindSuccessResponseBody struct {
	Operation *string `json:"operation"`
}

func (c *client) Unbind(r *UnbindRequest) (*UnbindResponse, error) {
	if r.AcceptsIncomplete {
		if err := c.validateAlphaAPIMethodsAllowed(); err != nil {
			return nil, AsyncBindingOperationsNotAllowedError{
				reason: err.Error(),
			}
		}
	}

	if err := validateUnbindRequest(r); err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf(bindingURLFmt, c.URL, r.InstanceID, r.BindingID)
	params := map[string]string{}
	params[serviceIDKey] = r.ServiceID
	params[planIDKey] = r.PlanID
	if r.AcceptsIncomplete {
		params[asyncQueryParamKey] = "true"
	}

	response, err := c.prepareAndDo(http.MethodDelete, fullURL, params, nil, r.OriginatingIdentity)
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
	case http.StatusAccepted:
		if !r.AcceptsIncomplete {
			return nil, c.handleFailureResponse(response)
		}

		responseBodyObj := &unbindSuccessResponseBody{}
		if err := c.unmarshalResponse(response, responseBodyObj); err != nil {
			return nil, HTTPStatusCodeError{StatusCode: response.StatusCode, ResponseError: err}
		}

		var opPtr *OperationKey
		if responseBodyObj.Operation != nil {
			opStr := *responseBodyObj.Operation
			op := OperationKey(opStr)
			opPtr = &op
		}

		userResponse := &UnbindResponse{
			OperationKey: opPtr,
		}
		if response.StatusCode == http.StatusAccepted {
			if c.Verbose {
				glog.Infof("broker %q: received asynchronous response", c.Name)
			}
			userResponse.Async = true
		}

		return userResponse, nil
	default:
		return nil, c.handleFailureResponse(response)
	}
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
