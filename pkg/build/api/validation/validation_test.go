package validation

import (
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestBuildValidationSuccess(t *testing.T) {
	build := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "buildid", Namespace: "default"},
		Parameters: buildapi.BuildParameters{
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
				DockerImageReference: "repository/data",
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
		ObjectMeta: kapi.ObjectMeta{Name: "", Namespace: ""},
		Parameters: buildapi.BuildParameters{
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
				DockerImageReference: "repository/data",
			},
		},
		Status: buildapi.BuildStatusNew,
	}
	if result := ValidateBuild(build); len(result) != 2 {
		t.Errorf("Unexpected validation result: %v", result)
	}
}

func newDefaultParameters() buildapi.BuildParameters {
	return buildapi.BuildParameters{
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
			DockerImageReference: "repository/data",
		},
	}
}

func TestValidateBuildUpdate(t *testing.T) {

	old := &buildapi.Build{
		ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "my-build", ResourceVersion: "1"},
		Parameters: newDefaultParameters(),
	}

	errs := ValidateBuildUpdate(
		&buildapi.Build{
			ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "my-build", ResourceVersion: "1"},
			Parameters: newDefaultParameters(),
		},
		old,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A *buildapi.Build
		T fielderrors.ValidationErrorType
		F string
	}{
		"changed spec": {
			A: &buildapi.Build{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "my-build", ResourceVersion: "1"},
				Parameters: newDefaultParameters(),
			},
			T: fielderrors.ValidationErrorTypeInvalid,
			F: "spec",
		},
	}
	errorCases["changed spec"].A.Parameters.Source.Git.URI = "different"

	for k, v := range errorCases {
		errs := ValidateBuildUpdate(v.A, old)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].(*fielderrors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(*fielderrors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestBuildConfigValidationSuccess(t *testing.T) {
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "config-id", Namespace: "namespace"},
		Parameters: buildapi.BuildParameters{
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
				DockerImageReference: "repository/data",
			},
		},
	}
	if result := ValidateBuildConfig(buildConfig); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestBuildConfigValidationFailureRequiredName(t *testing.T) {
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "", Namespace: "foo"},
		Parameters: buildapi.BuildParameters{
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
				DockerImageReference: "repository/data",
			},
		},
	}
	errors := ValidateBuildConfig(buildConfig)
	if len(errors) != 1 {
		t.Fatalf("Unexpected validation errors %v", errors)
	}
	err := errors[0].(*fielderrors.ValidationError)
	if err.Type != fielderrors.ValidationErrorTypeRequired {
		t.Errorf("Unexpected error type, expected %s, got %s", fielderrors.ValidationErrorTypeRequired, err.Type)
	}
	if err.Field != "metadata.name" {
		t.Errorf("Unexpected field name expected metadata.name, got %s", err.Field)
	}
}

func TestBuildConfigValidationFailureTooManyICT(t *testing.T) {
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "bar", Namespace: "foo"},
		Parameters: buildapi.BuildParameters{
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
				DockerImageReference: "repository/data",
			},
		},
		Triggers: []buildapi.BuildTriggerPolicy{
			{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
			{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
		},
	}
	errors := ValidateBuildConfig(buildConfig)
	if len(errors) != 1 {
		t.Fatalf("Unexpected validation errors %v", errors)
	}
	err := errors[0].(*fielderrors.ValidationError)
	if err.Type != fielderrors.ValidationErrorTypeInvalid {
		t.Errorf("Unexpected error type, expected %s, got %s", fielderrors.ValidationErrorTypeInvalid, err.Type)
	}
	if err.Field != "triggers" {
		t.Errorf("Unexpected field name expected triggers, got %s", err.Field)
	}
	if !strings.Contains(err.Detail, "only one ImageChange trigger is allowed") {
		t.Errorf("Unexpected error details: %s", err.Detail)
	}
}

func TestBuildConfigValidationOutputFailure(t *testing.T) {
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: ""},
		Parameters: buildapi.BuildParameters{
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
				DockerImageReference: "repository/data",
				To: &kapi.ObjectReference{
					Name: "other",
				},
			},
		},
	}
	if result := ValidateBuildConfig(buildConfig); len(result) != 3 {
		t.Errorf("Unexpected validation result %v", result)
	}
}

