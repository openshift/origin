package v2

import (
	"fmt"
	"net/http"
)

func (c *client) DeprovisionInstance(r *DeprovisionRequest) (*DeprovisionResponse, error) {
	if err := validateDeprovisionRequest(r); err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf(serviceInstanceURLFmt, c.URL, r.InstanceID)

	params := map[string]string{
		serviceIDKey: string(r.ServiceID),
		planIDKey:    string(r.PlanID),
	}
	if r.AcceptsIncomplete {
		params[asyncQueryParamKey] = "true"
	}

	response, err := c.prepareAndDo(http.MethodDelete, fullURL, params, nil, r.OriginatingIdentity)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusOK, http.StatusGone:
		return &DeprovisionResponse{}, nil
	case http.StatusAccepted:
		if !r.AcceptsIncomplete {
			// If the client did not signify that it could handle asynchronous
			// operations, a '202 Accepted' response should be treated as an error.
			return nil, c.handleFailureResponse(response)
		}

		responseBodyObj := &asyncSuccessResponseBody{}
		if err := c.unmarshalResponse(response, responseBodyObj); err != nil {
			return nil, err
		}

		var opPtr *OperationKey
		if responseBodyObj.Operation != nil {
			opStr := *responseBodyObj.Operation
			op := OperationKey(opStr)
			opPtr = &op
		}

		userResponse := &DeprovisionResponse{
			Async:        true,
			OperationKey: opPtr,
		}

		return userResponse, nil
	default:
		return nil, c.handleFailureResponse(response)
	}
}

func validateDeprovisionRequest(request *DeprovisionRequest) error {
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
