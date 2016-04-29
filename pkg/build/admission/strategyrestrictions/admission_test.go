package strategyrestrictions

import (
	"fmt"
	"testing"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/auth/user"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/client/testclient"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
)

func TestBuildAdmission(t *testing.T) {
	tests := []struct {
		name             string
		kind             unversioned.GroupKind
		resource         unversioned.GroupResource
		subResource      string
		object           runtime.Object
		responseObject   runtime.Object
		reviewResponse   *authorizationapi.SubjectAccessReviewResponse
		expectedResource string
		expectAccept     bool
		expectedError    string
	}{
		{
			name:             "allowed source build",
			object:           testBuild(buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{}}),
			kind:             buildapi.Kind("Build"),
			resource:         buildsResource,
			reviewResponse:   newReviewResponse(true, ""),
			expectedResource: authorizationapi.SourceBuildResource,
			expectAccept:     true,
		},
		{
			name:             "allowed source build clone",
			object:           testBuildRequest("buildname"),
			responseObject:   testBuild(buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{}}),
			kind:             buildapi.Kind("Build"),
			resource:         buildsResource,
			subResource:      "clone",
			reviewResponse:   newReviewResponse(true, ""),
			expectedResource: authorizationapi.SourceBuildResource,
			expectAccept:     true,
		},
		{
			name:             "denied docker build",
			object:           testBuild(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:             buildapi.Kind("Build"),
			resource:         buildsResource,
			reviewResponse:   newReviewResponse(false, "cannot create build of type docker build"),
			expectAccept:     false,
			expectedResource: authorizationapi.DockerBuildResource,
		},
		{
			name:             "denied docker build clone",
			object:           testBuildRequest("buildname"),
			responseObject:   testBuild(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:             buildapi.Kind("Build"),
			resource:         buildsResource,
			subResource:      "clone",
			reviewResponse:   newReviewResponse(false, "cannot create build of type docker build"),
			expectAccept:     false,
			expectedResource: authorizationapi.DockerBuildResource,
		},
		{
			name:             "allowed custom build",
			object:           testBuild(buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{}}),
			kind:             buildapi.Kind("Build"),
			resource:         buildsResource,
			reviewResponse:   newReviewResponse(true, ""),
			expectedResource: authorizationapi.CustomBuildResource,
			expectAccept:     true,
		},
		{
			name:             "allowed build config",
			object:           testBuildConfig(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:             buildapi.Kind("BuildConfig"),
			resource:         buildConfigsResource,
			reviewResponse:   newReviewResponse(true, ""),
			expectAccept:     true,
			expectedResource: authorizationapi.DockerBuildResource,
		},
		{
			name:             "allowed build config instantiate",
			responseObject:   testBuildConfig(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			object:           testBuildRequest("buildname"),
			kind:             buildapi.Kind("Build"),
			resource:         buildConfigsResource,
			subResource:      "instantiate",
			reviewResponse:   newReviewResponse(true, ""),
			expectAccept:     true,
			expectedResource: authorizationapi.DockerBuildResource,
		},
		{
			name:             "forbidden build config",
			object:           testBuildConfig(buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{}}),
			kind:             buildapi.Kind("Build"),
			resource:         buildConfigsResource,
			reviewResponse:   newReviewResponse(false, ""),
			expectAccept:     false,
			expectedResource: authorizationapi.CustomBuildResource,
		},
		{
			name:             "forbidden build config instantiate",
			responseObject:   testBuildConfig(buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{}}),
			object:           testBuildRequest("buildname"),
			kind:             buildapi.Kind("Build"),
			resource:         buildConfigsResource,
			subResource:      "instantiate",
			reviewResponse:   newReviewResponse(false, ""),
			expectAccept:     false,
			expectedResource: authorizationapi.CustomBuildResource,
		},
		{
			name:           "unrecognized request object",
			object:         &fakeObject{},
			kind:           buildapi.Kind("BuildConfig"),
			resource:       buildConfigsResource,
			reviewResponse: newReviewResponse(true, ""),
			expectAccept:   false,
			expectedError:  "Internal error occurred: [Unrecognized request object &admission.fakeObject{}, couldn't find ObjectMeta field in admission.fakeObject{}]",
		},
		{
			name:           "details on forbidden docker build",
			object:         testBuild(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:           buildapi.Kind("Build"),
			resource:       buildsResource,
			subResource:    "details",
			reviewResponse: newReviewResponse(false, "cannot create build of type docker build"),
			expectAccept:   true,
		},
	}

	ops := []admission.Operation{admission.Create, admission.Update}
	for _, test := range tests {
		for _, op := range ops {
			client := fakeClient(test.expectedResource, test.reviewResponse, test.responseObject)
			c := NewBuildByStrategy()
			c.(oadmission.WantsOpenshiftClient).SetOpenshiftClient(client)
			attrs := admission.NewAttributesRecord(test.object, test.kind.WithVersion("version"), "default", "name", test.resource.WithVersion("version"), test.subResource, op, fakeUser())
			err := c.Admit(attrs)
			if err != nil && test.expectAccept {
				t.Errorf("%s: unexpected error: %v", test.name, err)
			}

			if !apierrors.IsForbidden(err) && !test.expectAccept {
				if (len(test.expectedError) != 0) || (test.expectedError == err.Error()) {
					continue
				}
				t.Errorf("%s: expecting reject error, got %v", test.name, err)
			}
		}
	}
}

