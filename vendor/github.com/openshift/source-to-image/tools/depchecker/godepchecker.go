package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// Import represents a Golang dependency
type Import struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
	Repo    string `yaml:"repo,omitempty"`
}

// Glide represents a glide yaml file
type Glide struct {
	Imports []Import `yaml:"imports"`
}

func readGlide(filename string) (map[string]Import, error) {
	deps := make(map[string]Import)

	f, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	glide := Glide{}
	err = yaml.Unmarshal(f, &glide)
	if err != nil {
		return nil, err
	}
	for _, dep := range glide.Imports {
		deps[dep.Name] = dep
	}
	return deps, nil
}

func main() {
	s2i, err := readGlide("glide.lock")
	if err != nil {
		fmt.Printf("error: can't read glide.lock: %v\n", err)
		os.Exit(1)
	}

	origin, err := readGlide("../origin/glide.lock")
	if err != nil {
		fmt.Printf("info: can't read ../origin/glide.lock: %v, not continuing\n", err)
		os.Exit(1)
	}

	code := 0
	for s2iDepName, s2idep := range s2i {
		origindep, found := origin[s2iDepName]
		if !found {
			if !strings.HasPrefix(s2iDepName, "github.com/openshift/origin") {
				fmt.Printf("warning: origin missing godep %s\n", s2iDepName)
				code = 1
			}
			continue
		}

		if origindep.Version != s2idep.Version {
			fmt.Printf("warning: differing glide dep(version)%s: origin %q vs s2i %q\n", s2iDepName, origindep.Version, s2idep.Version)
			code = 1
		}
		if origindep.Repo != s2idep.Repo {
			fmt.Printf("warning: differing glide dep(repo) %s: origin %q vs s2i %q\n", s2iDepName, origindep.Repo, s2idep.Repo)
			code = 1
		}
	}

	os.Exit(code)
}
