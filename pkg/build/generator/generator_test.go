package generator

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/apis/build/validation"
	mocks "github.com/openshift/origin/pkg/build/generator/test"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

type FakeDockerCfg map[string]map[string]string

const (
	originalImage = "originalimage"
	newImage      = originalImage + ":" + newTag

	tagName          = "test"
	unmatchedTagName = "unmatched"

	// immutable imageid associated w/ test tag
	newTag = "123"

	imageRepoName          = "testRepo"
	unmatchedImageRepoName = "unmatchedRepo"
	imageRepoNamespace     = "testns"

	dockerReference       = "dockerReference"
	latestDockerReference = "latestDockerReference"
)

func TestInstantiate(t *testing.T) {
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestInstantiateBinary(t *testing.T) {
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	build, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{Binary: &buildapi.BinaryBuildSource{}})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Spec.Source.Binary == nil {
		t.Errorf("build should have a binary source value, has nil")
	}
	build, err = generator.Clone(apirequest.NewDefaultContext(), &buildapi.BuildRequest{Binary: &buildapi.BinaryBuildSource{}})
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
			GetBuildConfigFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput()), nil
			},
			UpdateBuildConfigFunc: func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
				instantiationCalls++
				return fmt.Errorf("update-error")
			},
		}}

	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "update-error") {
		t.Errorf("Expected update-error, got different %v", err)
	}
}
*/

func TestInstantiateDeletingError(t *testing.T) {
	source := mocks.MockSource()
	generator := BuildGenerator{Client: TestingClient{
		GetBuildConfigFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
			bc := &buildapi.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						buildapi.BuildConfigPausedAnnotation: "true",
					},
				},
				Spec: buildapi.BuildConfigSpec{
					CommonSpec: buildapi.CommonSpec{
						Source: source,
						Revision: &buildapi.SourceRevision{
							Git: &buildapi.GitSourceRevision{
								Commit: "1234",
							},
						},
					},
				},
			}
			return bc, nil
		},
		GetBuildFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
			build := &buildapi.Build{
				Spec: buildapi.BuildSpec{
					CommonSpec: buildapi.CommonSpec{
						Source: source,
						Revision: &buildapi.SourceRevision{
							Git: &buildapi.GitSourceRevision{
								Commit: "1234",
							},
						},
					},
				},
				Status: buildapi.BuildStatus{
					Config: &kapi.ObjectReference{
						Name: "buildconfig",
					},
				},
			}
			return build, nil
		},
	}}
	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "BuildConfig is paused") {
		t.Errorf("Expected error, got different %v", err)
	}
	_, err = generator.Clone(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
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
	client.GetBuildConfigFunc = func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
		bc := &buildapi.BuildConfig{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{},
			},
			Spec: buildapi.BuildConfigSpec{
				CommonSpec: buildapi.CommonSpec{
					Source: buildapi.BuildSource{
						Binary: &buildapi.BinaryBuildSource{},
					},
				},
			},
		}
		return bc, nil
	}
	client.GetBuildFunc = func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
		build := &buildapi.Build{
			Spec: buildapi.BuildSpec{
				CommonSpec: buildapi.CommonSpec{
					Source: buildapi.BuildSource{
						Binary: &buildapi.BinaryBuildSource{},
					},
				},
			},
			Status: buildapi.BuildStatus{
				Config: &kapi.ObjectReference{
					Name: "buildconfig",
				},
			},
		}
		return build, nil
	}

	build, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Spec.Source.Binary != nil {
		t.Errorf("build should not have a binary source value, has %v", build.Spec.Source.Binary)
	}
	build, err = generator.Clone(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
	if build.Spec.Source.Binary != nil {
		t.Errorf("build should not have a binary source value, has %v", build.Spec.Source.Binary)
	}
}

func TestInstantiateGetBuildConfigError(t *testing.T) {
	generator := BuildGenerator{Client: TestingClient{
		GetBuildConfigFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
			return nil, fmt.Errorf("get-error")
		},
		GetImageStreamFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
			return nil, fmt.Errorf("get-error")
		},
		GetImageStreamImageFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
			return nil, fmt.Errorf("get-error")
		},
		GetImageStreamTagFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
			return nil, fmt.Errorf("get-error")
		},
	}}

	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
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
			GetBuildConfigFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return nil, fmt.Errorf("get-error")
			},
		}}

	_, err := generator.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "get-error") {
		t.Errorf("Expected get-error, got different %v", err)
	}
}

