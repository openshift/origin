package v2

import (
	"fmt"
	"net/http"
	"testing"
)

const okBindingBytes = `{
  "credentials": {
    "test-key": "foo"
  }
}`

func defaultGetBindingRequest() *GetBindingRequest {
	return &GetBindingRequest{
		InstanceID: testInstanceID,
		BindingID:  testBindingID,
	}
}

func okGetBindingResponse() *GetBindingResponse {
	response := &GetBindingResponse{}
	response.Credentials = map[string]interface{}{
		"test-key": "foo",
	}
	return response
}

func TestGetBinding(t *testing.T) {
	cases := []struct {
		name               string
		enableAlpha        bool
		request            *GetBindingRequest
		APIVersion         APIVersion
		httpReaction       httpReaction
		expectedResponse   *GetBindingResponse
		expectedErrMessage string
		expectedErr        error
	}{
		{
			name:        "success",
			enableAlpha: true,
			httpReaction: httpReaction{
				status: http.StatusOK,
				body:   okBindingBytes,
			},
			expectedResponse: okGetBindingResponse(),
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
			name:               "alpha features disabled",
			enableAlpha:        false,
			expectedErrMessage: "GetBinding not allowed: alpha API methods not allowed: alpha features must be enabled",
		},
		{
			name:        "unsupported API version",
			enableAlpha: true,
			APIVersion:  Version2_11(),
			expectedErr: testGetBindingNotAllowedErrorUnsupportedAPIVersion(),
		},
	}

	for _, tc := range cases {
		if tc.request == nil {
			tc.request = defaultGetBindingRequest()
		}

		httpChecks := httpChecks{
			URL: "/v2/service_instances/test-instance-id/service_bindings/test-binding-id",
		}

		if tc.APIVersion.label == "" {
			tc.APIVersion = LatestAPIVersion()
		}

		klient := newTestClient(t, tc.name, tc.APIVersion, tc.enableAlpha, httpChecks, tc.httpReaction)

		response, err := klient.GetBinding(tc.request)

		doResponseChecks(t, tc.name, response, err, tc.expectedResponse, tc.expectedErrMessage, tc.expectedErr)
	}
}
