package v1

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion/queryparams"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	kinternal "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	internal "github.com/openshift/origin/pkg/build/apis/build"
)

var scheme = runtime.NewScheme()
var Convert = scheme.Convert
var codecs = serializer.NewCodecFactory(scheme)

func init() {
	Install(scheme)
}

func TestFieldSelectorConversions(t *testing.T) {

	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{Install},
		Kind:          v1.GroupVersion.WithKind("Build"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"status", "podName"},
		FieldKeyEvaluatorFn:      internal.BuildFieldSelector,
	}.Check(t)

}

func TestBinaryBuildRequestOptions(t *testing.T) {
	r := &internal.BinaryBuildRequestOptions{
		AsFile: "Dockerfile",
		Commit: "abcdef",
	}
	versioned, err := scheme.ConvertToVersion(r, v1.GroupVersion)
	if err != nil {
		t.Fatal(err)
	}
	params, err := queryparams.Convert(versioned)
	if err != nil {
		t.Fatal(err)
	}
	decoded := &v1.BinaryBuildRequestOptions{}
	if err := scheme.Convert(&params, decoded, nil); err != nil {
		t.Fatal(err)
	}
	if decoded.Commit != "abcdef" || decoded.AsFile != "Dockerfile" {
		t.Errorf("unexpected decoded object: %#v", decoded)
	}
}

