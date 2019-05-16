package watchdog

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
)

func TestReadLink(t *testing.T) {
	tests := []struct {
		name       string
		evalResult func(string, string, string, *testing.T)
		preRun     func(t *testing.T) (target, linkPath, dirName string)
		postRun    func(linkPath, dirName string, t *testing.T)
	}{
		{
			name: "target exists",
			evalResult: func(target, link, result string, t *testing.T) {
				if result != target {
					t.Errorf("expected %q to match %q", result, target)
				}
			},
			preRun: func(t *testing.T) (string, string, string) {
				tmpDir, err := ioutil.TempDir("", "existing")
				if err != nil {
					t.Fatalf("unable to create temp dir: %v", err)
				}
				if err := ioutil.WriteFile(filepath.Join(tmpDir, "testfile"), []byte{1}, os.ModePerm); err != nil {
					t.Fatalf("unable to write file: %v", err)
				}
				if err := os.Symlink(filepath.Join(tmpDir, "testfile"), filepath.Join(tmpDir, "newfile")); err != nil {
					t.Fatalf("unable to make symlink: %v", err)
				}
				return filepath.Join(tmpDir, "testfile"), filepath.Join(tmpDir, "newfile"), tmpDir
			},
			postRun: func(linkPath, dirName string, t *testing.T) {
				if err := os.RemoveAll(dirName); err != nil {
					t.Fatalf("unable to remove %q: %v", dirName, err)
				}
			},
		},
		{
			name: "target does not exists",
			evalResult: func(target, link, result string, t *testing.T) {
				if result != target {
					t.Errorf("expected %q to match %q", result, target)
				}
			},
			preRun: func(t *testing.T) (string, string, string) {
				tmpDir, err := ioutil.TempDir("", "broken")
				if err != nil {
					t.Fatalf("unable to create temp dir: %v", err)
				}
				if err := os.Symlink(filepath.Join(tmpDir, "testfile"), filepath.Join(tmpDir, "newfile")); err != nil {
					t.Fatalf("unable to make symlink: %v", err)
				}
				return filepath.Join(tmpDir, "testfile"), filepath.Join(tmpDir, "newfile"), tmpDir
			},
			postRun: func(linkPath, dirName string, t *testing.T) {
				if err := os.RemoveAll(dirName); err != nil {
					t.Fatalf("unable to remove %q: %v", dirName, err)
				}
			},
		},
		{
			name: "source does not exists",
			evalResult: func(target, link, result string, t *testing.T) {
				if len(result) > 0 {
					t.Errorf("expected result be empty, got: %q", result)
				}
			},
			preRun: func(t *testing.T) (string, string, string) {
				tmpDir, err := ioutil.TempDir("", "broken-source")
				if err != nil {
					t.Fatalf("unable to create temp dir: %v", err)
				}
				return filepath.Join(tmpDir, "testfile"), filepath.Join(tmpDir, "newfile"), tmpDir
			},
			postRun: func(linkPath, dirName string, t *testing.T) {
				if err := os.RemoveAll(dirName); err != nil {
					t.Fatalf("unable to remove %q: %v", dirName, err)
				}
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			target, link, tempDir := test.preRun(t)
			result := readlink(link)
			test.evalResult(target, link, result, t)
			test.postRun(link, tempDir, t)
		})
	}
}
