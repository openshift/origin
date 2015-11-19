package dockerfile

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

type file struct {
	isDir bool
	name  string
	path  string
	err   error
}

func (f file) Name() string {
	return f.name
}

func (f file) Size() int64 {
	return 0
}

func (f file) Mode() os.FileMode {
	if f.isDir {
		return os.ModeDir
	}
	return os.ModePerm
}

func (f file) ModTime() time.Time {
	return time.Now()
}

func (f file) IsDir() bool {
	return f.isDir
}

func (f file) Sys() interface{} {
	return nil
}

func TestFind(t *testing.T) {
	files := []file{
		{
			name:  "test",
			path:  "test",
			isDir: true,
		},
		{
			name:  "Dockerfile",
			path:  "test/Dockerfile",
			isDir: false,
		},
		{
			name:  "Dockerfile",
			path:  "test2/Dockerfile",
			isDir: true,
		},
		{
			name:  ".hidden",
			path:  ".hidden",
			isDir: true,
		},
		{
			// Should not be in result because it's in a hidden directory
			name:  "Dockerfile",
			path:  ".hidden/Dockerfile",
			isDir: false,
		},
		{
			name:  "Dockerfile",
			path:  "Dockerfile",
			isDir: false,
		},
	}
	f := finder{makeWalkFunc(files)}
	dockerfiles, err := f.Find(".")
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if len(dockerfiles) != 2 {
		t.Errorf("Unexpected number of Dockerfiles returned: %d. Expected: 2", len(dockerfiles))
	}
	expectedResult := []string{"test/Dockerfile", "Dockerfile"}
	if !reflect.DeepEqual(dockerfiles, expectedResult) {
		t.Errorf("Unexpected result: %v. Expected: %v", dockerfiles, expectedResult)
	}
}

func TestFindError(t *testing.T) {
	err := fmt.Errorf("File error")
	files := []file{
		{
			name:  "test",
			path:  "test",
			isDir: true,
		},
		{
			name:  "Dockerfile",
			path:  "test/Dockerfile",
			isDir: false,
		},
		{
			name:  "error",
			path:  "error",
			isDir: false,
			err:   err,
		},
	}
	f := finder{makeWalkFunc(files)}
	_, findErr := f.Find(".")
	if findErr != err {
		t.Errorf("Did not get expected error: %v. Got: %v", err, findErr)
	}
}

func makeWalkFunc(files []file) func(string, filepath.WalkFunc) error {
	return func(dir string, walkFunc filepath.WalkFunc) error {
		skipping := ""
		for _, f := range files {
			if skipping != "" {
				if strings.HasPrefix(f.path, skipping) {
					continue
				} else {
					skipping = ""
				}
			}
			err := walkFunc(f.path, f, f.err)
			if err != nil {
				if err == filepath.SkipDir {
					skipping = f.path
				} else {
					return err
				}
			}
		}
		return nil
	}
}