func TestV1APIBuildConfigConversion(t *testing.T) {
	buildConfigs := []*v1.BuildConfig{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: v1.BuildConfigSpec{
				CommonSpec: v1.CommonSpec{
					Source: v1.BuildSource{
						Type: v1.BuildSourceGit,
						Git: &v1.GitBuildSource{
							URI: "http://github.com/my/repository",
						},
						ContextDir: "context",
					},
					Strategy: v1.BuildStrategy{
						Type: v1.DockerBuildStrategyType,
						DockerStrategy: &v1.DockerBuildStrategy{
							From: &corev1.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
					Output: v1.BuildOutput{
						To: &corev1.ObjectReference{
							Kind: "ImageStream",
							Name: "outputstream",
						},
					},
				},
				Triggers: []v1.BuildTriggerPolicy{
					{
						Type: v1.ImageChangeBuildTriggerTypeDeprecated,
					},
					{
						Type: v1.GitHubWebHookBuildTriggerTypeDeprecated,
					},
					{
						Type: v1.GenericWebHookBuildTriggerTypeDeprecated,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: v1.BuildConfigSpec{
				CommonSpec: v1.CommonSpec{
					Source: v1.BuildSource{
						Type: v1.BuildSourceGit,
						Git: &v1.GitBuildSource{
							URI: "http://github.com/my/repository",
						},
						ContextDir: "context",
					},
					Strategy: v1.BuildStrategy{
						Type: v1.SourceBuildStrategyType,
						SourceStrategy: &v1.SourceBuildStrategy{
							From: corev1.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
					Output: v1.BuildOutput{
						To: &corev1.ObjectReference{
							Kind: "ImageStream",
							Name: "outputstream",
						},
					},
				},
				Triggers: []v1.BuildTriggerPolicy{
					{
						Type: v1.ImageChangeBuildTriggerTypeDeprecated,
					},
					{
						Type: v1.GitHubWebHookBuildTriggerTypeDeprecated,
					},
					{
						Type: v1.GenericWebHookBuildTriggerTypeDeprecated,
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: v1.BuildConfigSpec{
				CommonSpec: v1.CommonSpec{
					Source: v1.BuildSource{
						Type: v1.BuildSourceGit,
						Git: &v1.GitBuildSource{
							URI: "http://github.com/my/repository",
						},
						ContextDir: "context",
					},
					Strategy: v1.BuildStrategy{
						Type: v1.CustomBuildStrategyType,
						CustomStrategy: &v1.CustomBuildStrategy{
							From: corev1.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
					Output: v1.BuildOutput{
						To: &corev1.ObjectReference{
							Kind: "ImageStream",
							Name: "outputstream",
						},
					},
				},
				Triggers: []v1.BuildTriggerPolicy{
					{
						Type: v1.ImageChangeBuildTriggerTypeDeprecated,
					},
					{
						Type: v1.GitHubWebHookBuildTriggerTypeDeprecated,
					},
					{
						Type: v1.GenericWebHookBuildTriggerTypeDeprecated,
					},
				},
			},
		},
	}

	for c, bc := range buildConfigs {

		var internalbuild internal.BuildConfig

		Convert(bc, &internalbuild, nil)
		switch bc.Spec.Strategy.Type {
		case v1.SourceBuildStrategyType:
			if internalbuild.Spec.Strategy.SourceStrategy.From.Kind != "ImageStreamTag" {
				t.Errorf("[%d] Expected From Kind %s, got %s", c, "ImageStreamTag", internalbuild.Spec.Strategy.SourceStrategy.From.Kind)
			}
		case v1.DockerBuildStrategyType:
			if internalbuild.Spec.Strategy.DockerStrategy.From.Kind != "ImageStreamTag" {
				t.Errorf("[%d]Expected From Kind %s, got %s", c, "ImageStreamTag", internalbuild.Spec.Strategy.DockerStrategy.From.Kind)
			}
		case v1.CustomBuildStrategyType:
			if internalbuild.Spec.Strategy.CustomStrategy.From.Kind != "ImageStreamTag" {
				t.Errorf("[%d]Expected From Kind %s, got %s", c, "ImageStreamTag", internalbuild.Spec.Strategy.CustomStrategy.From.Kind)
			}
		}
		if internalbuild.Spec.Output.To.Kind != "ImageStreamTag" {
			t.Errorf("[%d]Expected Output Kind %s, got %s", c, "ImageStreamTag", internalbuild.Spec.Output.To.Kind)
		}
		var foundImageChange, foundGitHub, foundGeneric bool
		for _, trigger := range internalbuild.Spec.Triggers {
			switch trigger.Type {
			case internal.ImageChangeBuildTriggerType:
				foundImageChange = true

			case internal.GenericWebHookBuildTriggerType:
				foundGeneric = true

			case internal.GitHubWebHookBuildTriggerType:
				foundGitHub = true
			}
		}
		if !foundImageChange {
			t.Errorf("ImageChangeTriggerType not converted correctly: %v", internalbuild.Spec.Triggers)
		}
		if !foundGitHub {
			t.Errorf("GitHubWebHookTriggerType not converted correctly: %v", internalbuild.Spec.Triggers)
		}
		if !foundGeneric {
			t.Errorf("GenericWebHookTriggerType not converted correctly: %v", internalbuild.Spec.Triggers)
		}
	}
}

func TestAPIV1NoSourceBuildConfigConversion(t *testing.T) {
	buildConfigs := []*internal.BuildConfig{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: internal.BuildConfigSpec{
				CommonSpec: internal.CommonSpec{
					Source: internal.BuildSource{},
					Strategy: internal.BuildStrategy{
						DockerStrategy: &internal.DockerBuildStrategy{
							From: &kinternal.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: internal.BuildConfigSpec{
				CommonSpec: internal.CommonSpec{
					Source: internal.BuildSource{},
					Strategy: internal.BuildStrategy{
						SourceStrategy: &internal.SourceBuildStrategy{
							From: kinternal.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: internal.BuildConfigSpec{
				CommonSpec: internal.CommonSpec{
					Source: internal.BuildSource{},
					Strategy: internal.BuildStrategy{
						CustomStrategy: &internal.CustomBuildStrategy{
							From: kinternal.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
				},
			},
		},
	}

	for c, bc := range buildConfigs {

		var internalbuild v1.BuildConfig

		Convert(bc, &internalbuild, nil)
		if internalbuild.Spec.Source.Type != v1.BuildSourceNone {
			t.Errorf("Unexpected source type at index %d: %s", c, internalbuild.Spec.Source.Type)
		}
	}
}

func TestInvalidImageChangeTriggerRemoval(t *testing.T) {
	buildConfig := v1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: v1.BuildConfigSpec{
			CommonSpec: v1.CommonSpec{
				Strategy: v1.BuildStrategy{
					Type: v1.DockerBuildStrategyType,
					DockerStrategy: &v1.DockerBuildStrategy{
						From: &corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "fromimage",
						},
					},
				},
			},
			Triggers: []v1.BuildTriggerPolicy{
				{
					Type:        v1.ImageChangeBuildTriggerType,
					ImageChange: &v1.ImageChangeTrigger{},
				},
				{
					Type: v1.ImageChangeBuildTriggerType,
					ImageChange: &v1.ImageChangeTrigger{
						From: &corev1.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "imagestream",
						},
					},
				},
			},
		},
	}

	var internalBC internal.BuildConfig

	Convert(&buildConfig, &internalBC, nil)
	if len(internalBC.Spec.Triggers) != 1 {
		t.Errorf("Expected 1 trigger, got %d", len(internalBC.Spec.Triggers))
	}
	if internalBC.Spec.Triggers[0].ImageChange.From == nil {
		t.Errorf("Expected remaining trigger to have a From value")
	}

}

func TestImageChangeTriggerNilImageChangePointer(t *testing.T) {
	buildConfig := v1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: v1.BuildConfigSpec{
			CommonSpec: v1.CommonSpec{
				Strategy: v1.BuildStrategy{
					Type:           v1.SourceBuildStrategyType,
					SourceStrategy: &v1.SourceBuildStrategy{},
				},
			},
			Triggers: []v1.BuildTriggerPolicy{
				{
					Type:        v1.ImageChangeBuildTriggerType,
					ImageChange: nil,
				},
			},
		},
	}

	var internalBC internal.BuildConfig

	Convert(&buildConfig, &internalBC, nil)
	for _, ic := range internalBC.Spec.Triggers {
		if ic.ImageChange == nil {
			t.Errorf("Expected trigger to have ImageChange value")
		}
	}
}
