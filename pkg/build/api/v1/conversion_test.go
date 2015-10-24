package v1_test

import (
	"testing"

	knewer "k8s.io/kubernetes/pkg/api"
	kolder "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/conversion/queryparams"

	newer "github.com/openshift/origin/pkg/build/api"
	older "github.com/openshift/origin/pkg/build/api/v1"
)

var Convert = knewer.Scheme.Convert

func TestBinaryBuildRequestOptions(t *testing.T) {
	r := &newer.BinaryBuildRequestOptions{
		AsFile: "Dockerfile",
		Commit: "abcdef",
	}
	versioned, err := knewer.Scheme.ConvertToVersion(r, "v1")
	if err != nil {
		t.Fatal(err)
	}
	params, err := queryparams.Convert(versioned)
	if err != nil {
		t.Fatal(err)
	}
	decoded := &older.BinaryBuildRequestOptions{}
	if err := knewer.Scheme.Convert(&params, decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Commit != "abcdef" || decoded.AsFile != "Dockerfile" {
		t.Errorf("unexpected decoded object: %#v", decoded)
	}
}

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

		Convert(bc, &internalbuild)
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
