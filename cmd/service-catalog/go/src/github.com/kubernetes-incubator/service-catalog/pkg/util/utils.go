/*
Copyright 2016 The Kubernetes Authors.

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

package util

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"

	"github.com/golang/glog"
)

// WriteResponse will serialize 'object' to the HTTP ResponseWriter
// using the 'code' as the HTTP status code
func WriteResponse(w http.ResponseWriter, code int, object interface{}) {
	data, err := json.Marshal(object)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(code)
	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, string(data))
}

// BodyToObject will convert the incoming HTTP request into the
// passed in 'object'
func BodyToObject(r *http.Request, object interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, object)
	if err != nil {
		return err
	}

	return nil
}

// ResponseBodyToObject will reading the HTTP response into the
// passed in 'object'
func ResponseBodyToObject(r *http.Response, object interface{}) error {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}
	glog.Info(string(body))

	err = json.Unmarshal(body, object)
	if err != nil {
		return err
	}

	return nil
}

// ExecCmd executes a command and returns the stdout + error, if any
func ExecCmd(cmd string) (string, error) {
	fmt.Println("command: " + cmd)

	parts := strings.Fields(cmd)
	head := parts[0]
	parts = parts[1:]

	out, err := exec.Command(head, parts...).CombinedOutput()
	if err != nil {
		fmt.Printf("Command failed with: %s\n", err)
		fmt.Printf("Output: %s\n", out)
		return "", err
	}
	return string(out), nil
}

// Fetch will do an HTTP GET to the passed in URL, returning
// HTTP Body of the response or any error
func Fetch(u string) (string, error) {
	fmt.Printf("Fetching: %s\n", u)
	resp, err := http.Get(u)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// FetchObject will do an HTTP GET to the passed in URL, returning
// the response parsed as a JSON object, or any error
func FetchObject(u string, object interface{}) error {
	r, err := http.Get(u)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, object)
	if err != nil {
		return err
	}
	return nil
}
