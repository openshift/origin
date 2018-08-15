package dockerfile

import (
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
)

func TestGetImageScriptsDir(t *testing.T) {
	type testCase struct {
		Config        *api.Config
		ExpectedDir   string
		HasAllScripts bool
	}

	cases := []testCase{
		{
			Config:      &api.Config{},
			ExpectedDir: defaultScriptsDir,
		},
		{
			Config: &api.Config{
				ScriptsURL: "image:///usr/some/dir",
			},
			ExpectedDir:   "/usr/some/dir",
			HasAllScripts: true,
		},
		{
			Config: &api.Config{
				ScriptsURL: "https://github.com/openshift/source-to-image",
			},
			ExpectedDir: defaultScriptsDir,
		},
		{
			Config: &api.Config{
				ImageScriptsURL: "image:///usr/some/dir",
			},
			ExpectedDir: "/usr/some/dir",
		},
		{
			Config: &api.Config{
				ImageScriptsURL: "https://github.com/openshift/source-to-image",
			},
			ExpectedDir: defaultScriptsDir,
		},
		{
			Config: &api.Config{
				ScriptsURL:      "https://github.com/openshift/source-to-image",
				ImageScriptsURL: "image:///usr/some/dir",
			},
			ExpectedDir: "/usr/some/dir",
		},
		{
			Config: &api.Config{
				ScriptsURL:      "image:///usr/some/dir",
				ImageScriptsURL: "image:///usr/other/dir",
			},
			ExpectedDir:   "/usr/some/dir",
			HasAllScripts: true,
		},
	}
	for _, tc := range cases {
		output, hasScripts := getImageScriptsDir(tc.Config)
		if output != tc.ExpectedDir {
			t.Errorf("Expected image scripts dir %s to be %s", output, tc.ExpectedDir)
		}
		if hasScripts != tc.HasAllScripts {
			t.Errorf("Expected has all scripts indicator:\n%v\nto be: %v", hasScripts, tc.HasAllScripts)
		}
	}
}
