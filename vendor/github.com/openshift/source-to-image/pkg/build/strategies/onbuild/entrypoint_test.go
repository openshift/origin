package onbuild

import (
	"os"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/test"
	"github.com/openshift/source-to-image/pkg/util"
)

func TestGuessEntrypoint(t *testing.T) {

	testMatrix := map[string][]os.FileInfo{
		"run": {
			&util.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "run", FileIsDir: false, FileMode: 0777},
		},
		"start.sh": {
			&util.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "start.sh", FileIsDir: false, FileMode: 0777},
		},
		"execute": {
			&util.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "execute", FileIsDir: false, FileMode: 0777},
		},
		"ERR:run_not_executable": {
			&util.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "run", FileIsDir: false, FileMode: 0600},
		},
		"ERR:run_is_dir": {
			&util.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "run", FileIsDir: true, FileMode: 0777},
		},
		"ERR:none": {
			&util.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&util.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
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
