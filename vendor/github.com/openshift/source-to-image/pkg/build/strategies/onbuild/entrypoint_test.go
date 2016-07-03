package onbuild

import (
	"os"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/test"
)

func TestGuessEntrypoint(t *testing.T) {

	testMatrix := map[string][]os.FileInfo{
		"run": {
			&test.FakeFile{"config.ru", false, 0600},
			&test.FakeFile{"app.rb", false, 0600},
			&test.FakeFile{"run", false, 0777},
		},
		"start.sh": {
			&test.FakeFile{"config.ru", false, 0600},
			&test.FakeFile{"app.rb", false, 0600},
			&test.FakeFile{"start.sh", false, 0777},
		},
		"execute": {
			&test.FakeFile{"config.ru", false, 0600},
			&test.FakeFile{"app.rb", false, 0600},
			&test.FakeFile{"execute", false, 0777},
		},
		"ERR:run_not_executable": {
			&test.FakeFile{"config.ru", false, 0600},
			&test.FakeFile{"app.rb", false, 0600},
			&test.FakeFile{"run", false, 0600},
		},
		"ERR:run_is_dir": {
			&test.FakeFile{"config.ru", false, 0600},
			&test.FakeFile{"app.rb", false, 0600},
			&test.FakeFile{"run", true, 0777},
		},
		"ERR:none": {
			&test.FakeFile{"config.ru", false, 0600},
			&test.FakeFile{"app.rb", false, 0600},
		},
	}

	for expectedEntrypoint, files := range testMatrix {
		f := &test.FakeFileSystem{Files: files}
		result, err := GuessEntrypoint(f, "/test")

		if strings.HasPrefix(expectedEntrypoint, "ERR:") {
			if len(result) > 0 {
				t.Errorf("Expected error for %s, got %s", expectedEntrypoint, result)
			}
			continue
		}

		if err != nil {
			t.Errorf("[%s] %s", expectedEntrypoint, err)
		}
		if result != expectedEntrypoint {
			t.Errorf("Expected '%s' entrypoint, got '%v'", expectedEntrypoint, result)
		}
	}
}