func TestInstantiateWithImageTrigger(t *testing.T) {
	imageID := "the-image-id-12345"
	defaultTriggers := func() []buildapi.BuildTriggerPolicy {
		return []buildapi.BuildTriggerPolicy{
			{
				Type: buildapi.GenericWebHookBuildTriggerType,
			},
			{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					From: &kapi.ObjectReference{
						Name: "image1:tag1",
						Kind: "ImageStreamTag",
					},
				},
			},
			{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					From: &kapi.ObjectReference{
						Name:      "image2:tag2",
						Namespace: "image2ns",
						Kind:      "ImageStreamTag",
					},
				},
			},
		}
	}
	triggersWithImageID := func() []buildapi.BuildTriggerPolicy {
		triggers := defaultTriggers()
		triggers[2].ImageChange.LastTriggeredImageID = imageID
		return triggers
	}
	tests := []struct {
		name          string
		reqFrom       *kapi.ObjectReference
		triggerIndex  int // index of trigger that will be updated with the image id, if -1, no update expected
		triggers      []buildapi.BuildTriggerPolicy
		errorExpected bool
	}{
		{
			name: "default trigger",
			reqFrom: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "image3:tag3",
			},
			triggerIndex: 1,
			triggers:     defaultTriggers(),
		},
		{
			name: "trigger with from",
			reqFrom: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "image1:tag1",
			},
			triggerIndex: 2,
			triggers:     defaultTriggers(),
		},
		{
			name: "trigger with from and namespace",
			reqFrom: &kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      "image2:tag2",
				Namespace: "image2ns",
			},
			triggerIndex: 3,
			triggers:     defaultTriggers(),
		},
		{
			name: "existing image id",
			reqFrom: &kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "image1:tag1",
			},
			triggers:      triggersWithImageID(),
			errorExpected: true,
		},
	}

	source := mocks.MockSource()
	for _, tc := range tests {
		bc := &buildapi.BuildConfig{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault},
			Spec: buildapi.BuildConfigSpec{
				CommonSpec: buildapi.CommonSpec{
					Strategy: buildapi.BuildStrategy{
						SourceStrategy: &buildapi.SourceBuildStrategy{
							From: kapi.ObjectReference{
								Name: "image3:tag3",
								Kind: "ImageStreamTag",
							},
						},
					},
					Source: source,
					Revision: &buildapi.SourceRevision{
						Git: &buildapi.GitSourceRevision{
							Commit: "1234",
						},
					},
				},
				Triggers: tc.triggers,
			},
		}
		imageStreamTagFunc := func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
			return &imageapi.ImageStreamTag{
				Image: imageapi.Image{
					ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
					DockerImageReference: "ref@" + name,
				},
			}, nil
		}

		generator := mockBuildGenerator(nil, nil, nil, nil, nil, imageStreamTagFunc, nil)
		client := generator.Client.(TestingClient)
		client.GetBuildConfigFunc =
			func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return bc, nil
			}
		client.UpdateBuildConfigFunc =
			func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
				bc = buildConfig
				return nil
			}
		client.GetImageStreamFunc =
			func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
				return &imageapi.ImageStream{
					ObjectMeta: metav1.ObjectMeta{Name: name},
					Status: imageapi.ImageStreamStatus{
						DockerImageRepository: originalImage,
						Tags: map[string]imageapi.TagEventList{
							"tag1": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: "ref/" + name + ":tag1",
									},
								},
							},
							"tag2": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: "ref/" + name + ":tag2",
									},
								},
							},
							"tag3": {
								Items: []imageapi.TagEvent{
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

		req := &buildapi.BuildRequest{
			TriggeredByImage: &kapi.ObjectReference{
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
			// Ensure that other triggers are updated with the latest docker image ref
			if bc.Spec.Triggers[i].Type == buildapi.ImageChangeBuildTriggerType {
				from := bc.Spec.Triggers[i].ImageChange.From
				if from == nil {
					from = buildapi.GetInputReference(bc.Spec.Strategy)
				}
				if bc.Spec.Triggers[i].ImageChange.LastTriggeredImageID != ("ref/" + from.Name) {
					t.Errorf("%s: expected LastTriggeredImageID for trigger at %d (%+v) to be %s. Got: %s", tc.name, i, bc.Spec.Triggers[i].ImageChange.From, "ref/"+from.Name, bc.Spec.Triggers[i].ImageChange.LastTriggeredImageID)
				}
			}
		}
	}
}

func TestInstantiateWithBuildRequestEnvs(t *testing.T) {
	buildRequestWithEnv := buildapi.BuildRequest{
		Env: []kapi.EnvVar{{Name: "FOO", Value: "BAR"}},
	}
	buildRequestWithoutEnv := buildapi.BuildRequest{}

	tests := []struct {
		bcfunc           func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error)
		req              buildapi.BuildRequest
		expectedEnvValue string
	}{
		{
			bcfunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithEnv,
			expectedEnvValue: "BAR",
		},
		{
			bcfunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockDockerStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithEnv,
			expectedEnvValue: "BAR",
		},
		{
			bcfunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockCustomStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithEnv,
			expectedEnvValue: "BAR",
		},
		{
			bcfunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockJenkinsStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithEnv,
			expectedEnvValue: "BAR",
		},
		{
			bcfunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithoutEnv,
			expectedEnvValue: "VAR",
		},
		{
			bcfunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockDockerStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithoutEnv,
			expectedEnvValue: "VAR",
		},
		{
			bcfunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
				return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockCustomStrategyForEnvs(), mocks.MockOutput()), nil
			},
			req:              buildRequestWithoutEnv,
			expectedEnvValue: "VAR",
		},
		{
			bcfunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
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
	c.GetBuildConfigFunc = func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
		bc := mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput())
		bc.Status.LastVersion = 1
		return bc, nil
	}
	g.Client = c

	// Version not specified
	_, err := g.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	// Version specified and it matches
	lastVersion := int64(1)
	_, err = g.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{LastVersion: &lastVersion})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}

	// Version specified, but doesn't match
	lastVersion = 0
	_, err = g.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{LastVersion: &lastVersion})
	if err == nil {
		t.Errorf("Expected an error and did not get one")
	}
}

