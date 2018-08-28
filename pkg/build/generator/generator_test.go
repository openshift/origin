package generator

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/kubernetes/pkg/api/legacyscheme"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/apis/build/validation"
	mocks "github.com/openshift/origin/pkg/build/generator/test"
	buildutil "github.com/openshift/origin/pkg/build/util"
)

const (
	originalImage = "originalimage"
	newImage      = originalImage + ":" + newTag

	tagName = "test"

	// immutable imageid associated w/ test tag
	newTag = "123"

	imageRepoName      = "testRepo"
	imageRepoNamespace = "testns"

	dockerReference       = "dockerReference"
	latestDockerReference = "latestDockerReference"
)

func TestInstantiate(t *testing.T) {
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestInstantiateBinary(t *testing.T) {
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	build, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{Binary: &buildv1.BinaryBuildSource{}})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Spec.Source.Binary == nil {
		t.Errorf("build should have a binary source value, has nil")
	}
	build, err = generator.Clone(apirequest.NewDefaultContext(), &buildv1.BuildRequest{Binary: &buildv1.BinaryBuildSource{}})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	// TODO: we should enable this flow.
	if build.Spec.Source.Binary != nil {
		t.Errorf("build should not have a binary source value, has %v", build.Spec.Source.Binary)
	}
}

// TODO(agoldste): I'm not sure the intent of this test. Using the previous logic for
// the generator, which would try to update the build config before creating
// the build, I can see why the UpdateBuildConfigFunc is set up to return an
// error, but nothing is checking the value of instantiationCalls. We could
// update this test to fail sooner, when the build is created, but that's
// already handled by TestCreateBuildCreateError. We may just want to delete
// this test.
/*
func TestInstantiateRetry(t *testing.T) {
	instantiationCalls := 0
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         testclient.NewSimpleFake(fakeSecrets...),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		TestingClient: TestingClient{
			GetBuildConfigFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput()), nil
			},
			UpdateBuildConfigFunc: func(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
				instantiationCalls++
				return fmt.Errorf("update-error")
			},
		}}

	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "update-error") {
		t.Errorf("Expected update-error, got different %v", err)
	}
}
*/

func TestInstantiateDeletingError(t *testing.T) {
	source := mocks.MockSource()
	generator := BuildGenerator{Client: TestingClient{
		GetBuildConfigFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
			bc := &buildv1.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						buildutil.BuildConfigPausedAnnotation: "true",
					},
				},
				Spec: buildv1.BuildConfigSpec{
					CommonSpec: buildv1.CommonSpec{
						Source: source,
						Revision: &buildv1.SourceRevision{
							Git: &buildv1.GitSourceRevision{
								Commit: "1234",
							},
						},
					},
				},
			}
			return bc, nil
		},
		GetBuildFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
			build := &buildv1.Build{
				Spec: buildv1.BuildSpec{
					CommonSpec: buildv1.CommonSpec{
						Source: source,
						Revision: &buildv1.SourceRevision{
							Git: &buildv1.GitSourceRevision{
								Commit: "1234",
							},
						},
					},
				},
				Status: buildv1.BuildStatus{
					Config: &corev1.ObjectReference{
						Name: "buildconfig",
					},
				},
			}
			return build, nil
		},
	}}
	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "BuildConfig is paused") {
		t.Errorf("Expected error, got different %v", err)
	}
	_, err = generator.Clone(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "BuildConfig is paused") {
		t.Errorf("Expected error, got different %v", err)
	}
}

// TestInstantiateBinaryClear ensures that when instantiating or cloning from a buildconfig/build
// that has a binary source value, the resulting build does not have a binary source value
// (because the request did not include one)
func TestInstantiateBinaryRemoved(t *testing.T) {
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	client := generator.Client.(TestingClient)
	client.GetBuildConfigFunc = func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
		bc := &buildv1.BuildConfig{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			Spec: buildv1.BuildConfigSpec{
				CommonSpec: buildv1.CommonSpec{
					Source: buildv1.BuildSource{
						Binary: &buildv1.BinaryBuildSource{},
					},
				},
			},
		}
		return bc, nil
	}
	client.GetBuildFunc = func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
		build := &buildv1.Build{
			Spec: buildv1.BuildSpec{
				CommonSpec: buildv1.CommonSpec{
					Source: buildv1.BuildSource{
						Binary: &buildv1.BinaryBuildSource{},
					},
				},
			},
			Status: buildv1.BuildStatus{
				Config: &corev1.ObjectReference{
					Name: "buildconfig",
				},
			},
		}
		return build, nil
	}

	build, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Spec.Source.Binary != nil {
		t.Errorf("build should not have a binary source value, has %v", build.Spec.Source.Binary)
	}
	build, err = generator.Clone(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Spec.Source.Binary != nil {
		t.Errorf("build should not have a binary source value, has %v", build.Spec.Source.Binary)
	}
}

func TestInstantiateGetBuildConfigError(t *testing.T) {
	generator := BuildGenerator{Client: TestingClient{
		GetBuildConfigFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
			return nil, fmt.Errorf("get-error")
		},
		GetImageStreamFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
			return nil, fmt.Errorf("get-error")
		},
		GetImageStreamImageFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error) {
			return nil, fmt.Errorf("get-error")
		},
		GetImageStreamTagFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
			return nil, fmt.Errorf("get-error")
		},
	}}

	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "get-error") {
		t.Errorf("Expected get-error, got different %v", err)
	}
}

func TestInstantiateGenerateBuildError(t *testing.T) {
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	generator := BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).Core(),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: TestingClient{
			GetBuildConfigFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return nil, fmt.Errorf("get-error")
			},
		}}

	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "get-error") {
		t.Errorf("Expected get-error, got different %v", err)
	}
}

