package dockerfile

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/api/constants"
	"github.com/openshift/source-to-image/pkg/util/fs"
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

func TestInstallScripts(t *testing.T) {
	allErrs := map[string]bool{
		constants.Assemble:      true,
		constants.Run:           true,
		constants.SaveArtifacts: true,
	}

	tests := []struct {
		name                string
		url                 string
		createAssemble      bool
		createRun           bool
		createSaveArtifacts bool
		scriptErrs          map[string]bool
	}{
		{
			name:       "empty",
			scriptErrs: allErrs,
		},
		{
			name:       "bad url",
			url:        "https://foobadbar.com",
			scriptErrs: allErrs,
		},
		{
			// image:// URLs should always report success
			name: "image url",
			url:  "image://path/to/scripts",
		},
		{
			name:           "assemble script",
			createAssemble: true,
			scriptErrs: map[string]bool{
				constants.Assemble:      false,
				constants.Run:           true,
				constants.SaveArtifacts: true,
			},
		},
		{
			name:      "run script",
			createRun: true,
			scriptErrs: map[string]bool{
				constants.Assemble:      true,
				constants.Run:           false,
				constants.SaveArtifacts: true,
			},
		},
		{
			name:                "save-artifacts script",
			createSaveArtifacts: true,
			scriptErrs: map[string]bool{
				constants.Assemble:      true,
				constants.Run:           true,
				constants.SaveArtifacts: false,
			},
		},
		{
			name:                "all scripts",
			createAssemble:      true,
			createRun:           true,
			createSaveArtifacts: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			workDir, err := ioutil.TempDir("", "s2i-dockerfile-uploads")
			if err != nil {
				t.Fatalf("failed to create working dir: %v", err)
			}
			defer os.RemoveAll(workDir)
			config := &api.Config{
				WorkingDir: workDir,
			}
			fileSystem := fs.NewFileSystem()
			for _, v := range workingDirs {
				err = fileSystem.MkdirAllWithPermissions(filepath.Join(workDir, v), 0755)
				if err != nil {
					t.Fatalf("failed to create working dir: %v", err)
				}
			}

			tempDir, err := ioutil.TempDir("", "s2i-dockerfile-scripts")
			if err != nil {
				t.Fatalf("could not create temp dir: %v", err)
			}
			defer os.RemoveAll(tempDir)
			if tc.createAssemble {
				err := createTestScript(tempDir, constants.Assemble)
				if err != nil {
					t.Fatalf("failed to write %s script: %v", constants.Assemble, err)
				}
				tc.url = fmt.Sprintf("file://%s", filepath.ToSlash(tempDir))
			}
			if tc.createRun {
				err := createTestScript(tempDir, constants.Run)
				if err != nil {
					t.Fatalf("failed to write %s script: %v", constants.Run, err)
				}
				tc.url = fmt.Sprintf("file://%s", filepath.ToSlash(tempDir))
			}
			if tc.createSaveArtifacts {
				err := createTestScript(tempDir, constants.SaveArtifacts)
				if err != nil {
					t.Fatalf("failed to write %s script: %v", constants.SaveArtifacts, err)
				}
				tc.url = fmt.Sprintf("file://%s", filepath.ToSlash(tempDir))
			}
			builder, _ := New(config, fileSystem)
			results := builder.installScripts(tc.url, config)
			for _, script := range results {
				expectErr := tc.scriptErrs[script.Script]
				if expectErr && script.Error == nil {
					t.Errorf("expected error for %s, got nil", script.Script)
				}
				if script.Error != nil && !expectErr {
					t.Errorf("received unexpected error: %v", script.Error)
				}
			}
		})
	}
}

func createTestScript(dir string, name string) error {
	script := "echo \"test script\""
	path := filepath.Join(dir, name)
	err := ioutil.WriteFile(path, []byte(script), 0700)
	return err
}
