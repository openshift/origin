// +build linux

package cniserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	utiltesting "k8s.io/client-go/util/testing"

	cnitypes "github.com/containernetworking/cni/pkg/types"
	cni020 "github.com/containernetworking/cni/pkg/types/020"
)

func clientDoCNI(t *testing.T, client *http.Client, req *CNIRequest) ([]byte, int) {
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal CNI request %v: %v", req, err)
	}

	url := fmt.Sprintf("http://dummy/")
	resp, err := client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to send CNI request: %v", err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read CNI request response body: %v", err)
	}
	return body, resp.StatusCode
}

var expectedResult cnitypes.Result

func serverHandleCNI(request *PodRequest) ([]byte, error) {
	if request.Command == CNI_ADD {
		return json.Marshal(&expectedResult)
	} else if request.Command == CNI_DEL {
		return nil, nil
	} else if request.Command == CNI_UPDATE {
		return nil, nil
	}
	return nil, fmt.Errorf("unhandled CNI command %v", request.Command)
}

func TestCNIServer(t *testing.T) {
	tmpDir, err := utiltesting.MkTmpdir("cniserver")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)
	socketPath := filepath.Join(tmpDir, CNIServerSocketName)

	s := NewCNIServer(tmpDir, &Config{MTU: 1500})
	if err := s.Start(serverHandleCNI); err != nil {
		t.Fatalf("error starting CNI server: %v", err)
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(proto, addr string) (net.Conn, error) {
				return net.Dial("unix", socketPath)
			},
		},
	}

	expectedIP, expectedNet, _ := net.ParseCIDR("10.0.0.2/24")
	expectedResult = &cni020.Result{
		IP4: &cni020.IPConfig{
			IP: net.IPNet{
				IP:   expectedIP,
				Mask: expectedNet.Mask,
			},
		},
	}

	type testcase struct {
		name        string
		request     *CNIRequest
		result      cnitypes.Result
		errorPrefix string
	}

	testcases := []testcase{
		// Normal ADD request
		{
			name: "ADD",
			request: &CNIRequest{
				Env: map[string]string{
					"CNI_COMMAND":     string(CNI_ADD),
					"CNI_CONTAINERID": "adsfadsfasfdasdfasf",
					"CNI_NETNS":       "/path/to/something",
					"CNI_ARGS":        "K8S_POD_NAMESPACE=awesome-namespace;K8S_POD_NAME=awesome-name",
				},
				Config:   []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
				HostVeth: "vethABC",
			},
			result: expectedResult,
		},
		// Normal DEL request
		{
			name: "DEL",
			request: &CNIRequest{
				Env: map[string]string{
					"CNI_COMMAND":     string(CNI_DEL),
					"CNI_CONTAINERID": "adsfadsfasfdasdfasf",
					"CNI_NETNS":       "/path/to/something",
					"CNI_ARGS":        "K8S_POD_NAMESPACE=awesome-namespace;K8S_POD_NAME=awesome-name",
				},
				Config: []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
			},
			result: nil,
		},
		// Normal UPDATE request
		{
			name: "UPDATE",
			request: &CNIRequest{
				Env: map[string]string{
					"CNI_COMMAND":     string(CNI_UPDATE),
					"CNI_CONTAINERID": "adsfadsfasfdasdfasf",
					"CNI_NETNS":       "/path/to/something",
					"CNI_ARGS":        "K8S_POD_NAMESPACE=awesome-namespace;K8S_POD_NAME=awesome-name",
				},
				Config: []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
			},
			result: nil,
		},
		// Missing CNI_ARGS
		{
			name: "ARGS1",
			request: &CNIRequest{
				Env: map[string]string{
					"CNI_COMMAND":     string(CNI_ADD),
					"CNI_CONTAINERID": "adsfadsfasfdasdfasf",
					"CNI_NETNS":       "/path/to/something",
				},
				Config:   []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
				HostVeth: "vethABC",
			},
			result:      nil,
			errorPrefix: "missing CNI_ARGS",
		},
		// Missing CNI_NETNS
		{
			name: "ARGS2",
			request: &CNIRequest{
				Env: map[string]string{
					"CNI_COMMAND":     string(CNI_ADD),
					"CNI_CONTAINERID": "adsfadsfasfdasdfasf",
					"CNI_ARGS":        "K8S_POD_NAMESPACE=awesome-namespace;K8S_POD_NAME=awesome-name",
				},
				Config:   []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
				HostVeth: "vethABC",
			},
			result:      nil,
			errorPrefix: "missing CNI_NETNS",
		},
		// Missing CNI_COMMAND
		{
			name: "ARGS3",
			request: &CNIRequest{
				Env: map[string]string{
					"CNI_CONTAINERID": "adsfadsfasfdasdfasf",
					"CNI_NETNS":       "/path/to/something",
					"CNI_ARGS":        "K8S_POD_NAMESPACE=awesome-namespace;K8S_POD_NAME=awesome-name",
				},
				Config:   []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
				HostVeth: "vethABC",
			},
			result:      nil,
			errorPrefix: "unexpected or missing CNI_COMMAND",
		},
		// Missing HostVeth
		{
			name: "ARGS4",
			request: &CNIRequest{
				Env: map[string]string{
					"CNI_COMMAND":     string(CNI_ADD),
					"CNI_CONTAINERID": "adsfadsfasfdasdfasf",
					"CNI_NETNS":       "/path/to/something",
					"CNI_ARGS":        "K8S_POD_NAMESPACE=awesome-namespace;K8S_POD_NAME=awesome-name",
				},
				Config: []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
			},
			result:      nil,
			errorPrefix: "missing HostVeth",
		},
	}

	for _, tc := range testcases {
		body, code := clientDoCNI(t, client, tc.request)
		if tc.errorPrefix == "" {
			if code != http.StatusOK {
				t.Fatalf("[%s] expected status %v but got %v", tc.name, http.StatusOK, code)
			}
			if tc.result != nil {
				result := &cni020.Result{}
				if err := json.Unmarshal(body, result); err != nil {
					t.Fatalf("[%s] failed to unmarshal response '%s': %v", tc.name, string(body), err)
				}
				if !reflect.DeepEqual(result, tc.result) {
					t.Fatalf("[%s] expected result %v but got %v", tc.name, tc.result, result)
				}
			}
		} else {
			if code != http.StatusBadRequest {
				t.Fatalf("[%s] expected status %v but got %v", tc.name, http.StatusBadRequest, code)
			}
			if !strings.HasPrefix(string(body), tc.errorPrefix) {
				t.Fatalf("[%s] unexpected error message '%v'", tc.name, string(body))
			}
		}
	}
}
