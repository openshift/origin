package validation

import (
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

func TestBuildConfigValidationFailure(t *testing.T) {
	buildConfig := &buildapi.BuildConfig{
		ObjectMeta: kapi.ObjectMeta{Name: "", Namespace: "foo"},
		Triggers: []buildapi.BuildTriggerPolicy{
			buildapi.BuildTriggerPolicy{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
			buildapi.BuildTriggerPolicy{
				Type:        buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{},
			},
		},
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
	if result := ValidateBuildConfig(buildConfig); len(result) != 2 {
		t.Errorf("Unexpected validation result %v", result)
	}
}

func TestValidateBuildRequest(t *testing.T) {
	testCases := map[string]*buildapi.BuildRequest{
		"": {ObjectMeta: kapi.ObjectMeta{Name: "requestName"}},
		string(fielderrors.ValidationErrorTypeRequired) + "name": {},
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
		{ // 0
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
						Kind: "DockerImage",
						Name: "some/long/value/with/no/meaning",
					},
				},
			},
		},
		{ // 1
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
		{ // 2
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
					To: &kapi.ObjectReference{},
				},
			},
		},
		{ // 3
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
						Kind:      "DockerImage",
						Name:      "test",
						Namespace: "not_a_valid_subdomain",
					},
				},
			},
		},
		{ // 4
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
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/data",
					},
				},
			},
		},
		{ // 5
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
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/data",
					},
				},
			},
		},

		{ // 6 - invalid because from is not specified in the sti strategy definition
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
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/data",
					},
				},
			},
		},
		{ // 7 - invalid because from is not specified in the custom strategy definition
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
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "repository/data",
					},
				},
			},
		},
	}

	for i, config := range errorCases {
		errors := validateBuildParameters(config.BuildParameters)
		if len(errors) != 1 {
			t.Errorf("(%d) %s: Unexpected validation result: %v", i, config.err, errors)
		}
		err := errors[0].(*fielderrors.ValidationError)
		errDesc := string(err.Type) + err.Field
		if config.err != errDesc {
			t.Errorf("(%d) unexpected validation result for %s: expected %s, got %s", i, err.Field, config.err, errDesc)
		}
	}
}

