// +build linux

package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	cniskel "github.com/containernetworking/cni/pkg/skel"
	cnitypes "github.com/containernetworking/cni/pkg/types"
	cni020 "github.com/containernetworking/cni/pkg/types/020"

	"github.com/openshift/origin/pkg/sdn/plugin/cniserver"
	utiltesting "k8s.io/client-go/util/testing"
)

var expectedResult cnitypes.Result
var generateError bool

func serverHandleCNI(request *cniserver.PodRequest) ([]byte, error) {
	if request.Command == cniserver.CNI_ADD {
		return json.Marshal(&expectedResult)
	} else if request.Command == cniserver.CNI_DEL {
		return nil, nil
	}
	return nil, fmt.Errorf("unhandled CNI command %v", request.Command)
}

const (
	CNI_COMMAND     string = "CNI_COMMAND"
	CNI_CONTAINERID string = "CNI_CONTAINERID"
	CNI_NETNS       string = "CNI_NETNS"
	CNI_IFNAME      string = "CNI_IFNAME"
	CNI_ARGS        string = "CNI_ARGS"
	CNI_PATH        string = "CNI_PATH"
)

func skelArgsToEnv(command cniserver.CNICommand, args *cniskel.CmdArgs) {
	os.Setenv(CNI_COMMAND, fmt.Sprintf("%v", command))
	os.Setenv(CNI_CONTAINERID, args.ContainerID)
	os.Setenv(CNI_NETNS, args.Netns)
	os.Setenv(CNI_IFNAME, args.IfName)
	os.Setenv(CNI_ARGS, args.Args)
	os.Setenv(CNI_PATH, args.Path)
}

func clearEnv() {
	for _, ev := range []string{CNI_COMMAND, CNI_CONTAINERID, CNI_NETNS, CNI_IFNAME, CNI_ARGS, CNI_PATH} {
		os.Unsetenv(ev)
	}
}

func TestOpenshiftSdnCNIPlugin(t *testing.T) {
	tmpDir, err := utiltesting.MkTmpdir("cniserver")
	if err != nil {
		t.Fatalf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "cni-server.sock")
	server := cniserver.NewCNIServer(path)
	if err := server.Start(serverHandleCNI); err != nil {
		t.Fatalf("error starting CNI server: %v", err)
	}

	cniPlugin := NewCNIPlugin(path)

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
		skelArgs    *cniskel.CmdArgs
		reqType     cniserver.CNICommand
		result      cnitypes.Result
		errorPrefix string
	}

	testcases := []testcase{
		// Normal ADD request
		{
			name:    "ADD",
			reqType: cniserver.CNI_ADD,
			skelArgs: &cniskel.CmdArgs{
				ContainerID: "adsfadsfasfdasdfasf",
				Netns:       "/path/to/something",
				IfName:      "eth0",
				Args:        "K8S_POD_NAMESPACE=awesome-namespace;K8S_POD_NAME=awesome-name",
				Path:        "/some/path",
				StdinData:   []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
			},
			result: expectedResult,
		},
		// Normal DEL request
		{
			name:    "DEL",
			reqType: cniserver.CNI_DEL,
			skelArgs: &cniskel.CmdArgs{
				ContainerID: "adsfadsfasfdasdfasf",
				Netns:       "/path/to/something",
				IfName:      "eth0",
				Args:        "K8S_POD_NAMESPACE=awesome-namespace;K8S_POD_NAME=awesome-name",
				Path:        "/some/path",
				StdinData:   []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
			},
		},
		// Missing args
		{
			name:    "NO ARGS",
			reqType: cniserver.CNI_ADD,
			skelArgs: &cniskel.CmdArgs{
				ContainerID: "adsfadsfasfdasdfasf",
				Netns:       "/path/to/something",
				IfName:      "eth0",
				Path:        "/some/path",
				StdinData:   []byte("{\"cniVersion\": \"0.1.0\",\"name\": \"openshift-sdn\",\"type\": \"openshift-sdn\"}"),
			},
			errorPrefix: "CNI request failed with status 400: 'invalid CNI_ARG",
		},
	}

	for _, tc := range testcases {
		var result cnitypes.Result
		var err error

		skelArgsToEnv(tc.reqType, tc.skelArgs)
		switch tc.reqType {
		case cniserver.CNI_ADD:
			result, err = cniPlugin.CmdAdd(tc.skelArgs)
		case cniserver.CNI_DEL:
			err = cniPlugin.CmdDel(tc.skelArgs)
		default:
			t.Fatalf("[%s] unhandled CNI command type", tc.name)
		}
		clearEnv()

		if tc.errorPrefix == "" {
			if tc.result != nil && !reflect.DeepEqual(result, tc.result) {
				t.Fatalf("[%s] expected result %v but got %v", tc.name, tc.result, result)
			}
		} else if !strings.HasPrefix(fmt.Sprintf("%v", err), tc.errorPrefix) {
			t.Fatalf("[%s] unexpected error message '%v'", tc.name, err)
		}
	}
}
