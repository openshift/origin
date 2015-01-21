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
	}
	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.Image != oldVersion.BuilderImage {
		t.Errorf("expected %v, actual %v", oldVersion.BuilderImage, actual.Image)
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
