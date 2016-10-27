package v1_test

import (
	"testing"

	knewer "k8s.io/kubernetes/pkg/api"
	kolder "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/conversion/queryparams"

	"reflect"

	newer "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
	older "github.com/openshift/origin/pkg/build/api/v1"
	testutil "github.com/openshift/origin/test/util/api"
)

var Convert = knewer.Scheme.Convert

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "Build",
		// Ensure all currently returned labels are supported
		newer.BuildToSelectableFields(&newer.Build{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"name", "status", "podName",
	)

	testutil.CheckFieldLabelConversions(t, "v1", "BuildConfig",
		// Ensure all currently returned labels are supported
		newer.BuildConfigToSelectableFields(&newer.BuildConfig{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"name",
	)
}

func TestBinaryBuildRequestOptions(t *testing.T) {
	r := &newer.BinaryBuildRequestOptions{
		AsFile: "Dockerfile",
		Commit: "abcdef",
	}
	versioned, err := knewer.Scheme.ConvertToVersion(r, kolder.SchemeGroupVersion)
	if err != nil {
		t.Fatal(err)
	}
	params, err := queryparams.Convert(versioned)
	if err != nil {
		t.Fatal(err)
	}
	decoded := &older.BinaryBuildRequestOptions{}
	if err := knewer.Scheme.Convert(&params, decoded, nil); err != nil {
		t.Fatal(err)
	}
	if decoded.Commit != "abcdef" || decoded.AsFile != "Dockerfile" {
		t.Errorf("unexpected decoded object: %#v", decoded)
	}
}

func TestBuildConfigDefaulting(t *testing.T) {
	buildConfig := &older.BuildConfig{
		Spec: older.BuildConfigSpec{
			CommonSpec: older.CommonSpec{
				Source: older.BuildSource{
					Type: older.BuildSourceBinary,
				},
				Strategy: older.BuildStrategy{
					Type: older.DockerBuildStrategyType,
				},
			},
		},
	}

	var internalBuild newer.BuildConfig
	Convert(buildConfig, &internalBuild, nil)

	binary := internalBuild.Spec.Source.Binary
	if binary == (*newer.BinaryBuildSource)(nil) || *binary != (newer.BinaryBuildSource{}) {
		t.Errorf("Expected non-nil but empty Source.Binary as default for Spec")
	}

	dockerStrategy := internalBuild.Spec.Strategy.DockerStrategy
	// DeepEqual needed because DockerBuildStrategy contains slices
	if dockerStrategy == (*newer.DockerBuildStrategy)(nil) || !reflect.DeepEqual(*dockerStrategy, newer.DockerBuildStrategy{}) {
		t.Errorf("Expected non-nil but empty Strategy.DockerStrategy as default for Spec")
	}
}

func TestV1APIBuildConfigConversion(t *testing.T) {
	buildConfigs := []*older.BuildConfig{
		{
			ObjectMeta: kolder.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: older.BuildConfigSpec{
				CommonSpec: older.CommonSpec{
					Source: older.BuildSource{
						Type: older.BuildSourceGit,
						Git: &older.GitBuildSource{
							URI: "http://github.com/my/repository",
						},
						ContextDir: "context",
					},
					Strategy: older.BuildStrategy{
						Type: older.DockerBuildStrategyType,
						DockerStrategy: &older.DockerBuildStrategy{
							From: &kolder.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
					Output: older.BuildOutput{
						To: &kolder.ObjectReference{
							Kind: "ImageStream",
							Name: "outputstream",
						},
					},
				},
				Triggers: []older.BuildTriggerPolicy{
					{
						Type: older.ImageChangeBuildTriggerTypeDeprecated,
					},
					{
						Type: older.GitHubWebHookBuildTriggerTypeDeprecated,
					},
					{
						Type: older.GenericWebHookBuildTriggerTypeDeprecated,
					},
				},
			},
		},
		{
			ObjectMeta: kolder.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: older.BuildConfigSpec{
				CommonSpec: older.CommonSpec{
					Source: older.BuildSource{
						Type: older.BuildSourceGit,
						Git: &older.GitBuildSource{
							URI: "http://github.com/my/repository",
						},
						ContextDir: "context",
					},
					Strategy: older.BuildStrategy{
						Type: older.SourceBuildStrategyType,
						SourceStrategy: &older.SourceBuildStrategy{
							From: kolder.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
					Output: older.BuildOutput{
						To: &kolder.ObjectReference{
							Kind: "ImageStream",
							Name: "outputstream",
						},
					},
				},
				Triggers: []older.BuildTriggerPolicy{
					{
						Type: older.ImageChangeBuildTriggerTypeDeprecated,
					},
					{
						Type: older.GitHubWebHookBuildTriggerTypeDeprecated,
					},
					{
						Type: older.GenericWebHookBuildTriggerTypeDeprecated,
					},
				},
			},
		},
		{
			ObjectMeta: kolder.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: older.BuildConfigSpec{
				CommonSpec: older.CommonSpec{
					Source: older.BuildSource{
						Type: older.BuildSourceGit,
						Git: &older.GitBuildSource{
							URI: "http://github.com/my/repository",
						},
						ContextDir: "context",
					},
					Strategy: older.BuildStrategy{
						Type: older.CustomBuildStrategyType,
						CustomStrategy: &older.CustomBuildStrategy{
							From: kolder.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
					Output: older.BuildOutput{
						To: &kolder.ObjectReference{
							Kind: "ImageStream",
							Name: "outputstream",
						},
					},
				},
				Triggers: []older.BuildTriggerPolicy{
					{
						Type: older.ImageChangeBuildTriggerTypeDeprecated,
					},
					{
						Type: older.GitHubWebHookBuildTriggerTypeDeprecated,
					},
					{
						Type: older.GenericWebHookBuildTriggerTypeDeprecated,
					},
				},
			},
		},
	}

	for c, bc := range buildConfigs {

		var internalbuild newer.BuildConfig

		Convert(bc, &internalbuild, nil)
		switch bc.Spec.Strategy.Type {
		case older.SourceBuildStrategyType:
			if internalbuild.Spec.Strategy.SourceStrategy.From.Kind != "ImageStreamTag" {
				t.Errorf("[%d] Expected From Kind %s, got %s", c, "ImageStreamTag", internalbuild.Spec.Strategy.SourceStrategy.From.Kind)
			}
		case older.DockerBuildStrategyType:
			if internalbuild.Spec.Strategy.DockerStrategy.From.Kind != "ImageStreamTag" {
				t.Errorf("[%d]Expected From Kind %s, got %s", c, "ImageStreamTag", internalbuild.Spec.Strategy.DockerStrategy.From.Kind)
			}
		case older.CustomBuildStrategyType:
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
			case newer.ImageChangeBuildTriggerType:
				foundImageChange = true

			case newer.GenericWebHookBuildTriggerType:
				foundGeneric = true

			case newer.GitHubWebHookBuildTriggerType:
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
	buildConfigs := []*newer.BuildConfig{
		{
			ObjectMeta: knewer.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: newer.BuildConfigSpec{
				CommonSpec: newer.CommonSpec{
					Source: newer.BuildSource{},
					Strategy: newer.BuildStrategy{
						DockerStrategy: &newer.DockerBuildStrategy{
							From: &knewer.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: knewer.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: newer.BuildConfigSpec{
				CommonSpec: newer.CommonSpec{
					Source: newer.BuildSource{},
					Strategy: newer.BuildStrategy{
						SourceStrategy: &newer.SourceBuildStrategy{
							From: knewer.ObjectReference{
								Kind: "ImageStream",
								Name: "fromstream",
							},
						},
					},
				},
			},
		},
		{
			ObjectMeta: knewer.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: newer.BuildConfigSpec{
				CommonSpec: newer.CommonSpec{
					Source: newer.BuildSource{},
					Strategy: newer.BuildStrategy{
						CustomStrategy: &newer.CustomBuildStrategy{
							From: knewer.ObjectReference{
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

		var internalbuild older.BuildConfig

		Convert(bc, &internalbuild, nil)
		if internalbuild.Spec.Source.Type != older.BuildSourceNone {
			t.Errorf("Unexpected source type at index %d: %s", c, internalbuild.Spec.Source.Type)
		}
	}
}

func TestInvalidImageChangeTriggerRemoval(t *testing.T) {
	buildConfig := older.BuildConfig{
		ObjectMeta: kolder.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: older.BuildConfigSpec{
			CommonSpec: older.CommonSpec{
				Strategy: older.BuildStrategy{
					Type: older.DockerBuildStrategyType,
					DockerStrategy: &older.DockerBuildStrategy{
						From: &kolder.ObjectReference{
							Kind: "DockerImage",
							Name: "fromimage",
						},
					},
				},
			},
			Triggers: []older.BuildTriggerPolicy{
				{
					Type:        older.ImageChangeBuildTriggerType,
					ImageChange: &older.ImageChangeTrigger{},
				},
				{
					Type: older.ImageChangeBuildTriggerType,
					ImageChange: &older.ImageChangeTrigger{
						From: &kolder.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "imagestream",
						},
					},
				},
			},
		},
	}

	var internalBC newer.BuildConfig

	Convert(&buildConfig, &internalBC, nil)
	if len(internalBC.Spec.Triggers) != 1 {
		t.Errorf("Expected 1 trigger, got %d", len(internalBC.Spec.Triggers))
	}
	if internalBC.Spec.Triggers[0].ImageChange.From == nil {
		t.Errorf("Expected remaining trigger to have a From value")
	}

}

func TestImageChangeTriggerNilImageChangePointer(t *testing.T) {
	buildConfig := older.BuildConfig{
		ObjectMeta: kolder.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: older.BuildConfigSpec{
			CommonSpec: older.CommonSpec{
				Strategy: older.BuildStrategy{
					Type:           older.SourceBuildStrategyType,
					SourceStrategy: &older.SourceBuildStrategy{},
				},
			},
			Triggers: []older.BuildTriggerPolicy{
				{
					Type:        older.ImageChangeBuildTriggerType,
					ImageChange: nil,
				},
			},
		},
	}

	var internalBC newer.BuildConfig

	Convert(&buildConfig, &internalBC, nil)
	for _, ic := range internalBC.Spec.Triggers {
		if ic.ImageChange == nil {
			t.Errorf("Expected trigger to have ImageChange value")
		}
	}
}
