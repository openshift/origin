package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
)

// FixInjectionsWithRelativePath fixes the injections that does not specify the
// destination directory or the directory is relative to use the provided
// working directory.
func FixInjectionsWithRelativePath(workdir string, injections *api.InjectionList) {
	if len(*injections) == 0 {
		return
	}
	newList := api.InjectionList{}
	for _, injection := range *injections {
		changed := false
		if filepath.Clean(injection.DestinationDir) == "." {
			injection.DestinationDir = workdir
			changed = true
		}
		if !filepath.IsAbs(injection.DestinationDir) {
			injection.DestinationDir = filepath.Join(workdir, injection.DestinationDir)
			changed = true
		}
		if changed {
			glog.V(5).Infof("Using %q as a destination for injecting %q", injection.DestinationDir, injection.SourcePath)
		}
		newList = append(newList, injection)
	}
	*injections = newList
}

// ExpandInjectedFiles returns a flat list of all files that are injected into a
// container. All files from nested directories are returned in the list.
func ExpandInjectedFiles(injections api.InjectionList) ([]string, error) {
	result := []string{}
	for _, s := range injections {
		info, err := os.Stat(s.SourcePath)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("the %q must be a valid directory", s.SourcePath)
		}
		err = filepath.Walk(s.SourcePath, func(path string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if f.IsDir() {
				return nil
			}
			newPath := filepath.Join(s.DestinationDir, strings.TrimPrefix(path, s.SourcePath))
			result = append(result, newPath)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// CreateInjectedFilesRemovalScript creates a shell script that contains truncation
// of all files we injected into the container. The path to the script is returned.
// When the scriptName is provided, it is also truncated together with all
// secrets.
func CreateInjectedFilesRemovalScript(files []string, scriptName string) (string, error) {
	rmScript := "set -e\n"
	for _, s := range files {
		rmScript += fmt.Sprintf("truncate -s0 %q\n", s)
	}

	f, err := ioutil.TempFile("", "s2i-injection-remove")
	if err != nil {
		return "", err
	}
	if len(scriptName) > 0 {
		rmScript += fmt.Sprintf("truncate -s0 %q\n", scriptName)
	}
	rmScript += "set +e\n"
	err = ioutil.WriteFile(f.Name(), []byte(rmScript), 0700)
	return f.Name(), err
}
