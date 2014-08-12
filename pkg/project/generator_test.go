package project

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
)

const projectExampleJSON = "./example/project.json"

var projectTempl Project

func TestTemplateUnmarshal(t *testing.T) {
	jsonFile, _ := ioutil.ReadFile(projectExampleJSON)
	err := json.Unmarshal(jsonFile, &projectTempl)
	if err != nil {
		t.Errorf("Unable to parse the sample project.json: %v", err)
	}
}

func TestProcessParameters(t *testing.T) {
	projectTempl.ProcessParameters()

	for _, p := range projectTempl.Parameters {
		if p.Value == "" {
			t.Errorf("Failed to process '%s' parameter", p.Name)
		}
		fmt.Printf("%s -> %s = %s\n", p.Name, p.Generate, p.Value)
	}
}

func TestSubstituteEnvValues(t *testing.T) {
	projectTempl.SubstituteEnvValues()

	for _, c := range projectTempl.Containers() {
		for _, e := range c.Env {
			if strings.Contains(string(e.Value), "${") {
				if e.Name != "FOO" {
					t.Errorf("Failed to substitute %s environment variable: %s", e.Name, e.Value)
				}
			}
			fmt.Printf("%s=%s\n", e.Name, e.Value)
		}
	}

	for _, s := range projectTempl.ServiceLinks {
		for _, e := range s.Export {
			if strings.Contains(string(e.Value), "${") {
				if e.Name != "FOO" {
					t.Errorf("Failed to substitute %s environment variable: %s", e.Name, e.Value)
				}
			}
			fmt.Printf("%s=%s\n", e.Name, e.Value)
		}
	}
}
