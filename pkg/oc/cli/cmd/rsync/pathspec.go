package rsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	kvalidation "k8s.io/kubernetes/pkg/apis/core/validation"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

// pathSpec represents a path (remote or local) given as a source or destination
// argument to the rsync command
type pathSpec struct {
	PodName string
	Path    string
}

// Local returns true if the path is a local machine path
func (s *pathSpec) Local() bool {
	return len(s.PodName) == 0
}

// RsyncPath returns a pathSpec in the form that can be used directly by the OS rsync command
func (s *pathSpec) RsyncPath() string {
	if len(s.PodName) > 0 {
		return fmt.Sprintf("%s:%s", s.PodName, s.Path)
	}
	if isWindows() {
		return convertWindowsPath(s.Path)
	}
	return s.Path
}

// Validate returns an error if the pathSpec is not valid.
func (s *pathSpec) Validate() error {
	if s.Local() {
		info, err := os.Stat(s.Path)
		if err != nil {
			return fmt.Errorf("invalid path %s: %v", s.Path, err)
		}
		if !info.IsDir() {
			return fmt.Errorf("path %s must point to a directory", s.Path)
		}
	}
	return nil
}

// isPathForPod receives a path and returns true
// if it matches a <podName>:/path format
func isPathForPod(path string) bool {
	parts := strings.SplitN(path, ":", 2)
	if len(parts) == 1 || (isWindows() && len(parts[0]) == 1) {
		return false
	}

	return true
}

// parsePathSpec parses a string argument into a pathSpec object
func parsePathSpec(path string) (*pathSpec, error) {
	parts := strings.SplitN(path, ":", 2)
	if !isPathForPod(path) {
		return &pathSpec{
			Path: path,
		}, nil
	}
	if reasons := kvalidation.ValidatePodName(parts[0], false); len(reasons) != 0 {
		return nil, fmt.Errorf("invalid pod name %s: %s", parts[0], strings.Join(reasons, ", "))
	}
	return &pathSpec{
		PodName: parts[0],
		Path:    parts[1],
	}, nil
}

// resolveResourceKindPath determines if a given path contains a resource
// formatted as resource/kind, and returns the resource name without the
// <kind/> segment, ensuring that the resource is of type Pod, if it exists.
func resolveResourceKindPath(f kcmdutil.Factory, path, namespace string) (string, error) {
	parts := strings.SplitN(path, ":", 2)
	if !isPathForPod(path) {
		return path, nil
	}

	podName := parts[0]

	// if the specified pod name is given in the <kind>/<name> format
	// validate the podName without the <kind/> segment.
	if podSegs := strings.Split(podName, "/"); len(podSegs) > 1 {
		podName = podSegs[1]
	}

	r := f.NewBuilder().
		Internal().
		NamespaceParam(namespace).
		SingleResourceType().
		ResourceNames("pods", podName).
		Do()

	if err := r.Err(); err != nil {
		return "", err
	}
	infos, err := r.Infos()
	if err != nil {
		return "", err
	}

	// if there were no errors, we should expect
	// one resource to exist
	if len(infos) == 0 || infos[0].Mapping.Resource != "pods" {
		return "", fmt.Errorf("error: expected resource to be of type pod, got %q", infos[0].Mapping.Resource)
	}

	return fmt.Sprintf("%s:%s", podName, parts[1]), nil
}

// convertWindowsPath converts a windows native path to a path that can be used by
// the rsync command in windows.
// It can take one of three forms:
// 1 - relative to current dir or relative to current drive
//     \mydir\subdir or subdir
//     For these, it's only sufficient to change '\' to '/'
// 2 - absolute path with drive
//     d:\mydir\subdir
//     These need to be converted to /cygdrive/<drive-letter>/rest/of/path
// 3 - UNC path
//     \\server\c$\mydir\subdir
//     For these it should be sufficient to change '\' to '/'
func convertWindowsPath(path string) string {
	// If the path starts with a single letter followed by a ":", it needs to
	// be converted /cygwin/<drive>/path form
	parts := strings.SplitN(path, ":", 2)
	if len(parts) > 1 && len(parts[0]) == 1 {
		return fmt.Sprintf("/cygdrive/%s/%s", strings.ToLower(parts[0]), strings.TrimPrefix(filepath.ToSlash(parts[1]), "/"))
	}
	return filepath.ToSlash(path)
}
