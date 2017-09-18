package server

import (
	"os"
	"testing"
	"time"
)

func TestGetBoolOption(t *testing.T) {
	for _, tc := range []struct {
		name          string
		envName       string
		exportEnv     map[string]string
		option        string
		options       map[string]interface{}
		defaultValue  bool
		expected      bool
		expectedError bool
	}{
		{
			name:         "default to false",
			defaultValue: false,
			option:       "opt",
			expected:     false,
		},

		{
			name:         "default to true",
			defaultValue: true,
			option:       "opt",
			envName:      "VBOOL",
			expected:     true,
		},

		{
			name:         "given option",
			defaultValue: false,
			option:       "opt",
			options:      map[string]interface{}{"opt": "true"},
			expected:     true,
		},

		{
			name:         "env value with missing option",
			defaultValue: true,
			option:       "opt",
			envName:      "VBOOL",
			exportEnv:    map[string]string{"VBOOL": "false"},
			expected:     false,
		},

		{
			name:         "given option and env var",
			defaultValue: false,
			option:       "opt",
			options:      map[string]interface{}{"opt": "false"},
			envName:      "VAR",
			exportEnv:    map[string]string{"VAR": "true"},
			expected:     true,
		},

		{
			name:         "disable with env var",
			defaultValue: true,
			option:       "opt",
			options:      map[string]interface{}{"opt": "true"},
			envName:      "VAR",
			exportEnv:    map[string]string{"VAR": "false"},
			expected:     false,
		},

		{
			name:          "given option with bad env value",
			defaultValue:  false,
			option:        "opt",
			options:       map[string]interface{}{"opt": "true"},
			envName:       "VAR",
			exportEnv:     map[string]string{"VAR": "falsed"},
			expected:      true,
			expectedError: true,
		},

		{
			name:          "env value with wrong option type",
			defaultValue:  true,
			option:        "opt",
			options:       map[string]interface{}{"opt": 1},
			envName:       "VAR",
			exportEnv:     map[string]string{"VAR": "true"},
			expected:      true,
			expectedError: true,
		},

		{
			name:          "env value with bad option value",
			defaultValue:  true,
			option:        "opt",
			options:       map[string]interface{}{"opt": "falsed"},
			envName:       "VAR",
			exportEnv:     map[string]string{"VAR": "false"},
			expected:      false,
			expectedError: true,
		},

		{
			name:          "bad env value with bad option value",
			defaultValue:  false,
			option:        "opt",
			options:       map[string]interface{}{"opt": "turk"},
			envName:       "VAR",
			exportEnv:     map[string]string{"VAR": "truth"},
			expected:      false,
			expectedError: true,
		},
	} {
		for key, value := range tc.exportEnv {
			os.Setenv(key, value)
		}
		d, err := getBoolOption(tc.envName, tc.option, tc.defaultValue, tc.options)
		if err == nil && tc.expectedError {
			t.Errorf("[%s] unexpected non-error", tc.name)
		} else if err != nil && !tc.expectedError {
			t.Errorf("[%s] unexpected error: %v", tc.name, err)
		}
		if d != tc.expected {
			t.Errorf("[%s] got unexpected duration: %t != %t", tc.name, d, tc.expected)
		}
	}
}

func TestGetDurationOption(t *testing.T) {
	for _, tc := range []struct {
		name             string
		envName          string
		exportEnv        map[string]string
		option           string
		options          map[string]interface{}
		defaultValue     time.Duration
		expectedDuration time.Duration
		expectedError    bool
	}{
		{
			name:             "no option, no env",
			defaultValue:     time.Minute,
			option:           "opt",
			expectedDuration: time.Minute,
		},

		{
			name:             "given option",
			defaultValue:     time.Minute,
			option:           "opt",
			options:          map[string]interface{}{"opt": "4000ns"},
			expectedDuration: 4000,
		},

		{
			name:             "env value with missing option",
			defaultValue:     time.Minute,
			option:           "opt",
			envName:          "VAR",
			exportEnv:        map[string]string{"VAR": "1s"},
			expectedDuration: time.Second,
		},

		{
			name:             "given option and env var",
			defaultValue:     time.Minute,
			option:           "opt",
			options:          map[string]interface{}{"opt": "4000us"},
			envName:          "VAR",
			exportEnv:        map[string]string{"VAR": "1s"},
			expectedDuration: time.Second,
		},

		{
			name:             "given option with bad env value",
			defaultValue:     time.Minute,
			option:           "opt",
			options:          map[string]interface{}{"opt": "1s"},
			envName:          "VAR",
			exportEnv:        map[string]string{"VAR": "bad"},
			expectedDuration: time.Second,
			expectedError:    true,
		},

		{
			name:             "env value with wrong option type",
			defaultValue:     time.Minute,
			option:           "opt",
			options:          map[string]interface{}{"opt": false},
			envName:          "VAR",
			exportEnv:        map[string]string{"VAR": "2s"},
			expectedDuration: time.Second * 2,
			expectedError:    true,
		},

		{
			name:             "env value with bad option value",
			defaultValue:     time.Minute,
			option:           "opt",
			options:          map[string]interface{}{"opt": "bad"},
			envName:          "VAR",
			exportEnv:        map[string]string{"VAR": "2s"},
			expectedDuration: time.Second * 2,
			expectedError:    true,
		},

		{
			name:             "bad env value with bad option value",
			defaultValue:     time.Minute,
			option:           "opt",
			options:          map[string]interface{}{"opt": "bad"},
			envName:          "VAR",
			exportEnv:        map[string]string{"VAR": "bad"},
			expectedDuration: time.Minute,
			expectedError:    true,
		},
	} {
		for key, value := range tc.exportEnv {
			os.Setenv(key, value)
		}
		d, err := getDurationOption(tc.envName, tc.option, tc.defaultValue, tc.options)
		if err == nil && tc.expectedError {
			t.Errorf("[%s] unexpected non-error", tc.name)
		} else if err != nil && !tc.expectedError {
			t.Errorf("[%s] unexpected error: %v", tc.name, err)
		}
		if d != tc.expectedDuration {
			t.Errorf("[%s] got unexpected duration: %s != %s", tc.name, d.String(), tc.expectedDuration.String())
		}
	}
}
