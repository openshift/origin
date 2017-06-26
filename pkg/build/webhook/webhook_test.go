package webhook

import (
	"testing"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

func newBuildSource(ref string) *buildapi.BuildSource {
	return &buildapi.BuildSource{
		Git: &buildapi.GitBuildSource{
			Ref: ref,
		},
	}
}

func newBuildConfig() *buildapi.BuildConfig {
	return &buildapi.BuildConfig{
		Spec: buildapi.BuildConfigSpec{
			Triggers: []buildapi.BuildTriggerPolicy{
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret101",
					},
				},
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret:   "secret100",
						AllowEnv: true,
					},
				},
				{
					Type: buildapi.GenericWebHookBuildTriggerType,
					GenericWebHook: &buildapi.WebHookTrigger{
						Secret: "secret102",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret201",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret200",
					},
				},
				{
					Type: buildapi.GitHubWebHookBuildTriggerType,
					GitHubWebHook: &buildapi.WebHookTrigger{
						Secret: "secret202",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret301",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret300",
					},
				},
				{
					Type: buildapi.GitLabWebHookBuildTriggerType,
					GitLabWebHook: &buildapi.WebHookTrigger{
						Secret: "secret302",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret401",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret400",
					},
				},
				{
					Type: buildapi.BitbucketWebHookBuildTriggerType,
					BitbucketWebHook: &buildapi.WebHookTrigger{
						Secret: "secret402",
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
	_, err := FindTriggerPolicy(buildapi.ImageChangeBuildTriggerType, buildConfig)
	if err != ErrHookNotEnabled {
		t.Errorf("Expected error %s got %s", ErrHookNotEnabled, err)
	}
}

func TestFindTriggerPolicyMatchedGenericWebHook(t *testing.T) {
	buildConfig := newBuildConfig()
	triggers, err := FindTriggerPolicy(buildapi.GenericWebHookBuildTriggerType, buildConfig)

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
	triggers, err := FindTriggerPolicy(buildapi.GitHubWebHookBuildTriggerType, buildConfig)

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

func TestFindTriggerPolicyMatchedGitLabWebHook(t *testing.T) {
	buildConfig := newBuildConfig()
	triggers, err := FindTriggerPolicy(buildapi.GitLabWebHookBuildTriggerType, buildConfig)

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

func TestFindTriggerPolicyMatchedBitbucketWebHook(t *testing.T) {
	buildConfig := newBuildConfig()
	triggers, err := FindTriggerPolicy(buildapi.BitbucketWebHookBuildTriggerType, buildConfig)

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

func TestValidateMatchGitLabWebHookSecret(t *testing.T) {
	secret := "secret301"
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

func TestValidateMatchBitbucketWebHookSecret(t *testing.T) {
	secret := "secret401"
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
