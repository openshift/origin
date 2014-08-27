package validation

import (
	"testing"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/openshift/origin/pkg/build/api"
)

func TestBuildValdationSuccess(t *testing.T) {
	build := &api.Build{
		JSONBase: kubeapi.JSONBase{ID: "buildId"},
		Input: api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://github.com/my/repository",
			ImageTag:  "repository/data",
		},
		Status: api.BuildNew,
	}
	if result := ValidateBuild(build); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestBuildValidationFailure(t *testing.T) {
	build := &api.Build{
		JSONBase: kubeapi.JSONBase{ID: ""},
		Input: api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://github.com/my/repository",
			ImageTag:  "repository/data",
		},
		Status: api.BuildNew,
	}
	if result := ValidateBuild(build); len(result) != 1 {
		t.Errorf("Unexpected validation result: %v", result)
	}
}

func TestBuildConfigValidationSuccess(t *testing.T) {
	buildConfig := &api.BuildConfig{
		JSONBase: kubeapi.JSONBase{ID: "configId"},
		DesiredInput: api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://github.com/my/repository",
			ImageTag:  "repository/data",
		},
	}
	if result := ValidateBuildConfig(buildConfig); len(result) > 0 {
		t.Errorf("Unexpected validation error returned %v", result)
	}
}

func TestBuildConfigValidationFailure(t *testing.T) {
	buildConfig := &api.BuildConfig{
		JSONBase: kubeapi.JSONBase{ID: ""},
		DesiredInput: api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://github.com/my/repository",
			ImageTag:  "repository/data",
		},
	}
	if result := ValidateBuildConfig(buildConfig); len(result) != 1 {
		t.Errorf("Unexpected validation result %v", result)
	}
}

func TestValidateBuildInput(t *testing.T) {
	errorCases := map[string]*api.BuildInput{
		"No source URI": &api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "",
			ImageTag:  "repository/data",
		},
		"Invalid source URI": &api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "::",
			ImageTag:  "repository/data",
		},
		"No image tag": &api.BuildInput{
			Type:      api.DockerBuildType,
			SourceURI: "http://github.com/test/uri",
			ImageTag:  "",
		},
		"No builder image with STIBuildType": &api.BuildInput{
			Type:         api.STIBuildType,
			SourceURI:    "http://github.com/test/uri",
			ImageTag:     "repository/data",
			BuilderImage: "",
		},
		"Builder image with DockerBuildType": &api.BuildInput{
			Type:         api.DockerBuildType,
			SourceURI:    "http://github.com/test/uri",
			ImageTag:     "repository/data",
			BuilderImage: "builder/image",
		},
	}

	for desc, config := range errorCases {
		errors := validateBuildInput(config)
		if len(errors) != 1 {
			t.Errorf("%s: Unexpected validation result: %v", desc, errors)
		}
		// TODO: Verify we got the right type of validation error.
	}
}