func TestInstantiateWithMissingImageStream(t *testing.T) {
	g := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)
	c := g.Client.(TestingClient)
	c.GetImageStreamFunc = func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
		return nil, errors.NewNotFound(imageapi.Resource("imagestreams"), "testRepo")
	}
	g.Client = c

	_, err := g.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
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
	c.GetBuildConfigFunc = func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
		bc := mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput())
		bc.Status.LastVersion = 1
		return bc, nil
	}
	g.Client = c

	req := &buildapi.BuildRequest{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{
				"a_1": "a_value1",
				// build number is set as an annotation on the generated build, so we
				// shouldn't be able to ovewrite it here.
				buildapi.BuildNumberAnnotation: "bad_annotation",
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
	if build.Annotations["a_1"] != "a_value1" || build.Annotations[buildapi.BuildNumberAnnotation] == "bad_annotation" {
		t.Errorf("Build annotations were merged incorrectly: %v", build.Annotations)
	}
	if build.Labels["l_1"] != "l_value1" || build.Labels[buildapi.BuildLabel] == "bad_label" {
		t.Errorf("Build labels were merged incorrectly: %v", build.Labels)
	}
}

func TestFindImageTrigger(t *testing.T) {
	defaultTrigger := &buildapi.ImageChangeTrigger{}
	image1Trigger := &buildapi.ImageChangeTrigger{
		From: &kapi.ObjectReference{
			Name: "image1:tag1",
		},
	}
	image2Trigger := &buildapi.ImageChangeTrigger{
		From: &kapi.ObjectReference{
			Name:      "image2:tag2",
			Namespace: "image2ns",
		},
	}
	image4Trigger := &buildapi.ImageChangeTrigger{
		From: &kapi.ObjectReference{
			Name: "image4:tag4",
		},
	}
	image5Trigger := &buildapi.ImageChangeTrigger{
		From: &kapi.ObjectReference{
			Name:      "image5:tag5",
			Namespace: "bcnamespace",
		},
	}
	bc := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testbc",
			Namespace: "bcnamespace",
		},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Strategy: buildapi.BuildStrategy{
					SourceStrategy: &buildapi.SourceBuildStrategy{
						From: kapi.ObjectReference{
							Name: "image3:tag3",
							Kind: "ImageStreamTag",
						},
					},
				},
			},
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: defaultTrigger,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: image1Trigger,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: image2Trigger,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: image4Trigger,
				},
				{
					Type:        buildapi.ImageChangeBuildTriggerType,
					ImageChange: image5Trigger,
				},
			},
		},
	}

	tests := []struct {
		name   string
		input  *kapi.ObjectReference
		expect *buildapi.ImageChangeTrigger
	}{
		{
			name:   "nil reference",
			input:  nil,
			expect: nil,
		},
		{
			name: "match name",
			input: &kapi.ObjectReference{
				Name: "image1:tag1",
			},
			expect: image1Trigger,
		},
		{
			name: "mismatched namespace",
			input: &kapi.ObjectReference{
				Name:      "image1:tag1",
				Namespace: "otherns",
			},
			expect: nil,
		},
		{
			name: "match name and namespace",
			input: &kapi.ObjectReference{
				Name:      "image2:tag2",
				Namespace: "image2ns",
			},
			expect: image2Trigger,
		},
		{
			name: "match default trigger",
			input: &kapi.ObjectReference{
				Name: "image3:tag3",
			},
			expect: defaultTrigger,
		},
		{
			name: "input includes bc namespace",
			input: &kapi.ObjectReference{
				Name:      "image4:tag4",
				Namespace: "bcnamespace",
			},
			expect: image4Trigger,
		},
		{
			name: "implied namespace in trigger input",
			input: &kapi.ObjectReference{
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
		CreateBuildFunc: func(ctx apirequest.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
			return &buildapi.Build{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-build-1",
					Namespace: metav1.NamespaceDefault,
				},
			}, nil
		},
	}}

	_, err := generator.Clone(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error %v", err)
	}
}

