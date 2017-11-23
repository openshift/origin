package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Dependency represents a Golang dependency
type Dependency struct {
	ImportPath string
	Comment    string `json:",omitempty"`
	Rev        string
}

// Godeps represents a Godeps/Godeps.json file
type Godeps struct {
	ImportPath   string
	GoVersion    string
	GodepVersion string
	Packages     []string
	Deps         map[string]Dependency
}

// UnmarshalJSON unmarshals the contents of a Godeps/Godeps.json file
func (g *Godeps) UnmarshalJSON(data []byte) error {
	var v struct {
		ImportPath   string
		GoVersion    string
		GodepVersion string
		Packages     []string `json:",omitempty"`
		Deps         []Dependency
	}

	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}

	g.ImportPath = v.ImportPath
	g.GoVersion = v.GoVersion
	g.GodepVersion = v.GodepVersion
	g.Packages = v.Packages
	g.Deps = map[string]Dependency{}
	for _, dep := range v.Deps {
		g.Deps[dep.ImportPath] = dep
	}
	return nil
}

func readJSON(filename string) (*Godeps, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	godeps := &Godeps{}
	json.NewDecoder(f).Decode(godeps)
	return godeps, nil
}

func main() {
	s2i, err := readJSON("Godeps/Godeps.json")
	if err != nil {
		fmt.Printf("error: can't read Godeps/Godeps.json: %v\n", err)
		os.Exit(1)
	}

	origin, err := readJSON("../origin/Godeps/Godeps.json")
	if err != nil {
		fmt.Printf("info: can't read ../origin/Godeps/Godeps.json: %v, not continuing\n", err)
		return
	}

	code := 0
	for importPath, s2idep := range s2i.Deps {
		origindep, found := origin.Deps[importPath]
		if !found {
			if !strings.HasPrefix(importPath, "github.com/openshift/origin") {
				fmt.Printf("warning: origin missing godep %s\n", importPath)
				code = 1
			}
			continue
		}

		if origindep.Rev != s2idep.Rev {
			fmt.Printf("warning: differing godep %s: origin %q vs s2i %q\n", importPath, origindep.Rev, s2idep.Rev)
			code = 1
		}
	}

	os.Exit(code)
}
