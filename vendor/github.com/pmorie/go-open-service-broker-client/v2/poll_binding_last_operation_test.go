package v2

import (
	"fmt"
	"net/http"
	"testing"
)

func defaultBindingLastOperationRequest() *BindingLastOperationRequest {
	return &BindingLastOperationRequest{
		InstanceID: testInstanceID,
		BindingID:  testBindingID,
		ServiceID:  strPtr(testServiceID),
		PlanID:     strPtr(testPlanID),
	}
}

const successBindingLastOperationRequestBody = `{"service_id":"test-service-id","plan_id":"test-plan-id"}`

func TestPollBindingLastOperation(t *testing.T) {
	cases := []struct {
		name                string
		enableAlpha         bool
		originatingIdentity *OriginatingIdentity
		request             *BindingLastOperationRequest
		APIVersion          APIVersion
		httpChecks          httpChecks
		httpReaction        httpReaction
		expectedResponse    *LastOperationResponse
		expectedErrMessage  string
		expectedErr         error
	}{
		{
			name:        "op succeeded",
			enableAlpha: true,
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   successLastOperationResponseBody,
			},
			expectedResponse: successLastOperationResponse(),
		},
		{
			name:        "op in progress",
			enableAlpha: true,
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   inProgressLastOperationResponseBody,
			},
			expectedResponse: inProgressLastOperationResponse(),
		},
		{
			name:        "op failed",
			enableAlpha: true,
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   failedLastOperationResponseBody,
			},
			expectedResponse: failedLastOperationResponse(),
		},
		{
			name:        "http error",
			enableAlpha: true,
			httpReaction: httpReaction{
				err: fmt.Errorf("http error"),
			},
			expectedErrMessage: "http error",
		},
		{
			name:        "200 with malformed response",
			enableAlpha: true,
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   malformedResponse,
			},
			expectedErrMessage: "Status: 200; ErrorMessage: <nil>; Description: <nil>; ResponseError: unexpected end of JSON input",
		},
		{
			name:        "500 with malformed response",
			enableAlpha: true,
			httpReaction: httpReaction{
				status: http.StatusInternalServerError,
				body:   malformedResponse,
			},
			expectedErrMessage: "Status: 500; ErrorMessage: <nil>; Description: <nil>; ResponseError: unexpected end of JSON input",
		},
		{
			name:        "500 with conventional response",
			enableAlpha: true,
			httpReaction: httpReaction{
				status: http.StatusInternalServerError,
				body:   conventionalFailureResponseBody,
			},
			expectedErr: testHTTPStatusCodeError(),
		},
		{
			name:        "op succeeded",
			enableAlpha: true,
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   successLastOperationResponseBody,
			},
			expectedResponse: successLastOperationResponse(),
		},
		{
			name:                "originating identity included",
			enableAlpha:         true,
			originatingIdentity: testOriginatingIdentity,
			httpChecks:          httpChecks{headers: map[string]string{OriginatingIdentityHeader: testOriginatingIdentityHeaderValue}},
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   successLastOperationResponseBody,
			},
			expectedResponse: successLastOperationResponse(),
		},
		{
			name:                "originating identity excluded",
			enableAlpha:         true,
			originatingIdentity: nil,
			httpChecks:          httpChecks{headers: map[string]string{OriginatingIdentityHeader: ""}},
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   successLastOperationResponseBody,
			},
			expectedResponse: successLastOperationResponse(),
		},
		{
			name:               "alpha features disabled",
			enableAlpha:        false,
			expectedErrMessage: "Asynchronous binding operations are not allowed: alpha API methods not allowed: alpha features must be enabled",
		},
		{
			name:               "unsupported API version",
			enableAlpha:        true,
			APIVersion:         Version2_12(),
			expectedErrMessage: "Asynchronous binding operations are not allowed: alpha API methods not allowed: must have latest API Version. Current: 2.12, Expected: 2.13",
		},
	}

	for _, tc := range cases {
		if tc.request == nil {
			tc.request = defaultBindingLastOperationRequest()
		}

		tc.request.OriginatingIdentity = tc.originatingIdentity

		if tc.httpChecks.URL == "" {
			tc.httpChecks.URL = "/v2/service_instances/test-instance-id/service_bindings/test-binding-id/last_operation"
		}

		if len(tc.httpChecks.params) == 0 {
			tc.httpChecks.params = map[string]string{}
			tc.httpChecks.params[serviceIDKey] = testServiceID
			tc.httpChecks.params[planIDKey] = testPlanID
		}

		if tc.APIVersion.label == "" {
			tc.APIVersion = LatestAPIVersion()
		}

		klient := newTestClient(t, tc.name, tc.APIVersion, tc.enableAlpha, tc.httpChecks, tc.httpReaction)

		response, err := klient.PollBindingLastOperation(tc.request)

		doResponseChecks(t, tc.name, response, err, tc.expectedResponse, tc.expectedErrMessage, tc.expectedErr)
	}
}

func TestValidateBindingLastOperationRequest(t *testing.T) {
	cases := []struct {
		name    string
		request *BindingLastOperationRequest
		valid   bool
	}{
		{
			name:    "valid",
			request: defaultBindingLastOperationRequest(),
			valid:   true,
		},
		{
			name: "missing instance ID",
			request: func() *BindingLastOperationRequest {
				r := defaultBindingLastOperationRequest()
				r.InstanceID = ""
				return r
			}(),
			valid: false,
		},
		{
			name: "missing binding ID",
			request: func() *BindingLastOperationRequest {
				r := defaultBindingLastOperationRequest()
				r.BindingID = ""
				return r
			}(),
			valid: false,
		},
	}

	for _, tc := range cases {
		err := validateBindingLastOperationRequest(tc.request)
		if err != nil {
			if tc.valid {
				t.Errorf("%v: expected valid, got error: %v", tc.name, err)
			}
		} else if !tc.valid {
			t.Errorf("%v: expected invalid, got valid", tc.name)
		}
	}
}