func TestInstantiateWithImageTrigger(t *testing.T) {
	imageID := "the-imagev1-id-12345"
	defaultTriggers := func() []buildv1.BuildTriggerPolicy {
		return []buildv1.BuildTriggerPolicy{
			{
				Type: buildv1.GenericWebHookBuildTriggerType,
			},
			{
				Type:        buildv1.ImageChangeBuildTriggerType,
				ImageChange: &buildv1.ImageChangeTrigger{},
			},
			{
				Type: buildv1.ImageChangeBuildTriggerType,
				ImageChange: &buildv1.ImageChangeTrigger{
					From: &corev1.ObjectReference{
						Name: "image1:tag1",
						Kind: "ImageStreamTag",
					},
				},
			},
			{
				Type: buildv1.ImageChangeBuildTriggerType,
				ImageChange: &buildv1.ImageChangeTrigger{
					From: &corev1.ObjectReference{
						Name:      "image2:tag2",
						Namespace: "image2ns",
						Kind:      "ImageStreamTag",
					},
				},
			},
		}
	}
	triggersWithImageID := func() []buildv1.BuildTriggerPolicy {
		triggers := defaultTriggers()
		triggers[2].ImageChange.LastTriggeredImageID = imageID
		return triggers
	}
	tests := []struct {
		name          string
		reqFrom       *corev1.ObjectReference
		triggerIndex  int // index of trigger that will be updated with the imagev1 id, if -1, no update expected
		triggers      []buildv1.BuildTriggerPolicy
		errorExpected bool
	}{
		{
			name: "default trigger",
			reqFrom: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "image3:tag3",
			},
			triggerIndex: 1,
			triggers:     defaultTriggers(),
		},
		{
			name: "trigger with from",
			reqFrom: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "image1:tag1",
			},
			triggerIndex: 2,
			triggers:     defaultTriggers(),
		},
		{
			name: "trigger with from and namespace",
			reqFrom: &corev1.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      "image2:tag2",
				Namespace: "image2ns",
			},
			triggerIndex: 3,
			triggers:     defaultTriggers(),
		},
		{
			name: "existing imagev1 id",
			reqFrom: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "image1:tag1",
			},
			triggers:      triggersWithImageID(),
			errorExpected: true,
		},
	}

	source := mocks.MockSource()
	for _, tc := range tests {
		bc := &buildv1.BuildConfig{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault},
			Spec: buildv1.BuildConfigSpec{
				CommonSpec: buildv1.CommonSpec{
					Strategy: buildv1.BuildStrategy{
						SourceStrategy: &buildv1.SourceBuildStrategy{
							From: corev1.ObjectReference{
								Name: "image3:tag3",
								Kind: "ImageStreamTag",
							},
						},
					},
					Source: source,
					Revision: &buildv1.SourceRevision{
						Git: &buildv1.GitSourceRevision{
							Commit: "1234",
						},
					},
				},
				Triggers: tc.triggers,
			},
		}
		imageStreamTagFunc := func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
			return &imagev1.ImageStreamTag{
				Image: imagev1.Image{
					ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
					DockerImageReference: "ref@" + name,
				},
			}, nil
		}

		generator := mockBuildGenerator(nil, nil, nil, nil, nil, imageStreamTagFunc, nil)
		client := generator.Client.(TestingClient)
		client.GetBuildConfigFunc =
			func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return bc, nil
			}
		client.UpdateBuildConfigFunc =
			func(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
				bc = buildConfig
				return nil
			}
		client.GetImageStreamFunc =
			func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
				return &imagev1.ImageStream{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Status: imagev1.ImageStreamStatus{
						DockerImageRepository: originalImage,
						Tags: []imagev1.NamedTagEventList{
							{
								Tag: "tag1",
								Items: []imagev1.TagEvent{
									{
										DockerImageReference: "ref/" + name + ":tag1",
									},
								},
							},
							{
								Tag: "tag2",
								Items: []imagev1.TagEvent{
									{
										DockerImageReference: "ref/" + name + ":tag2",
									},
								},
							},
							{
								Tag: "tag3",
								Items: []imagev1.TagEvent{
									{
										DockerImageReference: "ref/" + name + ":tag3",
									},
								},
							},
						},
					},
				}, nil
			}
		generator.Client = client

		req := &buildv1.BuildRequest{
			TriggeredByImage: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: imageID,
			},
			From: tc.reqFrom,
		}
		_, err := generator.Instantiate(apirequest.NewDefaultContext(), req)
		if err != nil && !tc.errorExpected {
			t.Errorf("%s: unexpected error %v", tc.name, err)
			continue
		}
		if err == nil && tc.errorExpected {
			t.Errorf("%s: expected error but didn't get one", tc.name)
			continue
		}
		if tc.errorExpected {
			continue
		}
		for i := range bc.Spec.Triggers {
			if i == tc.triggerIndex {
				// Verify that the trigger got updated
				if bc.Spec.Triggers[i].ImageChange.LastTriggeredImageID != imageID {
					t.Errorf("%s: expected trigger at index %d to contain imageID %s", tc.name, i, imageID)
				}
				continue
			}
			// Ensure that other triggers are updated with the latest docker imagev1 ref
			if bc.Spec.Triggers[i].Type == buildv1.ImageChangeBuildTriggerType {
				from := bc.Spec.Triggers[i].ImageChange.From
				if from == nil {
					from = buildutil.GetInputReference(bc.Spec.Strategy)
				}
				if bc.Spec.Triggers[i].ImageChange.LastTriggeredImageID != ("ref/" + from.Name) {
					t.Errorf("%s: expected LastTriggeredImageID for trigger at %d (%+v) to be %s. Got: %s", tc.name, i, bc.Spec.Triggers[i].ImageChange.From, "ref/"+from.Name, bc.Spec.Triggers[i].ImageChange.LastTriggeredImageID)
				}
			}
		}
	}
}

func TestInstantiateWithBuildRequestEnvs(t *testing.T) {
	buildRequestWithEnv := buildv1.BuildRequest{
		Env: []corev1.EnvVar{{Name: "FOO", Value: "BAR"}},
	}
	buildRequestWithoutEnv := buildv1.BuildRequest{}

	tests := []struct {
		bcfunc           func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error)
		req              buildv1.BuildRequest
		expectedEnvValue string
	}{
		{
			bcfunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithEnv,
			expectedEnvValue: "BAR",
		},
		{
			bcfunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockDockerStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithEnv,
			expectedEnvValue: "BAR",
		},
		{
			bcfunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockCustomStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithEnv,
			expectedEnvValue: "BAR",
		},
		{
			bcfunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockJenkinsStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithEnv,
			expectedEnvValue: "BAR",
		},
		{
			bcfunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithoutEnv,
			expectedEnvValue: "VAR",
		},
		{
			bcfunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockDockerStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithoutEnv,
			expectedEnvValue: "VAR",
		},
		{
			bcfunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockCustomStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithoutEnv,
			expectedEnvValue: "VAR",
		},
		{
			bcfunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockJenkinsStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithoutEnv,
			expectedEnvValue: "VAR",
		},
	}

	for _, tc := range tests {
		generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
		client := generator.Client.(TestingClient)
		client.GetBuildConfigFunc = tc.bcfunc
		generator.Client = client
		build, err := generator.Instantiate(apirequest.NewDefaultContext(), &tc.req)
		if err != nil {
			t.Errorf("unexpected error %v", err)
		} else {
			switch {
			case build.Spec.Strategy.SourceStrategy != nil:
				if len(build.Spec.Strategy.SourceStrategy.Env) == 0 {
					t.Errorf("no envs set for src bc and req %#v, expected %s", tc.req, tc.expectedEnvValue)
				} else if build.Spec.Strategy.SourceStrategy.Env[0].Value != tc.expectedEnvValue {
					t.Errorf("unexpected value %s for src bc and req %#v, expected %s", build.Spec.Strategy.SourceStrategy.Env[0].Value, tc.req, tc.expectedEnvValue)
				}
			case build.Spec.Strategy.DockerStrategy != nil:
				if len(build.Spec.Strategy.DockerStrategy.Env) == 0 {
					t.Errorf("no envs set for dock bc and req %#v, expected %s", tc.req, tc.expectedEnvValue)
				} else if build.Spec.Strategy.DockerStrategy.Env[0].Value != tc.expectedEnvValue {
					t.Errorf("unexpected value %s for dock bc and req %#v, expected %s", build.Spec.Strategy.DockerStrategy.Env[0].Value, tc.req, tc.expectedEnvValue)
				}
			case build.Spec.Strategy.CustomStrategy != nil:
				if len(build.Spec.Strategy.CustomStrategy.Env) == 0 {
					t.Errorf("no envs set for cust bc and req %#v, expected %s", tc.req, tc.expectedEnvValue)
				} else {
					// custom strategy will also have OPENSHIFT_CUSTOM_BUILD_BASE_IMAGE injected, could be in either order
					found := false
					for _, env := range build.Spec.Strategy.CustomStrategy.Env {
						if env.Value == tc.expectedEnvValue {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("unexpected values %#v for cust bc and req %#v, expected %s", build.Spec.Strategy.CustomStrategy.Env, tc.req, tc.expectedEnvValue)
					}
				}
			case build.Spec.Strategy.JenkinsPipelineStrategy != nil:
				if len(build.Spec.Strategy.JenkinsPipelineStrategy.Env) == 0 {
					t.Errorf("no envs set for jenk bc and req %#v, expected %s", tc.req, tc.expectedEnvValue)
				} else if build.Spec.Strategy.JenkinsPipelineStrategy.Env[0].Value != tc.expectedEnvValue {
					t.Errorf("unexpected value %s for jenk bc and req %#v, expected %s", build.Spec.Strategy.JenkinsPipelineStrategy.Env[0].Value, tc.req, tc.expectedEnvValue)
				}
			}
		}
	}
}

