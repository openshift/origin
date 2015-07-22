package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"

	"github.com/docker/distribution/registry/auth"
	"golang.org/x/net/context"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/authorization/api"
	userapi "github.com/openshift/origin/pkg/user/api"
)

// TestVerifyImageStreamAccess mocks openshift http request/response and
// tests invalid/valid/scoped openshift tokens.
func TestVerifyImageStreamAccess(t *testing.T) {
	tests := []struct {
		openshiftResponse response
		expectedError     error
	}{
		{
			// Test invalid openshift bearer token
			openshiftResponse: response{401, "Unauthorized"},
			expectedError:     ErrOpenShiftAccessDenied,
		},
		{
			// Test valid openshift bearer token but token *not* scoped for create operation
			openshiftResponse: response{
				200,
				runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{
					Namespace: "foo",
					Allowed:   false,
					Reason:    "not authorized!",
				}),
			},
			expectedError: ErrOpenShiftAccessDenied,
		},
		{
			// Test valid openshift bearer token and token scoped for create operation
			openshiftResponse: response{
				200,
				runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{
					Namespace: "foo",
					Allowed:   true,
					Reason:    "authorized!",
				}),
			},
			expectedError: nil,
		},
	}
	for _, test := range tests {
		server, _ := simulateOpenShiftMaster([]response{test.openshiftResponse})
		client, err := NewUserOpenShiftClient("magic bearer token")
		if err != nil {
			t.Fatal(err)
		}
		err = verifyImageStreamAccess("foo", "bar", "create", client)
		if err == nil || test.expectedError == nil {
			if err != test.expectedError {
				t.Fatalf("verifyImageStreamAccess did not get expected error - got %s - expected %s", err, test.expectedError)
			}
		} else if err.Error() != test.expectedError.Error() {
			t.Fatalf("verifyImageStreamAccess did not get expected error - got %s - expected %s", err, test.expectedError)
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

	tests := map[string]struct {
		access             []auth.Access
		basicToken         string
		openshiftResponses []response
		expectedError      error
		expectedChallenge  bool
		expectedActions    []string
	}{
		"no token": {
			access:            []auth.Access{},
			basicToken:        "",
			expectedError:     ErrTokenRequired,
			expectedChallenge: true,
		},
		"invalid registry token": {
			access: []auth.Access{{
				Resource: auth.Resource{Type: "repository"},
			}},
			basicToken:        "ab-cd-ef-gh",
			expectedError:     ErrTokenInvalid,
			expectedChallenge: true,
		},
		"invalid openshift bearer token": {
			access: []auth.Access{{
				Resource: auth.Resource{Type: "repository"},
			}},
			basicToken:        "abcdefgh",
			expectedError:     ErrOpenShiftTokenRequired,
			expectedChallenge: true,
		},
		"valid openshift token but invalid namespace": {
			access: []auth.Access{{
				Resource: auth.Resource{
					Type: "repository",
					Name: "bar",
				},
				Action: "pull",
			}},
			basicToken:        "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			expectedError:     ErrNamespaceRequired,
			expectedChallenge: false,
		},
		"registry token but does not involve any repository operation": {
			access:            []auth.Access{{}},
			basicToken:        "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			expectedError:     ErrUnsupportedResource,
			expectedChallenge: false,
		},
		"registry token but does not involve any known action": {
			access: []auth.Access{{
				Resource: auth.Resource{
					Type: "repository",
					Name: "foo/bar",
				},
				Action: "blah",
			}},
			basicToken:        "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			expectedError:     ErrUnsupportedAction,
			expectedChallenge: false,
		},
		"docker login with invalid openshift creds": {
			basicToken:         "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftResponses: []response{{403, ""}},
			expectedError:      ErrOpenShiftAccessDenied,
			expectedChallenge:  true,
			expectedActions:    []string{"GET /oapi/v1/users/~"},
		},
		"docker login with valid openshift creds": {
			basicToken: "dXNyMTphd2Vzb21l",
			openshiftResponses: []response{
				{200, runtime.EncodeOrDie(latest.Codec, &userapi.User{ObjectMeta: kapi.ObjectMeta{Name: "usr1"}})},
			},
			expectedError:     nil,
			expectedChallenge: false,
			expectedActions:   []string{"GET /oapi/v1/users/~"},
		},
		"error running subject access review": {
			access: []auth.Access{{
				Resource: auth.Resource{
					Type: "repository",
					Name: "foo/bar",
				},
				Action: "pull",
			}},
			basicToken: "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftResponses: []response{
				{500, "Uh oh"},
			},
			expectedError:     errors.New("an error on the server has prevented the request from succeeding (post localSubjectAccessReviews)"),
			expectedChallenge: false,
			expectedActions:   []string{"POST /oapi/v1/namespaces/foo/localsubjectaccessreviews"},
		},
		"valid openshift token but token not scoped for the given repo operation": {
			access: []auth.Access{{
				Resource: auth.Resource{
					Type: "repository",
					Name: "foo/bar",
				},
				Action: "pull",
			}},
			basicToken: "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftResponses: []response{
				{200, runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{Namespace: "foo", Allowed: false, Reason: "unauthorized!"})},
			},
			expectedError:     ErrOpenShiftAccessDenied,
			expectedChallenge: true,
			expectedActions:   []string{"POST /oapi/v1/namespaces/foo/localsubjectaccessreviews"},
		},
		"partially valid openshift token": {
			// Check all the different resource-type/verb combinations we allow to make sure they validate and continue to validate remaining Resource requests
			access: []auth.Access{
				{Resource: auth.Resource{Type: "repository", Name: "foo/aaa"}, Action: "pull"},
				{Resource: auth.Resource{Type: "repository", Name: "bar/bbb"}, Action: "push"},
				{Resource: auth.Resource{Type: "admin"}, Action: "prune"},
				{Resource: auth.Resource{Type: "repository", Name: "baz/ccc"}, Action: "pull"},
			},
			basicToken: "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftResponses: []response{
				{200, runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{Namespace: "foo", Allowed: true, Reason: "authorized!"})},
				{200, runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{Namespace: "bar", Allowed: true, Reason: "authorized!"})},
				{200, runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{Namespace: "", Allowed: true, Reason: "authorized!"})},
				{200, runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{Namespace: "baz", Allowed: false, Reason: "no!"})},
			},
			expectedError:     ErrOpenShiftAccessDenied,
			expectedChallenge: true,
			expectedActions: []string{
				"POST /oapi/v1/namespaces/foo/localsubjectaccessreviews",
				"POST /oapi/v1/namespaces/bar/localsubjectaccessreviews",
				"POST /oapi/v1/subjectaccessreviews",
				"POST /oapi/v1/namespaces/baz/localsubjectaccessreviews",
			},
		},
		"valid openshift token": {
			access: []auth.Access{{
				Resource: auth.Resource{
					Type: "repository",
					Name: "foo/bar",
				},
				Action: "pull",
			}},
			basicToken: "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftResponses: []response{
				{200, runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{Namespace: "foo", Allowed: true, Reason: "authorized!"})},
			},
			expectedError:     nil,
			expectedChallenge: false,
			expectedActions:   []string{"POST /oapi/v1/namespaces/foo/localsubjectaccessreviews"},
		},
		"pruning": {
			access: []auth.Access{
				{
					Resource: auth.Resource{
						Type: "admin",
					},
					Action: "prune",
				},
				{
					Resource: auth.Resource{
						Type: "repository",
						Name: "foo/bar",
					},
					Action: "*",
				},
			},
			basicToken: "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftResponses: []response{
				{200, runtime.EncodeOrDie(latest.Codec, &api.SubjectAccessReviewResponse{Allowed: true, Reason: "authorized!"})},
			},
			expectedError:     nil,
			expectedChallenge: false,
			expectedActions: []string{
				"POST /oapi/v1/subjectaccessreviews",
			},
		},
	}

	for k, test := range tests {
		req, err := http.NewRequest("GET", options["addr"].(string), nil)
		if err != nil {
			t.Errorf("%s: %v", k, err)
			continue
		}
		if len(test.basicToken) > 0 {
			req.Header.Set("Authorization", fmt.Sprintf("Basic %s", test.basicToken))
		}
		ctx := context.WithValue(nil, "http.request", req)

		server, actions := simulateOpenShiftMaster(test.openshiftResponses)
		authCtx, err := accessController.Authorized(ctx, test.access...)
		server.Close()

		expectedActions := test.expectedActions
		if expectedActions == nil {
			expectedActions = []string{}
		}
		if !reflect.DeepEqual(actions, &expectedActions) {
			t.Errorf("%s: expected\n\t%#v\ngot\n\t%#v", k, &expectedActions, actions)
			continue
		}

		if err == nil || test.expectedError == nil {
			if err != test.expectedError {
				t.Errorf("%s: accessController did not get expected error - got %v - expected %v", k, err, test.expectedError)
				continue
			}
			if authCtx == nil {
				t.Errorf("%s: expected auth context but got nil", k)
				continue
			}
		} else {
			_, isChallenge := err.(auth.Challenge)
			if test.expectedChallenge != isChallenge {
				t.Errorf("%s: expected challenge=%v, accessController returned challenge=%v", k, test.expectedChallenge, isChallenge)
				continue
			}

			if err.Error() != test.expectedError.Error() {
				t.Errorf("%s: accessController did not get expected error - got %s - expected %s", k, err, test.expectedError)
				continue
			}
			if authCtx != nil {
				t.Errorf("%s: expected nil auth context but got %s", k, authCtx)
				continue
			}
		}
	}
}

type response struct {
	code int
	body string
}

func simulateOpenShiftMaster(responses []response) (*httptest.Server, *[]string) {
	i := 0
	actions := []string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := response{500, "No response registered"}
		if i < len(responses) {
			response = responses[i]
		}
		i++
		w.WriteHeader(response.code)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintln(w, response.body)
		actions = append(actions, r.Method+" "+r.URL.Path)
	}))

	os.Setenv("OPENSHIFT_MASTER", server.URL)
	os.Setenv("OPENSHIFT_INSECURE", "true")
	return server, &actions
}
