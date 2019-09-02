// Copyright 2015 CNI authors
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

package main

import (
	"context"
	"crypto/sha512"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/libcni"
)

const (
	EnvCNIPath        = "CNI_PATH"
	EnvNetDir         = "NETCONFPATH"
	EnvCapabilityArgs = "CAP_ARGS"
	EnvCNIArgs        = "CNI_ARGS"
	EnvCNIIfname      = "CNI_IFNAME"

	DefaultNetDir = "/etc/cni/net.d"

	CmdAdd   = "add"
	CmdCheck = "check"
	CmdDel   = "del"
)

func parseArgs(args string) ([][2]string, error) {
	var result [][2]string

	pairs := strings.Split(args, ";")
	for _, pair := range pairs {
		kv := strings.Split(pair, "=")
		if len(kv) != 2 || kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("invalid CNI_ARGS pair %q", pair)
		}

		result = append(result, [2]string{kv[0], kv[1]})
	}

	return result, nil
}

func main() {
	if len(os.Args) < 4 {
		usage()
		return
	}

	netdir := os.Getenv(EnvNetDir)
	if netdir == "" {
		netdir = DefaultNetDir
	}
	netconf, err := libcni.LoadConfList(netdir, os.Args[2])
	if err != nil {
		exit(err)
	}

	var capabilityArgs map[string]interface{}
	capabilityArgsValue := os.Getenv(EnvCapabilityArgs)
	if len(capabilityArgsValue) > 0 {
		if err = json.Unmarshal([]byte(capabilityArgsValue), &capabilityArgs); err != nil {
			exit(err)
		}
	}

	var cniArgs [][2]string
	args := os.Getenv(EnvCNIArgs)
	if len(args) > 0 {
		cniArgs, err = parseArgs(args)
		if err != nil {
			exit(err)
		}
	}

	ifName, ok := os.LookupEnv(EnvCNIIfname)
	if !ok {
		ifName = "eth0"
	}

	netns := os.Args[3]
	netns, err = filepath.Abs(netns)
	if err != nil {
		exit(err)
	}

	// Generate the containerid by hashing the netns path
	s := sha512.Sum512([]byte(netns))
	containerID := fmt.Sprintf("cnitool-%x", s[:10])

	cninet := libcni.NewCNIConfig(filepath.SplitList(os.Getenv(EnvCNIPath)), nil)

	rt := &libcni.RuntimeConf{
		ContainerID:    containerID,
		NetNS:          netns,
		IfName:         ifName,
		Args:           cniArgs,
		CapabilityArgs: capabilityArgs,
	}

	switch os.Args[1] {
	case CmdAdd:
		result, err := cninet.AddNetworkList(context.TODO(), netconf, rt)
		if result != nil {
			_ = result.Print()
		}
		exit(err)
	case CmdCheck:
		err := cninet.CheckNetworkList(context.TODO(), netconf, rt)
		exit(err)
	case CmdDel:
		exit(cninet.DelNetworkList(context.TODO(), netconf, rt))
	}
}

func usage() {
	exe := filepath.Base(os.Args[0])

	fmt.Fprintf(os.Stderr, "%s: Add, check, or remove network interfaces from a network namespace\n", exe)
	fmt.Fprintf(os.Stderr, "  %s add   <net> <netns>\n", exe)
	fmt.Fprintf(os.Stderr, "  %s check <net> <netns>\n", exe)
	fmt.Fprintf(os.Stderr, "  %s del   <net> <netns>\n", exe)
	os.Exit(1)
}

func exit(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		os.Exit(1)
	}
	os.Exit(0)
}