func TestInstantiateWithLastVersion(t *testing.T) {
	g := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	c := g.Client.(TestingClient)
	c.GetBuildConfigFunc = func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
		bc := mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput())
		bc.Status.LastVersion = 1
		return bc, nil
	}
	g.Client = c

	// Version not specified
	_, err := g.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	// Version specified and it matches
	lastVersion := int64(1)
	_, err = g.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{LastVersion: &lastVersion})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	// Version specified, but doesn't match
	lastVersion = 0
	_, err = g.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{LastVersion: &lastVersion})
	if err == nil {
		t.Errorf("Expected an error and did not get one")
	}
}

func TestInstantiateWithMissingImageStream(t *testing.T) {
	g := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	c := g.Client.(TestingClient)
	c.GetImageStreamFunc = func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
		return nil, errors.NewNotFound(imagev1.Resource("imagestreams"), "testRepo")
	}
	g.Client = c

	_, err := g.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	se, ok := err.(*errors.StatusError)

	if !ok {
		t.Fatalf("Expected errors.StatusError, got %T", err)
	}

	if se.ErrStatus.Code != http.StatusUnprocessableEntity {
		t.Errorf("Expected status 422, got %d", se.ErrStatus.Code)
	}

	if !strings.Contains(se.ErrStatus.Message, "testns") {
		t.Errorf("Error message does not contain namespace: %q", se.ErrStatus.Message)
	}
}

func TestInstantiateWithLabelsAndAnnotations(t *testing.T) {
	g := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	c := g.Client.(TestingClient)
	c.GetBuildConfigFunc = func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
		bc := mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput())
		bc.Status.LastVersion = 1
		return bc, nil
	}
	g.Client = c

	req := &buildv1.BuildRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"a_1": "a_value1",
				// build number is set as an annotation on the generated build, so we
				// shouldn't be able to ovewrite it here.
				buildutil.BuildNumberAnnotation: "bad_annotation",
			},
			Labels: map[string]string{
				"l_1": "l_value1",
				// testbclabel is defined as a label on the mockBuildConfig so we shouldn't
				// be able to overwrite it here.
				"testbclabel": "bad_label",
			},
		},
	}

	build, err := g.Instantiate(apirequest.NewDefaultContext(), req)
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Annotations["a_1"] != "a_value1" || build.Annotations[buildutil.BuildNumberAnnotation] == "bad_annotation" {
		t.Errorf("Build annotations were merged incorrectly: %v", build.Annotations)
	}
	if build.Labels["l_1"] != "l_value1" || build.Labels[buildutil.BuildLabel] == "bad_label" {
		t.Errorf("Build labels were merged incorrectly: %v", build.Labels)
	}
}

func TestFindImageTrigger(t *testing.T) {
	defaultTrigger := &buildv1.ImageChangeTrigger{}
	image1Trigger := &buildv1.ImageChangeTrigger{
		From: &corev1.ObjectReference{
			Name: "image1:tag1",
		},
	}
	image2Trigger := &buildv1.ImageChangeTrigger{
		From: &corev1.ObjectReference{
			Name:      "image2:tag2",
			Namespace: "image2ns",
		},
	}
	image4Trigger := &buildv1.ImageChangeTrigger{
		From: &corev1.ObjectReference{
			Name: "image4:tag4",
		},
	}
	image5Trigger := &buildv1.ImageChangeTrigger{
		From: &corev1.ObjectReference{
			Name:      "image5:tag5",
			Namespace: "bcnamespace",
		},
	}
	bc := &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testbc",
			Namespace: "bcnamespace",
		},
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Strategy: buildv1.BuildStrategy{
					SourceStrategy: &buildv1.SourceBuildStrategy{
						From: corev1.ObjectReference{
							Name: "image3:tag3",
							Kind: "ImageStreamTag",
						},
					},
				},
			},
			Triggers: []buildv1.BuildTriggerPolicy{
				{
					Type: buildv1.GenericWebHookBuildTriggerType,
				},
				{
					Type:        buildv1.ImageChangeBuildTriggerType,
					ImageChange: defaultTrigger,
				},
				{
					Type:        buildv1.ImageChangeBuildTriggerType,
					ImageChange: image1Trigger,
				},
				{
					Type:        buildv1.ImageChangeBuildTriggerType,
					ImageChange: image2Trigger,
				},
				{
					Type:        buildv1.ImageChangeBuildTriggerType,
					ImageChange: image4Trigger,
				},
				{
					Type:        buildv1.ImageChangeBuildTriggerType,
					ImageChange: image5Trigger,
				},
			},
		},
	}

	tests := []struct {
		name   string
		input  *corev1.ObjectReference
		expect *buildv1.ImageChangeTrigger
	}{
		{
			name:   "nil reference",
			input:  nil,
			expect: nil,
		},
		{
			name: "match name",
			input: &corev1.ObjectReference{
				Name: "image1:tag1",
			},
			expect: image1Trigger,
		},
		{
			name: "mismatched namespace",
			input: &corev1.ObjectReference{
				Name:      "image1:tag1",
				Namespace: "otherns",
			},
			expect: nil,
		},
		{
			name: "match name and namespace",
			input: &corev1.ObjectReference{
				Name:      "image2:tag2",
				Namespace: "image2ns",
			},
			expect: image2Trigger,
		},
		{
			name: "match default trigger",
			input: &corev1.ObjectReference{
				Name: "image3:tag3",
			},
			expect: defaultTrigger,
		},
		{
			name: "input includes bc namespace",
			input: &corev1.ObjectReference{
				Name:      "image4:tag4",
				Namespace: "bcnamespace",
			},
			expect: image4Trigger,
		},
		{
			name: "implied namespace in trigger input",
			input: &corev1.ObjectReference{
				Name: "image5:tag5",
			},
			expect: image5Trigger,
		},
	}

	for _, tc := range tests {
		result := findImageChangeTrigger(bc, tc.input)
		if result != tc.expect {
			t.Errorf("%s: unexpected trigger for %#v: %#v", tc.name, tc.input, result)
		}
	}

}

func TestClone(t *testing.T) {
	generator := BuildGenerator{Client: TestingClient{
		CreateBuildFunc: func(ctx context.Context, build *buildv1.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
			return &buildv1.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build-1",
					Namespace: metav1.NamespaceDefault,
				},
			}, nil
		},
	}}

	_, err := generator.Clone(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCloneError(t *testing.T) {
	generator := BuildGenerator{Client: TestingClient{
		GetBuildFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
			return nil, fmt.Errorf("get-error")
		},
	}}

	_, err := generator.Clone(apirequest.NewContext(), &buildv1.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "get-error") {
		t.Errorf("Expected get-error, got different %v", err)
	}
}

func TestCreateBuild(t *testing.T) {
	build := &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: metav1.NamespaceDefault,
		},
	}
	generator := BuildGenerator{Client: TestingClient{
		CreateBuildFunc: func(ctx context.Context, build *buildv1.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
			return build, nil
		},
	}}

	build, err := generator.createBuild(apirequest.NewDefaultContext(), build)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.CreationTimestamp.IsZero() || len(build.UID) == 0 {
		t.Error("Expected meta fields being filled in!")
	}
}

