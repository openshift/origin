package v2

import (
	"fmt"
	"net/http"
)

// internal message body types

type deprovisionInstanceRequestBody struct {
	serviceID *string `json:"service_id"`
	planID    *string `json:"plan_id,omitempty"`
}

func (c *client) DeprovisionInstance(r *DeprovisionRequest) (*DeprovisionResponse, error) {
	if err := validateDeprovisionRequest(r); err != nil {
		return nil, err
	}

	fullURL := fmt.Sprintf(serviceInstanceURLFmt, c.URL, r.InstanceID)

	params := map[string]string{}
	if r.AcceptsIncomplete {
		params[asyncQueryParamKey] = "true"
	}

	requestServiceID := string(r.ServiceID)
	requestPlanID := string(r.PlanID)

	requestBody := &deprovisionInstanceRequestBody{
		serviceID: &requestServiceID,
		planID:    &requestPlanID,
	}

	response, err := c.prepareAndDo(http.MethodDelete, fullURL, params, requestBody)
	if err != nil {
		return nil, err
	}

	switch response.StatusCode {
	case http.StatusOK, http.StatusGone:
		return &DeprovisionResponse{}, nil
	case http.StatusAccepted:
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

	return nil, nil
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
