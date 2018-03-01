/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"text/template"

	"github.com/kubernetes-incubator/service-catalog/cmd/svcat/plugin"
	"github.com/kubernetes-incubator/service-catalog/internal/test"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var catalogRequestRegex = regexp.MustCompile("/apis/servicecatalog.k8s.io/v1beta1/(.*)")

func TestCommandValidation(t *testing.T) {
	testcases := []struct {
		name      string // Test Name
		cmd       string // Command to run
		wantError string // Substring that should be present in the error, empty indicates no error
	}{
		{"viper bug workaround: provision", "provision name --class class --plan plan", ""},
		{"viper bug workaround: bind", "bind name", ""},
		{"describe broker requires name", "describe broker", "name is required"},
		{"describe class requires name", "describe class", "name or uuid is required"},
		{"describe plan requires name", "describe plan", "name or uuid is required"},
		{"describe instance requires name", "describe instance", "name is required"},
		{"describe binding requires name", "describe binding", "name is required"},
		{"unbind requires arg", "unbind", "instance or binding name is required"},
		{"sync requires names", "sync broker", "name is required"},
		{"deprovision requires name", "deprovision", "name is required"},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			validateCommand(t, tc.cmd, tc.wantError)
		})
	}
}

func TestCommandOutput(t *testing.T) {
	testcases := []struct {
		name            string // Test Name
		cmd             string // Command to run
		golden          string // Relative path to a golden file, compared to the command output
		continueOnError bool   // Should the test stop immediately if the command fails or continue and capture the console output
	}{
		{name: "list all brokers", cmd: "get brokers", golden: "output/get-brokers.txt"},
		{name: "get broker", cmd: "get broker ups-broker", golden: "output/get-broker.txt"},
		{name: "describe broker", cmd: "describe broker ups-broker", golden: "output/describe-broker.txt"},

		{name: "list all classes", cmd: "get classes", golden: "output/get-classes.txt"},
		{name: "get class by name", cmd: "get class user-provided-service", golden: "output/get-class.txt"},
		{name: "get class by uuid", cmd: "get class --uuid 4f6e6cf6-ffdd-425f-a2c7-3c9258ad2468", golden: "output/get-class.txt"},
		{name: "describe class by name", cmd: "describe class user-provided-service", golden: "output/describe-class.txt"},
		{name: "describe class uuid", cmd: "describe class --uuid 4f6e6cf6-ffdd-425f-a2c7-3c9258ad2468", golden: "output/describe-class.txt"},

		{name: "list all plans", cmd: "get plans", golden: "output/get-plans.txt"},
		{name: "get plan by name", cmd: "get plan default", golden: "output/get-plan.txt"},
		{name: "get plan by uuid", cmd: "get plan --uuid 86064792-7ea2-467b-af93-ac9694d96d52", golden: "output/get-plan.txt"},
		{name: "get plan by class/plan name combo", cmd: "get plan user-provided-service/default", golden: "output/get-plan.txt"},
		{name: "describe plan by name", cmd: "describe plan default", golden: "output/describe-plan.txt"},
		{name: "describe plan by uuid", cmd: "describe plan --uuid 86064792-7ea2-467b-af93-ac9694d96d52", golden: "output/describe-plan.txt"},
		{name: "describe plan by class/plan name combo", cmd: "describe plan user-provided-service/default", golden: "output/describe-plan.txt"},
		{name: "describe plan with schemas", cmd: "describe plan premium", golden: "output/describe-plan-with-schemas.txt"},
		{name: "describe plan without schemas", cmd: "describe plan premium --show-schemas=false", golden: "output/describe-plan-without-schemas.txt"},

		{name: "list all instances", cmd: "get instances -n test-ns", golden: "output/get-instances.txt"},
		{name: "get instance", cmd: "get instance ups-instance -n test-ns", golden: "output/get-instance.txt"},
		{name: "describe instance", cmd: "describe instance ups-instance -n test-ns", golden: "output/describe-instance.txt"},

		{name: "list all bindings", cmd: "get bindings -n test-ns", golden: "output/get-bindings.txt"},
		{name: "get binding", cmd: "get binding ups-binding -n test-ns", golden: "output/get-binding.txt"},
		{name: "describe binding", cmd: "describe binding ups-binding -n test-ns", golden: "output/describe-binding.txt"},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			output := executeCommand(t, tc.cmd, tc.continueOnError)
			test.AssertEqualsGoldenFile(t, tc.golden, output)
		})
	}
}

