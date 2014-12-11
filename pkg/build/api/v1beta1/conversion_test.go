package v1beta1_test

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	newer "github.com/openshift/origin/pkg/build/api"
	current "github.com/openshift/origin/pkg/build/api/v1beta1"
)

var Convert = api.Scheme.Convert

func TestSTIBuildStrategyConversion(t *testing.T) {
	var got newer.STIBuildStrategy
	oldVersion := current.STIBuildStrategy{
		BuilderImage: "testimage",
	}
	err := Convert(&oldVersion, &got)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Image != oldVersion.BuilderImage {
		t.Error("expected %v, got %v", oldVersion.BuilderImage, got.Image)
	}
}
