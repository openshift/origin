package resourceaccessreview

import (
	"errors"
	"reflect"
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/authorizer"
)

type testAuthorizer struct {
	requestedAttributes *authorizer.DefaultAuthorizationAttributes
}

func (a *testAuthorizer) Authorize(attributes authorizer.AuthorizationAttributes) (allowed bool, reason string, err error) {
	return false, "", errors.New("unsupported")
}
func (a *testAuthorizer) GetAllowedSubjects(passedAttributes authorizer.AuthorizationAttributes) ([]string, []string, error) {
	attributes, ok := passedAttributes.(*authorizer.DefaultAuthorizationAttributes)
	if !ok {
		return nil, nil, errors.New("unexpected type for test")
	}

	a.requestedAttributes = attributes
	return nil, nil, nil
}

func TestCreateValid(t *testing.T) {
	authorizer := &testAuthorizer{}
	storage := REST{authorizer}

	reviewRequest := &authorizationapi.ResourceAccessReview{
		Spec: authorizationapi.ResourceAccessReviewSpec{
			Verb:         "get",
			ResourceKind: "pods",
		},
	}
	expectedStatus := authorizationapi.ResourceAccessReviewStatus{
		Users:  make([]string, 0),
		Groups: make([]string, 0),
	}

	ctx := kapi.WithNamespace(kapi.NewContext(), "unittest")
	channel, err := storage.Create(ctx, reviewRequest)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	select {
	case result := <-channel:
		switch obj := result.Object.(type) {
		case *kapi.Status:
			t.Errorf("Unexpected operation error: %v", obj)

		case *authorizationapi.ResourceAccessReview:
			if reflect.DeepEqual(expectedStatus, obj.Status) {
				t.Errorf("expected %v, got %v", expectedStatus, obj.Status)
			}

		default:
			t.Errorf("Unexpected result type: %v", result)
		}
	case <-time.After(time.Millisecond * 100):
		t.Error("Unexpected timeout from async channel")
	}

	if authorizer.requestedAttributes.GetVerb() != reviewRequest.Spec.Verb {
		t.Errorf("expected %v, got %v", reviewRequest.Spec.Verb, authorizer.requestedAttributes.GetVerb())
	}
	if authorizer.requestedAttributes.GetResourceKind() != reviewRequest.Spec.ResourceKind {
		t.Errorf("expected %v, got %v", reviewRequest.Spec.Verb, authorizer.requestedAttributes.GetVerb())
	}
}
