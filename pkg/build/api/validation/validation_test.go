package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/openshift/origin/pkg/build/api"
)

func TestBuildValdationSuccess(t *testing.T) {
	build := &api.Build{
		TypeMeta: kapi.TypeMeta{ID: "buildId"},
		Source: api.BuildSource{
			Type: api.BuildSourceGit,
			Git: &api.GitBuildSource{
				URI: "http://github.com/my/repository",
			},
		},
		Input: api.BuildInput{
			ImageTag: "repository/data",
		},
		Status: api.BuildNew,
	}
	if result := ValidateBuild(build); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestBuildValidationFailure(t *testing.T) {
	build := &api.Build{
		TypeMeta: kapi.TypeMeta{ID: ""},
		Source: api.BuildSource{
			Type: api.BuildSourceGit,
			Git: &api.GitBuildSource{
				URI: "http://github.com/my/repository",
			},
		},
		Input: api.BuildInput{
			ImageTag: "repository/data",
		},
		Status: api.BuildNew,
	}
	if result := ValidateBuild(build); len(result) != 1 {
		t.Errorf("Unexpected validation result: %v", result)
	}
}

func TestBuildConfigValidationSuccess(t *testing.T) {
	buildConfig := &api.BuildConfig{
		TypeMeta: kapi.TypeMeta{ID: "configId"},
		Source: api.BuildSource{
			Type: api.BuildSourceGit,
			Git: &api.GitBuildSource{
				URI: "http://github.com/my/repository",
			},
		},
		DesiredInput: api.BuildInput{
			ImageTag: "repository/data",
		},
	}
	if result := ValidateBuildConfig(buildConfig); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestBuildConfigValidationFailure(t *testing.T) {
	buildConfig := &api.BuildConfig{
		TypeMeta: kapi.TypeMeta{ID: ""},
		Source: api.BuildSource{
			Type: api.BuildSourceGit,
			Git: &api.GitBuildSource{
				URI: "http://github.com/my/repository",
			},
		},
		DesiredInput: api.BuildInput{
			ImageTag: "repository/data",
		},
	}
	if result := ValidateBuildConfig(buildConfig); len(result) != 1 {
		t.Errorf("Unexpected validation result %v", result)
	}
}

func TestValidateSource(t *testing.T) {
	errorCases := map[string]*api.BuildSource{
		string(errs.ValidationErrorTypeRequired) + "Git.URI": &api.BuildSource{
			Type: api.BuildSourceGit,
			Git: &api.GitBuildSource{
				URI: "",
			},
		},
		string(errs.ValidationErrorTypeInvalid) + "Git.URI": &api.BuildSource{
			Type: api.BuildSourceGit,
			Git: &api.GitBuildSource{
				URI: "::",
			},
		},
	}
	for desc, config := range errorCases {
		errors := validateBuildSource(config)
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

func TestValidateBuildInput(t *testing.T) {
	errorCases := map[string]*api.BuildInput{
		string(errs.ValidationErrorTypeRequired) + "imageTag": &api.BuildInput{
			ImageTag: "",
		},
		string(errs.ValidationErrorTypeRequired) + "stiBuild.builderImage": &api.BuildInput{
			ImageTag: "repository/data",
			STIInput: &api.STIBuildInput{
				BuilderImage: "",
			},
		},
	}

	for desc, config := range errorCases {
		errors := validateBuildInput(config)
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
