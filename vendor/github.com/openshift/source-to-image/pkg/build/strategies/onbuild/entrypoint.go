package onbuild

import (
	"errors"
	"path/filepath"
	"regexp"

	"github.com/openshift/source-to-image/pkg/util/fs"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
)

var glog = utilglog.StderrLog

var validEntrypoints = []*regexp.Regexp{
	regexp.MustCompile(`^run(\.sh)?$`),
	regexp.MustCompile(`^start(\.sh)?$`),
	regexp.MustCompile(`^exec(\.sh)?$`),
	regexp.MustCompile(`^execute(\.sh)?$`),
}

// GuessEntrypoint tries to guess the valid entrypoint from the source code
// repository. The valid entrypoints are defined above (run,start,exec,execute)
func GuessEntrypoint(fs fs.FileSystem, sourceDir string) (string, error) {
	files, err := fs.ReadDir(sourceDir)
	if err != nil {
		return "", err
	}
	for _, f := range files {
		if f.IsDir() || !f.Mode().IsRegular() {
			continue
		}
		if isValidEntrypoint(fs, filepath.Join(sourceDir, f.Name())) {
			glog.V(2).Infof("Found valid ENTRYPOINT: %s", f.Name())
			return f.Name(), nil
		}
	}
	return "", errors.New("no valid entrypoint specified")
}

// isValidEntrypoint checks if the given file exists and if it is a regular
// file. Valid ENTRYPOINT must be an executable file, so the executable bit must
// be set.
func isValidEntrypoint(fs fs.FileSystem, path string) bool {
	stat, err := fs.Stat(path)
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
