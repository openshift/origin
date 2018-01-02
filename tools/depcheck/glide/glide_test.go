package glide

import (
	"testing"
	"time"
)

func TestMissingImports(t *testing.T) {
	yamlFile := &YamlFile{
		Package: "test",
		Imports: []*YamlFileImport{
			{
				Package: "pkg/one",
				Version: "1",
			},
			{
				Package: "pkg/two",
				Version: "2",
			},
		},
	}

	lockFile := &LockFile{
		Hash:    "hash",
		Updated: time.Now(),
		Imports: []*LockFileImport{
			{
				Name:    "pkg/one",
				Version: "1",
			},
			{
				Name:    "pkg/two",
				Version: "2",
			},
			{
				Name:    "pkg/three",
				Version: "3",
			},
			{
				Name:    "pkg/four",
				Repo:    "repo",
				Version: "4",
			},
		},
	}

	tests := []struct {
		name            string
		lockfile        *LockFile
		yamlfile        *YamlFile
		expectedImports []*YamlFileImport
		expectErr       bool
		matchErr        string
	}{
		{
			name:            "no lockfile or yaml file",
			lockfile:        lockFile,
			yamlfile:        nil,
			expectedImports: nil,
			expectErr:       true,
			matchErr:        "both a lockfile and a yamlfile are required",
		},
		{
			name:     "skip imports with a repo field",
			lockfile: lockFile,
			yamlfile: yamlFile,
			expectedImports: []*YamlFileImport{
				{
					Package: "pkg/three",
					Version: "3",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			missing, _, err := MissingImports(tc.lockfile, tc.yamlfile)
			switch {
			case err != nil && !tc.expectErr:
				t.Fatal(err)
			case err != nil && tc.matchErr != err.Error():
				t.Fatalf("expected error with message %q, but got %q", tc.matchErr, err.Error())
			case err == nil && tc.expectErr:
				t.Fatalf("expected error %q but no error returned", tc.matchErr)
			}

			if len(missing) != len(tc.expectedImports) {
				t.Fatalf("expected missing imports %#v, but got %#v", tc.expectedImports, missing)
			}

			for i := range tc.expectedImports {
				if tc.expectedImports[i].Package != missing[i].Package || tc.expectedImports[i].Version != missing[i].Version {
					t.Errorf("expedcted package %v but got %v", tc.expectedImports[i], missing[i])
				}
			}
		})
	}
}
