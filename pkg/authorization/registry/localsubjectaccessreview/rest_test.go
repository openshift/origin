package localsubjectaccessreview

import (
	"errors"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
)

type subjectAccessTest struct {
	authorizer     *testAuthorizer
	reviewRequest  *authorizationapi.LocalSubjectAccessReview
	requestingUser *user.DefaultInfo

	expectedUserInfo *user.DefaultInfo
	expectedError    string
}

type testAuthorizer struct {
	allowed bool
	reason  string
	err     string

	actualAttributes authorizer.DefaultAuthorizationAttributes
	actualUserInfo   user.Info
}

func (a *testAuthorizer) Authorize(ctx kapi.Context, passedAttributes authorizer.Action) (allowed bool, reason string, err error) {
	a.actualUserInfo, _ = kapi.UserFrom(ctx)

	// allow the initial check for "can I run this SAR at all"
	if passedAttributes.GetResource() == "localsubjectaccessreviews" {
		return true, "", nil
	}

	attributes, ok := passedAttributes.(authorizer.DefaultAuthorizationAttributes)
	if !ok {
		return false, "ERROR", errors.New("unexpected type for test")
	}

	a.actualAttributes = attributes

	if len(a.err) == 0 {
		return a.allowed, a.reason, nil
	}
	return a.allowed, a.reason, errors.New(a.err)
}
func (a *testAuthorizer) GetAllowedSubjects(ctx kapi.Context, passedAttributes authorizer.Action) (sets.String, sets.String, error) {
	return sets.String{}, sets.String{}, nil
}

func TestNoNamespace(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			allowed: false,
		},
		reviewRequest: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{
				Namespace: "",
				Verb:      "get",
				Resource:  "pods",
			},
			User:   "foo",
			Groups: sets.NewString(),
		},
		expectedError: "namespace is required on this type: ",
	}

	test.runTest(t)
}

func TestConflictingNamespace(t *testing.T) {
	authorizer := &testAuthorizer{
		allowed: false,
	}
	reviewRequest := &authorizationapi.LocalSubjectAccessReview{
		Action: authorizationapi.Action{
			Namespace: "foo",
			Verb:      "get",
			Resource:  "pods",
		},
		User:   "foo",
		Groups: sets.NewString(),
	}

	storage := NewREST(subjectaccessreview.NewRegistry(subjectaccessreview.NewREST(authorizer)))
	ctx := kapi.WithNamespace(kapi.NewContext(), "bar")
	_, err := storage.Create(ctx, reviewRequest)
	if err == nil {
		t.Fatalf("unexpected non-error: %v", err)
	}
	if e, a := `namespace: Invalid value: "foo": namespace must be: bar`, err.Error(); e != a {
		t.Fatalf("expected %v, got %v", e, a)
	}
}

func TestEmptyReturn(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			allowed: false,
			reason:  "because reasons",
		},
		reviewRequest: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{
				Namespace: "unittest",
				Verb:      "get",
				Resource:  "pods",
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
		reviewRequest: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{
				Namespace: "unittest",
				Verb:      "delete",
				Resource:  "deploymentConfigs",
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
		reviewRequest: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{
				Namespace: "unittest",
				Verb:      "get",
				Resource:  "pods",
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
		reviewRequest: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{
				Namespace: "unittest",
				Verb:      "delete",
				Resource:  "deploymentConfigs",
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
		reviewRequest: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{
				Namespace: "unittest",
				Verb:      "delete",
				Resource:  "deploymentConfigs",
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
		reviewRequest: &authorizationapi.LocalSubjectAccessReview{
			Action: authorizationapi.Action{
				Namespace: "unittest",
				Verb:      "delete",
				Resource:  "deploymentConfigs",
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
	storage := NewREST(subjectaccessreview.NewRegistry(subjectaccessreview.NewREST(r.authorizer)))

	expectedResponse := &authorizationapi.SubjectAccessReviewResponse{
		Namespace:       r.reviewRequest.Action.Namespace,
		Allowed:         r.authorizer.allowed,
		Reason:          r.authorizer.reason,
		EvaluationError: r.authorizer.err,
	}

	expectedAttributes := authorizer.ToDefaultAuthorizationAttributes(r.reviewRequest.Action)

	ctx := kapi.WithNamespace(kapi.NewContext(), r.reviewRequest.Action.Namespace)
	if r.requestingUser != nil {
		ctx = kapi.WithUser(ctx, r.requestingUser)
	}

	obj, err := storage.Create(ctx, r.reviewRequest)
	switch {
	case err == nil && len(r.expectedError) == 0:
	case err == nil && len(r.expectedError) != 0:
		t.Fatalf("missing expected error: %v", r.expectedError)
	case err != nil && len(r.expectedError) == 0:
		t.Fatalf("unexpected error: %v", r.expectedError)
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

	if !reflect.DeepEqual(expectedAttributes, r.authorizer.actualAttributes) {
		t.Errorf("diff %v", diff.ObjectGoPrintDiff(expectedAttributes, r.authorizer.actualAttributes))
	}

	if !reflect.DeepEqual(r.expectedUserInfo, r.authorizer.actualUserInfo) {
		t.Errorf("diff %v", diff.ObjectGoPrintDiff(r.expectedUserInfo, r.authorizer.actualUserInfo))
	}

}
