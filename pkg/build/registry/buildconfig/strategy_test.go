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
			RunPolicy: buildapi.BuildRunPolicySerial,
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					GitHubWebHook: &buildapi.WebHookTrigger{Secret: "12345"},
					Type:          buildapi.GitHubWebHookBuildTriggerType,
				},
				{
					Type: "unknown",
				},
			},
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
					ContextDir: "context",
				},
				Strategy: buildapi.BuildStrategy{
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
		Status: buildapi.BuildConfigStatus{
			LastVersion: 10,
		},
	}
	newBC := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Spec: buildapi.BuildConfigSpec{
			RunPolicy: buildapi.BuildRunPolicySerial,
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					GitHubWebHook: &buildapi.WebHookTrigger{Secret: "12345"},
					Type:          buildapi.GitHubWebHookBuildTriggerType,
				},
				{
					Type: "unknown",
				},
			},
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
					ContextDir: "context",
				},
				Strategy: buildapi.BuildStrategy{
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
		Status: buildapi.BuildConfigStatus{
			LastVersion: 9,
		},
	}
	Strategy.PrepareForCreate(ctx, buildConfig)
	errs := Strategy.Validate(ctx, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	// lastversion cannot go backwards
	newBC.Status.LastVersion = 9
	Strategy.PrepareForUpdate(ctx, newBC, buildConfig)
	if newBC.Status.LastVersion != buildConfig.Status.LastVersion {
		t.Errorf("Expected version=%d, got %d", buildConfig.Status.LastVersion, newBC.Status.LastVersion)
	}

	// lastversion can go forwards
	newBC.Status.LastVersion = 11
	Strategy.PrepareForUpdate(ctx, newBC, buildConfig)
	if newBC.Status.LastVersion != 11 {
		t.Errorf("Expected version=%d, got %d", 11, newBC.Status.LastVersion)
	}

	Strategy.PrepareForCreate(ctx, buildConfig)
	errs = Strategy.Validate(ctx, buildConfig)
	if len(errs) != 0 {
		t.Errorf("Unexpected error validating %v", errs)
	}

	invalidBuildConfig := &buildapi.BuildConfig{}
	errs = Strategy.Validate(ctx, invalidBuildConfig)
	if len(errs) == 0 {
		t.Errorf("Expected error validating")
	}
}
