/*
Copyright 2014 Google Inc. All rights reserved.

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

// A set of common functions needed by cmd/kubectl and pkg/kubectl packages.
package kubectl

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"reflect"
	"strings"

	api "github.com/openshift/origin/pkg/api2"
	client "github.com/openshift/origin/pkg/client2"
	"github.com/openshift/origin/pkg/labels"
	version "github.com/openshift/origin/pkg/version2"
)

var apiVersionToUse = "v1beta1"

func GetKubeClient(config *client.Config, matchVersion bool) (*client.Client, error) {
	// TODO: get the namespace context when kubectl ns is completed
	c, err := client.New(config)
	if err != nil {
		return nil, err
	}

	if matchVersion {
		clientVersion := version.Get()
		serverVersion, err := c.ServerVersion()
		if err != nil {
			return nil, fmt.Errorf("Couldn't read version from server: %v\n", err)
		}
		if s := *serverVersion; !reflect.DeepEqual(clientVersion, s) {
			return nil, fmt.Errorf("Server version (%#v) differs from client version (%#v)!\n", s, clientVersion)
		}
	}

	return c, nil
}

type AuthInfo struct {
	User        string
	Password    string
	CAFile      string
	CertFile    string
	KeyFile     string
	BearerToken string
	Insecure    *bool
}

// LoadAuthInfo parses an AuthInfo object from a file path. It prompts user and creates file if it doesn't exist.
func LoadAuthInfo(path string, r io.Reader) (*AuthInfo, error) {
	var auth AuthInfo
	if _, err := os.Stat(path); os.IsNotExist(err) {
		auth.User = promptForString("Username", r)
		auth.Password = promptForString("Password", r)
		data, err := json.Marshal(auth)
		if err != nil {
			return &auth, err
		}
		err = ioutil.WriteFile(path, data, 0600)
		return &auth, err
	}
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &auth)
	if err != nil {
		return nil, err
	}
	return &auth, err
}

func promptForString(field string, r io.Reader) string {
	fmt.Printf("Please enter %s: ", field)
	var result string
	fmt.Fscan(r, &result)
	return result
}

// TODO Move to labels package.
func formatLabels(labelMap map[string]string) string {
	l := labels.Set(labelMap).String()
	if l == "" {
		l = "<none>"
	}
	return l
}

func makeImageList(manifest api.ContainerManifest) string {
	var images []string
	for _, container := range manifest.Containers {
		images = append(images, container.Image)
	}
	return strings.Join(images, ",")
}
