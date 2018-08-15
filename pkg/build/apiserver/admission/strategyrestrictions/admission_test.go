package strategyrestrictions

import (
	"fmt"
	"testing"

	authorizationv1 "k8s.io/api/authorization/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	fakekubeclient "k8s.io/client-go/kubernetes/fake"
	clientgotesting "k8s.io/client-go/testing"

	buildapiv1 "github.com/openshift/api/build/v1"
	fakebuildclient "github.com/openshift/client-go/build/clientset/versioned/fake"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"

	"github.com/openshift/api/build"
	_ "github.com/openshift/origin/pkg/build/apis/build/install"
)

func TestBuildAdmission(t *testing.T) {
	tests := []struct {
		name                string
		kind                schema.GroupKind
		resource            schema.GroupResource
		subResource         string
		object              runtime.Object
		oldObject           runtime.Object
		responseObject      runtime.Object
		reviewResponse      *authorizationv1.SubjectAccessReview
		expectedResource    string
		expectedSubresource string
		expectAccept        bool
		expectedError       string
	}{
		{
			name:                "allowed source build",
			object:              internalTestBuild(buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{}}),
			kind:                build.Kind("Build"),
			resource:            build.Resource("builds"),
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "source",
			expectAccept:        true,
		},
		{
			name:                "allowed source build clone",
			object:              testBuildRequest("test-build"),
			responseObject:      v1TestBuild(buildapiv1.BuildStrategy{SourceStrategy: &buildapiv1.SourceBuildStrategy{}}),
			kind:                build.Kind("Build"),
			resource:            build.Resource("builds"),
			subResource:         "clone",
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "source",
			expectAccept:        true,
		},
		{
			name:                "denied docker build",
			object:              internalTestBuild(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:                build.Kind("Build"),
			resource:            build.Resource("builds"),
			reviewResponse:      reviewResponse(false, "cannot create build of type docker build"),
			expectAccept:        false,
			expectedResource:    "builds",
			expectedSubresource: "docker",
		},
		{
			name:                "denied docker build clone",
			object:              testBuildRequest("buildname"),
			responseObject:      v1TestBuild(buildapiv1.BuildStrategy{DockerStrategy: &buildapiv1.DockerBuildStrategy{}}),
			kind:                build.Kind("Build"),
			resource:            build.Resource("builds"),
			subResource:         "clone",
			reviewResponse:      reviewResponse(false, "cannot create build of type docker build"),
			expectAccept:        false,
			expectedResource:    "builds",
			expectedSubresource: "docker",
		},
		{
			name:                "allowed custom build",
			object:              internalTestBuild(buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{}}),
			kind:                build.Kind("Build"),
			resource:            build.Resource("builds"),
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "custom",
			expectAccept:        true,
		},
		{
			name:                "allowed build config",
			object:              internalTestBuildConfig(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:                build.Kind("BuildConfig"),
			resource:            build.Resource("buildconfigs"),
			reviewResponse:      reviewResponse(true, ""),
			expectAccept:        true,
			expectedResource:    "builds",
			expectedSubresource: "docker",
		},
		{
			name:                "allowed build config instantiate",
			responseObject:      v1TestBuildConfig(buildapiv1.BuildStrategy{DockerStrategy: &buildapiv1.DockerBuildStrategy{}}),
			object:              testBuildRequest("test-buildconfig"),
			kind:                build.Kind("Build"),
			resource:            build.Resource("buildconfigs"),
			subResource:         "instantiate",
			reviewResponse:      reviewResponse(true, ""),
			expectAccept:        true,
			expectedResource:    "builds",
			expectedSubresource: "docker",
		},
		{
			name:                "forbidden build config",
			object:              internalTestBuildConfig(buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{}}),
			kind:                build.Kind("Build"),
			resource:            build.Resource("buildconfigs"),
			reviewResponse:      reviewResponse(false, ""),
			expectAccept:        false,
			expectedResource:    "builds",
			expectedSubresource: "custom",
		},
		{
			name:                "forbidden build config instantiate",
			responseObject:      v1TestBuildConfig(buildapiv1.BuildStrategy{CustomStrategy: &buildapiv1.CustomBuildStrategy{}}),
			object:              testBuildRequest("buildname"),
			kind:                build.Kind("Build"),
			resource:            build.Resource("buildconfigs"),
			subResource:         "instantiate",
			reviewResponse:      reviewResponse(false, ""),
			expectAccept:        false,
			expectedResource:    "builds",
			expectedSubresource: "custom",
		},
		{
			name:           "unrecognized request object",
			object:         &fakeObject{},
			kind:           build.Kind("BuildConfig"),
			resource:       build.Resource("buildconfigs"),
			reviewResponse: reviewResponse(true, ""),
			expectAccept:   false,
			expectedError:  "Internal error occurred: [Unrecognized request object &admission.fakeObject{}, couldn't find ObjectMeta field in admission.fakeObject{}]",
		},
		{
			name:           "details on forbidden docker build",
			object:         internalTestBuild(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:           build.Kind("Build"),
			resource:       build.Resource("builds"),
			subResource:    "details",
			reviewResponse: reviewResponse(false, "cannot create build of type docker build"),
			expectAccept:   true,
		},
		{
			name:                "allowed jenkins pipeline build",
			object:              internalTestBuild(buildapi.BuildStrategy{JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{}}),
			kind:                build.Kind("Build"),
			resource:            build.Resource("builds"),
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "jenkinspipeline",
			expectAccept:        true,
		},
		{
			name:                "allowed jenkins pipeline build clone",
			object:              testBuildRequest("test-build"),
			responseObject:      v1TestBuild(buildapiv1.BuildStrategy{JenkinsPipelineStrategy: &buildapiv1.JenkinsPipelineBuildStrategy{}}),
			kind:                build.Kind("Build"),
			resource:            build.Resource("builds"),
			subResource:         "clone",
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "jenkinspipeline",
			expectAccept:        true,
		},
	}

	emptyResponse := &authorizationv1.SubjectAccessReview{}
	ops := []admission.Operation{admission.Create, admission.Update}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, op := range ops {
				fakeBuildClient := fakebuildclient.NewSimpleClientset()
				if test.responseObject != nil {
					fakeBuildClient = fakebuildclient.NewSimpleClientset(test.responseObject)
				}

				fakeKubeClient := fakekubeclient.NewSimpleClientset()
				fakeKubeClient.PrependReactor("create", "subjectaccessreviews", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
					review, ok := action.(clientgotesting.CreateAction).GetObject().(*authorizationv1.SubjectAccessReview)
					if !ok {
						return true, emptyResponse, fmt.Errorf("unexpected object received: %#v", review)
					}
					if review.Spec.ResourceAttributes.Group != buildapi.GroupName {
						return true, emptyResponse, fmt.Errorf("unexpected group received: %s. expected: %s",
							review.Spec.ResourceAttributes.Group, buildapi.GroupName)
					}
					if review.Spec.ResourceAttributes.Resource != test.expectedResource {
						return true, emptyResponse, fmt.Errorf("unexpected resource received: %s. expected: %s",
							review.Spec.ResourceAttributes.Resource, test.expectedResource)
					}
					if review.Spec.ResourceAttributes.Subresource != test.expectedSubresource {
						return true, emptyResponse, fmt.Errorf("unexpected subresource received: %s. expected: %s",
							review.Spec.ResourceAttributes.Subresource, test.expectedSubresource)
					}
					return true, test.reviewResponse, nil
				})

				c := NewBuildByStrategy()
				c.(*buildByStrategy).sarClient = fakeKubeClient.AuthorizationV1().SubjectAccessReviews()
				c.(*buildByStrategy).buildClient = fakeBuildClient
				attrs := admission.NewAttributesRecord(test.object, test.oldObject, test.kind.WithVersion("version"), "foo", "test-build", test.resource.WithVersion("version"), test.subResource, op, fakeUser())
				err := c.(admission.MutationInterface).Admit(attrs)
				if err != nil && test.expectAccept {
					t.Errorf("unexpected error: %v", err)
				}

				if !apierrors.IsForbidden(err) && !test.expectAccept {
					if (len(test.expectedError) != 0) || (test.expectedError == err.Error()) {
						continue
					}
					t.Errorf("expecting reject error, got %v", err)
				}
			}
		})
	}
}

