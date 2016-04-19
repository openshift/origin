package webhook

import (
	"testing"

	"github.com/openshift/origin/pkg/build/api"
)

func newBuildSource(ref string) *api.BuildSource {
	return &api.BuildSource{
		Git: &api.GitBuildSource{
			Ref: ref,
		},
	}
}

func newBuildConfig() *api.BuildConfig {
	return &api.BuildConfig{
		Spec: api.BuildConfigSpec{
			Triggers: []api.BuildTriggerPolicy{
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret101",
					},
				},
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret:   "secret100",
						AllowEnv: true,
					},
				},
				{
					Type: api.GenericWebHookBuildTriggerType,
					GenericWebHook: &api.WebHookTrigger{
						Secret: "secret102",
					},
				},
				{
					Type: api.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &api.WebHookTrigger{
						Secret: "secret201",
					},
				},
				{
					Type: api.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &api.WebHookTrigger{
						Secret: "secret200",
					},
				},
				{
					Type: api.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &api.WebHookTrigger{
						Secret: "secret202",
					},
				},
			},
		},
	}
}

func TestWebHookEventUnmatchedRef(t *testing.T) {
	buildSourceGit := newBuildSource("wrongref")
	refMatch := GitRefMatches("master", DefaultConfigRef, buildSourceGit)
	if refMatch {
		t.Errorf("Expected Event Ref to not match BuildConfig Git Ref")
	}
}

func TestWebHookEventMatchedRef(t *testing.T) {
	buildSourceGit := newBuildSource("master")
	refMatch := GitRefMatches("master", DefaultConfigRef, buildSourceGit)
	if !refMatch {
		t.Errorf("Expected WebHook Event Ref to match BuildConfig Git Ref")
	}
}

func TestWebHookEventNoRef(t *testing.T) {
	buildSourceGit := newBuildSource("")
	refMatch := GitRefMatches("master", DefaultConfigRef, buildSourceGit)
	if !refMatch {
		t.Errorf("Expected WebHook Event Ref to match BuildConfig Git Ref")
	}
}

func TestFindTriggerPolicyWebHookError(t *testing.T) {
	buildConfig := newBuildConfig()
	_, err := FindTriggerPolicy(api.ImageChangeBuildTriggerType, buildConfig)
	if err != ErrHookNotEnabled {
		t.Errorf("Expected error %s got %s", ErrHookNotEnabled, err)
	}
}

func TestFindTriggerPolicyMatchedGenericWebHook(t *testing.T) {
	buildConfig := newBuildConfig()
	triggers, err := FindTriggerPolicy(api.GenericWebHookBuildTriggerType, buildConfig)

	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if triggers == nil {
		t.Error("Expected a slice of matched 'triggers', got nil")
	}

	if len(triggers) != 3 {
		t.Errorf("Expected a slice of 3 matched triggers, got %d", len(triggers))
	}
}

func TestFindTriggerPolicyMatchedGithubWebHook(t *testing.T) {
	buildConfig := newBuildConfig()
	triggers, err := FindTriggerPolicy(api.GitHubWebHookBuildTriggerType, buildConfig)

	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if triggers == nil {
		t.Error("Expected a slice of matched 'triggers', got nil")
	}

	if len(triggers) != 3 {
		t.Errorf("Expected a slice of 3 matched triggers, got %d", len(triggers))
	}
}

func TestValidateWrongWebHookSecretError(t *testing.T) {
	buildConfig := newBuildConfig()
	_, err := ValidateWebHookSecret(buildConfig.Spec.Triggers, "wrongsecret")
	if err != ErrSecretMismatch {
		t.Errorf("Expected error %s, got %s", ErrSecretMismatch, err)
	}
}

func TestValidateMatchGenericWebHookSecret(t *testing.T) {
	secret := "secret101"
	buildconfig := newBuildConfig()
	trigger, err := ValidateWebHookSecret(buildconfig.Spec.Triggers, secret)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}
	if trigger.Secret != secret {
		t.Errorf("Expected returned 'secret'(%s) to match %s", trigger.Secret, secret)
	}

	if trigger.AllowEnv {
		t.Errorf("Expected AllowEnv to be false for %s", secret)
	}
}

func TestValidateMatchGitHubWebHookSecret(t *testing.T) {
	secret := "secret201"
	buildconfig := newBuildConfig()
	trigger, err := ValidateWebHookSecret(buildconfig.Spec.Triggers, secret)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if trigger.Secret != secret {
		t.Errorf("Expected returned 'secret'(%s) to match %s", trigger.Secret, secret)
	}

	if trigger.AllowEnv {
		t.Errorf("Expected AllowEnv to be false for %s", secret)
	}
}

func TestValidateEnvVarsGenericWebHook(t *testing.T) {
	secret := "secret100"
	buildconfig := newBuildConfig()
	trigger, err := ValidateWebHookSecret(buildconfig.Spec.Triggers, secret)
	if err != nil {
		t.Errorf("Expected error to be nil, got %s", err)
	}

	if trigger.Secret != secret {
		t.Errorf("Expected returned 'secret'(%s) to match %s", trigger.Secret, secret)
	}

	if !trigger.AllowEnv {
		t.Errorf("Expected AllowEnv to be true for %s", secret)
	}
}