func TestCreateBuildNamespaceError(t *testing.T) {
	build := &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-build",
		},
	}
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)

	_, err := generator.createBuild(apirequest.NewContext(), build)
	if err == nil || !strings.Contains(err.Error(), "Build.Namespace") {
		t.Errorf("Expected namespace error, got different %v", err)
	}
}

func TestCreateBuildCreateError(t *testing.T) {
	build := &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: metav1.NamespaceDefault,
		},
	}
	generator := BuildGenerator{Client: TestingClient{
		CreateBuildFunc: func(ctx context.Context, build *buildv1.Build) error {
			return fmt.Errorf("create-error")
		},
	}}

	_, err := generator.createBuild(apirequest.NewDefaultContext(), build)
	if err == nil || !strings.Contains(err.Error(), "create-error") {
		t.Errorf("Expected create-error, got different %v", err)
	}
}

func TestGenerateBuildFromConfig(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockDockerStrategyForDockerImage(originalImage, &metav1.GetOptions{})
	output := mocks.MockOutput()
	resources := mockResources()
	bc := &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "test-uid",
			Name:      "test-build-config",
			Namespace: metav1.NamespaceDefault,
			Labels:    map[string]string{"testlabel": "testvalue"},
		},
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: source,
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy:  strategy,
				Output:    output,
				Resources: resources,
			},
		},
		Status: buildv1.BuildConfigStatus{
			LastVersion: 12,
		},
	}
	revision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Commit: "abcd",
		},
	}
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)

	build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, revision, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if !reflect.DeepEqual(source, build.Spec.Source) {
		t.Errorf("Build source does not match BuildConfig source")
	}
	// FIXME: This is disabled because the strategies does not match since we plug the
	//        pullSecret into the build strategy.
	/*
		if !reflect.DeepEqual(strategy, build.Spec.Strategy) {
			t.Errorf("Build strategy does not match BuildConfig strategy %+v != %+v", strategy.DockerStrategy, build.Spec.Strategy.DockerStrategy)
		}
	*/
	if !reflect.DeepEqual(output, build.Spec.Output) {
		t.Errorf("Build output does not match BuildConfig output")
	}
	if !reflect.DeepEqual(revision, build.Spec.Revision) {
		t.Errorf("Build revision does not match passed in revision")
	}
	if !reflect.DeepEqual(resources, build.Spec.Resources) {
		t.Errorf("Build resources does not match passed in resources")
	}
	if build.Labels["testlabel"] != bc.Labels["testlabel"] {
		t.Errorf("Build does not contain labels from BuildConfig")
	}
	if build.Annotations[buildutil.BuildConfigAnnotation] != bc.Name {
		t.Errorf("Build does not contain annotation from BuildConfig")
	}
	if build.Labels[buildutil.BuildConfigLabel] != bc.Name {
		t.Errorf("Build does not contain labels from BuildConfig")
	}
	if build.Labels[buildutil.BuildConfigLabelDeprecated] != bc.Name {
		t.Errorf("Build does not contain labels from BuildConfig")
	}
	if build.Status.Config.Name != bc.Name || build.Status.Config.Namespace != bc.Namespace || build.Status.Config.Kind != "BuildConfig" {
		t.Errorf("Build does not contain correct BuildConfig reference: %v", build.Status.Config)
	}
	if build.Annotations[buildutil.BuildNumberAnnotation] != "13" {
		t.Errorf("Build number annotation value %s does not match expected value 13", build.Annotations[buildutil.BuildNumberAnnotation])
	}
	if len(build.OwnerReferences) == 0 || build.OwnerReferences[0].Kind != "BuildConfig" || build.OwnerReferences[0].Name != bc.Name {
		t.Errorf("generated build does not have OwnerReference to parent BuildConfig")
	}
	if build.OwnerReferences[0].Controller == nil || !*build.OwnerReferences[0].Controller {
		t.Errorf("generated build does not have OwnerReference to parent BuildConfig marked as a controller relationship")
	}
	// Test long name
	bc.Name = strings.Repeat("a", 100)
	build, err = generator.generateBuildFromConfig(apirequest.NewContext(), bc, revision, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	build.Namespace = "test-namespace"

	// TODO: We have to convert this to internal as the validation/apiserver is still using internal build...
	internalBuild := &buildapi.Build{}
	if err := legacyscheme.Scheme.Convert(build, internalBuild, nil); err != nil {
		t.Fatalf("unable to convert to internal build: %v", err)
	}
	validateErrors := validation.ValidateBuild(internalBuild)
	if len(validateErrors) > 0 {
		t.Fatalf("Unexpected validation errors %v", validateErrors)
	}
}

func TestGenerateBuildWithImageTagForSourceStrategyImageRepository(t *testing.T) {
	source := mocks.MockSource()
	strategy := mocks.MockSourceStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-config",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: source,
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	is := mocks.MockImageStream("", originalImage, map[string]string{tagName: newTag})
	generator := BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).Core(),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: TestingClient{
			GetImageStreamFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
				return &imagev1.ImageStream{
					ObjectMeta: metav1.ObjectMeta{Name: imageRepoName},
					Status: imagev1.ImageStreamStatus{
						DockerImageRepository: originalImage,
						Tags: is.Status.Tags,
					},
				}, nil
			},
			GetImageStreamTagFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
				return &imagev1.ImageStreamTag{
					Image: imagev1.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error) {
				return &imagev1.ImageStreamImage{
					Image: imagev1.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},

			UpdateBuildConfigFunc: func(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
				return nil
			},
		}}

	build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Spec.Strategy.SourceStrategy.From.Name != newImage {
		t.Errorf("source-to-image base image value %s does not match expected value %s", build.Spec.Strategy.SourceStrategy.From.Name, newImage)
	}
}

func TestGenerateBuildWithImageTagForDockerStrategyImageRepository(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockDockerStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-config",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: source,
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	is := mocks.MockImageStream("", originalImage, map[string]string{tagName: newTag})
	generator := BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).Core(),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: TestingClient{
			GetImageStreamFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
				return &imagev1.ImageStream{
					ObjectMeta: metav1.ObjectMeta{Name: imageRepoName},
					Status: imagev1.ImageStreamStatus{
						DockerImageRepository: originalImage,
						Tags: is.Status.Tags,
					},
				}, nil
			},
			GetImageStreamTagFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
				return &imagev1.ImageStreamTag{
					Image: imagev1.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error) {
				return &imagev1.ImageStreamImage{
					Image: imagev1.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			UpdateBuildConfigFunc: func(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
				return nil
			},
		}}

	build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Spec.Strategy.DockerStrategy.From.Name != newImage {
		t.Errorf("Docker base image value %s does not match expected value %s", build.Spec.Strategy.DockerStrategy.From.Name, newImage)
	}
}

func TestGenerateBuildWithImageTagForCustomStrategyImageRepository(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-config",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: buildv1.BuildConfigSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: source,
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	is := mocks.MockImageStream("", originalImage, map[string]string{tagName: newTag})
	generator := BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).Core(),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: TestingClient{
			GetImageStreamFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
				return &imagev1.ImageStream{
					ObjectMeta: metav1.ObjectMeta{Name: imageRepoName},
					Status: imagev1.ImageStreamStatus{
						DockerImageRepository: originalImage,
						Tags: is.Status.Tags,
					},
				}, nil
			},
			GetImageStreamTagFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
				return &imagev1.ImageStreamTag{
					Image: imagev1.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error) {
				return &imagev1.ImageStreamImage{
					Image: imagev1.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			UpdateBuildConfigFunc: func(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
				return nil
			},
		}}

	build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}
	if build.Spec.Strategy.CustomStrategy.From.Name != newImage {
		t.Errorf("Custom base image value %s does not match expected value %s", build.Spec.Strategy.CustomStrategy.From.Name, newImage)
	}
}

