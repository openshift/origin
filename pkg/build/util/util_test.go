package util

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

func TestGetBuildPodName(t *testing.T) {
	if expected, actual := "mybuild-build", GetBuildPodName(&buildapi.Build{ObjectMeta: kapi.ObjectMeta{Name: "mybuild"}}); expected != actual {
		t.Errorf("Expected %s, got %s", expected, actual)
	}
}

func TestGetBuildLabel(t *testing.T) {
	type getBuildLabelTest struct {
		labels         map[string]string
		expectedValue  string
		expectedExists bool
	}

	tests := []getBuildLabelTest{
		{
			// 0 - new label
			labels:         map[string]string{buildapi.BuildLabel: "value"},
			expectedValue:  "value",
			expectedExists: true,
		},
		{
			// 1 - deprecated label
			labels:         map[string]string{buildapi.DeprecatedBuildLabel: "value"},
			expectedValue:  "value",
			expectedExists: true,
		},
		{
			// 2 - deprecated label
			labels:         map[string]string{},
			expectedValue:  "",
			expectedExists: false,
		},
	}
	for i, tc := range tests {
		value, exists := GetBuildLabel(&kapi.Pod{ObjectMeta: kapi.ObjectMeta{Labels: tc.labels}})
		if value != tc.expectedValue {
			t.Errorf("(%d) unexpected value, expected %s, got %s", i, tc.expectedValue, value)
		}
		if exists != tc.expectedExists {
			t.Errorf("(%d) unexpected exists flag, expected %v, got %v", i, tc.expectedExists, exists)
		}
	}
}