func TestCloneError(t *testing.T) {
	generator := BuildGenerator{Client: TestingClient{
		GetBuildFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
			return nil, fmt.Errorf("get-error")
		},
	}}

	_, err := generator.Clone(apirequest.NewContext(), &buildapi.BuildRequest{})
	if err == nil || !strings.Contains(err.Error(), "get-error") {
		t.Errorf("Expected get-error, got different %v", err)
	}
}

func TestCreateBuild(t *testing.T) {
	build := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: metav1.NamespaceDefault,
		},
	}
	generator := BuildGenerator{Client: TestingClient{
		CreateBuildFunc: func(ctx apirequest.Context, build *buildapi.Build) error {
			return nil
		},
		GetBuildFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
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
	build := &buildapi.Build{
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
	build := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build",
			Namespace: metav1.NamespaceDefault,
		},
	}
	generator := BuildGenerator{Client: TestingClient{
		CreateBuildFunc: func(ctx apirequest.Context, build *buildapi.Build) error {
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
	bc := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			UID:       "test-uid",
			Name:      "test-build-config",
			Namespace: metav1.NamespaceDefault,
			Labels:    map[string]string{"testlabel": "testvalue"},
		},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: source,
				Revision: &buildapi.SourceRevision{
					Git: &buildapi.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy:  strategy,
				Output:    output,
				Resources: resources,
			},
		},
		Status: buildapi.BuildConfigStatus{
			LastVersion: 12,
		},
	}
	revision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
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
	if build.Annotations[buildapi.BuildConfigAnnotation] != bc.Name {
		t.Errorf("Build does not contain annotation from BuildConfig")
	}
	if build.Labels[buildapi.BuildConfigLabel] != bc.Name {
		t.Errorf("Build does not contain labels from BuildConfig")
	}
	if build.Labels[buildapi.BuildConfigLabelDeprecated] != bc.Name {
		t.Errorf("Build does not contain labels from BuildConfig")
	}
	if build.Status.Config.Name != bc.Name || build.Status.Config.Namespace != bc.Namespace || build.Status.Config.Kind != "BuildConfig" {
		t.Errorf("Build does not contain correct BuildConfig reference: %v", build.Status.Config)
	}
	if build.Annotations[buildapi.BuildNumberAnnotation] != "13" {
		t.Errorf("Build number annotation value %s does not match expected value 13", build.Annotations[buildapi.BuildNumberAnnotation])
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
	errors := validation.ValidateBuild(build)
	if len(errors) > 0 {
		t.Fatalf("Unexpected validation errors %v", errors)
	}
}

func TestGenerateBuildWithImageTagForSourceStrategyImageRepository(t *testing.T) {
	source := mocks.MockSource()
	strategy := mocks.MockSourceStrategyForImageRepository()
	output := mocks.MockOutput()
	bc := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-config",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: source,
				Revision: &buildapi.SourceRevision{
					Git: &buildapi.GitSourceRevision{
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
	generator := BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).Core(),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: TestingClient{
			GetImageStreamFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
				return &imageapi.ImageStream{
					ObjectMeta: metav1.ObjectMeta{Name: imageRepoName},
					Status: imageapi.ImageStreamStatus{
						DockerImageRepository: originalImage,
						Tags: map[string]imageapi.TagEventList{
							tagName: {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("%s:%s", originalImage, newTag),
										Image:                newTag,
									},
								},
							},
						},
					},
				}, nil
			},
			GetImageStreamTagFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{
					Image: imageapi.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{
					Image: imageapi.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},

			UpdateBuildConfigFunc: func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
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
	bc := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-config",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: source,
				Revision: &buildapi.SourceRevision{
					Git: &buildapi.GitSourceRevision{
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
	generator := BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).Core(),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: TestingClient{
			GetImageStreamFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
				return &imageapi.ImageStream{
					ObjectMeta: metav1.ObjectMeta{Name: imageRepoName},
					Status: imageapi.ImageStreamStatus{
						DockerImageRepository: originalImage,
						Tags: map[string]imageapi.TagEventList{
							tagName: {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("%s:%s", originalImage, newTag),
										Image:                newTag,
									},
								},
							},
						},
					},
				}, nil
			},
			GetImageStreamTagFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{
					Image: imageapi.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{
					Image: imageapi.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			UpdateBuildConfigFunc: func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
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
	bc := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-build-config",
			Namespace: metav1.NamespaceDefault,
		},
		Spec: buildapi.BuildConfigSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: source,
				Revision: &buildapi.SourceRevision{
					Git: &buildapi.GitSourceRevision{
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
	generator := BuildGenerator{
		Secrets:         fake.NewSimpleClientset(fakeSecrets...).Core(),
		ServiceAccounts: mocks.MockBuilderServiceAccount(mocks.MockBuilderSecrets()),
		Client: TestingClient{
			GetImageStreamFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
				return &imageapi.ImageStream{
					ObjectMeta: metav1.ObjectMeta{Name: imageRepoName},
					Status: imageapi.ImageStreamStatus{
						DockerImageRepository: originalImage,
						Tags: map[string]imageapi.TagEventList{
							tagName: {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("%s:%s", originalImage, newTag),
										Image:                newTag,
									},
								},
							},
						},
					},
				}, nil
			},
			GetImageStreamTagFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
				return &imageapi.ImageStreamTag{
					Image: imageapi.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			GetImageStreamImageFunc: func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
				return &imageapi.ImageStreamImage{
					Image: imageapi.Image{
						ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":@id"},
						DockerImageReference: originalImage + ":" + newTag,
					},
				}, nil
			},
			UpdateBuildConfigFunc: func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
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
	build := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-build",
			Annotations: map[string]string{
				buildapi.BuildJenkinsStatusJSONAnnotation:      "foo",
				buildapi.BuildJenkinsLogURLAnnotation:          "bar",
				buildapi.BuildJenkinsConsoleLogURLAnnotation:   "bar",
				buildapi.BuildJenkinsBlueOceanLogURLAnnotation: "bar",
				buildapi.BuildJenkinsBuildURIAnnotation:        "baz",
				buildapi.BuildPodNameAnnotation:                "ruby-sample-build-1-build",
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
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: source,
				Revision: &buildapi.SourceRevision{
					Git: &buildapi.GitSourceRevision{
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
	if _, ok := newBuild.ObjectMeta.Annotations[buildapi.BuildJenkinsStatusJSONAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildapi.BuildJenkinsStatusJSONAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildapi.BuildJenkinsLogURLAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildapi.BuildJenkinsLogURLAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildapi.BuildJenkinsConsoleLogURLAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildapi.BuildJenkinsConsoleLogURLAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildapi.BuildJenkinsBlueOceanLogURLAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildapi.BuildJenkinsBlueOceanLogURLAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildapi.BuildJenkinsBuildURIAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildapi.BuildJenkinsBuildURIAnnotation)
	}
	if _, ok := newBuild.ObjectMeta.Annotations[buildapi.BuildPodNameAnnotation]; ok {
		t.Errorf("%s annotation exists, expected it not to", buildapi.BuildPodNameAnnotation)
	}
	if !reflect.DeepEqual(build.ObjectMeta.OwnerReferences, newBuild.ObjectMeta.OwnerReferences) {
		t.Errorf("Build OwnerReferences does not match the original Build OwnerReferences")
	}

}

func TestGenerateBuildFromBuildWithBuildConfig(t *testing.T) {
	source := mocks.MockSource()
	strategy := mockDockerStrategyForImageRepository()
	output := mocks.MockOutput()
	annotatedBuild := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "annotatedBuild",
			Annotations: map[string]string{
				buildapi.BuildCloneAnnotation: "sourceOfBuild",
			},
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: source,
				Revision: &buildapi.SourceRevision{
					Git: &buildapi.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}
	nonAnnotatedBuild := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nonAnnotatedBuild",
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: source,
				Revision: &buildapi.SourceRevision{
					Git: &buildapi.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}

	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "buildConfigName",
		},
		Status: buildapi.BuildConfigStatus{
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
	if newBuild.Annotations[buildapi.BuildNumberAnnotation] != "6" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildapi.BuildNumberAnnotation], "6")
	}
	if newBuild.Annotations[buildapi.BuildCloneAnnotation] != "annotatedBuild" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildapi.BuildCloneAnnotation], "annotatedBuild")
	}

	newBuild = generateBuildFromBuild(nonAnnotatedBuild, buildConfig)
	if !reflect.DeepEqual(nonAnnotatedBuild.Spec, newBuild.Spec) {
		t.Errorf("Build parameters does not match the original Build parameters")
	}
	if !reflect.DeepEqual(nonAnnotatedBuild.ObjectMeta.Labels, newBuild.ObjectMeta.Labels) {
		t.Errorf("Build labels does not match the original Build labels")
	}
	// was incremented by previous test, so expect 7 now.
	if newBuild.Annotations[buildapi.BuildNumberAnnotation] != "7" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildapi.BuildNumberAnnotation], "7")
	}
	if newBuild.Annotations[buildapi.BuildCloneAnnotation] != "nonAnnotatedBuild" {
		t.Errorf("Build number annotation is %s expected %s", newBuild.Annotations[buildapi.BuildCloneAnnotation], "nonAnnotatedBuild")
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

	// Full custom build with a Image and a well defined environment variable image value,
	// both should be replaced.  Additional environment variables should not be touched.
	build.Spec.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Spec.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	build.Spec.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: originalImage}
	UpdateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
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

	// Full custom build with base image that is not matched
	// Base image name should be unchanged
	UpdateCustomImageEnv(build.Spec.Strategy.CustomStrategy, "dummy")
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
	build.Spec.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 2)
	build.Spec.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someEnvVar", Value: originalImage}
	build.Spec.Strategy.CustomStrategy.Env[1] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: "dummy"}
	UpdateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
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
	build.Spec.Strategy.CustomStrategy.Env = make([]kapi.EnvVar, 1)
	build.Spec.Strategy.CustomStrategy.Env[0] = kapi.EnvVar{Name: "someImage", Value: originalImage}
	UpdateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Value != originalImage {
		t.Errorf("Random env variable was improperly substituted in custom strategy")
	}
	if build.Spec.Strategy.CustomStrategy.Env[1].Name != buildapi.CustomBuildStrategyBaseImageKey || build.Spec.Strategy.CustomStrategy.Env[1].Value != newImage {
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
	UpdateCustomImageEnv(build.Spec.Strategy.CustomStrategy, newImage)
	if build.Spec.Strategy.CustomStrategy.Env[0].Name != buildapi.CustomBuildStrategyBaseImageKey || build.Spec.Strategy.CustomStrategy.Env[0].Value != newImage {
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
		buildName := getNextBuildNameFromBuild(&buildapi.Build{ObjectMeta: metav1.ObjectMeta{Name: tc.value}}, nil)
		if matched, err := regexp.MatchString(tc.expected, buildName); !matched || err != nil {
			t.Errorf("(%d) Unexpected build name, got %s expected %s", i, buildName, tc.expected)
		}
	}
}