func TestGenerateBuildFromBuild(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockDockerStrategyForImageRepository()
	output := mocks.MockOutput()
	build := &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-build",
			Annotations: map[string]string{
				buildutil.BuildJenkinsStatusJSONAnnotation:      "foo",
				buildutil.BuildJenkinsLogURLAnnotation:          "bar",
				buildutil.BuildJenkinsConsoleLogURLAnnotation:   "bar",
				buildutil.BuildJenkinsBlueOceanLogURLAnnotation: "bar",
				buildutil.BuildJenkinsBuildURIAnnotation:        "baz",
				buildutil.BuildPodNameAnnotation:                "ruby-sample-build-1-build",
			},
			OwnerReferences: []metav1.OwnerReference{
				{
					Name:       "test-owner",
					Kind:       "BuildConfig",
					APIVersion: "v1",
					UID:        "foo",
				},
			},
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: source,
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}

	newBuild := generateBuildFromBuild(build, nil)
	if !reflect.DeepEqual(build.Spec, newBuild.Spec) {
		t.Errorf("Build parameters does not match the original Build parameters")
	}
	if !reflect.DeepEqual(build.ObjectMeta.Labels, newBuild.ObjectMeta.Labels) {
		t.Errorf("Build labels does not match the original Build labels")
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildutil.BuildJenkinsStatusJSONAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildutil.BuildJenkinsStatusJSONAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildutil.BuildJenkinsLogURLAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildutil.BuildJenkinsLogURLAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildutil.BuildJenkinsConsoleLogURLAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildutil.BuildJenkinsConsoleLogURLAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildutil.BuildJenkinsBlueOceanLogURLAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildutil.BuildJenkinsBlueOceanLogURLAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildutil.BuildJenkinsBuildURIAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildutil.BuildJenkinsBuildURIAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildutil.BuildPodNameAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildutil.BuildPodNameAnnotation)
	}
	if !reflect.DeepEqual(build.ObjectMeta.OwnerReferences, newBuild.ObjectMeta.OwnerReferences) {
		t.Errorf("Build OwnerReferences does not match the original Build OwnerReferences")
	}

}

func TestGenerateBuildFromBuildWithBuildConfig(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockDockerStrategyForImageRepository()
	output := mocks.MockOutput()
	annotatedBuild := &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "annotatedBuild",
			Annotations: map[string]string{
				buildutil.BuildCloneAnnotation: "sourceOfBuild",
			},
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: source,
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}
	nonAnnotatedBuild := &buildv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nonAnnotatedBuild",
		},
		Spec: buildv1.BuildSpec{
			CommonSpec: buildv1.CommonSpec{
				Source: source,
				Revision: &buildv1.SourceRevision{
					Git: &buildv1.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}

	buildConfig := &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "buildConfigName",
		},
		Status: buildv1.BuildConfigStatus{
			LastVersion: 5,
		},
	}

	newBuild := generateBuildFromBuild(annotatedBuild, buildConfig)
	if !reflect.DeepEqual(annotatedBuild.Spec, newBuild.Spec) {
		t.Errorf("Build parameters does not match the original Build parameters")
	}
	if !reflect.DeepEqual(annotatedBuild.ObjectMeta.Labels, newBuild.ObjectMeta.Labels) {
		t.Errorf("Build labels does not match the original Build labels")
	}
	if newBuild.Annotations[buildutil.BuildNumberAnnotation] != "6" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildutil.BuildNumberAnnotation], "6")
	}
	if newBuild.Annotations[buildutil.BuildCloneAnnotation] != "annotatedBuild" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildutil.BuildCloneAnnotation], "annotatedBuild")
	}

	newBuild = generateBuildFromBuild(nonAnnotatedBuild, buildConfig)
	if !reflect.DeepEqual(nonAnnotatedBuild.Spec, newBuild.Spec) {
		t.Errorf("Build parameters does not match the original Build parameters")
	}
	if !reflect.DeepEqual(nonAnnotatedBuild.ObjectMeta.Labels, newBuild.ObjectMeta.Labels) {
		t.Errorf("Build labels does not match the original Build labels")
	}
	// was incremented by previous test, so expect 7 now.
	if newBuild.Annotations[buildutil.BuildNumberAnnotation] != "7" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildutil.BuildNumberAnnotation], "7")
	}
	if newBuild.Annotations[buildutil.BuildCloneAnnotation] != "nonAnnotatedBuild" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildutil.BuildCloneAnnotation], "nonAnnotatedBuild")
	}

}

func TestSubstituteImageCustomAllMatch(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForDockerImage(originalImage, &metav1.GetOptions{})
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with a Image and a well defined environment variable imagev1 value,
	// both should be replaced.  Additional environment variables should not be touched.
	build.Spec.Strategy.CustomStrategy.Env = make([]corev1.EnvVar, 2)
	build.Spec.Strategy.CustomStrategy.Env[0] = corev1.EnvVar{Name: "someImage", Value: originalImage}
	build.Spec.Strategy.CustomStrategy.Env[1] = corev1.EnvVar{Name: buildutil.CustomBuildStrategyBaseImageKey, Value: originalImage}
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable %s was improperly substituted in custom strategy", build.Spec.Strategy.CustomStrategy.Env[0].Name)
	}
	if build.Spec.Strategy.CustomStrategy.Env[1].Value != newImage {
		t.Errorf("Image env variable was not properly substituted in custom strategy")
	}
	if c := len(build.Spec.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
	if bc.Spec.Strategy.CustomStrategy.From.Name != originalImage {
		t.Errorf("Custom BuildConfig Image was updated when Build was modified %s!=%s", bc.Spec.Strategy.CustomStrategy.From.Name, originalImage)
	}
	if len(bc.Spec.Strategy.CustomStrategy.Env) != 0 {
		t.Errorf("Custom BuildConfig Env was updated when Build was modified")
	}
}

func TestSubstituteImageCustomAllMismatch(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForDockerImage(originalImage, &metav1.GetOptions{})
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with base imagev1 that is not matched
	// Base imagev1 name should be unchanged
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, "dummy")
	if build.Spec.Strategy.CustomStrategy.From.Name != originalImage {
		t.Errorf("Base image name was improperly substituted in custom strategy %s %s", build.Spec.Strategy.CustomStrategy.From.Name, originalImage)
	}
}

func TestSubstituteImageCustomBaseMatchEnvMismatch(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Full custom build with a Image and a well defined environment variable image value that does not match the new image
	// Environment variables should not be updated.
	build.Spec.Strategy.CustomStrategy.Env = make([]corev1.EnvVar, 2)
	build.Spec.Strategy.CustomStrategy.Env[0] = corev1.EnvVar{Name: "someEnvVar", Value: originalImage}
	build.Spec.Strategy.CustomStrategy.Env[1] = corev1.EnvVar{Name: buildutil.CustomBuildStrategyBaseImageKey, Value: "dummy"}
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable %s was improperly substituted in custom strategy", build.Spec.Strategy.CustomStrategy.Env[0].Name)
	}
	if build.Spec.Strategy.CustomStrategy.Env[1].Value != newImage {
		t.Errorf("Image env variable was not substituted in custom strategy")
	}
	if c := len(build.Spec.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
}

