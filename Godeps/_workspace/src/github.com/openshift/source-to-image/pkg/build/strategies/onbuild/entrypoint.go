package onbuild

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"

	"github.com/golang/glog"
)

var validEntrypoints = []*regexp.Regexp{
	regexp.MustCompile(`^run(\.sh)?$`),
	regexp.MustCompile(`^start(\.sh)?$`),
	regexp.MustCompile(`^exec(cute)?(\.sh)?$`),
}

// GuessEntrypoint tries to guess the valid entrypoint from the source code
// repository. The valid entrypoints are defined above (run,start,exec,execute)
func GuessEntrypoint(sourceDir string) (string, error) {
	files, err := ioutil.ReadDir(sourceDir)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if f.IsDir() || !f.Mode().IsRegular() {
			continue
		}
		if isValidEntrypoint(filepath.Join(sourceDir, f.Name())) {
			glog.V(2).Infof("Found valid ENTRYPOINT: %s", f.Name())
			return f.Name(), nil
		}
	}
	return "", errors.New("No valid entrypoint specified")
}

// isValidEntrypoint checks if the given file exists and if it is a regular
// file. Valid ENTRYPOINT must be an executable file, so the executable bit must
// be set.
func isValidEntrypoint(path string) bool {
	stat, err := os.Stat(path)
	if err != nil {
		return false
	}
	found := false
	for _, pattern := range validEntrypoints {
		if pattern.MatchString(stat.Name()) {
			found = true
			break
		}
	}
	if !found {
		return false
	}
	mode := stat.Mode()
	return mode&0111 != 0
}