func TestGetNextBuildNameFromBuildWithBuildConfig(t *testing.T) {
	testCases := []struct {
		value       string
		buildConfig *buildapi.BuildConfig
		expected    string
	}{
		// 0
		{
			"mybuild-1",
			&buildapi.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "buildConfigName",
				},
				Status: buildapi.BuildConfigStatus{
					LastVersion: 5,
				},
			},
			`^buildConfigName-6$`,
		},
		// 1
		{
			"mybuild-1-1426794070",
			&buildapi.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: "buildConfigName",
				},
				Status: buildapi.BuildConfigStatus{
					LastVersion: 5,
				},
			},
			`^buildConfigName-6$`,
		},
	}

	for i, tc := range testCases {
		buildName := getNextBuildNameFromBuild(&buildapi.Build{ObjectMeta: metav1.ObjectMeta{Name: tc.value}}, tc.buildConfig)
		if matched, err := regexp.MatchString(tc.expected, buildName); !matched || err != nil {
			t.Errorf("(%d) Unexpected build name, got %s expected %s", i, buildName, tc.expected)
		}
	}
}

func TestResolveImageStreamRef(t *testing.T) {
	type resolveTest struct {
		streamRef         kapi.ObjectReference
		tag               string
		expectedSuccess   bool
		expectedDockerRef string
	}
	generator := mockBuildGenerator(nil, nil, nil, nil, nil, nil, nil)

	tests := []resolveTest{
		{
			streamRef: kapi.ObjectReference{
				Name: imageRepoName,
			},
			tag:               tagName,
			expectedSuccess:   false,
			expectedDockerRef: dockerReference,
		},
		{
			streamRef: kapi.ObjectReference{
				Kind: "ImageStreamTag",
				Name: imageRepoName + ":" + tagName,
			},
			expectedSuccess:   true,
			expectedDockerRef: dockerReference,
		},
		{
			streamRef: kapi.ObjectReference{
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
		ref, error := generator.resolveImageStreamReference(apirequest.NewDefaultContext(), test.streamRef, "")
		if error != nil {
			if test.expectedSuccess {
				t.Errorf("Scenario %d: Unexpected error %v", i, error)
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

func mockResources() kapi.ResourceRequirements {
	res := kapi.ResourceRequirements{}
	res.Limits = kapi.ResourceList{}
	res.Limits[kapi.ResourceCPU] = resource.MustParse("100m")
	res.Limits[kapi.ResourceMemory] = resource.MustParse("100Mi")
	return res
}

func mockDockerStrategyForNilImage() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		DockerStrategy: &buildapi.DockerBuildStrategy{
			NoCache: true,
		},
	}
}

func mockDockerStrategyForDockerImage(name string, options *metav1.GetOptions) buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		DockerStrategy: &buildapi.DockerBuildStrategy{
			NoCache: true,
			From: &kapi.ObjectReference{
				Kind: "DockerImage",
				Name: name,
			},
		},
	}
}

func mockDockerStrategyForImageRepository() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		DockerStrategy: &buildapi.DockerBuildStrategy{
			NoCache: true,
			From: &kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
	}
}

func mockCustomStrategyForDockerImage(name string, options *metav1.GetOptions) buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		CustomStrategy: &buildapi.CustomBuildStrategy{
			From: kapi.ObjectReference{
				Kind: "DockerImage",
				Name: originalImage,
			},
		},
	}
}

