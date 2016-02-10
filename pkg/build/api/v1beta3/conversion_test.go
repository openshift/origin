package v1beta3_test

import (
	"testing"

	knewer "k8s.io/kubernetes/pkg/api"
	kolder "k8s.io/kubernetes/pkg/api/v1beta3"

	newer "github.com/openshift/origin/pkg/build/api"
	_ "github.com/openshift/origin/pkg/build/api/install"
	older "github.com/openshift/origin/pkg/build/api/v1beta3"
)

var Convert = knewer.Scheme.Convert

func TestBuildConfigConversion(t *testing.T) {
	buildConfigs := []*older.BuildConfig{
		{
			ObjectMeta: kolder.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: older.BuildConfigSpec{
				BuildSpec: older.BuildSpec{
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
						Type: older.ImageChangeBuildTriggerType,
					},
					{
						Type: older.GitHubWebHookBuildTriggerType,
					},
					{
						Type: older.GenericWebHookBuildTriggerType,
					},
				},
			},
		},
		{
			ObjectMeta: kolder.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: older.BuildConfigSpec{
				BuildSpec: older.BuildSpec{
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
						Type: older.ImageChangeBuildTriggerType,
					},
					{
						Type: older.GitHubWebHookBuildTriggerType,
					},
					{
						Type: older.GenericWebHookBuildTriggerType,
					},
				},
			},
		},
		{
			ObjectMeta: kolder.ObjectMeta{Name: "config-id", Namespace: "namespace"},
			Spec: older.BuildConfigSpec{
				BuildSpec: older.BuildSpec{
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
						Type: older.ImageChangeBuildTriggerType,
					},
					{
						Type: older.GitHubWebHookBuildTriggerType,
					},
					{
						Type: older.GenericWebHookBuildTriggerType,
					},
				},
			},
		},
	}

	for _, bc := range buildConfigs {

		var internalbuild newer.BuildConfig

		Convert(bc, &internalbuild)
		switch bc.Spec.Strategy.Type {
		case older.SourceBuildStrategyType:
			if internalbuild.Spec.Strategy.SourceStrategy.From.Kind != "ImageStreamTag" {
				t.Errorf("Expected From Kind %s, got %s", "ImageStreamTag", internalbuild.Spec.Strategy.SourceStrategy.From.Kind)
			}
		case older.DockerBuildStrategyType:
			if internalbuild.Spec.Strategy.DockerStrategy.From.Kind != "ImageStreamTag" {
				t.Errorf("Expected From Kind %s, got %s", "ImageStreamTag", internalbuild.Spec.Strategy.DockerStrategy.From.Kind)
			}
		case older.CustomBuildStrategyType:
			if internalbuild.Spec.Strategy.CustomStrategy.From.Kind != "ImageStreamTag" {
				t.Errorf("Expected From Kind %s, got %s", "ImageStreamTag", internalbuild.Spec.Strategy.CustomStrategy.From.Kind)
			}
		}
		if internalbuild.Spec.Output.To.Kind != "ImageStreamTag" {
			t.Errorf("Expected Output Kind %s, got %s", "ImageStreamTag", internalbuild.Spec.Output.To.Kind)
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

func TestBuildTriggerPolicyOldToNewConversion(t *testing.T) {
	testCases := map[string]struct {
		Olds                     []older.BuildTriggerType
		ExpectedBuildTriggerType newer.BuildTriggerType
	}{
		"ImageChange": {
			Olds: []older.BuildTriggerType{
				older.ImageChangeBuildTriggerType,
				older.BuildTriggerType(newer.ImageChangeBuildTriggerType),
			},
			ExpectedBuildTriggerType: newer.ImageChangeBuildTriggerType,
		},
		"Generic": {
			Olds: []older.BuildTriggerType{
				older.GenericWebHookBuildTriggerType,
				older.BuildTriggerType(newer.GenericWebHookBuildTriggerType),
			},
			ExpectedBuildTriggerType: newer.GenericWebHookBuildTriggerType,
		},
		"GitHub": {
			Olds: []older.BuildTriggerType{
				older.GitHubWebHookBuildTriggerType,
				older.BuildTriggerType(newer.GitHubWebHookBuildTriggerType),
			},
			ExpectedBuildTriggerType: newer.GitHubWebHookBuildTriggerType,
		},
	}
	for s, testCase := range testCases {
		expected := testCase.ExpectedBuildTriggerType
		for _, old := range testCase.Olds {
			var actual newer.BuildTriggerPolicy
			oldVersion := older.BuildTriggerPolicy{
				Type: old,
			}
			err := Convert(&oldVersion, &actual)
			if err != nil {
				t.Fatalf("%s (%s -> %s): unexpected error: %v", s, old, expected, err)
			}
			if actual.Type != testCase.ExpectedBuildTriggerType {
				t.Errorf("%s (%s -> %s): expected %v, actual %v", s, old, expected, expected, actual.Type)
			}
		}
	}
}

func TestBuildTriggerPolicyNewToOldConversion(t *testing.T) {
	testCases := map[string]struct {
		New                      newer.BuildTriggerType
		ExpectedBuildTriggerType older.BuildTriggerType
	}{
		"ImageChange": {
			New: newer.ImageChangeBuildTriggerType,
			ExpectedBuildTriggerType: older.ImageChangeBuildTriggerType,
		},
		"Generic": {
			New: newer.GenericWebHookBuildTriggerType,
			ExpectedBuildTriggerType: older.GenericWebHookBuildTriggerType,
		},
		"GitHub": {
			New: newer.GitHubWebHookBuildTriggerType,
			ExpectedBuildTriggerType: older.GitHubWebHookBuildTriggerType,
		},
	}
	for s, testCase := range testCases {
		var actual older.BuildTriggerPolicy
		newVersion := newer.BuildTriggerPolicy{
			Type: testCase.New,
		}
		err := Convert(&newVersion, &actual)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", s, err)
		}
		if actual.Type != testCase.ExpectedBuildTriggerType {
			t.Errorf("%s: expected %v, actual %v", s, testCase.ExpectedBuildTriggerType, actual.Type)
		}
	}
}