type fakeObject struct{}

func (*fakeObject) GetObjectKind() schema.ObjectKind { return nil }

func (*fakeObject) DeepCopyObject() runtime.Object { return nil }

func fakeUser() user.Info {
	return &user.DefaultInfo{
		Name: "testuser",
	}
}

func internalTestBuild(strategy buildapi.BuildStrategy) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "test-build",
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Strategy: strategy,
			},
		},
	}
}

func v1TestBuild(strategy buildapiv1.BuildStrategy) *buildapiv1.Build {
	return &buildapiv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "test-build",
		},
		Spec: buildapiv1.BuildSpec{
			CommonSpec: buildapiv1.CommonSpec{
				Strategy: strategy,
			},
		},
	}
}

func internalTestBuildConfig(strategy buildapi.BuildStrategy) *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "test-buildconfig",
		},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Strategy: strategy,
			},
		},
	}
}

func v1TestBuildConfig(strategy buildapiv1.BuildStrategy) *buildapiv1.BuildConfig {
	return &buildapiv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "test-buildconfig",
		},
		Spec: buildapiv1.BuildConfigSpec{
			CommonSpec: buildapiv1.CommonSpec{
				Strategy: strategy,
			},
		},
	}
}

func reviewResponse(allowed bool, msg string) *authorizationv1.SubjectAccessReview {
	return &authorizationv1.SubjectAccessReview{
		Status: authorizationv1.SubjectAccessReviewStatus{
			Allowed: allowed,
			Reason:  msg,
		},
	}
}

func testBuildRequest(name string) runtime.Object {
	return &buildapi.BuildRequest{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      name,
		},
	}
}