func TestSubstituteImageCustomBaseMatchEnvMissing(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Custom build with a base Image but no image environment variable.
	// base image should be replaced, new image environment variable should be added,
	// existing environment variable should be untouched
	build.Spec.Strategy.CustomStrategy.Env = make([]corev1.EnvVar, 1)
	build.Spec.Strategy.CustomStrategy.Env[0] = corev1.EnvVar{Name: "someImage", Value: originalImage}
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable was improperly substituted in custom strategy")
	}
	if build.Spec.Strategy.CustomStrategy.Env[1].Name != buildutil.CustomBuildStrategyBaseImageKey || build.Spec.Strategy.CustomStrategy.Env[1].
		Value != newImage {
		t.Errorf("Image env variable was not added in custom strategy %s %s |", build.Spec.Strategy.CustomStrategy.Env[1].Name, build.Spec.Strategy.CustomStrategy.Env[1].Value)
	}
	if c := len(build.Spec.Strategy.CustomStrategy.Env); c != 2 {
		t.Errorf("Expected %d, found %d environment variables", 2, c)
	}
}

func TestSubstituteImageCustomBaseMatchEnvNil(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockCustomStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := mocks.MockBuildConfig(source, strategy, output)
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, nil, nil)
	if err != nil {
		t.Fatalf("Unexpected error %v", err)
	}

	// Custom build with a base Image but no environment variables
	// base image should be replaced, new image environment variable should be added
	updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Name != buildutil.CustomBuildStrategyBaseImageKey || build.Spec.Strategy.CustomStrategy.Env[0].
		Value != newImage {
		t.Errorf("New image name variable was not added to environment list in custom strategy")
	}
	if c := len(build.Spec.Strategy.CustomStrategy.Env); c != 1 {
		t.Errorf("Expected %d, found %d environment variables", 1, c)
	}
}

func TestGetNextBuildName(t *testing.T) {
	bc := mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput())
	if expected, actual := bc.Name+"-1", getNextBuildName(bc); expected != actual {
		t.Errorf("Wrong buildName, expected %s, got %s", expected, actual)
	}
	if expected, actual := int64(1), bc.Status.LastVersion; expected != actual {
		t.Errorf("Wrong version, expected %d, got %d", expected, actual)
	}
}

func TestGetNextBuildNameFromBuild(t *testing.T) {
	testCases := []struct {
		value    string
		expected string
	}{
		// 0
		{"mybuild-1", `^mybuild-1-\d+$`},
		// 1
		{"mybuild-1-1426794070", `^mybuild-1-\d+$`},
		// 2
		{"mybuild-1-1426794070-1-1426794070", `^mybuild-1-1426794070-1-\d+$`},
		// 3
		{"my-build-1", `^my-build-1-\d+$`},
		// 4
		{"mybuild-10-1426794070", `^mybuild-10-\d+$`},
	}

	for i, tc := range testCases {
		buildName := getNextBuildNameFromBuild(&buildv1.Build{ObjectMeta: metav1.ObjectMeta{Name: tc.value}}, nil)
		if matched, err := regexp.MatchString(tc.expected, buildName); !matched || err != nil {
			t.Errorf("(%d) Unexpected build name, got %s expected %s", i, buildName, tc.expected)
		}
	}
}

func TestGetNextBuildNameFromBuildWithBuildConfig(t *testing.T) {
	testCases := []struct {
		value       string
		buildConfig *buildv1.BuildConfig
		expected    string
	}{
		// 0
		{
			"mybuild-1",
			&buildv1.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "buildConfigName",
				},
				Status: buildv1.BuildConfigStatus{
					LastVersion: 5,
				},
			},
			`^buildConfigName-6$`,
		},
		// 1
		{
			"mybuild-1-1426794070",
			&buildv1.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "buildConfigName",
				},
				Status: buildv1.BuildConfigStatus{
					LastVersion: 5,
				},
			},
			`^buildConfigName-6$`,
		},
	}

	for i, tc := range testCases {
		buildName := getNextBuildNameFromBuild(&buildv1.Build{ObjectMeta: metav1.ObjectMeta{Name: tc.value}}, tc.buildConfig)
		if matched, err := regexp.MatchString(tc.expected, buildName); !matched || err != nil {
			t.Errorf("(%d) Unexpected build name, got %s expected %s", i, buildName, tc.expected)
		}
	}
}

func TestResolveImageStreamRef(t *testing.T) {
	type resolveTest struct {
		streamRef         corev1.ObjectReference
		tag               string
		expectedSuccess   bool
		expectedDockerRef string
	}
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)

	tests := []resolveTest{
		{
			streamRef: corev1.ObjectReference{
				Name: imageRepoName,
			},
			tag:               tagName,
			expectedSuccess:   false,
			expectedDockerRef: dockerReference,
		},
		{
			streamRef: corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: imageRepoName + ":" + tagName,
			},
			expectedSuccess:   true,
			expectedDockerRef: dockerReference,
		},
		{
			streamRef: corev1.ObjectReference{
				Kind: "ImageStreamImage",
				Name: imageRepoName + "@myid",
			},
			expectedSuccess: true,
			// until we default to the "real" pull by id logic,
			// the @id is applied as a :tag when resolving the repository.
			expectedDockerRef: latestDockerReference,
		},
	}
	for i, test := range tests {
		ref, err := generator.resolveImageStreamReference(apirequest.NewDefaultContext(), test.streamRef, "")
		if err != nil {
			if test.expectedSuccess {
				t.Errorf("Scenario %d: Unexpected error %v", i, err)
			}
			continue
		} else if !test.expectedSuccess {
			t.Errorf("Scenario %d: did not get expected error", i)
		}
		if ref != test.expectedDockerRef {
			t.Errorf("Scenario %d: Resolved reference %q did not match expected value %q", i, ref, test.expectedDockerRef)
		}
	}
}

func mockResources() corev1.ResourceRequirements {
	res := corev1.ResourceRequirements{}
	res.Limits = corev1.ResourceList{}
	res.Limits[corev1.ResourceCPU] = resource.MustParse("100m")
	res.Limits[corev1.ResourceMemory] = resource.MustParse("100Mi")
	return res
}

func mockDockerStrategyForDockerImage(name string, options *metav1.GetOptions) buildv1.BuildStrategy {
	return buildv1.BuildStrategy{
		DockerStrategy: &buildv1.DockerBuildStrategy{
			NoCache: true,
			From: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: name,
			},
		},
	}
}

func mockDockerStrategyForImageRepository() buildv1.BuildStrategy {
	return buildv1.BuildStrategy{
		DockerStrategy: &buildv1.DockerBuildStrategy{
			NoCache: true,
			From: &corev1.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
	}
}

func mockCustomStrategyForDockerImage(name string, options *metav1.GetOptions) buildv1.BuildStrategy {
	return buildv1.BuildStrategy{
		CustomStrategy: &buildv1.CustomBuildStrategy{
			From: corev1.ObjectReference{
				Kind: "DockerImage",
				Name: originalImage,
			},
		},
	}
}

func mockCustomStrategyForImageRepository() buildv1.BuildStrategy {
	return buildv1.BuildStrategy{
		CustomStrategy: &buildv1.CustomBuildStrategy{
			From: corev1.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
	}
}

func mockOutputWithImageName(name string, options *metav1.GetOptions) buildv1.BuildOutput {
	return buildv1.BuildOutput{
		To: &corev1.ObjectReference{
			Kind: "DockerImage",
			Name: name,
		},
	}
}

func getBuildConfigFunc(buildConfigFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error)) func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
	if buildConfigFunc == nil {
		return func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
			return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput()), nil
		}
	}
	return buildConfigFunc
}

