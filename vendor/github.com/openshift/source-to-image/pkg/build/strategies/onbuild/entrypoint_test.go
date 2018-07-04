package onbuild

import (
	"os"
	"strings"
	"testing"

	testfs "github.com/openshift/source-to-image/pkg/test/fs"
	"github.com/openshift/source-to-image/pkg/util/fs"
)

func TestGuessEntrypoint(t *testing.T) {

	testMatrix := map[string][]os.FileInfo{
		"run": {
			&fs.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "run", FileIsDir: false, FileMode: 0777},
		},
		"start.sh": {
			&fs.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "start.sh", FileIsDir: false, FileMode: 0777},
		},
		"execute": {
			&fs.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "execute", FileIsDir: false, FileMode: 0777},
		},
		"ERR:run_not_executable": {
			&fs.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "run", FileIsDir: false, FileMode: 0600},
		},
		"ERR:run_is_dir": {
			&fs.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "run", FileIsDir: true, FileMode: 0777},
		},
		"ERR:none": {
			&fs.FileInfo{FileName: "config.ru", FileIsDir: false, FileMode: 0600},
			&fs.FileInfo{FileName: "app.rb", FileIsDir: false, FileMode: 0600},
		},
	}

	for expectedEntrypoint, files := range testMatrix {
		f := &testfs.FakeFileSystem{Files: files}
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
