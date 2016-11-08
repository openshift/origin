package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/source-to-image/pkg/api"
)

// FixInjectionsWithRelativePath fixes the injections that does not specify the
// destination directory or the directory is relative to use the provided
// working directory.
func FixInjectionsWithRelativePath(workdir string, injections api.VolumeList) api.VolumeList {
	if len(injections) == 0 {
		return injections
	}
	newList := api.VolumeList{}
	for _, injection := range injections {
		changed := false
		if filepath.Clean(injection.Destination) == "." {
			injection.Destination = workdir
			changed = true
		}
		if !filepath.IsAbs(injection.Destination) {
			injection.Destination = filepath.Join(workdir, injection.Destination)
			changed = true
		}
		if changed {
			glog.V(5).Infof("Using %q as a destination for injecting %q", injection.Destination, injection.Source)
		}
		newList = append(newList, injection)
	}
	return newList
}

// ExpandInjectedFiles returns a flat list of all files that are injected into a
// container. All files from nested directories are returned in the list.
func ExpandInjectedFiles(injections api.VolumeList) ([]string, error) {
	result := []string{}
	for _, s := range injections {
		if _, err := os.Stat(s.Source); err != nil {
			return nil, err
		}
		err := filepath.Walk(s.Source, func(path string, f os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			// Detected files will be truncated. k8s' AtomicWriter creates
			// directories and symlinks to directories in order to inject files.
			// An attempt to truncate either a dir or symlink to a dir will fail.
			// Thus, we need to dereference symlinks to see if they might point
			// to a directory.
			// Do not try to simplify this logic to simply return nil if a symlink
			// is detected. During the tar transfer to an assemble image, symlinked
			// files are turned concrete (i.e. they will be turned into regular files
			// containing the content of their target). These newly concrete files
			// need to be truncated as well.

			if f.Mode()&os.ModeSymlink != 0 {
				linkDest, err := filepath.EvalSymlinks(path)
				if err != nil {
					return fmt.Errorf("Unable to evaluate symlink [%v]: %v", path, err)
				}
				// Evaluate the destination of the link.
				f, err = os.Lstat(linkDest)
				if err != nil {
					// This is not a fatal error. If AtomicWrite tried multiple times, a symlink might not point
					// to a valid destination.
					glog.Warningf("Unable to lstat symlink destination [%v]->[%v]. Partial atomic write?", path, linkDest, err)
					return nil
				}
			}

			if f.IsDir() {
				return nil
			}

			newPath := filepath.Join(s.Destination, strings.TrimPrefix(path, s.Source))
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

// HandleInjectionError handles the error caused by injection and provide
// reasonable suggestion to users.
func HandleInjectionError(p api.VolumeSpec, err error) error {
	if err == nil {
		return nil
	}
	if strings.Contains(err.Error(), "no such file or directory") {
		glog.Errorf("The destination directory for %q injection must exist in container (%q)", p.Source, p.Destination)
		return err
	}
	glog.Errorf("Error occurred during injecting %q to %q: %v", p.Source, p.Destination, err)
	return err
}
