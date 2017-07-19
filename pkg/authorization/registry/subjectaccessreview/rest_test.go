package subjectaccessreview

import (
	"errors"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apiserver/pkg/authentication/user"
	kauthorizer "k8s.io/apiserver/pkg/authorization/authorizer"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

type subjectAccessTest struct {
	authorizer     *testAuthorizer
	reviewRequest  *authorizationapi.SubjectAccessReview
	requestingUser *user.DefaultInfo

	expectedUserInfo *user.DefaultInfo
	expectedError    string
}

type testAuthorizer struct {
	allowed          bool
	reason           string
	err              string
	deniedNamespaces sets.String

	actualAttributes kauthorizer.Attributes
}

func (a *testAuthorizer) Authorize(passedAttributes kauthorizer.Attributes) (allowed bool, reason string, err error) {
	// allow the initial check for "can I run this SAR at all"
	if passedAttributes.GetResource() == "localsubjectaccessreviews" {
		if len(a.deniedNamespaces) != 0 && a.deniedNamespaces.Has(passedAttributes.GetNamespace()) {
			return false, "denied initial check", nil
		}

		return true, "", nil
	}

	a.actualAttributes = passedAttributes

	if len(a.err) == 0 {
		return a.allowed, a.reason, nil
	}
	return a.allowed, a.reason, errors.New(a.err)
}
func (a *testAuthorizer) GetAllowedSubjects(passedAttributes kauthorizer.Attributes) (sets.String, sets.String, error) {
	return sets.String{}, sets.String{}, nil
}

func TestDeniedNamespace(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			allowed:          false,
			err:              "denied initial check",
			deniedNamespaces: sets.NewString("foo"),
		},
		reviewRequest: &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Namespace: "foo",
				Verb:      "get",
				Resource:  "pods",
			},
			User:   "foo",
			Groups: sets.NewString(),
		},
		expectedError: "denied initial check",
	}

	test.runTest(t)
}

func TestEmptyReturn(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			allowed: false,
			reason:  "because reasons",
		},
		reviewRequest: &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "get",
				Resource: "pods",
			},
			User:   "foo",
			Groups: sets.NewString(),
		},
		expectedUserInfo: &user.DefaultInfo{
			Name:   "foo",
			Groups: []string{},
			Extra:  map[string][]string{},
		},
	}

	test.runTest(t)
}

func TestNoErrors(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			allowed: true,
			reason:  "because good things",
		},
		reviewRequest: &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "delete",
				Resource: "deploymentConfigs",
			},
			Groups: sets.NewString("not-master"),
		},
		expectedUserInfo: &user.DefaultInfo{
			Name:   "",
			Groups: []string{"not-master"},
			Extra:  map[string][]string{},
		},
	}

	test.runTest(t)
}

func TestErrors(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			err: "some-random-failure",
		},
		reviewRequest: &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "get",
				Resource: "pods",
			},
			User:   "foo",
			Groups: sets.NewString("first", "second"),
		},
		expectedUserInfo: &user.DefaultInfo{
			Name:   "foo",
			Groups: []string{"first", "second"},
			Extra:  map[string][]string{},
		},
	}

	test.runTest(t)
}

func TestRegularWithScopes(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			allowed: true,
			reason:  "because good things",
		},
		reviewRequest: &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "delete",
				Resource: "deploymentConfigs",
			},
			Groups: sets.NewString("not-master"),
			Scopes: []string{"scope-01"},
		},
		expectedUserInfo: &user.DefaultInfo{
			Name:   "",
			Groups: []string{"not-master"},
			Extra:  map[string][]string{authorizationapi.ScopesKey: {"scope-01"}},
		},
		requestingUser: &user.DefaultInfo{
			Name:   "",
			Groups: []string{"different"},
			Extra:  map[string][]string{authorizationapi.ScopesKey: {"scope-02"}},
		},
	}

	test.runTest(t)
}
func TestSelfWithDefaultScopes(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			allowed: true,
			reason:  "because good things",
		},
		reviewRequest: &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "delete",
				Resource: "deploymentConfigs",
			},
		},
		expectedUserInfo: &user.DefaultInfo{
			Name:   "me",
			Groups: []string{"group"},
			Extra:  map[string][]string{authorizationapi.ScopesKey: {"scope-02"}},
		},
		requestingUser: &user.DefaultInfo{
			Name:   "me",
			Groups: []string{"group"},
			Extra:  map[string][]string{authorizationapi.ScopesKey: {"scope-02"}},
		},
	}

	test.runTest(t)
}

func TestSelfWithClearedScopes(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			allowed: true,
			reason:  "because good things",
		},
		reviewRequest: &authorizationapi.SubjectAccessReview{
			Action: authorizationapi.Action{
				Verb:     "delete",
				Resource: "deploymentConfigs",
			},
			Scopes: []string{},
		},
		expectedUserInfo: &user.DefaultInfo{
			Name:   "me",
			Groups: []string{"group"},
			Extra:  map[string][]string{},
		},
		requestingUser: &user.DefaultInfo{
			Name:   "me",
			Groups: []string{"group"},
			Extra:  map[string][]string{authorizationapi.ScopesKey: {"scope-02"}},
		},
	}

	test.runTest(t)
}

func (r *subjectAccessTest) runTest(t *testing.T) {
	storage := REST{r.authorizer}

	expectedResponse := &authorizationapi.SubjectAccessReviewResponse{
		Namespace:       r.reviewRequest.Action.Namespace,
		Allowed:         r.authorizer.allowed,
		Reason:          r.authorizer.reason,
		EvaluationError: r.authorizer.err,
	}

	ctx := apirequest.WithNamespace(apirequest.NewContext(), kapi.NamespaceAll)
	if r.requestingUser != nil {
		ctx = apirequest.WithUser(ctx, r.requestingUser)
	} else {
		ctx = apirequest.WithUser(ctx, &user.DefaultInfo{Name: "dummy"})
	}

	obj, err := storage.Create(ctx, r.reviewRequest, false)
	switch {
	case err == nil && len(r.expectedError) == 0:
	case err == nil && len(r.expectedError) != 0:
		t.Fatalf("missing expected error: %v", r.expectedError)
	case err != nil && len(r.expectedError) == 0:
		t.Fatalf("unexpected error: %v", err)
	case err != nil && len(r.expectedError) == 0 && err.Error() != r.expectedError:
		t.Fatalf("unexpected error: %v", r.expectedError)
	}
	if len(r.expectedError) > 0 {
		return
	}

	switch obj.(type) {
	case *authorizationapi.SubjectAccessReviewResponse:
		if !reflect.DeepEqual(expectedResponse, obj) {
			t.Errorf("diff %v", diff.ObjectGoPrintDiff(expectedResponse, obj))
		}

	default:
		t.Errorf("Unexpected obj type: %v", obj)
	}

	expectedAttributes := authorizer.ToDefaultAuthorizationAttributes(r.expectedUserInfo, kapi.NamespaceAll, r.reviewRequest.Action)
	if !reflect.DeepEqual(expectedAttributes, r.authorizer.actualAttributes) {
		t.Errorf("diff %v", diff.ObjectGoPrintDiff(expectedAttributes, r.authorizer.actualAttributes))
	}
}
