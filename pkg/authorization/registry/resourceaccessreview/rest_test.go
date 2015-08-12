package resourceaccessreview

import (
	"errors"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

type resourceAccessTest struct {
	authorizer    *testAuthorizer
	reviewRequest *authorizationapi.ResourceAccessReview
}

type testAuthorizer struct {
	users  util.StringSet
	groups util.StringSet
	err    string

	actualAttributes *authorizer.DefaultAuthorizationAttributes
}

func (a *testAuthorizer) Authorize(ctx kapi.Context, attributes authorizer.AuthorizationAttributes) (allowed bool, reason string, err error) {
	return false, "", errors.New("unsupported")
}
func (a *testAuthorizer) GetAllowedSubjects(ctx kapi.Context, passedAttributes authorizer.AuthorizationAttributes) (util.StringSet, util.StringSet, error) {
	attributes, ok := passedAttributes.(*authorizer.DefaultAuthorizationAttributes)
	if !ok {
		return nil, nil, errors.New("unexpected type for test")
	}

	a.actualAttributes = attributes
	if len(a.err) == 0 {
		return a.users, a.groups, nil
	}
	return a.users, a.groups, errors.New(a.err)
}

func TestEmptyReturn(t *testing.T) {
	test := &resourceAccessTest{
		authorizer: &testAuthorizer{
			users:  util.StringSet{},
			groups: util.StringSet{},
		},
		reviewRequest: &authorizationapi.ResourceAccessReview{
			Verb:     "get",
			Resource: "pods",
		},
	}

	test.runTest(t)
}

func TestNoErrors(t *testing.T) {
	test := &resourceAccessTest{
		authorizer: &testAuthorizer{
			users:  util.NewStringSet("one", "two"),
			groups: util.NewStringSet("three", "four"),
		},
		reviewRequest: &authorizationapi.ResourceAccessReview{
			Verb:     "delete",
			Resource: "deploymentConfig",
		},
	}

	test.runTest(t)
}

func TestErrors(t *testing.T) {
	test := &resourceAccessTest{
		authorizer: &testAuthorizer{
			users:  util.StringSet{},
			groups: util.StringSet{},
			err:    "some-random-failure",
		},
		reviewRequest: &authorizationapi.ResourceAccessReview{
			Verb:     "get",
			Resource: "pods",
		},
	}

	test.runTest(t)
}

func (r *resourceAccessTest) runTest(t *testing.T) {
	const namespace = "unittest"

	storage := REST{r.authorizer}

	expectedResponse := &authorizationapi.ResourceAccessReviewResponse{
		Namespace: namespace,
		Users:     r.authorizer.users,
		Groups:    r.authorizer.groups,
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
	case *authorizationapi.ResourceAccessReviewResponse:
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
