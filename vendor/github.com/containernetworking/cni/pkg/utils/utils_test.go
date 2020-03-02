// Copyright 2019 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils_test

import (
	"reflect"
	"testing"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/utils"
)

func TestValidateContainerID(t *testing.T) {
	testData := []struct {
		description string
		containerID string
		err         *types.Error
	}{
		{
			description: "empty containerID",
			containerID: "",
			err:         types.NewError(types.ErrUnknownContainer, "missing containerID", ""),
		},
		{
			description: "invalid characters in containerID",
			containerID: "1234%%%",
			err:         types.NewError(types.ErrInvalidEnvironmentVariables, "invalid characters in containerID", "1234%%%"),
		},
		{
			description: "normal containerID",
			containerID: "a51debf7e1eb",
			err:         nil,
		},
	}

	for _, tt := range testData {
		err := utils.ValidateContainerID(tt.containerID)
		if !reflect.DeepEqual(tt.err, err) {
			t.Errorf("Expected '%v' but got '%v'", tt.err, err)
		}
	}
}

func TestValidateNetworkName(t *testing.T) {
	testData := []struct {
		description string
		networkName string
		err         *types.Error
	}{
		{
			description: "empty networkName",
			networkName: "",
			err:         types.NewError(types.ErrInvalidNetworkConfig, "missing network name:", ""),
		},
		{
			description: "invalid characters in networkName",
			networkName: "1234%%%",
			err:         types.NewError(types.ErrInvalidNetworkConfig, "invalid characters found in network name", "1234%%%"),
		},
		{
			description: "normal networkName",
			networkName: "eth0",
			err:         nil,
		},
	}

	for _, tt := range testData {
		err := utils.ValidateNetworkName(tt.networkName)
		if !reflect.DeepEqual(tt.err, err) {
			t.Errorf("Expected '%v' but got '%v'", tt.err, err)
		}
	}
}

func TestValidateInterfaceName(t *testing.T) {
	testData := []struct {
		description   string
		interfaceName string
		err           *types.Error
	}{
		{
			description:   "empty interfaceName",
			interfaceName: "",
			err:           types.NewError(types.ErrInvalidEnvironmentVariables, "interface name is empty", ""),
		},
		{
			description:   "more than 16 characters in interfaceName",
			interfaceName: "testnamemorethan16",
			err:           types.NewError(types.ErrInvalidEnvironmentVariables, "interface name is too long", "interface name should be less than 16 characters"),
		},
		{
			description:   "interfaceName is .",
			interfaceName: ".",
			err:           types.NewError(types.ErrInvalidEnvironmentVariables, "interface name is . or ..", ""),
		},
		{
			description:   "interfaceName contains /",
			interfaceName: "/testname",
			err:           types.NewError(types.ErrInvalidEnvironmentVariables, "interface name contains / or : or whitespace characters", ""),
		},
		{
			description:   "interfaceName contains whitespace characters",
			interfaceName: "test name",
			err:           types.NewError(types.ErrInvalidEnvironmentVariables, "interface name contains / or : or whitespace characters", ""),
		},
		{
			description:   "normal interfaceName",
			interfaceName: "testname",
			err:           nil,
		},
	}

	for _, tt := range testData {
		err := utils.ValidateInterfaceName(tt.interfaceName)
		if !reflect.DeepEqual(tt.err, err) {
			t.Errorf("Expected '%v' but got '%v'", tt.err, err)
		}
	}
}