func TestValidateBuildParametersSuccess(t *testing.T) {
	testCases := []struct {
		*buildapi.BuildParameters
	}{
		{ // 0
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
						From: &kapi.ObjectReference{
							Name: "reponame",
							Kind: "ImageStreamTag",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		{ // 1
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
						From: &kapi.ObjectReference{
							Name: "reponame",
							Kind: "ImageStreamTag",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
		{ // 2
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
		{ // 3
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
							Kind: "ImageStreamTag",
						},
					},
				},
				Output: buildapi.BuildOutput{
					DockerImageReference: "repository/data",
				},
			},
		},
	}

	for i, config := range testCases {
		errors := validateBuildParameters(config.BuildParameters)
		if len(errors) != 0 {
			t.Errorf("(%d) unexpected validation error: %v", i, errors)
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
		"github type with no github webhook": {
			trigger:  buildapi.BuildTriggerPolicy{Type: buildapi.GithubWebHookBuildTriggerType},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("github")},
		},
		"github trigger with no secret": {
			trigger: buildapi.BuildTriggerPolicy{
				Type:          buildapi.GithubWebHookBuildTriggerType,
				GithubWebHook: &buildapi.WebHookTrigger{},
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("github.secret")},
		},
		"github trigger with generic webhook": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GithubWebHookBuildTriggerType,
				GenericWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldInvalid("generic", "", "long description")},
		},
		"generic trigger with no generic webhook": {
			trigger:  buildapi.BuildTriggerPolicy{Type: buildapi.GenericWebHookBuildTriggerType},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("generic")},
		},
		"generic trigger with no secret": {
			trigger: buildapi.BuildTriggerPolicy{
				Type:           buildapi.GenericWebHookBuildTriggerType,
				GenericWebHook: &buildapi.WebHookTrigger{},
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("generic.secret")},
		},
		"generic trigger with github webhook": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GenericWebHookBuildTriggerType,
				GithubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldInvalid("github", "", "long github description")},
		},
		"imageChange trigger without params": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.ImageChangeBuildTriggerType,
			},
			expected: []*fielderrors.ValidationError{fielderrors.NewFieldRequired("imageChange")},
		},
		"valid github trigger": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GithubWebHookBuildTriggerType,
				GithubWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
		"valid generic trigger": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.GenericWebHookBuildTriggerType,
				GenericWebHook: &buildapi.WebHookTrigger{
					Secret: "secret101",
				},
			},
		},
		"valid imageChange trigger": {
			trigger: buildapi.BuildTriggerPolicy{
				Type: buildapi.ImageChangeBuildTriggerType,
				ImageChange: &buildapi.ImageChangeTrigger{
					LastTriggeredImageID: "asdf1234",
				},
			},
		},
		"valid imageChange trigger with empty fields": {
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

func TestValidateBuildStrategyFromKindOK(t *testing.T) {
	sourceStrategy := &buildapi.BuildStrategy{
		Type:           buildapi.SourceBuildStrategyType,
		SourceStrategy: &buildapi.SourceBuildStrategy{},
	}
	dockerStrategy := &buildapi.BuildStrategy{
		Type:           buildapi.DockerBuildStrategyType,
		DockerStrategy: &buildapi.DockerBuildStrategy{},
	}
	customStrategy := &buildapi.BuildStrategy{
		Type:           buildapi.CustomBuildStrategyType,
		CustomStrategy: &buildapi.CustomBuildStrategy{},
	}

	validateError := func(strategy, kind string, errors fielderrors.ValidationErrorList) {
		if len(errors) != 0 {
			t.Errorf("%s, kind %s: unexpected errors %+v", strategy, kind, errors)
		}
	}
	testCases := []kapi.ObjectReference{
		{Kind: "ImageStreamImage", Name: "repo/image@id"},
		{Kind: "ImageStreamTag", Name: "repo/image:tag"},
		{Kind: "DockerImage", Name: "repo/image"},
	}
	for _, tc := range testCases {
		sourceStrategy.SourceStrategy.From = &tc
		validateError("SourceStrategy", tc.Kind, validateStrategy(sourceStrategy))

		dockerStrategy.DockerStrategy.From = &tc
		validateError("DockerStrategy", tc.Kind, validateStrategy(dockerStrategy))

		customStrategy.CustomStrategy.From = &tc
		validateError("CustomStrategy", tc.Kind, validateStrategy(customStrategy))
	}
}

func TestValidateBuildStrategyFromKindErrors(t *testing.T) {
	sourceStrategy := &buildapi.BuildStrategy{
		Type:           buildapi.SourceBuildStrategyType,
		SourceStrategy: &buildapi.SourceBuildStrategy{},
	}
	dockerStrategy := &buildapi.BuildStrategy{
		Type:           buildapi.DockerBuildStrategyType,
		DockerStrategy: &buildapi.DockerBuildStrategy{},
	}
	customStrategy := &buildapi.BuildStrategy{
		Type:           buildapi.CustomBuildStrategyType,
		CustomStrategy: &buildapi.CustomBuildStrategy{},
	}

	validateError := func(strategy, kind string, errors fielderrors.ValidationErrorList) {
		expected := string(fielderrors.ValidationErrorTypeInvalid) + strategy + ".from.kind"
		if len(errors) == 0 {
			t.Errorf("%s, kind %s: expected errors, got none", strategy, kind)
			return
		}
		err := errors[0].(*fielderrors.ValidationError)
		actual := string(err.Type) + err.Field
		if actual != expected {
			t.Errorf("%s, kind %s: unexpected validation result expected %s, got %s", strategy, kind, expected, actual)
		}
	}
	testCases := []kapi.ObjectReference{
		{Kind: "ImageStream", Name: "repo/image"},
		{Kind: "other", Name: "some_other_string"},
		{Kind: "", Name: "<empty>"},
	}
	for _, tc := range testCases {
		sourceStrategy.SourceStrategy.From = &tc
		validateError("stiStrategy", tc.Kind, validateStrategy(sourceStrategy))

		dockerStrategy.DockerStrategy.From = &tc
		validateError("dockerStrategy", tc.Kind, validateStrategy(dockerStrategy))

		customStrategy.CustomStrategy.From = &tc
		validateError("customStrategy", tc.Kind, validateStrategy(customStrategy))
	}
}

func TestValidateBuildOutputToKindOK(t *testing.T) {
	testCases := []kapi.ObjectReference{
		{Kind: "ImageStreamTag", Name: "repo/image:tag"},
		{Kind: "DockerImage", Name: "repo/image"},
	}
	for _, tc := range testCases {
		if errors := validateBuildOutput(&buildapi.BuildOutput{To: &tc}); len(errors) != 0 {
			t.Errorf("Kind %s: unexpected errors %+v", tc.Kind, errors)
		}
	}
}

func TestValidateBuildOutputToKindErrors(t *testing.T) {
	testCases := []kapi.ObjectReference{
		{Kind: "ImageStream", Name: "repo/image"},
		{Kind: "ImageStreamImage", Name: "repo/image"},
		{Kind: "other", Name: "some_other_string"},
		{Kind: "", Name: "<empty>"},
	}
	for _, tc := range testCases {
		expected := string(fielderrors.ValidationErrorTypeInvalid) + "to.kind"
		errors := validateBuildOutput(&buildapi.BuildOutput{To: &tc})
		if len(errors) == 0 {
			t.Errorf("Kind %s: expected errors, got none", tc.Kind)
			continue
		}
		err := errors[0].(*fielderrors.ValidationError)
		actual := string(err.Type) + err.Field
		if actual != expected {
			t.Errorf("Kind %s: unexpected validation result expected %s, got %s", tc.Kind, expected, actual)
		}
	}
}