func TestImplicitBuildAdmission(t *testing.T) {
	tests := []struct {
		name                  string
		kind                  unversioned.GroupKind
		resource              unversioned.GroupResource
		object                runtime.Object
		dockerReviewResponse  *authorizationapi.SubjectAccessReviewResponse
		otherReviewResponse   *authorizationapi.SubjectAccessReviewResponse
		expectedResource      string
		expectDisableImplicit bool
	}{
		{
			name:                  "allowed implicit build",
			object:                testBuild(buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{DisableImplicitBuild: false}}),
			kind:                  buildapi.Kind("Build"),
			resource:              buildsResource,
			dockerReviewResponse:  newReviewResponse(true, ""),
			otherReviewResponse:   newReviewResponse(true, ""),
			expectedResource:      authorizationapi.SourceBuildResource,
			expectDisableImplicit: false,
		},
		{
			name:                  "allowed implicit build config",
			object:                testBuildConfig(buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{DisableImplicitBuild: false}}),
			kind:                  buildapi.Kind("BuildConfig"),
			resource:              buildConfigsResource,
			dockerReviewResponse:  newReviewResponse(true, ""),
			otherReviewResponse:   newReviewResponse(true, ""),
			expectedResource:      authorizationapi.SourceBuildResource,
			expectDisableImplicit: false,
		},
		{
			name:                  "disallowed implicit build",
			object:                testBuild(buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{DisableImplicitBuild: false}}),
			kind:                  buildapi.Kind("Build"),
			resource:              buildsResource,
			dockerReviewResponse:  newReviewResponse(false, ""),
			otherReviewResponse:   newReviewResponse(true, ""),
			expectedResource:      authorizationapi.SourceBuildResource,
			expectDisableImplicit: true,
		},
		{
			name:                  "disallowed implicit build config",
			object:                testBuildConfig(buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{DisableImplicitBuild: false}}),
			kind:                  buildapi.Kind("BuildConfig"),
			resource:              buildConfigsResource,
			dockerReviewResponse:  newReviewResponse(false, ""),
			otherReviewResponse:   newReviewResponse(true, ""),
			expectedResource:      authorizationapi.SourceBuildResource,
			expectDisableImplicit: true,
		},
	}

	ops := []admission.Operation{admission.Create, admission.Update}
	for _, test := range tests {
		for _, op := range ops {
			client := fakeClient2(test.expectedResource, test.dockerReviewResponse, test.otherReviewResponse, nil)
			c := NewBuildByStrategy()
			c.(oadmission.WantsOpenshiftClient).SetOpenshiftClient(client)
			attrs := admission.NewAttributesRecord(test.object, test.kind.WithVersion("version"), "default", "name", test.resource.WithVersion("version"), "", op, fakeUser())
			err := c.Admit(attrs)
			if err != nil {
				t.Errorf("%s: unexpected error: %v", test.name, err)
			}

			switch obj := attrs.GetObject().(type) {
			case *buildapi.Build:
				if obj.Spec.Strategy.SourceStrategy.DisableImplicitBuild != test.expectDisableImplicit {
					t.Errorf("%s: expected implicit value %v, got %v", test.name, test.expectDisableImplicit, obj.Spec.Strategy.SourceStrategy.DisableImplicitBuild)
				}
			case *buildapi.BuildConfig:
				if obj.Spec.Strategy.SourceStrategy.DisableImplicitBuild != test.expectDisableImplicit {
					t.Errorf("%s: expected implicit value %v, got %v", test.name, test.expectDisableImplicit, obj.Spec.Strategy.SourceStrategy.DisableImplicitBuild)
				}
			}

		}
	}
}

