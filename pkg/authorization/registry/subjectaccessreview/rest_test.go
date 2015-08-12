package subjectaccessreview

import (
	"errors"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

type subjectAccessTest struct {
	authorizer    *testAuthorizer
	reviewRequest *authorizationapi.SubjectAccessReview
}

type testAuthorizer struct {
	allowed bool
	reason  string
	err     string

	actualAttributes *authorizer.DefaultAuthorizationAttributes
}

func (a *testAuthorizer) Authorize(ctx kapi.Context, passedAttributes authorizer.AuthorizationAttributes) (allowed bool, reason string, err error) {
	attributes, ok := passedAttributes.(*authorizer.DefaultAuthorizationAttributes)
	if !ok {
		return false, "ERROR", errors.New("unexpected type for test")
	}

	a.actualAttributes = attributes

	if len(a.err) == 0 {
		return a.allowed, a.reason, nil
	}
	return a.allowed, a.reason, errors.New(a.err)
}
func (a *testAuthorizer) GetAllowedSubjects(ctx kapi.Context, passedAttributes authorizer.AuthorizationAttributes) (util.StringSet, util.StringSet, error) {
	return util.StringSet{}, util.StringSet{}, nil
}

func TestEmptyReturn(t *testing.T) {
	test := &subjectAccessTest{
		authorizer: &testAuthorizer{
			allowed: false,
			reason:  "because reasons",
		},
		reviewRequest: &authorizationapi.SubjectAccessReview{
			User:     "foo",
			Groups:   util.NewStringSet(),
			Verb:     "get",
			Resource: "pods",
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
			Groups:   util.NewStringSet("not-master"),
			Verb:     "delete",
			Resource: "deploymentConfigs",
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
			User:     "foo",
			Groups:   util.NewStringSet("first", "second"),
			Verb:     "get",
			Resource: "pods",
		},
	}

	test.runTest(t)
}

func (r *subjectAccessTest) runTest(t *testing.T) {
	const namespace = "unittest"

	storage := REST{r.authorizer}

	expectedResponse := &authorizationapi.SubjectAccessReviewResponse{
		Namespace: namespace,
		Allowed:   r.authorizer.allowed,
		Reason:    r.authorizer.reason,
	}

	expectedAttributes := &authorizer.DefaultAuthorizationAttributes{
		Verb:     r.reviewRequest.Verb,
		Resource: r.reviewRequest.Resource,
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	obj, err := storage.Create(ctx, r.reviewRequest)
	if err != nil && len(r.authorizer.err) == 0 {
		t.Fatalf("unexpected error: %v", err)
	}
	if err == nil && len(r.authorizer.err) != 0 {
		t.Fatalf("unexpected non-error: %v", err)
	}

	switch obj.(type) {
	case *authorizationapi.SubjectAccessReviewResponse:
		if !reflect.DeepEqual(expectedResponse, obj) {
			t.Errorf("diff %v", util.ObjectGoPrintDiff(expectedResponse, obj))
		}
	case nil:
		if len(r.authorizer.err) == 0 {
			t.Fatal("unexpected nil object")
		}

	default:
		t.Errorf("Unexpected obj type: %v", obj)
	}

	if !reflect.DeepEqual(expectedAttributes, r.authorizer.actualAttributes) {
		t.Errorf("diff %v", util.ObjectGoPrintDiff(expectedAttributes, r.authorizer.actualAttributes))
	}
}
