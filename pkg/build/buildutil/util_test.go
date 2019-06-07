package buildutil

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestMergeEnvWithoutDuplicates(t *testing.T) {
	testCases := []struct {
		name                string
		useSourcePrecedence bool
		whitelist           []string
		input               []corev1.EnvVar
		currentOutput       []corev1.EnvVar
		expectedOutput      []corev1.EnvVar
	}{
		{
			name: "use target values",
			input: []corev1.EnvVar{
				// overrode by target value
				{Name: "foo", Value: "bar"},
				{Name: "input", Value: "inputVal"},
				// overrode by target value
				{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
				{Name: "BUILD_LOGLEVEL", Value: "source"},
				{Name: "LANG", Value: "en_US.utf8"},
			},
			currentOutput: []corev1.EnvVar{
				{Name: "foo", Value: "test"},
				{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
			},
			expectedOutput: []corev1.EnvVar{
				{Name: "foo", Value: "test"},
				{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
				{Name: "input", Value: "inputVal"},
				{Name: "BUILD_LOGLEVEL", Value: "source"},
				{Name: "LANG", Value: "en_US.utf8"},
			},
		},
		{
			name:                "use source values",
			useSourcePrecedence: true,
			input: []corev1.EnvVar{
				{Name: "foo", Value: "bar"},
				{Name: "input", Value: "inputVal"},
				// overrode by target value
				{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
				{Name: "BUILD_LOGLEVEL", Value: "source"},
				{Name: "LANG", Value: "en_US.utf8"},
			},
			currentOutput: []corev1.EnvVar{
				// overrode by source values
				{Name: "foo", Value: "test"},
				{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
				// unmodified in result
				{Name: "target", Value: "acquired"},
			},
			expectedOutput: []corev1.EnvVar{
				{Name: "foo", Value: "bar"},
				{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
				{Name: "target", Value: "acquired"},
				{Name: "input", Value: "inputVal"},
				{Name: "BUILD_LOGLEVEL", Value: "source"},
				{Name: "LANG", Value: "en_US.utf8"},
			},
		},
		{
			name:                "use source with trusted whitelist",
			useSourcePrecedence: true,
			whitelist:           WhitelistEnvVarNames,
			input: []corev1.EnvVar{
				// stripped by whitelist
				{Name: "foo", Value: "bar"},
				// stripped by whitelist
				{Name: "input", Value: "inputVal"},
				{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
				{Name: "BUILD_LOGLEVEL", Value: "source"},
				{Name: "LANG", Value: "en_US.utf8"},
			},
			currentOutput: []corev1.EnvVar{
				{Name: "foo", Value: "test"},
				{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
			},
			expectedOutput: []corev1.EnvVar{
				{Name: "foo", Value: "test"},
				{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
				{Name: "BUILD_LOGLEVEL", Value: "source"},
				{Name: "LANG", Value: "en_US.utf8"},
			},
		},
		{
			name:      "use target with trusted whitelist",
			whitelist: WhitelistEnvVarNames,
			input: []corev1.EnvVar{
				// stripped by whitelist
				{Name: "foo", Value: "bar"},
				// stripped by whitelist
				{Name: "input", Value: "inputVal"},
				// overrode by target value
				{Name: "GIT_SSL_NO_VERIFY", Value: "source"},
				{Name: "BUILD_LOGLEVEL", Value: "source"},
				{Name: "LANG", Value: "en_US.utf8"},
			},
			currentOutput: []corev1.EnvVar{
				{Name: "foo", Value: "test"},
				{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
			},
			expectedOutput: []corev1.EnvVar{
				{Name: "foo", Value: "test"},
				{Name: "GIT_SSL_NO_VERIFY", Value: "target"},
				{Name: "BUILD_LOGLEVEL", Value: "source"},
				{Name: "LANG", Value: "en_US.utf8"},
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := &tc.currentOutput
			MergeEnvWithoutDuplicates(tc.input, output, tc.useSourcePrecedence, tc.whitelist)
			outputVal := *output
			if len(outputVal) != len(tc.expectedOutput) {
				t.Fatalf("Expected output to be %d, got %d", len(tc.expectedOutput), len(*output))
			}
			for i, expected := range tc.expectedOutput {
				val := outputVal[i]
				if !reflect.DeepEqual(val, expected) {
					t.Errorf("Expected env var %+v, got %+v", expected, val)
				}
			}
		})
	}
}
