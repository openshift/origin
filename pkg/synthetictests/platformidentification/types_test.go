package platformidentification

import (
	"regexp"
	"testing"
)

func TestMostRecentlyCompletedVersionIsAtLeast(t *testing.T) {
	jt := JobType{MostRecentCompletedRelease: "1.2"}
	for _, testCase := range []struct {
		name          string
		version       string
		expectedError *regexp.Regexp
	}{
		{
			name:          "input patch version",
			version:       "1.2.3",
			expectedError: regexp.MustCompile(`^invalid MostRecentlyCompletedVersionIsAtLeast argument: "1[.]2[.]3" has 3 parts, but at most 2 parts are allowed$`),
		},
		{
			name:          "input string minor",
			version:       "1.notAnInteger",
			expectedError: regexp.MustCompile(`^invalid MostRecentlyCompletedVersionIsAtLeast argument: "1[.]notAnInteger" has a non-integer minor version "notAnInteger": strconv.Atoi: parsing "notAnInteger": invalid syntax$`),
		},
		{
			name:    "earlier major",
			version: "0.2",
		},
		{
			name:    "earlier minor",
			version: "1.0",
		},
		{
			name:    "same version",
			version: "1.2",
		},
		{
			name:          "later minor",
			version:       "1.3",
			expectedError: regexp.MustCompile(`^have completed 1[.]2, but not 1[.]3`),
		},
		{
			name:          "later major",
			version:       "2.0",
			expectedError: regexp.MustCompile(`^have completed 1[.]2, but not 2[.]0`),
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			err := jt.MostRecentlyCompletedVersionIsAtLeast(testCase.version)

			if err != nil && testCase.expectedError == nil {
				t.Errorf("unexpected error: %v", err)
			} else if testCase.expectedError != nil && err == nil {
				t.Errorf("unexpected success, expected: %s", testCase.expectedError)
			} else if testCase.expectedError != nil && !testCase.expectedError.MatchString(err.Error()) {
				t.Errorf("expected error %s, not: %v", testCase.expectedError, err)
			}
		})
	}
}
