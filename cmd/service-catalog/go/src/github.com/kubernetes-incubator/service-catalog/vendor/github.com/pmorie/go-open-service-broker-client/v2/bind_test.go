package v2

import (
	"fmt"
	"net/http"
	"testing"
)

const testBindingID = "test-binding-id"

func defaultBindRequest() *BindRequest {
	return &BindRequest{
		BindingID:  testBindingID,
		InstanceID: testInstanceID,
		ServiceID:  testServiceID,
		PlanID:     testPlanID,
	}
}

const defaultBindRequestBody = `{"service_id":"test-service-id","plan_id":"test-plan-id"}`

const successBindResponseBody = `{
  "credentials": {
    "uri": "mysql://mysqluser:pass@mysqlhost:3306/dbname",
    "username": "mysqluser",
    "password": "pass",
    "host": "mysqlhost",
    "port": 3306,
    "database": "dbname"
  }
}`

func successBindResponse() *BindResponse {
	return &BindResponse{
		Credentials: map[string]interface{}{
			"uri":      "mysql://mysqluser:pass@mysqlhost:3306/dbname",
			"username": "mysqluser",
			"password": "pass",
			"host":     "mysqlhost",
			"port":     float64(3306),
			"database": "dbname",
		},
	}
}

func optionalFieldsBindRequest() *BindRequest {
	r := defaultBindRequest()
	r.Parameters = map[string]interface{}{
		"foo": "bar",
		"blu": 2,
	}
	r.BindResource = &BindResource{
		AppGUID: strPtr("test-app-guid"),
		Route:   strPtr("test-route"),
	}
	return r
}

const optionalFieldsBindRequestBody = `{"service_id":"test-service-id","plan_id":"test-plan-id","parameters":{"blu":2,"foo":"bar"},"bind_resource":{"app_guid":"test-app-guid","route":"test-app-guid"}}`

func contextBindRequest() *BindRequest {
	r := defaultBindRequest()
	r.Context = map[string]interface{}{
		"foo": "bar",
	}
	return r
}

const contextBindRequestBody = `{"service_id":"test-service-id","plan_id":"test-plan-id","context":{"foo":"bar"}}`

func TestBind(t *testing.T) {
	cases := []struct {
		name                string
		version             APIVersion
		enableAlpha         bool
		originatingIdentity *OriginatingIdentity
		request             *BindRequest
		httpChecks          httpChecks
		httpReaction        httpReaction
		expectedResponse    *BindResponse
		expectedErrMessage  string
		expectedErr         error
	}{
		{
			name: "invalid request",
			request: func() *BindRequest {
				r := defaultBindRequest()
				r.InstanceID = ""
				return r
			}(),
			expectedErrMessage: "instanceID is required",
		},
		{
			name: "success - created",
			httpReaction: httpReaction{
				status: http.StatusCreated,
				body:   successBindResponseBody,
			},
			expectedResponse: successBindResponse(),
		},
		{
			name: "success - ok",
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   successBindResponseBody,
			},
			expectedResponse: successBindResponse(),
		},
		{
			name:    "success - optional fields",
			request: optionalFieldsBindRequest(),
			httpChecks: httpChecks{
				body: optionalFieldsBindRequestBody,
			},
			httpReaction: httpReaction{
				status: http.StatusCreated,
				body:   successBindResponseBody,
			},
			expectedResponse: successBindResponse(),
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
			expectedErr: testHTTPStatusCodeError(),
		},
		{
			name:    "context included if API version >= 2.13",
			version: Version2_13(),
			request: contextBindRequest(),
			httpChecks: httpChecks{
				body: contextBindRequestBody,
			},
			httpReaction: httpReaction{
				status: http.StatusCreated,
				body:   successBindResponseBody,
			},
			expectedResponse: successBindResponse(),
		},
		{
			name:    "context not included if API version < 2.13",
			version: Version2_12(),
			request: contextBindRequest(),
			httpChecks: httpChecks{
				body: defaultBindRequestBody,
			},
			httpReaction: httpReaction{
				status: http.StatusCreated,
				body:   successBindResponseBody,
			},
			expectedResponse: successBindResponse(),
		},
		{
			name:                "originating identity included",
			version:             Version2_13(),
			originatingIdentity: testOriginatingIdentity,
			httpChecks:          httpChecks{headers: map[string]string{OriginatingIdentityHeader: testOriginatingIdentityHeaderValue}},
			httpReaction: httpReaction{
				status: http.StatusCreated,
				body:   successBindResponseBody,
			},
			expectedResponse: successBindResponse(),
		},
		{
			name:                "originating identity excluded",
			version:             Version2_13(),
			originatingIdentity: nil,
			httpChecks:          httpChecks{headers: map[string]string{OriginatingIdentityHeader: ""}},
			httpReaction: httpReaction{
				status: http.StatusCreated,
				body:   successBindResponseBody,
			},
			expectedResponse: successBindResponse(),
		},
		{
			name:                "originating identity not sent unless API Version >= 2.13",
			version:             Version2_12(),
			originatingIdentity: testOriginatingIdentity,
			httpChecks:          httpChecks{headers: map[string]string{OriginatingIdentityHeader: ""}},
			httpReaction: httpReaction{
				status: http.StatusCreated,
				body:   successBindResponseBody,
			},
			expectedResponse: successBindResponse(),
		},
	}

	for _, tc := range cases {
		if tc.request == nil {
			tc.request = defaultBindRequest()
		}

		tc.request.OriginatingIdentity = tc.originatingIdentity

		if tc.httpChecks.URL == "" {
			tc.httpChecks.URL = "/v2/service_instances/test-instance-id/service_bindings/test-binding-id"
		}

		if tc.httpChecks.body == "" {
			tc.httpChecks.body = defaultBindRequestBody
		}

		if tc.version.label == "" {
			tc.version = Version2_11()
		}

		klient := newTestClient(t, tc.name, tc.version, tc.enableAlpha, tc.httpChecks, tc.httpReaction)

		response, err := klient.Bind(tc.request)

		doResponseChecks(t, tc.name, response, err, tc.expectedResponse, tc.expectedErrMessage, tc.expectedErr)
	}
}

func TestValidateBindRequest(t *testing.T) {
	cases := []struct {
		name    string
		request *BindRequest
		valid   bool
	}{
		{
			name:    "valid",
			request: defaultBindRequest(),
			valid:   true,
		},
		{
			name: "missing binding ID",
			request: func() *BindRequest {
				r := defaultBindRequest()
				r.BindingID = ""
				return r
			}(),
			valid: false,
		},
		{
			name: "missing instance ID",
			request: func() *BindRequest {
				r := defaultBindRequest()
				r.InstanceID = ""
				return r
			}(),
			valid: false,
		},
		{
			name: "missing service ID",
			request: func() *BindRequest {
				r := defaultBindRequest()
				r.ServiceID = ""
				return r
			}(),
			valid: false,
		},
		{
			name: "missing plan ID",
			request: func() *BindRequest {
				r := defaultBindRequest()
				r.PlanID = ""
				return r
			}(),
			valid: false,
		},
	}

	for _, tc := range cases {
		err := validateBindRequest(tc.request)
		if err != nil {
			if tc.valid {
				t.Errorf("%v: expected valid, got error: %v", tc.name, err)
			}
		} else if !tc.valid {
			t.Errorf("%v: expected invalid, got valid", tc.name)
		}
	}
}
