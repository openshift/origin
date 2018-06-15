package scmauth

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/util/file"
)

func secretDir(t *testing.T, files ...string) string {
	dir, err := ioutil.TempDir("", "test")
	if err != nil {
		t.Fatalf("error creating temp dir: %v", err)
	}
	for _, f := range files {
		err := ioutil.WriteFile(filepath.Join(dir, f), []byte("test"), 0600)
		if err != nil {
			t.Fatalf("error creating test file: %v", err)
		}
	}
	return dir
}

func cleanupConfig(config string) {
	if len(config) == 0 {
		return
	}
	lines, err := file.ReadLines(config)
	if err != nil {
		// abort cleanup on error
		return
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "path = ") {
			continue
		}
		cleanupDir(strings.TrimPrefix(line, "path = "))
	}
	cleanupDir(filepath.Dir(config))
}

func cleanupDir(path string) {
	os.RemoveAll(path)
}

func validateConfig(t *testing.T, config string, search string) {
	if len(config) == 0 {
		return
	}
	lines, err := file.ReadLines(config)
	if err != nil {
		t.Fatalf("cannot read file %s: %v", config, err)
	}
	for _, line := range lines {
		if !strings.HasPrefix(line, "path = ") {
			continue
		}
		includedConfig := strings.TrimPrefix(line, "path = ")
		validateConfigContent(t, includedConfig, search)
	}
}

func validateConfigContent(t *testing.T, config string, search string) {
	lines, err := file.ReadLines(config)
	if err != nil {
		t.Fatalf("cannot read file %s: %v", config, err)
	}
	for _, line := range lines {
		if strings.Contains(line, search) {
			// search string was found. Test was successful
			return
		}
	}

	t.Errorf("Could not find search string %q in config file %s", search, config)
}