func mockCustomStrategyForImageRepository() buildapi.BuildStrategy {
	return buildapi.BuildStrategy{
		CustomStrategy: &buildapi.CustomBuildStrategy{
			From: kapi.ObjectReference{
				Kind:      "ImageStreamTag",
				Name:      imageRepoName + ":" + tagName,
				Namespace: imageRepoNamespace,
			},
		},
	}
}

func mockOutputWithImageName(name string, options *metav1.GetOptions) buildapi.BuildOutput {
	return buildapi.BuildOutput{
		To: &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: name,
		},
	}
}

func mockBuild(source buildapi.BuildSource, strategy buildapi.BuildStrategy, output buildapi.BuildOutput) *buildapi.Build {
	return &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-build",
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: source,
				Revision: &buildapi.SourceRevision{
					Git: &buildapi.GitSourceRevision{
						Commit: "1234",
					},
				},
				Strategy: strategy,
				Output:   output,
			},
		},
	}
}

func getBuildConfigFunc(buildConfigFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error)) func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
	if buildConfigFunc == nil {
		return func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
			return mocks.MockBuildConfig(mocks.MockSource(), mocks.MockSourceStrategyForImageRepository(), mocks.MockOutput()), nil
		}
	}
	return buildConfigFunc
}

func getUpdateBuildConfigFunc(updateBuildConfigFunc func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error) func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
	if updateBuildConfigFunc == nil {
		return func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
			return nil
		}
	}
	return updateBuildConfigFunc
}

