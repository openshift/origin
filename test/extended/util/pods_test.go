package util

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestHasEnvVar(t *testing.T) {
	tests := []struct {
		name          string
		container     *corev1.Container
		envVarName    string
		expectedValue string
		expected      bool
	}{
		{
			name:          "nil container",
			container:     nil,
			envVarName:    "TEST_VAR",
			expectedValue: "test_value",
			expected:      false,
		},
		{
			name: "env var exists with matching value",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{Name: "TEST_VAR", Value: "test_value"},
					{Name: "OTHER_VAR", Value: "other_value"},
				},
			},
			envVarName:    "TEST_VAR",
			expectedValue: "test_value",
			expected:      true,
		},
		{
			name: "env var exists with non-matching value",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{Name: "TEST_VAR", Value: "wrong_value"},
					{Name: "OTHER_VAR", Value: "other_value"},
				},
			},
			envVarName:    "TEST_VAR",
			expectedValue: "test_value",
			expected:      false,
		},
		{
			name: "env var does not exist",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{Name: "OTHER_VAR", Value: "other_value"},
				},
			},
			envVarName:    "TEST_VAR",
			expectedValue: "test_value",
			expected:      false,
		},
		{
			name: "empty env vars",
			container: &corev1.Container{
				Env: []corev1.EnvVar{},
			},
			envVarName:    "TEST_VAR",
			expectedValue: "test_value",
			expected:      false,
		},
		{
			name: "env var with empty value matches empty expected value",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{Name: "TEST_VAR", Value: ""},
				},
			},
			envVarName:    "TEST_VAR",
			expectedValue: "",
			expected:      true,
		},
		{
			name: "env var with empty value does not match non-empty expected value",
			container: &corev1.Container{
				Env: []corev1.EnvVar{
					{Name: "TEST_VAR", Value: ""},
				},
			},
			envVarName:    "TEST_VAR",
			expectedValue: "test_value",
			expected:      false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := HasEnvVar(test.container, test.envVarName, test.expectedValue)
			if result != test.expected {
				t.Errorf("HasEnvVar() = %v, expected %v", result, test.expected)
			}
		})
	}
}
