package v1beta1_test

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"

	newer "github.com/openshift/origin/pkg/build/api"
	current "github.com/openshift/origin/pkg/build/api/v1beta1"
)

var Convert = api.Scheme.Convert

func TestSTIBuildStrategyConversion(t *testing.T) {
	var actual newer.STIBuildStrategy
	oldVersion := current.STIBuildStrategy{
		BuilderImage: "testimage",
		Clean:        true,
	}

	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.Image != oldVersion.BuilderImage {
		t.Errorf("expected %v, actual %v", oldVersion.BuilderImage, actual.Image)
	}
	if actual.Incremental == oldVersion.Clean {
		t.Errorf("expected %v, actual %v", oldVersion.Clean, actual.Incremental)
	}
}

func TestDockerBuildStrategyConversion(t *testing.T) {
	var actual newer.DockerBuildStrategy
	oldVersion := current.DockerBuildStrategy{
		BaseImage: "testimage",
	}
	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.Image != oldVersion.BaseImage {
		t.Errorf("expected %v, actual %v", oldVersion.BaseImage, actual.Image)
	}
}

func TestContextDirConversion(t *testing.T) {
	var actual newer.BuildParameters
	oldVersion := current.BuildParameters{
		Strategy: current.BuildStrategy{
			Type: current.DockerBuildStrategyType,
			DockerStrategy: &current.DockerBuildStrategy{
				ContextDir: "contextDir",
			},
		},
	}
	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.Source.ContextDir != oldVersion.Strategy.DockerStrategy.ContextDir {
		t.Errorf("expected %v, actual %v", oldVersion.Strategy.DockerStrategy.ContextDir, actual.Source.ContextDir)
	}
}

func TestImageChangeTriggerFromRename(t *testing.T) {
	old := current.ImageChangeTrigger{
		From: kapi.ObjectReference{
			Name: "foo",
		},
		ImageRepositoryRef: &kapi.ObjectReference{
			Name: "bar",
		},
	}
	actual := newer.ImageChangeTrigger{}
	if err := Convert(&old, &actual); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Name != "bar" {
		t.Error("expected %#v, actual %#v", old, actual)
	}

	old = current.ImageChangeTrigger{
		From: kapi.ObjectReference{
			Name: "foo",
		},
	}
	actual = newer.ImageChangeTrigger{}
	if err := Convert(&old, &actual); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Name != "foo" {
		t.Error("expected %#v, actual %#v", old, actual)
	}

	old = current.ImageChangeTrigger{
		From: kapi.ObjectReference{
			Name: "foo",
		},
		ImageRepositoryRef: &kapi.ObjectReference{},
	}
	actual = newer.ImageChangeTrigger{}
	if err := Convert(&old, &actual); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.From.Name != "" {
		t.Errorf("expected %#v, actual %#v", old, actual)
	}
}