func TestGenerateManifest(t *testing.T) {
	svcat := buildRootCommand()

	m := &plugin.Manifest{}
	m.Load(svcat)

	got, err := yaml.Marshal(&m)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	test.AssertEqualsGoldenFile(t, "plugin.yaml", string(got))
}

// executeCommand runs a svcat command against a fake k8s api,
// returning the cli output.
func executeCommand(t *testing.T, cmd string, continueOnErr bool) string {
	// Fake the k8s api server
	apisvr := newAPIServer()
	defer apisvr.Close()

	// Generate a test kubeconfig pointing at the server
	kubeconfig, err := writeTestKubeconfig(apisvr.URL)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	defer os.Remove(kubeconfig)

	// Setup the svcat command
	svcat, _, err := buildCommand(cmd, kubeconfig)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	// Capture all output: stderr and stdout
	output := &bytes.Buffer{}
	svcat.SetOutput(output)

	err = svcat.Execute()
	if err != nil && !continueOnErr {
		t.Fatalf("%+v", err)
	}

	return output.String()
}

// validateCommand validates a svcat command arguments
func validateCommand(t *testing.T, cmd string, wantError string) {
	// Fake the k8s api server
	apisvr := newAPIServer()
	defer apisvr.Close()

	// Generate a test kubeconfig pointing at the server
	kubeconfig, err := writeTestKubeconfig(apisvr.URL)
	if err != nil {
		t.Fatalf("%+v", err)
	}
	defer os.Remove(kubeconfig)

	// Setup the svcat command
	svcat, targetCmd, err := buildCommand(cmd, kubeconfig)
	if err != nil {
		t.Fatalf("%+v", err)
	}

	// Skip running the actual command because we are only validating
	targetCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return nil
	}

	// Capture all output: stderr and stdout
	output := &bytes.Buffer{}
	svcat.SetOutput(output)

	err = svcat.Execute()
	if wantError == "" {
		if err != nil {
			t.Fatalf("%+v", err)
		}
	} else {
		gotError := ""
		if err != nil {
			gotError = err.Error()
		}
		if !strings.Contains(gotError, wantError) {
			t.Fatalf("unexpected error \n\nWANT:\n%q\n\nGOT:\n%q\n", wantError, gotError)
		}
	}
}

// buildCommand parses a command string.
func buildCommand(cmd, kubeconfig string) (rootCmd *cobra.Command, targetCmd *cobra.Command, err error) {
	rootCmd = buildRootCommand()
	args := strings.Split(cmd, " ")
	args = append(args, "--kubeconfig", kubeconfig)
	rootCmd.SetArgs(args)

	targetCmd, _, err = rootCmd.Find(args)

	return rootCmd, targetCmd, err
}

func newAPIServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(apihandler))
}

// apihandler handles requests to the service catalog endpoint.
// When a request is received, it looks up the response from the testdata directory.
// Example:
// GET /apis/servicecatalog.k8s.io/v1beta1/clusterservicebrokers responds with testdata/clusterservicebrokers.json
func apihandler(w http.ResponseWriter, r *http.Request) {
	match := catalogRequestRegex.FindStringSubmatch(r.RequestURI)

	if len(match) == 0 {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("unexpected request %s %s", r.Method, r.RequestURI)))
		return
	}

	if r.Method != http.MethodGet {
		// Anything more interesting than a GET, i.e. it relies upon server behavior
		// probably should be an integration test instead
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("unexpected request %s %s", r.Method, r.RequestURI)))
		return
	}

	relpath, err := url.PathUnescape(match[1])
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("could not unescape path %s (%s)", match[1], err)))
		return
	}
	_, response, err := test.GetTestdata(filepath.Join("responses", relpath+".json"))
	if err != nil {
		w.WriteHeader(500)
		w.Write([]byte(fmt.Sprintf("unexpected request %s with no matching testdata (%s)", r.RequestURI, err)))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(response)
}

func writeTestKubeconfig(fakeURL string) (string, error) {
	_, configT, err := test.GetTestdata("kubeconfig.tmpl.yaml")
	if err != nil {
		return "", err
	}

	data := map[string]string{
		"Server": fakeURL,
	}
	t := template.Must(template.New("kubeconfig").Parse(string(configT)))

	f, err := ioutil.TempFile("", "kubeconfig")
	if err != nil {
		return "", errors.Wrap(err, "unable to create a temporary kubeconfig file")
	}
	defer f.Close()

	err = t.Execute(f, data)
	return f.Name(), errors.Wrap(err, "error executing the kubeconfig template")
}
