package server

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/docker/distribution/registry/auth"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/docker/distribution/context"
	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	userapi "github.com/openshift/origin/pkg/user/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
	"github.com/openshift/origin/pkg/client"
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
				runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{
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
				runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{
					Namespace: "foo",
					Allowed:   true,
					Reason:    "authorized!",
				}),
			},
			expectedError: nil,
		},
	}
	for _, test := range tests {
		ctx := context.Background()
		server, _ := simulateOpenShiftMaster([]response{test.openshiftResponse})
		client, err := client.New(&restclient.Config{BearerToken: "magic bearer token", Host: server.URL})
		if err != nil {
			t.Fatal(err)
		}
		err = verifyImageStreamAccess(ctx, "foo", "bar", "create", client)
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

	tests := map[string]struct {
		access             []auth.Access
		basicToken         string
		openshiftResponses []response
		expectedError      error
		expectedChallenge  bool
		expectedRepoErr    string
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
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &userapi.User{ObjectMeta: kapi.ObjectMeta{Name: "usr1"}})},
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
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Namespace: "foo", Allowed: false, Reason: "unauthorized!"})},
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
				{Resource: auth.Resource{Type: "repository", Name: "baz/ccc"}, Action: "push"},
			},
			basicToken: "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftResponses: []response{
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Namespace: "foo", Allowed: true, Reason: "authorized!"})},
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Namespace: "bar", Allowed: true, Reason: "authorized!"})},
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Namespace: "", Allowed: true, Reason: "authorized!"})},
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Namespace: "baz", Allowed: false, Reason: "no!"})},
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
		"deferred cross-mount error": {
			// cross-mount push requests check pull/push access on the target repo and pull access on the source repo.
			// we expect the access check failure for fromrepo/bbb to be added to the context as a deferred error,
			// which our blobstore will look for and prevent a cross mount from.
			access: []auth.Access{
				{Resource: auth.Resource{Type: "repository", Name: "pushrepo/aaa"}, Action: "pull"},
				{Resource: auth.Resource{Type: "repository", Name: "pushrepo/aaa"}, Action: "push"},
				{Resource: auth.Resource{Type: "repository", Name: "fromrepo/bbb"}, Action: "pull"},
			},
			basicToken: "b3BlbnNoaWZ0OmF3ZXNvbWU=",
			openshiftResponses: []response{
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Namespace: "pushrepo", Allowed: true, Reason: "authorized!"})},
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Namespace: "pushrepo", Allowed: true, Reason: "authorized!"})},
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Namespace: "fromrepo", Allowed: false, Reason: "no!"})},
			},
			expectedError:     nil,
			expectedChallenge: false,
			expectedRepoErr:   "fromrepo/bbb",
			expectedActions: []string{
				"POST /oapi/v1/namespaces/pushrepo/localsubjectaccessreviews",
				"POST /oapi/v1/namespaces/pushrepo/localsubjectaccessreviews",
				"POST /oapi/v1/namespaces/fromrepo/localsubjectaccessreviews",
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
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Namespace: "foo", Allowed: true, Reason: "authorized!"})},
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
				{200, runtime.EncodeOrDie(kapi.Codecs.LegacyCodec(registered.GroupOrDie(kapi.GroupName).GroupVersions[0]), &api.SubjectAccessReviewResponse{Allowed: true, Reason: "authorized!"})},
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
		ctx := context.WithValue(context.Background(), "http.request", req)

		server, actions := simulateOpenShiftMaster(test.openshiftResponses)
		DefaultRegistryClient = NewRegistryClient(&clientcmd.Config{
			CommonConfig: restclient.Config{
				Host:     server.URL,
				Insecure: true,
			},
			SkipEnv: true,
		})
		accessController, err := newAccessController(options)
		if err != nil {
			t.Fatal(err)
		}
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
			if !AuthPerformed(authCtx) {
				t.Errorf("%s: expected AuthPerformed to be true", k)
				continue
			}
			deferredErrors, hasDeferred := DeferredErrorsFrom(authCtx)
			if len(test.expectedRepoErr) > 0 {
				if !hasDeferred || deferredErrors[test.expectedRepoErr] == nil {
					t.Errorf("%s: expected deferred error for repo %s, got none", k, test.expectedRepoErr)
					continue
				}
			} else {
				if hasDeferred && len(deferredErrors) > 0 {
					t.Errorf("%s: didn't expect deferred errors, got %#v", k, deferredErrors)
					continue
				}
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(response.code)
		fmt.Fprintln(w, response.body)
		actions = append(actions, r.Method+" "+r.URL.Path)
	}))
	return server, &actions
}
