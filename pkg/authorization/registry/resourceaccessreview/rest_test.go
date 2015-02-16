package resourceaccessreview

import (
	"errors"
	"reflect"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

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

func (a *testAuthorizer) Authorize(attributes authorizer.AuthorizationAttributes) (allowed bool, reason string, err error) {
	return false, "", errors.New("unsupported")
}
func (a *testAuthorizer) GetAllowedSubjects(passedAttributes authorizer.AuthorizationAttributes) ([]string, []string, error) {
	attributes, ok := passedAttributes.(*authorizer.DefaultAuthorizationAttributes)
	if !ok {
		return nil, nil, errors.New("unexpected type for test")
	}

	a.actualAttributes = attributes
	if len(a.err) == 0 {
		return a.users.List(), a.groups.List(), nil
	}
	return a.users.List(), a.groups.List(), errors.New(a.err)
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
		Users:     r.authorizer.users.List(),
		Groups:    r.authorizer.groups.List(),
	}

	expectedAttributes := &authorizer.DefaultAuthorizationAttributes{
		Verb:      r.reviewRequest.Verb,
		Resource:  r.reviewRequest.Resource,
		Namespace: namespace,
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), namespace)
	channel, err := storage.Create(ctx, r.reviewRequest)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case result := <-channel:
		switch obj := result.Object.(type) {
		case *kapi.Status:
			if len(r.authorizer.err) == 0 {
				t.Errorf("Unexpected operation error: %v", obj)
			}

		case *authorizationapi.ResourceAccessReviewResponse:
			if !reflect.DeepEqual(expectedResponse, obj) {
				t.Errorf("diff %v", util.ObjectGoPrintDiff(expectedResponse, obj))
			}

		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}

	if !reflect.DeepEqual(expectedAttributes, r.authorizer.actualAttributes) {
		t.Errorf("diff %v", util.ObjectGoPrintDiff(expectedAttributes, r.authorizer.actualAttributes))
	}
}
