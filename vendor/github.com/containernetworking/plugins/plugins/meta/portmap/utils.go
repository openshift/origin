// Copyright 2017 CNI authors
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
	"crypto/sha512"
	"fmt"
	"net"

	"github.com/vishvananda/netlink"
)

const maxChainNameLength = 28

// fmtIpPort correctly formats ip:port literals for iptables and ip6tables -
// need to wrap v6 literals in a []
func fmtIpPort(ip net.IP, port int) string {
	if ip.To4() == nil {
		return fmt.Sprintf("[%s]:%d", ip.String(), port)
	}
	return fmt.Sprintf("%s:%d", ip.String(), port)
}

func localhostIP(isV6 bool) string {
	if isV6 {
		return "::1"
	}
	return "127.0.0.1"
}

// getRoutableHostIF will try and determine which interface routes the container's
// traffic. This is the one on which we disable martian filtering.
func getRoutableHostIF(containerIP net.IP) string {
	routes, err := netlink.RouteGet(containerIP)
	if err != nil {
		return ""
	}

	for _, route := range routes {
		link, err := netlink.LinkByIndex(route.LinkIndex)
		if err != nil {
			continue
		}

		return link.Attrs().Name
	}

	return ""
}

func formatChainName(prefix, name, id string) string {
	chainBytes := sha512.Sum512([]byte(name + id))
	chain := fmt.Sprintf("CNI-%s%x", prefix, chainBytes)
	return chain[:maxChainNameLength]
}