func getUpdateBuildConfigFunc(updateBuildConfigFunc func(ctx context.Context, buildConfig *buildv1.BuildConfig) error) func(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
	if updateBuildConfigFunc == nil {
		return func(ctx context.Context, buildConfig *buildv1.BuildConfig) error {
			return nil
		}
	}
	return updateBuildConfigFunc
}

func getCreateBuildFunc(createBuildConfigFunc func(ctx context.Context, build *buildv1.Build) error, b *buildv1.Build) func(ctx context.Context, build *buildv1.Build) error {
	if createBuildConfigFunc == nil {
		return func(ctx context.Context, build *buildv1.Build) error {
			*b = *build
			return nil
		}
	}
	return createBuildConfigFunc
}

func getGetBuildFunc(getBuildFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error), b *buildv1.Build) func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
	if getBuildFunc == nil {
		return func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error) {
			if b == nil {
				return &buildv1.Build{}, nil
			}
			return b, nil
		}
	}
	return getBuildFunc
}

func getGetImageStreamFunc(getImageStreamFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error)) func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
	if getImageStreamFunc == nil {
		return func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error) {
			if name != imageRepoName {
				return &imagev1.ImageStream{}, nil
			}
			return &imagev1.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      imageRepoName,
					Namespace: imageRepoNamespace,
				},
				Status: imagev1.ImageStreamStatus{
					DockerImageRepository: "repo/namespace/image",
					Tags: []imagev1.NamedTagEventList{
						{
							Tag: tagName,
							Items: []imagev1.TagEvent{
								{DockerImageReference: dockerReference},
							},
						},
						{
							Tag: "latest",
							Items: []imagev1.TagEvent{
								{DockerImageReference: latestDockerReference, Image: "myid"},
							},
						},
					},
				},
			}, nil
		}
	}
	return getImageStreamFunc
}

func getGetImageStreamTagFunc(getImageStreamTagFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error)) func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
	if getImageStreamTagFunc == nil {
		return func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error) {
			return &imagev1.ImageStreamTag{
				Image: imagev1.Image{
					ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
					DockerImageReference: latestDockerReference,
				},
			}, nil
		}
	}
	return getImageStreamTagFunc
}

func getGetImageStreamImageFunc(getImageStreamImageFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error)) func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error) {
	if getImageStreamImageFunc == nil {
		return func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error) {
			return &imagev1.ImageStreamImage{
				Image: imagev1.Image{
					ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":@id"},
					DockerImageReference: latestDockerReference,
				},
			}, nil
		}
	}
	return getImageStreamImageFunc
}

func mockBuildGenerator(buildConfigFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error),
	updateBuildConfigFunc func(ctx context.Context, buildConfig *buildv1.BuildConfig) error,
	createBuildFunc func(ctx context.Context, build *buildv1.Build) error,
	getBuildFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.Build, error),
	getImageStreamFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStream, error),
	getImageStreamTagFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamTag, error),
	getImageStreamImageFunc func(ctx context.Context, name string, options *metav1.GetOptions) (*imagev1.ImageStreamImage, error),
) *BuildGenerator {
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	b := buildv1.Build{}
	return &BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).Core(),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: TestingClient{
			GetBuildConfigFunc:      getBuildConfigFunc(buildConfigFunc),
			UpdateBuildConfigFunc:   getUpdateBuildConfigFunc(updateBuildConfigFunc),
			CreateBuildFunc:         getCreateBuildFunc(createBuildFunc, &b),
			GetBuildFunc:            getGetBuildFunc(getBuildFunc, &b),
			GetImageStreamFunc:      getGetImageStreamFunc(getImageStreamFunc),
			GetImageStreamTagFunc:   getGetImageStreamTagFunc(getImageStreamTagFunc),
			GetImageStreamImageFunc: getGetImageStreamImageFunc(getImageStreamImageFunc),
		}}
}

func TestGenerateBuildFromConfigWithSecrets(t *testing.T) {
	source := mocks.MockSource()
	revision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Commit: "abcd",
		},
	}
	dockerCfgTable := map[string]map[string][]byte{
		// FIXME: This imagev1 pull spec does not return ANY registry, but it should
		// return the hub.
		//"docker.io/secret2/image":     {".dockercfg": sampleDockerConfigs["hub"]},
		"secret1/image":               {".dockercfg": mocks.SampleDockerConfigs["hub"]},
		"1.1.1.1:5000/secret3/image":  {".dockercfg": mocks.SampleDockerConfigs["ipv4"]},
		"registry.host/secret4/image": {".dockercfg": mocks.SampleDockerConfigs["host"]},
	}
	for imageName := range dockerCfgTable {
		// Setup the BuildGenerator
		strategy := mockDockerStrategyForDockerImage(imageName, &metav1.GetOptions{})
		output := mockOutputWithImageName(imageName, &metav1.GetOptions{})
		generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
		bc := mocks.MockBuildConfig(source, strategy, output)
		build, err := generator.generateBuildFromConfig(apirequest.NewContext(), bc, revision, nil)

		if build.Spec.Strategy.DockerStrategy.PullSecret == nil {
			t.Errorf("Expected PullSecret for image '%s' to be set, got nil", imageName)
			continue
		}
		if len(build.Spec.Strategy.DockerStrategy.PullSecret.Name) == 0 {
			t.Errorf("Expected PullSecret for image %s to be set not empty", imageName)
		}
		if err != nil {
			t.Fatalf("Unexpected error %v", err)
		}
	}
}

func TestInstantiateBuildTriggerCauseConfigChange(t *testing.T) {
	changeMessage := buildutil.BuildTriggerCauseConfigMsg

	buildTriggerCauses := []buildv1.BuildTriggerCause{}
	buildRequest := &buildv1.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: changeMessage,
			},
		),
	}
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	buildObject, err := generator.Instantiate(apirequest.NewDefaultContext(), buildRequest)
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}

	for _, cause := range buildObject.Spec.TriggeredBy {
		if cause.Message != changeMessage {
			t.Errorf("Expected reason %s, got %s", changeMessage, cause.Message)
		}
	}
}

func TestInstantiateBuildTriggerCauseImageChange(t *testing.T) {
	buildTriggerCauses := []buildv1.BuildTriggerCause{}
	changeMessage := buildutil.BuildTriggerCauseImageMsg
	imageID := "centos@sha256:b3da5267165b"
	refName := "centos:7"
	refKind := "ImageStreamTag"

	buildRequest := &buildv1.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: changeMessage,
				ImageChangeBuild: &buildv1.ImageChangeCause{
					ImageID: imageID,
					FromRef: &corev1.ObjectReference{
						Name: refName,
						Kind: refKind,
					},
				},
			},
		),
	}

	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	buildObject, err := generator.Instantiate(apirequest.NewDefaultContext(), buildRequest)
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	for _, cause := range buildObject.Spec.TriggeredBy {
		if cause.Message != buildutil.BuildTriggerCauseImageMsg {
			t.Errorf("Expected reason %s, got %s", changeMessage, cause.Message)
		}
		if cause.ImageChangeBuild.ImageID != imageID {
			t.Errorf("Expected imageID: %s, got: %s", imageID, cause.ImageChangeBuild.ImageID)
		}
		if cause.ImageChangeBuild.FromRef.Name != refName {
			t.Errorf("Expected image name to be %s, got %s", refName, cause.ImageChangeBuild.FromRef.Name)
		}
		if cause.ImageChangeBuild.FromRef.Kind != refKind {
			t.Errorf("Expected image kind to be %s, got %s", refKind, cause.ImageChangeBuild.FromRef.Kind)
		}
	}
}