func getCreateBuildFunc(createBuildConfigFunc func(ctx apirequest.Context, build *buildapi.Build) error, b *buildapi.Build) func(ctx apirequest.Context, build *buildapi.Build) error {
	if createBuildConfigFunc == nil {
		return func(ctx apirequest.Context, build *buildapi.Build) error {
			*b = *build
			return nil
		}
	}
	return createBuildConfigFunc
}

func getGetBuildFunc(getBuildFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error), b *buildapi.Build) func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
	if getBuildFunc == nil {
		return func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
			if b == nil {
				return &buildapi.Build{}, nil
			}
			return b, nil
		}
	}
	return getBuildFunc
}

func getGetImageStreamFunc(getImageStreamFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error)) func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
	if getImageStreamFunc == nil {
		return func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
			if name != imageRepoName {
				return &imageapi.ImageStream{}, nil
			}
			return &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      imageRepoName,
					Namespace: imageRepoNamespace,
				},
				Status: imageapi.ImageStreamStatus{
					DockerImageRepository: "repo/namespace/image",
					Tags: map[string]imageapi.TagEventList{
						tagName: {
							Items: []imageapi.TagEvent{
								{DockerImageReference: dockerReference},
							},
						},
						imageapi.DefaultImageTag: {
							Items: []imageapi.TagEvent{
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

func getGetImageStreamTagFunc(getImageStreamTagFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error)) func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
	if getImageStreamTagFunc == nil {
		return func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
			return &imageapi.ImageStreamTag{
				Image: imageapi.Image{
					ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":" + newTag},
					DockerImageReference: latestDockerReference,
				},
			}, nil
		}
	}
	return getImageStreamTagFunc
}

func getGetImageStreamImageFunc(getImageStreamImageFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error)) func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
	if getImageStreamImageFunc == nil {
		return func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
			return &imageapi.ImageStreamImage{
				Image: imageapi.Image{
					ObjectMeta:           metav1.ObjectMeta{Name: imageRepoName + ":@id"},
					DockerImageReference: latestDockerReference,
				},
			}, nil
		}
	}
	return getImageStreamImageFunc
}

