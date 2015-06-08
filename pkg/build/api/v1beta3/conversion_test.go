package v1beta3_test

import (
	"testing"

	knewer "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	newer "github.com/openshift/origin/pkg/build/api"
	older "github.com/openshift/origin/pkg/build/api/v1beta3"
)

var Convert = knewer.Scheme.Convert

func TestBuildTriggerPolicyOldToNewConversion(t *testing.T) {
	testCases := map[string]struct {
		Olds                     []older.BuildTriggerType
		ExpectedBuildTriggerType newer.BuildTriggerType
	}{
		"ImageChange": {
			Olds: []older.BuildTriggerType{
				older.ImageChangeBuildTriggerType,
				older.BuildTriggerType(newer.ImageChangeBuildTriggerType),
			},
			ExpectedBuildTriggerType: newer.ImageChangeBuildTriggerType,
		},
		"Generic": {
			Olds: []older.BuildTriggerType{
				older.GenericWebHookBuildTriggerType,
				older.BuildTriggerType(newer.GenericWebHookBuildTriggerType),
			},
			ExpectedBuildTriggerType: newer.GenericWebHookBuildTriggerType,
		},
		"GitHub": {
			Olds: []older.BuildTriggerType{
				older.GitHubWebHookBuildTriggerType,
				older.BuildTriggerType(newer.GitHubWebHookBuildTriggerType),
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

func TestBuildTriggerPolicyNewToOldConversion(t *testing.T) {
	testCases := map[string]struct {
		New                      newer.BuildTriggerType
		ExpectedBuildTriggerType older.BuildTriggerType
	}{
		"ImageChange": {
			New: newer.ImageChangeBuildTriggerType,
			ExpectedBuildTriggerType: older.ImageChangeBuildTriggerType,
		},
		"Generic": {
			New: newer.GenericWebHookBuildTriggerType,
			ExpectedBuildTriggerType: older.GenericWebHookBuildTriggerType,
		},
		"GitHub": {
			New: newer.GitHubWebHookBuildTriggerType,
			ExpectedBuildTriggerType: older.GitHubWebHookBuildTriggerType,
		},
	}
	for s, testCase := range testCases {
		var actual older.BuildTriggerPolicy
		newVersion := newer.BuildTriggerPolicy{
			Type: testCase.New,
		}
		err := Convert(&newVersion, &actual)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", s, err)
		}
		if actual.Type != testCase.ExpectedBuildTriggerType {
			t.Errorf("%s: expected %v, actual %v", s, testCase.ExpectedBuildTriggerType, actual.Type)
		}
	}
}
