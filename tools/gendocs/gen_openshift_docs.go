package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/openshift/origin/pkg/cmd/admin"
	"github.com/openshift/origin/pkg/cmd/cli"
	"github.com/openshift/origin/pkg/cmd/util/gendocs"
)

func OutDir(path string) (string, error) {
	outDir, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	stat, err := os.Stat(outDir)
	if err != nil {
		return "", err
	}

	if !stat.IsDir() {
		return "", fmt.Errorf("output directory %s is not a directory\n", outDir)
	}
	outDir = outDir + "/"
	return outDir, nil
}

func main() {
	path := "docs/generated/"
	if len(os.Args) == 2 {
		path = os.Args[1]
	} else if len(os.Args) > 2 {
		fmt.Fprintf(os.Stderr, "usage: %s [output directory]\n", os.Args[0])
		os.Exit(1)
	}

	outDir, err := OutDir(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to get output directory: %v\n", err)
		os.Exit(1)
	}

	outFile := outDir + "oc_by_example_content.adoc"
	out := os.Stdout
	cmd := cli.NewCommandCLI("oc", "oc", &bytes.Buffer{}, out, ioutil.Discard)
	gendocs.GenDocs(cmd, outFile)

	outFile = outDir + "oadm_by_example_content.adoc"
	cmd = admin.NewCommandAdmin("oadm", "oadm", &bytes.Buffer{}, ioutil.Discard, ioutil.Discard)
	gendocs.GenDocs(cmd, outFile)
}
