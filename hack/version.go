package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
)

type Godep struct {
	Deps []Dep
}

type Dep struct {
	ImportPath string
	Rev        string
}

func main() {
	if len(os.Args[1:]) != 2 {
		fmt.Fprintf(os.Stderr, "Expects two arguments, a path to the Godep.json file and a package to get the commit for\n")
		os.Exit(1)
	}

	path := os.Args[1]
	pkg := os.Args[2]

	data, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read %s: %v\n", path, err)
		os.Exit(1)
	}
	godeps := &Godep{}
	if err := json.Unmarshal(data, godeps); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to read %s: %v\n", path, err)
		os.Exit(1)
	}

	for _, dep := range godeps.Deps {
		if dep.ImportPath != pkg {
			continue
		}
		if len(dep.Rev) > 7 {
			dep.Rev = dep.Rev[0:7]
		}
		fmt.Fprintf(os.Stdout, dep.Rev)
		return
	}

	fmt.Fprintf(os.Stderr, "Could not find %s in %s\n", pkg, path)
	os.Exit(1)
}
