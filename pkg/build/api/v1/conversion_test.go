package v1_test

import (
	"testing"

	knewer "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	/*kolder "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1"*/

	newer "github.com/openshift/origin/pkg/build/api"
	older "github.com/openshift/origin/pkg/build/api/v1"
)

var Convert = knewer.Scheme.Convert

func TestImageChangeTriggerDefaultValueConversion(t *testing.T) {
	var actual newer.BuildTriggerPolicy

	oldVersion := older.BuildTriggerPolicy{
		Type: older.ImageChangeBuildTriggerType,
	}
	err := Convert(&oldVersion, &actual)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if actual.ImageChange == nil {
		t.Errorf("expected %v, actual %v", &newer.ImageChangeTrigger{}, nil)
	}
}
