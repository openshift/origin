package v1_test

import (
	"testing"

	knewer "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

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

func TestBuildTriggerPolicyOldToNewConversion(t *testing.T) {
	testCases := map[string]struct {
		Olds                     []older.BuildTriggerType
		ExpectedBuildTriggerType newer.BuildTriggerType
	}{
		"ImageChange": {
			Olds: []older.BuildTriggerType{
				older.ImageChangeBuildTriggerType,
				older.ImageChangeBuildTriggerTypeDeprecated,
			},
			ExpectedBuildTriggerType: newer.ImageChangeBuildTriggerType,
		},
		"Generic": {
			Olds: []older.BuildTriggerType{
				older.GenericWebHookBuildTriggerType,
				older.GenericWebHookBuildTriggerTypeDeprecated,
			},
			ExpectedBuildTriggerType: newer.GenericWebHookBuildTriggerType,
		},
		"GitHub": {
			Olds: []older.BuildTriggerType{
				older.GitHubWebHookBuildTriggerType,
				older.GitHubWebHookBuildTriggerTypeDeprecated,
			},
			ExpectedBuildTriggerType: newer.GitHubWebHookBuildTriggerType,
		},
	}
	for s, testCase := range testCases {
		expected := testCase.ExpectedBuildTriggerType
		for _, old := range testCase.Olds {
			var actual newer.BuildTriggerPolicy
			oldVersion := older.BuildTriggerPolicy{
				Type: old,
			}
			err := Convert(&oldVersion, &actual)
			if err != nil {
				t.Fatalf("%s (%s -> %s): unexpected error: %v", s, old, expected, err)
			}
			if actual.Type != testCase.ExpectedBuildTriggerType {
				t.Errorf("%s (%s -> %s): expected %v, actual %v", s, old, expected, expected, actual.Type)
			}
		}
	}
}