func mockBuildGenerator(buildConfigFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error),
	updateBuildConfigFunc func(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error,
	createBuildFunc func(ctx apirequest.Context, build *buildapi.Build) error,
	getBuildFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error),
	getImageStreamFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error),
	getImageStreamTagFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error),
	getImageStreamImageFunc func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error),
) *BuildGenerator {
	fakeSecrets := []runtime.Object{}
	for _, s := range mocks.MockBuilderSecrets() {
		fakeSecrets = append(fakeSecrets, s)
	}
	b := buildapi.Build{}
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
	revision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Commit: "abcd",
		},
	}
	dockerCfgTable := map[string]map[string][]byte{
		// FIXME: This image pull spec does not return ANY registry, but it should
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
	changeMessage := buildapi.BuildTriggerCauseConfigMsg

	buildTriggerCauses := []buildapi.BuildTriggerCause{}
	buildRequest := &buildapi.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
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
	buildTriggerCauses := []buildapi.BuildTriggerCause{}
	changeMessage := buildapi.BuildTriggerCauseImageMsg
	imageID := "centos@sha256:b3da5267165b"
	refName := "centos:7"
	refKind := "ImageStreamTag"

	buildRequest := &buildapi.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: changeMessage,
				ImageChangeBuild: &buildapi.ImageChangeCause{
					ImageID: imageID,
					FromRef: &kapi.ObjectReference{
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
		if cause.Message != buildapi.BuildTriggerCauseImageMsg {
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
	buildTriggerCauses := []buildapi.BuildTriggerCause{}
	changeMessage := "Generic WebHook"
	webHookSecret := "<secret>"

	gitRevision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Author: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "johndoe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildRequest := &buildapi.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: changeMessage,
				GenericWebHook: &buildapi.GenericWebHookCause{
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
	buildTriggerCauses := []buildapi.BuildTriggerCause{}
	changeMessage := buildapi.BuildTriggerCauseGithubMsg
	webHookSecret := "<secret>"

	gitRevision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Author: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "johndoe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildRequest := &buildapi.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: changeMessage,
				GitHubWebHook: &buildapi.GitHubWebHookCause{
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
	buildTriggerCauses := []buildapi.BuildTriggerCause{}
	changeMessage := buildapi.BuildTriggerCauseGitLabMsg
	webHookSecret := "<secret>"

	gitRevision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Author: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "johndoe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildRequest := &buildapi.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: changeMessage,
				GitLabWebHook: &buildapi.GitLabWebHookCause{
					CommonWebHookCause: buildapi.CommonWebHookCause{
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
	buildTriggerCauses := []buildapi.BuildTriggerCause{}
	changeMessage := buildapi.BuildTriggerCauseBitbucketMsg
	webHookSecret := "<secret>"

	gitRevision := &buildapi.SourceRevision{
		Git: &buildapi.GitSourceRevision{
			Author: buildapi.SourceControlUser{
				Name:  "John Doe",
				Email: "johndoe@test.com",
			},
			Message: "A random act of kindness",
		},
	}

	buildRequest := &buildapi.BuildRequest{
		TriggeredBy: append(buildTriggerCauses,
			buildapi.BuildTriggerCause{
				Message: changeMessage,
				BitbucketWebHook: &buildapi.BitbucketWebHookCause{
					CommonWebHookCause: buildapi.CommonWebHookCause{
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
	buildConfigFunc := func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
		return &buildapi.BuildConfig{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: buildapi.BuildConfigSpec{
				CommonSpec: buildapi.CommonSpec{
					Source: mocks.MockSource(),
					Strategy: buildapi.BuildStrategy{
						DockerStrategy: &buildapi.DockerBuildStrategy{
							NoCache: true,
						},
					},
					Revision: &buildapi.SourceRevision{
						Git: &buildapi.GitSourceRevision{
							Commit: "1234",
						},
					},
				},
			},
		}, nil
	}

	g := mockBuildGenerator(buildConfigFunc, nil, nil, nil, nil, nil, nil)
	build, err := g.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error encountered:  %v", err)
	}
	if build.Spec.Strategy.DockerStrategy.NoCache != true {
		t.Errorf("Spec.Strategy.DockerStrategy.NoCache was overwritten by nil buildRequest option, but should not have been")
	}
}

func TestOverrideSourceStrategyIncrementalOption(t *testing.T) {
	myTrue := true
	buildConfigFunc := func(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
		return &buildapi.BuildConfig{
			ObjectMeta: metav1.ObjectMeta{},
			Spec: buildapi.BuildConfigSpec{
				CommonSpec: buildapi.CommonSpec{
					Source: mocks.MockSource(),
					Strategy: buildapi.BuildStrategy{
						SourceStrategy: &buildapi.SourceBuildStrategy{
							Incremental: &myTrue,
							From: kapi.ObjectReference{
								Kind:      "ImageStreamTag",
								Name:      "testRepo:test",
								Namespace: "testns",
							},
						},
					},
					Revision: &buildapi.SourceRevision{
						Git: &buildapi.GitSourceRevision{
							Commit: "1234",
						},
					},
				},
			},
		}, nil
	}

	g := mockBuildGenerator(buildConfigFunc, nil, nil, nil, nil, nil, nil)
	build, err := g.Instantiate(apirequest.NewDefaultContext(), &buildapi.BuildRequest{})
	if err != nil {
		t.Errorf("Unexpected error encountered:  %v", err)
	}
	if *build.Spec.Strategy.SourceStrategy.Incremental != true {
		t.Errorf("Spec.Strategy.SourceStrategy.Incremental was overwritten by nil buildRequest option, but should not have been")
	}
}