func TestInstantiateBuildTriggerCauseGenericWebHook(t *testing.T) {
	buildTriggerCauses := []buildv1.BuildTriggerCause{}
	changeMessage := "Generic WebHook"
	webHookSecret := "<secret>"

	gitRevision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Author: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "johndoe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildRequest := &buildv1.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: changeMessage,
				GenericWebHook: &buildv1.GenericWebHookCause{
					Secret:   "<secret>",
					Revision: gitRevision,
				},
			},
		),
	}

	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	buildObject, err := generator.Instantiate(apirequest.NewDefaultContext(), buildRequest)
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	for _, cause := range buildObject.Spec.TriggeredBy {
		if cause.Message != changeMessage {
			t.Errorf("Expected reason %s, got %s", changeMessage, cause.Message)
		}
		if cause.GenericWebHook.Secret != webHookSecret {
			t.Errorf("Expected WebHook secret %s, got %s", webHookSecret, cause.GenericWebHook.Secret)
		}
		if !reflect.DeepEqual(gitRevision, cause.GenericWebHook.Revision) {
			t.Errorf("Expected return revision to match")
		}
	}
}

func TestInstantiateBuildTriggerCauseGitHubWebHook(t *testing.T) {
	buildTriggerCauses := []buildv1.BuildTriggerCause{}
	changeMessage := buildutil.BuildTriggerCauseGithubMsg
	webHookSecret := "<secret>"

	gitRevision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Author: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "johndoe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildRequest := &buildv1.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: changeMessage,
				GitHubWebHook: &buildv1.GitHubWebHookCause{
					Secret:   "<secret>",
					Revision: gitRevision,
				},
			},
		),
	}

	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	buildObject, err := generator.Instantiate(apirequest.NewDefaultContext(), buildRequest)
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	for _, cause := range buildObject.Spec.TriggeredBy {
		if cause.Message != changeMessage {
			t.Errorf("Expected reason %s, got %s", changeMessage, cause.Message)
		}
		if cause.GitHubWebHook.Secret != webHookSecret {
			t.Errorf("Expected WebHook secret %s, got %s", webHookSecret, cause.GitHubWebHook.Secret)
		}
		if !reflect.DeepEqual(gitRevision, cause.GitHubWebHook.Revision) {
			t.Errorf("Expected return revision to match")
		}
	}
}

func TestInstantiateBuildTriggerCauseGitLabWebHook(t *testing.T) {
	buildTriggerCauses := []buildv1.BuildTriggerCause{}
	changeMessage := buildutil.BuildTriggerCauseGitLabMsg
	webHookSecret := "<secret>"

	gitRevision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Author: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "johndoe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildRequest := &buildv1.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: changeMessage,
				GitLabWebHook: &buildv1.GitLabWebHookCause{
					CommonWebHookCause: buildv1.CommonWebHookCause{
						Revision: gitRevision,
						Secret:   "<secret>",
					},
				},
			},
		),
	}

	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	buildObject, err := generator.Instantiate(apirequest.NewDefaultContext(), buildRequest)
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	for _, cause := range buildObject.Spec.TriggeredBy {
		if cause.Message != changeMessage {
			t.Errorf("Expected reason %s, got %s", changeMessage, cause.Message)
		}
		if cause.GitLabWebHook.Secret != webHookSecret {
			t.Errorf("Expected WebHook secret %s, got %s", webHookSecret, cause.GitLabWebHook.Secret)
		}
		if !reflect.DeepEqual(gitRevision, cause.GitLabWebHook.Revision) {
			t.Errorf("Expected return revision to match")
		}
	}
}

func TestInstantiateBuildTriggerCauseBitbucketWebHook(t *testing.T) {
	buildTriggerCauses := []buildv1.BuildTriggerCause{}
	changeMessage := buildutil.BuildTriggerCauseBitbucketMsg
	webHookSecret := "<secret>"

	gitRevision := &buildv1.SourceRevision{
		Git: &buildv1.GitSourceRevision{
			Author: buildv1.SourceControlUser{
				Name:  "John Doe",
				Email: "johndoe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildRequest := &buildv1.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildv1.BuildTriggerCause{
				Message: changeMessage,
				BitbucketWebHook: &buildv1.BitbucketWebHookCause{
					CommonWebHookCause: buildv1.CommonWebHookCause{
						Secret:   "<secret>",
						Revision: gitRevision,
					},
				},
			},
		),
	}

	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	buildObject, err := generator.Instantiate(apirequest.NewDefaultContext(), buildRequest)
	if err != nil {
		t.Errorf("Expected error to be nil, got %v", err)
	}
	for _, cause := range buildObject.Spec.TriggeredBy {
		if cause.Message != changeMessage {
			t.Errorf("Expected reason %s, got %s", changeMessage, cause.Message)
		}
		if cause.BitbucketWebHook.Secret != webHookSecret {
			t.Errorf("Expected WebHook secret %s, got %s", webHookSecret, cause.BitbucketWebHook.Secret)
		}
		if !reflect.DeepEqual(gitRevision, cause.BitbucketWebHook.Revision) {
			t.Errorf("Expected return revision to match")
		}
	}
}

func TestOverrideDockerStrategyNoCacheOption(t *testing.T) {
	buildConfigFunc := func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
		return &buildv1.BuildConfig{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: buildv1.BuildConfigSpec{
				CommonSpec: buildv1.CommonSpec{
					Source: mocks.MockSource(),
					Strategy: buildv1.BuildStrategy{
						DockerStrategy: &buildv1.DockerBuildStrategy{
							NoCache: true,
						},
					},
					Revision: &buildv1.SourceRevision{
						Git: &buildv1.GitSourceRevision{
							Commit: "1234",
						},
					},
				},
			},
		}, nil
	}

	g := mockBuildGenerator(buildConfigFunc, nil, nil, nil, nil, nil, nil)
	build, err := g.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error encountered:  %v", err)
	}
	if build.Spec.Strategy.DockerStrategy.NoCache != true {
		t.Errorf("Spec.Strategy.DockerStrategy.NoCache was overwritten by nil buildRequest option, but should not have been")
	}
}

func TestOverrideSourceStrategyIncrementalOption(t *testing.T) {
	myTrue := true
	buildConfigFunc := func(ctx context.Context, name string, options *metav1.GetOptions) (*buildv1.BuildConfig, error) {
		return &buildv1.BuildConfig{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: buildv1.BuildConfigSpec{
				CommonSpec: buildv1.CommonSpec{
					Source: mocks.MockSource(),
					Strategy: buildv1.BuildStrategy{
						SourceStrategy: &buildv1.SourceBuildStrategy{
							Incremental: &myTrue,
							From: corev1.ObjectReference{
								Kind:      "ImageStreamTag",
								Name:      "testRepo:test",
								Namespace: "testns",
							},
						},
					},
					Revision: &buildv1.SourceRevision{
						Git: &buildv1.GitSourceRevision{
							Commit: "1234",
						},
					},
				},
			},
		}, nil
	}

	g := mockBuildGenerator(buildConfigFunc, nil, nil, nil, nil, nil, nil)
	build, err := g.Instantiate(apirequest.NewDefaultContext(), &buildv1.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error encountered:  %v", err)
	}
	if *build.Spec.Strategy.SourceStrategy.Incremental != true {
		t.Errorf("Spec.Strategy.SourceStrategy.Incremental was overwritten by nil buildRequest option, but should not have been")
	}
}
