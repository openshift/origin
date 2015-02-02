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

func TestPodNameConversion(t *testing.T) {
	var actual newer.Build
	oldVersion := current.Build{
		ObjectMeta: kapi.ObjectMeta{Name: "name", Namespace: "namespace"},
		PodName:    "podname",
	}
	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.Name != oldVersion.Name {
		t.Errorf("expected %s, actual %s", oldVersion.Name, actual.Name)
	}
	if actual.PodRef == nil {
		t.Fatalf("unexpected nil PodRef")
	}
	if actual.PodRef.Name != oldVersion.PodName {
		t.Errorf("expected %v, actual %v", oldVersion.PodName, actual.PodRef.Name)
	}
	if actual.PodRef.Namespace != oldVersion.Namespace {
		t.Errorf("expected %v, actual %v", oldVersion.Namespace, actual.PodRef.Namespace)
	}
}
