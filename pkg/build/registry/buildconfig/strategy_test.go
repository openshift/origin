package buildconfig

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestBuildConfigStrategy(t *testing.T) {
	ctx := kapi.NewDefaultContext()
	if !Strategy.NamespaceScoped() {
		t.Errorf("BuildConfig is namespace scoped")
	}
	if Strategy.AllowCreateOnUpdate() {
		t.Errorf("BuildConfig should not allow create on update")
	}
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: buildapi.BuildConfigSpec{
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					GitHubWebHook: &buildapi.WebHookTrigger{Secret: "12345"},
					Type:          buildapi.GitHubWebHookBuildTriggerType,
				},
				{
					Type: "unknown",
				},
			},
			BuildSpec: buildapi.BuildSpec{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
					ContextDir: "context",
				},
				Strategy: buildapi.BuildStrategy{
					Type:           buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/data",
					},
				},
			},
		},
	}
	Strategy.PrepareForCreate(buildConfig)
	errs := Strategy.Validate(ctx, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	buildConfig.ResourceVersion = "foo"
	errs = Strategy.ValidateUpdate(ctx, buildConfig, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}
	invalidBuildConfig := &buildapi.BuildConfig{}
	errs = Strategy.Validate(ctx, invalidBuildConfig)
	if len(errs) == 0 {
		t.Errorf("Expected error validating")
	}
}
