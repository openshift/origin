package app

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	build "github.com/openshift/origin/pkg/build/api"
)

func TestWithType(t *testing.T) {
	out := &Generated{
		Items: []runtime.Object{
			&build.BuildConfig{
				ObjectMeta: kapi.ObjectMeta{
					Name: "foo",
				},
			},
			&kapi.Service{
				ObjectMeta: kapi.ObjectMeta{
					Name: "foo",
				},
			},
		},
	}

	builds := []build.BuildConfig{}
	if !out.WithType(&builds) {
		t.Errorf("expected true")
	}
	if len(builds) != 1 {
		t.Errorf("unexpected slice: %#v", builds)
	}

	buildPtrs := []*build.BuildConfig{}
	if out.WithType(&buildPtrs) {
		t.Errorf("expected false")
	}
	if len(buildPtrs) != 0 {
		t.Errorf("unexpected slice: %#v", buildPtrs)
	}
}

func TestSimpleBuildConfig(t *testing.T) {
	source, err := SourceRefForGitURL("https://github.com/openshift/origin.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	build := &BuildRef{Source: source}
	config, err := build.BuildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "origin" {
		t.Errorf("unexpected name: %#v", config)
	}

	output := &ImageRef{Registry: "myregistry", Namespace: "openshift", Name: "origin"}
	build = &BuildRef{Source: source, Output: output}
	config, err = build.BuildConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "origin" || config.Parameters.Output.ImageTag != "myregistry/openshift/origin" || config.Parameters.Output.Registry != "myregistry" {
		t.Errorf("unexpected name: %#v", config)
	}
}

func TestSimpleDeploymentConfig(t *testing.T) {
	image := &ImageRef{Registry: "myregistry", Namespace: "openshift", Name: "origin"}
	deploy := &DeploymentConfigRef{Images: []*ImageRef{image}}
	config, err := deploy.DeploymentConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if config.Name != "origin" || len(config.Triggers) != 2 || config.Template.ControllerTemplate.PodTemplate.DesiredState.Manifest.Containers[0].Image != image.pullSpec() {
		t.Errorf("unexpected value: %#v", config)
	}
}