func TestValidateBuildRequest(t *testing.T) {
	testCases := map[string]*buildapi.BuildRequest{
		string(fielderrors.ValidationErrorTypeRequired) + "metadata.namespace": {ObjectMeta: kapi.ObjectMeta{Name: "requestName"}},
		string(fielderrors.ValidationErrorTypeRequired) + "metadata.name":      {ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault}},
	}

	for desc, tc := range testCases {
		errors := ValidateBuildRequest(tc)
		if len(desc) == 0 && len(errors) > 0 {
			t.Errorf("%s: Unexpected validation result: %v", desc, errors)
		}
		if len(desc) > 0 && len(errors) != 1 {
			t.Errorf("%s: Unexpected validation result: %v", desc, errors)
		}
		if len(desc) > 0 {
			err := errors[0].(*fielderrors.ValidationError)
			errDesc := string(err.Type) + err.Field
			if desc != errDesc {
				t.Errorf("Unexpected validation result for %s: expected %s, got %s", err.Field, desc, errDesc)
			}
		}
	}
}

func TestValidateSource(t *testing.T) {
	errorCases := map[string]*buildapi.BuildSource{
		string(fielderrors.ValidationErrorTypeRequired) + "git.uri": {
			Type: buildapi.BuildSourceGit,
			Git: &buildapi.GitBuildSource{
				URI: "",
			},
		},
		string(fielderrors.ValidationErrorTypeInvalid) + "git.uri": {
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
		err := errors[0].(*fielderrors.ValidationError)
		errDesc := string(err.Type) + err.Field
		if desc != errDesc {
			t.Errorf("Unexpected validation result for %s: expected %s, got %s", err.Field, desc, errDesc)
		}
	}
}

func TestValidateBuildParameters(t *testing.T) {
	errorCases := []struct {
		err string
		*buildapi.BuildParameters
	}{
		{
			string(fielderrors.ValidationErrorTypeInvalid) + "output.dockerImageReference",
			&buildapi.BuildParameters{
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
					DockerImageReference: "some/long/value/with/no/meaning",
				},
			},
		},
		{
			string(fielderrors.ValidationErrorTypeInvalid) + "output.to.kind",
			&buildapi.BuildParameters{
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
						Kind: "Foo",
						Name: "test",
					},
				},
			},
		},
		{
			string(fielderrors.ValidationErrorTypeRequired) + "output.to.name",
			&buildapi.BuildParameters{
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
					To: &kapi.ObjectReference{},
				},
			},
		},
		{
			string(fielderrors.ValidationErrorTypeInvalid) + "output.to.name",
			&buildapi.BuildParameters{
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
						Name:      "not_a_valid_subdomain",
						Namespace: "subdomain",
					},
				},
			},
		},
		{
			string(fielderrors.ValidationErrorTypeInvalid) + "output.to.namespace",
			&buildapi.BuildParameters{
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
						Name:      "test",
						Namespace: "not_a_valid_subdomain",
					},
				},
			},
		},
		{
			string(fielderrors.ValidationErrorTypeInvalid) + "strategy.type",
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{Type: "invalid-type"},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		{
			string(fielderrors.ValidationErrorTypeRequired) + "strategy.type",
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		// invalid because from is not specified in the
		// sti strategy definition
		{
			string(fielderrors.ValidationErrorTypeRequired) + "strategy.stiStrategy.from",
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{
					Type:           buildapi.SourceBuildStrategyType,
					SourceStrategy: &buildapi.SourceBuildStrategy{},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		// invalid because from name is a bad format
		{
			string(fielderrors.ValidationErrorTypeInvalid) + "strategy.stiStrategy.from.name",
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.SourceBuildStrategyType,
					SourceStrategy: &buildapi.SourceBuildStrategy{
						From: kapi.ObjectReference{Kind: "ImageStreamTag", Name: "bad format"},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		// invalid because from is not specified in the
		// custom strategy definition
		{
			string(fielderrors.ValidationErrorTypeRequired) + "strategy.customStrategy.from",
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{
					Type:           buildapi.CustomBuildStrategyType,
					CustomStrategy: &buildapi.CustomBuildStrategy{},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		// invalid because from is not specified in the
		// custom strategy definition
		{
			string(fielderrors.ValidationErrorTypeInvalid) + "strategy.customStrategy.from.name",
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.CustomBuildStrategyType,
					CustomStrategy: &buildapi.CustomBuildStrategy{
						From: kapi.ObjectReference{Kind: "ImageStreamTag", Name: "bad format"},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
	}

	for _, config := range errorCases {
		errors := validateBuildParameters(config.BuildParameters)
		if len(errors) != 1 {
			t.Errorf("%s: Unexpected validation result: %v", config.err, errors)
		}
		err := errors[0].(*fielderrors.ValidationError)
		errDesc := string(err.Type) + err.Field
		if config.err != errDesc {
			t.Errorf("Unexpected validation result for %s: expected %s, got %s", err.Field, config.err, errDesc)
		}
	}
}

func TestValidateBuildParametersSuccess(t *testing.T) {
	testCases := []struct {
		*buildapi.BuildParameters
	}{
		{
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.SourceBuildStrategyType,
					SourceStrategy: &buildapi.SourceBuildStrategy{
						From: kapi.ObjectReference{
							Name: "reponame",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		{
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.CustomBuildStrategyType,
					CustomStrategy: &buildapi.CustomBuildStrategy{
						From: kapi.ObjectReference{
							Name: "reponame",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		{
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{
					Type:           buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		{
			&buildapi.BuildParameters{
				Source: buildapi.BuildSource{
					Type: buildapi.BuildSourceGit,
					Git: &buildapi.GitBuildSource{
						URI: "http://github.com/my/repository",
					},
				},
				Strategy: buildapi.BuildStrategy{
					Type: buildapi.DockerBuildStrategyType,
					DockerStrategy: &buildapi.DockerBuildStrategy{
						From: &kapi.ObjectReference{
							Name: "reponame",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
	}

	for _, config := range testCases {
		errors := validateBuildParameters(config.BuildParameters)
		if len(errors) != 0 {
			t.Errorf("Unexpected validation error: %v", errors)
		}
	}

}

func TestValidateTrigger(t *testing.T) {
	tests := map[string]struct {
		trigger  buildapi.BuildTriggerPolicy
		expected []*fielderrors.ValidationError
	}{
		"trigger without type": {
			trigger:  buildapi.BuildTriggerPolicy{},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("type")},
		},
		"trigger with unknown type": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: "UnknownTriggerType",
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldInvalid("type", "", "")},
		},
		"GitHub type with no github webhook": {
			trigger:  buildapi.BuildTriggerPolicy{Type: buildapi.GitHubWebHookBuildTriggerType},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("github")},
		},
		"GitHub trigger with no secret": {
			trigger: buildapi.BuildTriggerPolicy{
				Type:          buildapi.GitHubWebHookBuildTriggerType,
				GitHubWebHook: &buildapi.WebHookTrigger{},
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("github.secret")},
		},
		"GitHub trigger with generic webhook": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GitHubWebHookBuildTriggerType,
				GenericWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("github")},
		},
		"Generic trigger with no generic webhook": {
			trigger:  buildapi.BuildTriggerPolicy{Type: buildapi.GenericWebHookBuildTriggerType},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("generic")},
		},
		"Generic trigger with no secret": {
			trigger: buildapi.BuildTriggerPolicy{
				Type:           buildapi.GenericWebHookBuildTriggerType,
				GenericWebHook: &buildapi.WebHookTrigger{},
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("generic.secret")},
		},
		"Generic trigger with github webhook": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GenericWebHookBuildTriggerType,
				GitHubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("generic")},
		},
		"ImageChange trigger without params": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.ImageChangeBuildTriggerType,
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("imageChange")},
		},
		"valid GitHub trigger": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GitHubWebHookBuildTriggerType,
				GitHubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
		"valid Generic trigger": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GenericWebHookBuildTriggerType,
				GenericWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
		"valid ImageChange trigger": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					LastTriggeredImageID: "asdf1234",
				},
			},
		},
		"valid ImageChange trigger with empty fields": {
			trigger: buildapi.BuildTriggerPolicy{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
		},
	}
	for desc, test := range tests {
		errors := validateTrigger(&test.trigger)
		if len(test.expected) == 0 {
			if len(errors) != 0 {
				t.Errorf("%s: Got unexpected validation errors: %#v", desc, errors)
			}
			continue
		}
		if len(errors) != 1 {
			t.Errorf("%s: Expected one validation error, got %d", desc, len(errors))
			for i, err := range errors {
				validationError := err.(*fielderrors.ValidationError)
				t.Errorf("  %d. %v", i+1, validationError)
			}
			continue
		}
		err := errors[0]
		validationError := err.(*fielderrors.ValidationError)
		if validationError.Type != test.expected[0].Type {
			t.Errorf("%s: Unexpected error type: %s", desc, validationError.Type)
		}
		if validationError.Field != test.expected[0].Field {
			t.Errorf("%s: Unexpected error field: %s", desc, validationError.Field)
		}
	}
}