type fakeObject struct{}

func (*fakeObject) GetObjectKind() unversioned.ObjectKind { return nil }

func fakeUser() user.Info {
	return &user.DefaultInfo{
		Name: "testuser",
	}
}

func fakeClient(expectedResource string, reviewResponse *authorizationapi.SubjectAccessReviewResponse, obj runtime.Object) client.Interface {
	emptyResponse := &authorizationapi.SubjectAccessReviewResponse{}

	fake := &testclient.Fake{}
	fake.AddReactor("create", "localsubjectaccessreviews", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		review, ok := action.(ktestclient.CreateAction).GetObject().(*authorizationapi.LocalSubjectAccessReview)
		if !ok {
			return true, emptyResponse, fmt.Errorf("unexpected object received: %#v", review)
		}
		// source builds will see two resource checks, one against source, and the other against docker (to see if
		// implicit docker builds should be allowed for s2i builds)
		if review.Action.Resource != expectedResource &&
			(expectedResource == authorizationapi.SourceBuildResource && review.Action.Resource != authorizationapi.DockerBuildResource) {
			return true, emptyResponse, fmt.Errorf("unexpected resource received: %s. expected: %s",
				review.Action.Resource, expectedResource)
		}
		return true, reviewResponse, nil
	})
	fake.AddReactor("get", "buildconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})
	fake.AddReactor("get", "builds", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})

	return fake
}

func fakeClient2(expectedResource string, dockerReviewResponse *authorizationapi.SubjectAccessReviewResponse, otherReviewResponse *authorizationapi.SubjectAccessReviewResponse, obj runtime.Object) client.Interface {
	fake := &testclient.Fake{}
	fake.AddReactor("create", "localsubjectaccessreviews", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		review, _ := action.(ktestclient.CreateAction).GetObject().(*authorizationapi.LocalSubjectAccessReview)
		// source builds will see two resource checks, one against source, and the other against docker (to see if
		// implicit docker builds should be allowed for s2i builds)
		if review.Action.Resource == authorizationapi.DockerBuildResource {
			return true, dockerReviewResponse, nil
		}
		return true, otherReviewResponse, nil
	})
	fake.AddReactor("get", "buildconfigs", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})
	fake.AddReactor("get", "builds", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		return true, obj, nil
	})

	return fake
}

func testBuild(strategy buildapi.BuildStrategy) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-build",
		},
		Spec: buildapi.BuildSpec{
			Strategy: strategy,
		},
	}
}

func testBuildConfig(strategy buildapi.BuildStrategy) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{
			Name: "test-buildconfig",
		},
		Spec: buildapi.BuildConfigSpec{
			BuildSpec: buildapi.BuildSpec{
				Strategy: strategy,
			},
		},
	}
}

func newReviewResponse(allowed bool, msg string) *authorizationapi.SubjectAccessReviewResponse {
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
