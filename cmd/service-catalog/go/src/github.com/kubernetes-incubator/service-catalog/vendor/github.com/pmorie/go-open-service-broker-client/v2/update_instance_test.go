package v2

import (
	"fmt"
	"net/http"
	"testing"
)

func defaultUpdateInstanceRequest() *UpdateInstanceRequest {
	return &UpdateInstanceRequest{
		InstanceID: testInstanceID,
		ServiceID:  testServiceID,
		PlanID:     strPtr(testPlanID),
	}
}

func defaultAsyncUpdateInstanceRequest() *UpdateInstanceRequest {
	r := defaultUpdateInstanceRequest()
	r.AcceptsIncomplete = true
	return r
}

const successUpdateInstanceResponseBody = `{}`

func successUpdateInstanceResponse() *UpdateInstanceResponse {
	return &UpdateInstanceResponse{}
}

const successAsyncUpdateInstanceResponseBody = `{
  "operation": "test-operation-key"
}`

func successUpdateInstanceResponseAsync() *UpdateInstanceResponse {
	r := successUpdateInstanceResponse()
	r.Async = true
	r.OperationKey = &testOperation
	return r
}

func TestUpdateInstanceInstance(t *testing.T) {
	cases := []struct {
		name                string
		enableAlpha         bool
		originatingIdentity *AlphaOriginatingIdentity
		request             *UpdateInstanceRequest
		httpChecks          httpChecks
		httpReaction        httpReaction
		expectedResponse    *UpdateInstanceResponse
		expectedErrMessage  string
		expectedErr         error
	}{
		{
			name: "invalid request",
			request: func() *UpdateInstanceRequest {
				r := defaultUpdateInstanceRequest()
				r.InstanceID = ""
				return r
			}(),
			expectedErrMessage: "instanceID is required",
		},
		{
			name: "success - ok",
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   successUpdateInstanceResponseBody,
			},
			expectedResponse: successUpdateInstanceResponse(),
		},
		{
			name:    "success - async",
			request: defaultAsyncUpdateInstanceRequest(),
			httpChecks: httpChecks{
				params: map[string]string{
					asyncQueryParamKey: "true",
				},
			},
			httpReaction: httpReaction{
				status: http.StatusAccepted,
				body:   successAsyncUpdateInstanceResponseBody,
			},
			expectedResponse: successUpdateInstanceResponseAsync(),
		},
		{
			name:    "accepted with malformed response",
			request: defaultAsyncUpdateInstanceRequest(),
			httpChecks: httpChecks{
				params: map[string]string{
					asyncQueryParamKey: "true",
				},
			},
			httpReaction: httpReaction{
				status: http.StatusAccepted,
				body:   malformedResponse,
			},
			expectedErrMessage: "Status: 202; ErrorMessage: <nil>; Description: <nil>; ResponseError: unexpected end of JSON input",
		},
		{
			name: "http error",
			httpReaction: httpReaction{
				err: fmt.Errorf("http error"),
			},
			expectedErrMessage: "http error",
		},
		{
			name: "200 with malformed response",
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   malformedResponse,
			},
			expectedErrMessage: "Status: 200; ErrorMessage: <nil>; Description: <nil>; ResponseError: unexpected end of JSON input",
		},
		{
			name: "500 with malformed response",
			httpReaction: httpReaction{
				status: http.StatusInternalServerError,
				body:   malformedResponse,
			},
			expectedErrMessage: "Status: 500; ErrorMessage: <nil>; Description: <nil>; ResponseError: unexpected end of JSON input",
		},
		{
			name: "500 with conventional failure response",
			httpReaction: httpReaction{
				status: http.StatusInternalServerError,
				body:   conventionalFailureResponseBody,
			},
			expectedErr: testHttpStatusCodeError(),
		},
		{
			name:                "originating identity included",
			enableAlpha:         true,
			originatingIdentity: testOriginatingIdentity,
			httpChecks:          httpChecks{headers: map[string]string{OriginatingIdentityHeader: testOriginatingIdentityHeaderValue}},
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   successUpdateInstanceResponseBody,
			},
			expectedResponse: successUpdateInstanceResponse(),
		},
		{
			name:                "originating identity excluded",
			enableAlpha:         true,
			originatingIdentity: nil,
			httpChecks:          httpChecks{headers: map[string]string{OriginatingIdentityHeader: ""}},
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   successUpdateInstanceResponseBody,
			},
			expectedResponse: successUpdateInstanceResponse(),
		},
		{
			name:                "originating identity not sent unless alpha enabled",
			enableAlpha:         false,
			originatingIdentity: testOriginatingIdentity,
			httpChecks:          httpChecks{headers: map[string]string{OriginatingIdentityHeader: ""}},
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   successUpdateInstanceResponseBody,
			},
			expectedResponse: successUpdateInstanceResponse(),
		},
	}

	for _, tc := range cases {
		if tc.request == nil {
			tc.request = defaultUpdateInstanceRequest()
		}

		tc.request.OriginatingIdentity = tc.originatingIdentity

		if tc.httpChecks.URL == "" {
			tc.httpChecks.URL = "/v2/service_instances/test-instance-id"
		}

		if tc.httpChecks.body == "" {
			tc.httpChecks.body = "{}"
		}

		version := Version2_11()
		klient := newTestClient(t, tc.name, version, tc.enableAlpha, tc.httpChecks, tc.httpReaction)

		response, err := klient.UpdateInstance(tc.request)

		doResponseChecks(t, tc.name, response, err, tc.expectedResponse, tc.expectedErrMessage, tc.expectedErr)
	}
}

func TestValidateUpdateInstanceRequest(t *testing.T) {
	cases := []struct {
		name    string
		request *UpdateInstanceRequest
		valid   bool
	}{
		{
			name:    "valid",
			request: defaultUpdateInstanceRequest(),
			valid:   true,
		},
		{
			name: "missing instance ID",
			request: func() *UpdateInstanceRequest {
				r := defaultUpdateInstanceRequest()
				r.InstanceID = ""
				return r
			}(),
			valid: false,
		},
	}

	for _, tc := range cases {
		err := validateUpdateInstanceRequest(tc.request)
		if err != nil {
			if tc.valid {
				t.Errorf("%v: expected valid, got error: %v", tc.name, err)
			}
		} else if !tc.valid {
			t.Errorf("%v: expected invalid, got valid", tc.name)
		}
	}
}
