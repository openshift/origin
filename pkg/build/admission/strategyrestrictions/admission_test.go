package strategyrestrictions

import (
	"fmt"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/admission"
	"k8s.io/apiserver/pkg/authentication/user"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/authorization"
	fakekubeclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	kubeadmission "k8s.io/kubernetes/pkg/kubeapiserver/admission"

	buildapiv1 "github.com/openshift/api/build/v1"
	fakebuildclient "github.com/openshift/client-go/build/clientset/versioned/fake"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"

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
		reviewResponse      *authorization.SubjectAccessReview
		expectedResource    string
		expectedSubresource string
		expectAccept        bool
		expectedError       string
	}{
		{
			name:                "allowed source build",
			object:              testBuild(buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{}}),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("builds"),
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "source",
			expectAccept:        true,
		},
		{
			name:                "allowed source build clone",
			object:              testBuildRequest("test-build"),
			responseObject:      asV1Build(testBuild(buildapi.BuildStrategy{SourceStrategy: &buildapi.SourceBuildStrategy{}})),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("builds"),
			subResource:         "clone",
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "source",
			expectAccept:        true,
		},
		{
			name:                "denied docker build",
			object:              testBuild(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("builds"),
			reviewResponse:      reviewResponse(false, "cannot create build of type docker build"),
			expectAccept:        false,
			expectedResource:    "builds",
			expectedSubresource: "docker",
		},
		{
			name:                "denied docker build clone",
			object:              testBuildRequest("buildname"),
			responseObject:      asV1Build(testBuild(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}})),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("builds"),
			subResource:         "clone",
			reviewResponse:      reviewResponse(false, "cannot create build of type docker build"),
			expectAccept:        false,
			expectedResource:    "builds",
			expectedSubresource: "docker",
		},
		{
			name:                "allowed custom build",
			object:              testBuild(buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{}}),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("builds"),
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "custom",
			expectAccept:        true,
		},
		{
			name:                "allowed build config",
			object:              testBuildConfig(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:                buildapi.Kind("BuildConfig"),
			resource:            buildapi.Resource("buildconfigs"),
			reviewResponse:      reviewResponse(true, ""),
			expectAccept:        true,
			expectedResource:    "builds",
			expectedSubresource: "docker",
		},
		{
			name:                "allowed build config instantiate",
			responseObject:      asV1BuildConfig(testBuildConfig(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}})),
			object:              testBuildRequest("test-buildconfig"),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("buildconfigs"),
			subResource:         "instantiate",
			reviewResponse:      reviewResponse(true, ""),
			expectAccept:        true,
			expectedResource:    "builds",
			expectedSubresource: "docker",
		},
		{
			name:                "forbidden build config",
			object:              testBuildConfig(buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{}}),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("buildconfigs"),
			reviewResponse:      reviewResponse(false, ""),
			expectAccept:        false,
			expectedResource:    "builds",
			expectedSubresource: "custom",
		},
		{
			name:                "forbidden build config instantiate",
			responseObject:      asV1BuildConfig(testBuildConfig(buildapi.BuildStrategy{CustomStrategy: &buildapi.CustomBuildStrategy{}})),
			object:              testBuildRequest("buildname"),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("buildconfigs"),
			subResource:         "instantiate",
			reviewResponse:      reviewResponse(false, ""),
			expectAccept:        false,
			expectedResource:    "builds",
			expectedSubresource: "custom",
		},
		{
			name:           "unrecognized request object",
			object:         &fakeObject{},
			kind:           buildapi.Kind("BuildConfig"),
			resource:       buildapi.Resource("buildconfigs"),
			reviewResponse: reviewResponse(true, ""),
			expectAccept:   false,
			expectedError:  "Internal error occurred: [Unrecognized request object &admission.fakeObject{}, couldn't find ObjectMeta field in admission.fakeObject{}]",
		},
		{
			name:           "details on forbidden docker build",
			object:         testBuild(buildapi.BuildStrategy{DockerStrategy: &buildapi.DockerBuildStrategy{}}),
			kind:           buildapi.Kind("Build"),
			resource:       buildapi.Resource("builds"),
			subResource:    "details",
			reviewResponse: reviewResponse(false, "cannot create build of type docker build"),
			expectAccept:   true,
		},
		{
			name:                "allowed jenkins pipeline build",
			object:              testBuild(buildapi.BuildStrategy{JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{}}),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("builds"),
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "jenkinspipeline",
			expectAccept:        true,
		},
		{
			name:                "allowed jenkins pipeline build clone",
			object:              testBuildRequest("test-build"),
			responseObject:      asV1Build(testBuild(buildapi.BuildStrategy{JenkinsPipelineStrategy: &buildapi.JenkinsPipelineBuildStrategy{}})),
			kind:                buildapi.Kind("Build"),
			resource:            buildapi.Resource("builds"),
			subResource:         "clone",
			reviewResponse:      reviewResponse(true, ""),
			expectedResource:    "builds",
			expectedSubresource: "jenkinspipeline",
			expectAccept:        true,
		},
	}

	emptyResponse := &authorization.SubjectAccessReview{}
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
					review, ok := action.(clientgotesting.CreateAction).GetObject().(*authorization.SubjectAccessReview)
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
				c.(kubeadmission.WantsInternalKubeClientSet).SetInternalKubeClientSet(fakeKubeClient)
				c.(oadmission.WantsOpenshiftInternalBuildClient).SetOpenshiftInternalBuildClient(fakeBuildClient)
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

func testBuild(strategy buildapi.BuildStrategy) *buildapi.Build {
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

func asV1Build(in *buildapi.Build) *buildapiv1.Build {
	out := &buildapiv1.Build{}
	err := legacyscheme.Scheme.Convert(in, out, nil)
	if err != nil {
		panic(err)
	}
	return out
}

func testBuildConfig(strategy buildapi.BuildStrategy) *buildapi.BuildConfig {
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

func asV1BuildConfig(in *buildapi.BuildConfig) *buildapiv1.BuildConfig {
	out := &buildapiv1.BuildConfig{}
	err := legacyscheme.Scheme.Convert(in, out, nil)
	if err != nil {
		panic(err)
	}
	return out
}

func reviewResponse(allowed bool, msg string) *authorization.SubjectAccessReview {
	return &authorization.SubjectAccessReview{
		Status: authorization.SubjectAccessReviewStatus{
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
