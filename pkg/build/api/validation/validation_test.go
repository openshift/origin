package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestBuildValdationSuccess(t *testing.T) {
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildId"},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://github.com/my/repository",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "context",
				},
			},
			Output: buildapi.BuildOutput{
				ImageTag: "repository/data",
			},
		},
		Status: buildapi.BuildStatusNew,
	}
	if result := ValidateBuild(build); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestBuildValidationFailure(t *testing.T) {
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: ""},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://github.com/my/repository",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "context",
				},
			},
			Output: buildapi.BuildOutput{
				ImageTag: "repository/data",
			},
		},
		Status: buildapi.BuildStatusNew,
	}
	if result := ValidateBuild(build); len(result) != 1 {
		t.Errorf("Unexpected validation result: %v", result)
	}
}

func TestBuildConfigValidationSuccess(t *testing.T) {
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "configId"},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://github.com/my/repository",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "context",
				},
			},
			Output: buildapi.BuildOutput{
				ImageTag: "repository/data",
			},
		},
	}
	if result := ValidateBuildConfig(buildConfig); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestBuildConfigValidationFailure(t *testing.T) {
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: ""},
		Parameters: buildapi.BuildParameters{
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://github.com/my/repository",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "context",
				},
			},
			Output: buildapi.BuildOutput{
				ImageTag: "repository/data",
			},
		},
	}
	if result := ValidateBuildConfig(buildConfig); len(result) != 1 {
		t.Errorf("Unexpected validation result %v", result)
	}
}

func TestValidateSource(t *testing.T) {
	errorCases := map[string]*buildapi.BuildSource{
		string(errs.ValidationErrorTypeRequired) + "git.uri": {
			Type: buildapi.BuildSourceGit,
			Git: &buildapi.GitBuildSource{
				URI: "",
			},
		},
		string(errs.ValidationErrorTypeInvalid) + "git.uri": {
			Type: buildapi.BuildSourceGit,
			Git: &buildapi.GitBuildSource{
				URI: "::",
			},
		},
	}
	for desc, config := range errorCases {
		errors := validateSource(config)
		if len(errors) != 1 {
			t.Errorf("%s: Unexpected validation result: %v", desc, errors)
		}
		err := errors[0].(errs.ValidationError)
		errDesc := string(err.Type) + err.Field
		if desc != errDesc {
			t.Errorf("Unexpected validation result for %s: expected %s, got %s", err.Field, desc, errDesc)
		}
	}
}

func TestValidateBuildParameters(t *testing.T) {
	errorCases := map[string]*buildapi.BuildParameters{
		string(errs.ValidationErrorTypeRequired) + "output.imageTag": {
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://github.com/my/repository",
				},
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.DockerBuildStrategyType,
				DockerStrategy: &buildapi.DockerBuildStrategy{
					ContextDir: "context",
				},
			},
			Output: buildapi.BuildOutput{
				ImageTag: "",
			},
		},
		string(errs.ValidationErrorTypeRequired) + "strategy.stiStrategy.builderImage": {
			Source: buildapi.BuildSource{
				Type: buildapi.BuildSourceGit,
				Git: &buildapi.GitBuildSource{
					URI: "http://github.com/my/repository",
				},
			},
			Output: buildapi.BuildOutput{
				ImageTag: "repository/data",
			},
			Strategy: buildapi.BuildStrategy{
				Type: buildapi.STIBuildStrategyType,
				STIStrategy: &buildapi.STIBuildStrategy{
					BuilderImage: "",
				},
			},
		},
	}

	for desc, config := range errorCases {
		errors := validateBuildParameters(config)
		if len(errors) != 1 {
			t.Errorf("%s: Unexpected validation result: %v", desc, errors)
		}
		err := errors[0].(errs.ValidationError)
		errDesc := string(err.Type) + err.Field
		if desc != errDesc {
			t.Errorf("Unexpected validation result for %s: expected %s, got %s", err.Field, desc, errDesc)
		}
	}
}

func TestValidateTrigger(t *testing.T) {
	tests := map[string]struct {
		trigger  buildapi.BuildTriggerPolicy
		expected []errs.ValidationError
	}{
		"trigger without type": {
			trigger:  buildapi.BuildTriggerPolicy{},
			expected: []errs.ValidationError{errs.NewFieldRequired("type", "")},
		},
		"github type with no github webhook": {
			trigger:  buildapi.BuildTriggerPolicy{Type: buildapi.GithubWebHookType},
			expected: []errs.ValidationError{errs.NewFieldRequired("github", "")},
		},
		"github trigger with no secret": {
			trigger: buildapi.BuildTriggerPolicy{
				Type:          buildapi.GithubWebHookType,
				GithubWebHook: &buildapi.WebHookTrigger{},
			},
			expected: []errs.ValidationError{errs.NewFieldRequired("github.secret", "")},
		},
		"github trigger with generic webhook": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GithubWebHookType,
				GenericWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
			expected: []errs.ValidationError{errs.NewFieldInvalid("generic", "")},
		},
		"generic trigger with no generic webhook": {
			trigger:  buildapi.BuildTriggerPolicy{Type: buildapi.GenericWebHookType},
			expected: []errs.ValidationError{errs.NewFieldRequired("generic", "")},
		},
		"generic trigger with no secret": {
			trigger: buildapi.BuildTriggerPolicy{
				Type:           buildapi.GenericWebHookType,
				GenericWebHook: &buildapi.WebHookTrigger{},
			},
			expected: []errs.ValidationError{errs.NewFieldRequired("generic.secret", "")},
		},
		"generic trigger with github webhook": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GenericWebHookType,
				GithubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
			expected: []errs.ValidationError{errs.NewFieldInvalid("github", "")},
		},
		"valid github trigger": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GithubWebHookType,
				GithubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
		"valid generic trigger": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GenericWebHookType,
				GenericWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
	}
	for desc, test := range tests {
		errors := validateTrigger(&test.trigger)
		if len(test.expected) == 0 {
			if len(errors) != 0 {
				t.Errorf("%s: Got unexpected validation errors: %#v", errors)
			}
			continue
		}
		err := errors[0]
		validationError := err.(errs.ValidationError)
		if validationError.Type != test.expected[0].Type {
			t.Errorf("%s: Unexpected error type: %s", desc, validationError.Type)
		}
		if validationError.Field != test.expected[0].Field {
			t.Errorf("%s: Unexpected error field: %s", desc, validationError.Field)
		}
	}
}
