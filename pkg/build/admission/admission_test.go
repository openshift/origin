package admission

import (
	"fmt"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/testclient"
)

func TestBuildAdmission(t *testing.T) {
	tests := []struct {
		name             string
		kind             string
		resource         string
		subResource      string
		object           runtime.Object
		responseObject   runtime.Object
		reviewResponse   *authorizationapi.SubjectAccessReviewResponse
		expectedResource string
		expectAccept     bool
	}{
		{
			name:             "allowed source build",
			object:           testBuild(buildapi.SourceBuildStrategyType),
			kind:             "Build",
			resource:         buildsResource,
			reviewResponse:   reviewResponse(true, ""),
			expectedResource: authorizationapi.SourceBuildResource,
			expectAccept:     true,
		},
		{
			name:             "allowed source build clone",
			object:           testBuildRequest("buildname"),
			responseObject:   testBuild(buildapi.SourceBuildStrategyType),
			kind:             "Build",
			resource:         buildsResource,
			subResource:      "clone",
			reviewResponse:   reviewResponse(true, ""),
			expectedResource: authorizationapi.SourceBuildResource,
			expectAccept:     true,
		},
		{
			name:             "denied docker build",
			object:           testBuild(buildapi.DockerBuildStrategyType),
			kind:             "Build",
			resource:         buildsResource,
			reviewResponse:   reviewResponse(false, "cannot create build of type docker build"),
			expectAccept:     false,
			expectedResource: authorizationapi.DockerBuildResource,
		},
		{
			name:             "denied docker build clone",
			object:           testBuildRequest("buildname"),
			responseObject:   testBuild(buildapi.DockerBuildStrategyType),
			kind:             "Build",
			resource:         buildsResource,
			subResource:      "clone",
			reviewResponse:   reviewResponse(false, "cannot create build of type docker build"),
			expectAccept:     false,
			expectedResource: authorizationapi.DockerBuildResource,
		},
		{
			name:             "allowed custom build",
			object:           testBuild(buildapi.CustomBuildStrategyType),
			kind:             "Build",
			resource:         buildsResource,
			reviewResponse:   reviewResponse(true, ""),
			expectedResource: authorizationapi.CustomBuildResource,
			expectAccept:     true,
		},
		{
			name:             "allowed build config",
			object:           testBuildConfig(buildapi.DockerBuildStrategyType),
			kind:             "BuildConfig",
			resource:         buildConfigsResource,
			reviewResponse:   reviewResponse(true, ""),
			expectAccept:     true,
			expectedResource: authorizationapi.DockerBuildResource,
		},
		{
			name:             "allowed build config instantiate",
			responseObject:   testBuildConfig(buildapi.DockerBuildStrategyType),
			object:           testBuildRequest("buildname"),
			kind:             "BuildConfig",
			resource:         buildConfigsResource,
			subResource:      "instantiate",
			reviewResponse:   reviewResponse(true, ""),
			expectAccept:     true,
			expectedResource: authorizationapi.DockerBuildResource,
		},
		{
			name:             "forbidden build config",
			object:           testBuildConfig(buildapi.CustomBuildStrategyType),
			kind:             "BuildConfig",
			resource:         buildConfigsResource,
			reviewResponse:   reviewResponse(false, ""),
			expectAccept:     false,
			expectedResource: authorizationapi.CustomBuildResource,
		},
		{
			name:             "forbidden build config instantiate",
			responseObject:   testBuildConfig(buildapi.CustomBuildStrategyType),
			object:           testBuildRequest("buildname"),
			kind:             "BuildConfig",
			resource:         buildConfigsResource,
			subResource:      "instantiate",
			reviewResponse:   reviewResponse(false, ""),
			expectAccept:     false,
			expectedResource: authorizationapi.CustomBuildResource,
		},
		{
			name:           "unrecognized request object",
			object:         &fakeObject{},
			kind:           "BuildConfig",
			resource:       buildConfigsResource,
			reviewResponse: reviewResponse(true, ""),
			expectAccept:   false,
		},
	}

	for _, test := range tests {
		c := NewBuildByStrategy(fakeClient(test.expectedResource, test.reviewResponse, test.responseObject))
		attrs := admission.NewAttributesRecord(test.object, test.kind, "default", "name", test.resource, test.subResource, admission.Create, fakeUser())
		err := c.Admit(attrs)
		if err != nil && test.expectAccept {
			t.Errorf("%s: unexpected error: %v", test.name, err)
		}
		if (err == nil || !apierrors.IsForbidden(err)) && !test.expectAccept {
			t.Errorf("%s: expecting reject error", test.name)
		}
	}
}

type fakeObject struct{}

func (*fakeObject) IsAnAPIObject() {}

func fakeUser() user.Info {
	return &user.DefaultInfo{
		Name: "testuser",
	}
}

func fakeClient(expectedResource string, reviewResponse *authorizationapi.SubjectAccessReviewResponse, obj runtime.Object) client.Interface {
	emptyResponse := &authorizationapi.SubjectAccessReviewResponse{}
	return &testclient.Fake{
		ReactFn: func(action ktestclient.Action) (runtime.Object, error) {
			switch {
			case action.Matches("create", "subjectaccessreviews"):
				review, ok := action.(ktestclient.CreateAction).GetObject().(*authorizationapi.SubjectAccessReview)
				if !ok {
					return emptyResponse, fmt.Errorf("unexpected object received: %#v", review)
				}
				if review.Resource != expectedResource {
					return emptyResponse, fmt.Errorf("unexpected resource received: %s. expected: %s",
						review.Resource, expectedResource)
				}
				return reviewResponse, nil
			case action.Matches("get", "buildconfigs"):
				return obj, nil
			case action.Matches("get", "builds"):
				return obj, nil
			}
			return nil, nil
		},
	}
}

func testBuild(strategy buildapi.BuildStrategyType) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build",
		},
		Spec: buildapi.BuildSpec{
			Strategy: buildapi.BuildStrategy{
				Type: strategy,
			},
		},
	}
}

func testBuildConfig(strategy buildapi.BuildStrategyType) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-buildconfig",
		},
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
				Strategy: buildapi.BuildStrategy{
					Type: strategy,
				},
			},
		},
	}
}

func reviewResponse(allowed bool, msg string) *authorizationapi.SubjectAccessReviewResponse {
	return &authorizationapi.SubjectAccessReviewResponse{
		Allowed: allowed,
		Reason:  msg,
	}
}

func testBuildRequest(name string) runtime.Object {
	return &buildapi.BuildRequest{
		ObjectMeta: kapi.ObjectMeta{
			Name: name,
		},
	}
}
