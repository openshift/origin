package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v2"

	"github.com/go-openapi/loads"
	"github.com/openshift/origin/tools/genapidocs/apidocs"
)

func writeAPIDocs(root string) error {
	err := os.RemoveAll(root)
	if err != nil {
		return err
	}

	doc, err := loads.JSONSpec("api/swagger-spec/openshift-openapi-spec.json")
	if err != nil {
		return err
	}

	pages, err := apidocs.BuildPages(doc.Spec())
	if err != nil {
		return err
	}

	err = pages.Write(root)
	if err != nil {
		return err
	}

	topics := apidocs.BuildTopics(pages)

	b, err := yaml.Marshal(topics)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(root, "_topic_map.yml"), b, 0666)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "%s: usage: %[1]s root\n", os.Args[0])
		os.Exit(1)
	}
	err := writeAPIDocs(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s: %s\n", os.Args[0], err)
		os.Exit(1)
	}
}
