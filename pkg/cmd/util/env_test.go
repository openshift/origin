package util

import (
	"reflect"
	"testing"
)

func TestParseEnvironmentArguments(t *testing.T) {
	testcases := []struct {
		Strings        []string
		ExpectedResult map[string]string
		ExpectedDups   bool
		ExpectedError  bool
	}{
		{
			Strings:        []string{},
			ExpectedResult: map[string]string{},
			ExpectedDups:   false,
			ExpectedError:  false,
		},
		{
			Strings: []string{"FOO=BAR"},
			ExpectedResult: map[string]string{
				"FOO": "BAR",
			},
			ExpectedDups:  false,
			ExpectedError: false,
		},
		{
			Strings: []string{"FOO=BAR", "FOO=BAZ"},
			ExpectedResult: map[string]string{
				"FOO": "BAZ",
			},
			ExpectedDups:  true,
			ExpectedError: false,
		},
		{
			Strings: []string{"FOO=BAR", "ONE@testdata/file1.txt"},
			ExpectedResult: map[string]string{
				"FOO": "BAR",
				"ONE": "one\n",
			},
			ExpectedDups:  false,
			ExpectedError: false,
		},
		{
			Strings: []string{"ONE@testdata/file1.txt", "TWO@testdata/file2.txt"},
			ExpectedResult: map[string]string{
				"ONE": "one\n",
				"TWO": "two\n",
			},
			ExpectedDups:  false,
			ExpectedError: false,
		},
		{
			Strings: []string{"ONE@testdata/file1.txt", "ONE=otherone"},
			ExpectedResult: map[string]string{
				"ONE": "otherone",
			},
			ExpectedDups:  true,
			ExpectedError: false,
		},
		{
			Strings:        []string{"==="},
			ExpectedResult: map[string]string{},
			ExpectedDups:   false,
			ExpectedError:  true,
		},
		{
			Strings:        []string{"@@@"},
			ExpectedResult: map[string]string{},
			ExpectedDups:   false,
			ExpectedError:  true,
		},
		{
			Strings: []string{"FOO="},
			ExpectedResult: map[string]string{
				"FOO": "",
			},
			ExpectedDups:  false,
			ExpectedError: false,
		},
		{
			Strings:        []string{"FOO@"},
			ExpectedResult: map[string]string{},
			ExpectedDups:   false,
			ExpectedError:  true,
		},
		{
			Strings:        []string{"FOO@testdata"},
			ExpectedResult: map[string]string{},
			ExpectedDups:   false,
			ExpectedError:  true,
		},
		{
			Strings:        []string{"FOO@testdata/"},
			ExpectedResult: map[string]string{},
			ExpectedDups:   false,
			ExpectedError:  true,
		},
		{
			Strings:        []string{"FOO@testdata/doesntexist.bmp"},
			ExpectedResult: map[string]string{},
			ExpectedDups:   false,
			ExpectedError:  true,
		},
		{
			Strings:        []string{"FOO@http://docs.openshift.org/"},
			ExpectedResult: map[string]string{},
			ExpectedDups:   false,
			ExpectedError:  true,
		},
		{
			Strings: []string{"NOTHING@testdata/empty.txt"},
			ExpectedResult: map[string]string{
				"NOTHING": "",
			},
			ExpectedDups:  false,
			ExpectedError: false,
		},
	}

	for _, tc := range testcases {
		env, dups, errs := ParseEnvironmentArguments(tc.Strings, true)

		if len(errs) != 0 {
			if !tc.ExpectedError {
				t.Errorf("Unexpected error for %s: %s", tc.Strings, errs)
			}
			continue
		}

		if tc.ExpectedError {
			t.Errorf("Unexpected success for %s", tc.Strings)
			continue
		}

		if !reflect.DeepEqual(tc.ExpectedResult, map[string]string(env)) {
			t.Errorf("Unexpected result for %s: %#v != %#v", tc.Strings, tc.ExpectedResult, env)
			continue
		}

		if (len(dups) != 0) != tc.ExpectedDups {
			t.Errorf("Unexpected duplicates for %s: %s", tc.Strings, dups)
		}
	}
}
