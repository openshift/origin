package server

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/docker/distribution/registry/auth"
	"golang.org/x/net/context"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/authorization/api"
)

// TestVerifyImageStreamAccess mocks openshift http request/response and
// tests invalid/valid/scoped openshift tokens.
func TestVerifyImageStreamAccess(t *testing.T) {
	tests := []struct {
		openshiftStatusCode int
		openshiftResponse   string
		expectedError       error
	}{
		{
			// Test invalid openshift bearer token
			openshiftStatusCode: 401,
			openshiftResponse:   "Unauthorized",
			expectedError:       ErrOpenShiftAccessDenied,
		},
		{
			// Test valid openshift bearer token but token *not* scoped for create operation
			openshiftStatusCode: 200,
			openshiftResponse: runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{
				Namespace: "foo",
				Allowed:   false,
				Reason:    "not authorized!",
			}),
			expectedError: ErrOpenShiftAccessDenied,
		},
		{
			// Test valid openshift bearer token and token scoped for create operation
			openshiftStatusCode: 200,
			openshiftResponse: runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{
				Namespace: "foo",
				Allowed:   true,
				Reason:    "authorized!",
			}),
			expectedError: nil,
		},
	}
	for _, test := range tests {
		server := simulateOpenShiftMaster(test.openshiftStatusCode, test.openshiftResponse)
		client, err := NewUserOpenShiftClient("magic bearer token")
		if err != nil {
			t.Fatal(err)
		}
		err = verifyImageStreamAccess("foo", "bar", "create", client)
		if err == nil || test.expectedError == nil {
			if err != test.expectedError {
				t.Fatal("verifyImageStreamAccess did not get expected error - got %s - expected %s", err, test.expectedError)
			}
		} else if err.Error() != test.expectedError.Error() {
			t.Fatal("verifyImageStreamAccess did not get expected error - got %s - expected %s", err, test.expectedError)
		}
		server.Close()
	}
}

// TestAccessController tests complete integration of the v2 registry auth package.
func TestAccessController(t *testing.T) {
	options := map[string]interface{}{
		"addr":       "https://openshift-example.com/osapi",
		"apiVersion": latest.Version,
	}
	accessController, err := newAccessController(options)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		access              auth.Access
		skipAccess          bool
		skipBasicToken      bool
		basicToken          string
		openshiftStatusCode int
		openshiftResponse   string
		expectedError       error
	}{
		{
			// Test request with no token.
			access:         auth.Access{},
			skipBasicToken: true,
			basicToken:     "",
			expectedError:  ErrTokenRequired,
		},
		{
			// Test request with invalid registry token.
			access: auth.Access{
				Resource: auth.Resource{Type: "repository"},
			},
			basicToken:    "ab-cd-ef-gh",
			expectedError: ErrTokenInvalid,
		},
		{
			// Test request with invalid openshift bearer token.
			access: auth.Access{
				Resource: auth.Resource{Type: "repository"},
			},
			basicToken:    "abcdefgh",
			expectedError: ErrOpenShiftTokenRequired,
		},
		{
			// Test request with valid openshift token but invalid namespace.
			access: auth.Access{
				Resource: auth.Resource{
					Type: "repository",
					Name: "bar",
				},
				Action: "pull",
			},
			basicToken:    "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			expectedError: ErrNamespaceRequired,
		},
		{
			// Test request with registry token but does not involve any repository operation.
			access:        auth.Access{},
			basicToken:    "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			expectedError: nil,
		},
		{
			// Test request that simulates docker login with invalid openshift creds.
			skipAccess:          true,
			basicToken:          "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftStatusCode: 404,
			expectedError:       ErrOpenShiftAccessDenied,
		},
		{
			// Test request that simulates docker login with valid openshift creds.
			skipAccess:          true,
			basicToken:          "dXNyMTphd2Vzb21l",
			openshiftStatusCode: 200,
			openshiftResponse:   `{"name":"usr1","selfLink":"/osapi/` + latest.Version + `/users/usr1","identities":["anypassword:usr1"]}`,
			expectedError:       nil,
		},
		{
			// Test request with valid openshift token but token not scoped for the given repo operation.
			access: auth.Access{
				Resource: auth.Resource{
					Type: "repository",
					Name: "foo/bar",
				},
				Action: "pull",
			},
			basicToken:    "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			expectedError: ErrOpenShiftAccessDenied,
		},
		{
			// Test request with valid openshift token.
			access: auth.Access{
				Resource: auth.Resource{
					Type: "repository",
					Name: "foo/bar",
				},
				Action: "pull",
			},
			basicToken:          "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftStatusCode: 200,
			openshiftResponse: runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{
				Namespace: "foo",
				Allowed:   true,
				Reason:    "authorized!",
			}),
			expectedError: nil,
		},
	}
	for _, test := range tests {
		t.Logf("Processing test: %s", test)
		req, err := http.NewRequest("GET", options["addr"].(string), nil)
		if err != nil {
			t.Fatal(err)
		}
		if !test.skipBasicToken {
			req.Header.Set("Authorization", fmt.Sprintf("Basic %s", test.basicToken))
		}
		ctx := context.WithValue(nil, "http.request", req)
		if test.openshiftStatusCode != 0 || len(test.openshiftResponse) != 0 {
			server := simulateOpenShiftMaster(test.openshiftStatusCode, test.openshiftResponse)
			defer server.Close()
		}
		var authCtx context.Context
		if !test.skipAccess {
			authCtx, err = accessController.Authorized(ctx, test.access)
		} else {
			authCtx, err = accessController.Authorized(ctx)
		}
		if err == nil || test.expectedError == nil {
			if err != test.expectedError {
				t.Fatalf("accessController did not get expected error - got %s - expected %s", err, test.expectedError)
			}
			if authCtx == nil {
				t.Fatalf("expected auth context but got nil")
			}
		} else {
			challenge, ok := err.(auth.Challenge)
			if !ok {
				t.Fatal("accessController did not return a challenge")
			}
			if challenge.Error() != test.expectedError.Error() {
				t.Fatalf("accessController did not get expected error - got %s - expected %s", challenge, test.expectedError)
			}
			if authCtx != nil {
				t.Fatalf("expected nil auth context but got %s", authCtx)
			}
		}
	}
}

func simulateOpenShiftMaster(code int, body string) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(code)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, body)
	}))

	os.Setenv("OPENSHIFT_MASTER", server.URL)
	os.Setenv("OPENSHIFT_INSECURE", "true")
	return server
}
