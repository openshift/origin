package graph

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
)

var (
	openshiftDefaultExcludes = []string{
		"github.com/openshift/origin/images",
		"github.com/openshift/origin/pkg/build/builder",

		"github.com/openshift/origin/cmd/cluster-capacity",
		"github.com/openshift/origin/cmd/service-catalog",
	}

	openshiftImportPath = "github.com/openshift/origin"
)

type openShiftRepoInfo struct {
	Dir string
}

// getOpenShiftExcludes returns a list of known
// OpenShift-specific paths to exclude
func getOpenShiftExcludes() []string {
	return openshiftDefaultExcludes
}

// getOpenShiftFilters returns a list of known
// OpenShift-specific paths to use as filters
func getOpenShiftFilters() ([]string, error) {
	result := bytes.NewBuffer([]byte{})

	// obtain path to Origin repo
	goList := exec.Command("go", "list", "--json", "-f", "'{{ .Dir }}'", openshiftImportPath)
	goList.Stdout = result
	goList.Stderr = os.Stderr

	if err := goList.Run(); err != nil {
		return nil, err
	}

	info := &openShiftRepoInfo{}
	if err := json.Unmarshal(result.Bytes(), &info); err != nil {
		return nil, err
	}

	filters, err := listDirsN([]string{"pkg"}, 1, info.Dir, "")
	if err != nil {
		return nil, err
	}

	k8s, err := listDirsN([]string{"vendor/k8s.io"}, 1, info.Dir, "")
	if err != nil {
		return nil, err
	}
	filters = append(filters, k8s...)

	gopkg, err := listDirsN([]string{"vendor/gopkg.in"}, 1, info.Dir, "")
	if err != nil {
		return nil, err
	}
	filters = append(filters, gopkg...)

	github, err := listDirsN([]string{"vendor/github.com"}, 2, info.Dir, "")
	if err != nil {
		return nil, err
	}
	filters = append(filters, github...)

	vendor, err := listDirsN([]string{"vendor"}, 1, info.Dir, "")
	if err != nil {
		return nil, err
	}

	// filter out vendor pkgs we have expanded
	skip := map[string]bool{
		"vendor/k8s.io":     true,
		"vendor/gopkg.in":   true,
		"vendor/github.com": true,
	}
	for _, v := range vendor {
		if _, shouldSkip := skip[v]; shouldSkip {
			continue
		}
		filters = append(filters, v)
	}

	originFilters := []string{}
	for _, f := range filters {
		originFilters = append(originFilters, path.Join(openshiftImportPath, f))
	}

	return originFilters, nil
}

// listDirsN receives a list of directory names and returns
// up to N levels of nested directories for each one
func listDirsN(dirs []string, N int, root, prefix string) ([]string, error) {
	if N <= 0 {
		return dirs, nil
	}

	allDirs := []string{}
	for _, dir := range dirs {
		files, err := ioutil.ReadDir(path.Join(root, prefix, dir))
		if err != nil {
			return nil, err
		}

		childDirs := []string{}
		for _, f := range files {
			if !f.IsDir() {
				continue
			}

			childDirs = append(childDirs, f.Name())
		}

		nested, err := listDirsN(childDirs, N-1, root, path.Join(prefix, dir))
		if err != nil {
			return nil, err
		}

		for _, n := range nested {
			allDirs = append(allDirs, path.Join(dir, n))
		}
	}

	return allDirs, nil
}
