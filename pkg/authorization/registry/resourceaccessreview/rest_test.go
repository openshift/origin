package resourceaccessreview

import (
	"errors"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

type resourceAccessTest struct {
	authorizer    *testAuthorizer
	reviewRequest *authorizationapi.ResourceAccessReview
}

type testAuthorizer struct {
	users            sets.String
	groups           sets.String
	err              string
	deniedNamespaces sets.String

	actualAttributes authorizer.DefaultAuthorizationAttributes
}

func (a *testAuthorizer) Authorize(ctx kapi.Context, attributes authorizer.AuthorizationAttributes) (allowed bool, reason string, err error) {
	// allow the initial check for "can I run this RAR at all"
	if attributes.GetResource() == "localresourceaccessreviews" {
		if len(a.deniedNamespaces) != 0 && a.deniedNamespaces.Has(kapi.NamespaceValue(ctx)) {
			return false, "denied initial check", nil
		}

		return true, "", nil
	}

	return false, "", errors.New("unsupported")
}
func (a *testAuthorizer) GetAllowedSubjects(ctx kapi.Context, passedAttributes authorizer.AuthorizationAttributes) (sets.String, sets.String, error) {
	attributes, ok := passedAttributes.(authorizer.DefaultAuthorizationAttributes)
	if !ok {
		return nil, nil, errors.New("unexpected type for test")
	}

	a.actualAttributes = attributes
	if len(a.err) == 0 {
		return a.users, a.groups, nil
	}
	return a.users, a.groups, errors.New(a.err)
}

func TestDeniedNamespace(t *testing.T) {
	test := &resourceAccessTest{
		authorizer: &testAuthorizer{
			users:            sets.String{},
			groups:           sets.String{},
			err:              "denied initial check",
			deniedNamespaces: sets.NewString("foo"),
		},
		reviewRequest: &authorizationapi.ResourceAccessReview{
			Action: authorizationapi.AuthorizationAttributes{
				Namespace: "foo",
				Verb:      "get",
				Resource:  "pods",
			},
		},
	}

	test.runTest(t)
}

func TestEmptyReturn(t *testing.T) {
	test := &resourceAccessTest{
		authorizer: &testAuthorizer{
			users:  sets.String{},
			groups: sets.String{},
		},
		reviewRequest: &authorizationapi.ResourceAccessReview{
			Action: authorizationapi.AuthorizationAttributes{
				Verb:     "get",
				Resource: "pods",
			},
		},
	}

	test.runTest(t)
}

func TestNoErrors(t *testing.T) {
	test := &resourceAccessTest{
		authorizer: &testAuthorizer{
			users:  sets.NewString("one", "two"),
			groups: sets.NewString("three", "four"),
		},
		reviewRequest: &authorizationapi.ResourceAccessReview{
			Action: authorizationapi.AuthorizationAttributes{
				Verb:     "delete",
				Resource: "deploymentConfig",
			},
		},
	}

	test.runTest(t)
}

func TestErrors(t *testing.T) {
	test := &resourceAccessTest{
		authorizer: &testAuthorizer{
			users:  sets.String{},
			groups: sets.String{},
			err:    "some-random-failure",
		},
		reviewRequest: &authorizationapi.ResourceAccessReview{
			Action: authorizationapi.AuthorizationAttributes{
				Verb:     "get",
				Resource: "pods",
			},
		},
	}

	test.runTest(t)
}

func (r *resourceAccessTest) runTest(t *testing.T) {
	storage := REST{r.authorizer}

	expectedResponse := &authorizationapi.ResourceAccessReviewResponse{
		Namespace: r.reviewRequest.Action.Namespace,
		Users:     r.authorizer.users,
		Groups:    r.authorizer.groups,
	}

	expectedAttributes := authorizer.ToDefaultAuthorizationAttributes(r.reviewRequest.Action)

	ctx := kapi.WithNamespace(kapi.NewContext(), kapi.NamespaceAll)
	obj, err := storage.Create(ctx, r.reviewRequest)
	if err != nil && len(r.authorizer.err) == 0 {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.authorizer.err) != 0 {
		if err == nil {
			t.Fatalf("unexpected non-error: %v", err)
		}
		if e, a := r.authorizer.err, err.Error(); e != a {
			t.Fatalf("expected %v, got %v", e, a)
		}

		return
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
