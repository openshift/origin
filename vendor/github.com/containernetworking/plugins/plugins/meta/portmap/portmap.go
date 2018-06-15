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
	"fmt"
	"net"
	"strconv"

	"github.com/containernetworking/plugins/pkg/utils/sysctl"
	"github.com/coreos/go-iptables/iptables"
)

// This creates the chains to be added to iptables. The basic structure is
// a bit complex for efficiencies sake. We create 2 chains: a summary chain
// that is shared between invocations, and an invocation (container)-specific
// chain. This minimizes the number of operations on the top level, but allows
// for easy cleanup.
//
// We also create DNAT chains to rewrite destinations, and SNAT chains so that
// connections to localhost work.
//
// The basic setup (all operations are on the nat table) is:
//
// DNAT case (rewrite destination IP and port):
// PREROUTING, OUTPUT: --dst-type local -j CNI-HOSTPORT_DNAT
// CNI-HOSTPORT-DNAT: -j CNI-DN-abcd123
// CNI-DN-abcd123: -p tcp --dport 8080 -j DNAT --to-destination 192.0.2.33:80
// CNI-DN-abcd123: -p tcp --dport 8081 -j DNAT ...
//
// SNAT case (rewrite source IP from localhost after dnat):
// POSTROUTING: -s 127.0.0.1 ! -d 127.0.0.1 -j CNI-HOSTPORT-SNAT
// CNI-HOSTPORT-SNAT: -j CNI-SN-abcd123
// CNI-SN-abcd123: -p tcp -s 127.0.0.1 -d 192.0.2.33 --dport 80 -j MASQUERADE
// CNI-SN-abcd123: -p tcp -s 127.0.0.1 -d 192.0.2.33 --dport 90 -j MASQUERADE

// The names of the top-level summary chains.
// These should never be changed, or else upgrading will require manual
// intervention.
const TopLevelDNATChainName = "CNI-HOSTPORT-DNAT"
const TopLevelSNATChainName = "CNI-HOSTPORT-SNAT"

// forwardPorts establishes port forwarding to a given container IP.
// containerIP can be either v4 or v6.
func forwardPorts(config *PortMapConf, containerIP net.IP) error {
	isV6 := (containerIP.To4() == nil)

	var ipt *iptables.IPTables
	var err error
	var conditions *[]string

	if isV6 {
		ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv6)
		conditions = config.ConditionsV6
	} else {
		ipt, err = iptables.NewWithProtocol(iptables.ProtocolIPv4)
		conditions = config.ConditionsV4
	}
	if err != nil {
		return fmt.Errorf("failed to open iptables: %v", err)
	}

	toplevelDnatChain := genToplevelDnatChain()
	if err := toplevelDnatChain.setup(ipt, nil); err != nil {
		return fmt.Errorf("failed to create top-level DNAT chain: %v", err)
	}

	dnatChain := genDnatChain(config.Name, config.ContainerID, conditions)
	_ = dnatChain.teardown(ipt) // If we somehow collide on this container ID + network, cleanup

	dnatRules := dnatRules(config.RuntimeConfig.PortMaps, containerIP)
	if err := dnatChain.setup(ipt, dnatRules); err != nil {
		return fmt.Errorf("unable to setup DNAT: %v", err)
	}

	// Enable SNAT for connections to localhost.
	// This won't work for ipv6, since the kernel doesn't have the equvalent
	// route_localnet sysctl.
	if *config.SNAT && !isV6 {
		toplevelSnatChain := genToplevelSnatChain(isV6)
		if err := toplevelSnatChain.setup(ipt, nil); err != nil {
			return fmt.Errorf("failed to create top-level SNAT chain: %v", err)
		}

		snatChain := genSnatChain(config.Name, config.ContainerID)
		_ = snatChain.teardown(ipt)

		snatRules := snatRules(config.RuntimeConfig.PortMaps, containerIP)
		if err := snatChain.setup(ipt, snatRules); err != nil {
			return fmt.Errorf("unable to setup SNAT: %v", err)
		}
		if !isV6 {
			// Set the route_localnet bit on the host interface, so that
			// 127/8 can cross a routing boundary.
			hostIfName := getRoutableHostIF(containerIP)
			if hostIfName != "" {
				if err := enableLocalnetRouting(hostIfName); err != nil {
					return fmt.Errorf("unable to enable route_localnet: %v", err)
				}
			}
		}
	}

	return nil
}

// genToplevelDnatChain creates the top-level summary chain that we'll
// add our chain to. This is easy, because creating chains is idempotent.
// IMPORTANT: do not change this, or else upgrading plugins will require
// manual intervention.
func genToplevelDnatChain() chain {
	return chain{
		table: "nat",
		name:  TopLevelDNATChainName,
		entryRule: []string{
			"-m", "addrtype",
			"--dst-type", "LOCAL",
		},
		entryChains: []string{"PREROUTING", "OUTPUT"},
	}
}

// genDnatChain creates the per-container chain.
// Conditions are any static entry conditions for the chain.
func genDnatChain(netName, containerID string, conditions *[]string) chain {
	name := formatChainName("DN-", netName, containerID)
	comment := fmt.Sprintf(`dnat name: "%s" id: "%s"`, netName, containerID)

	ch := chain{
		table: "nat",
		name:  name,
		entryRule: []string{
			"-m", "comment",
			"--comment", comment,
		},
		entryChains: []string{TopLevelDNATChainName},
	}
	if conditions != nil && len(*conditions) != 0 {
		ch.entryRule = append(ch.entryRule, *conditions...)
	}

	return ch
}

// dnatRules generates the destination NAT rules, one per port, to direct
// traffic from hostip:hostport to podip:podport
func dnatRules(entries []PortMapEntry, containerIP net.IP) [][]string {
	out := make([][]string, 0, len(entries))
	for _, entry := range entries {
		rule := []string{
			"-p", entry.Protocol,
			"--dport", strconv.Itoa(entry.HostPort)}

		if entry.HostIP != "" {
			rule = append(rule,
				"-d", entry.HostIP)
		}

		rule = append(rule,
			"-j", "DNAT",
			"--to-destination", fmtIpPort(containerIP, entry.ContainerPort))

		out = append(out, rule)
	}
	return out
}

// genToplevelSnatChain creates the top-level summary snat chain.
// IMPORTANT: do not change this, or else upgrading plugins will require
// manual intervention
func genToplevelSnatChain(isV6 bool) chain {
	return chain{
		table: "nat",
		name:  TopLevelSNATChainName,
		entryRule: []string{
			"-s", localhostIP(isV6),
			"!", "-d", localhostIP(isV6),
		},
		entryChains: []string{"POSTROUTING"},
	}
}

// genSnatChain creates the snat (localhost) chain for this container.
func genSnatChain(netName, containerID string) chain {
	name := formatChainName("SN-", netName, containerID)
	comment := fmt.Sprintf(`snat name: "%s" id: "%s"`, netName, containerID)

	return chain{
		table: "nat",
		name:  name,
		entryRule: []string{
			"-m", "comment",
			"--comment", comment,
		},
		entryChains: []string{TopLevelSNATChainName},
	}
}

// snatRules sets up masquerading for connections to localhost:hostport,
// rewriting the source so that returning packets are correct.
func snatRules(entries []PortMapEntry, containerIP net.IP) [][]string {
	isV6 := (containerIP.To4() == nil)

	out := make([][]string, 0, len(entries))
	for _, entry := range entries {
		out = append(out, []string{
			"-p", entry.Protocol,
			"-s", localhostIP(isV6),
			"-d", containerIP.String(),
			"--dport", strconv.Itoa(entry.ContainerPort),
			"-j", "MASQUERADE",
		})
	}
	return out
}

// enableLocalnetRouting tells the kernel not to treat 127/8 as a martian,
// so that connections with a source ip of 127/8 can cross a routing boundary.
func enableLocalnetRouting(ifName string) error {
	routeLocalnetPath := "net.ipv4.conf." + ifName + ".route_localnet"
	_, err := sysctl.Sysctl(routeLocalnetPath, "1")
	return err
}

// unforwardPorts deletes any iptables rules created by this plugin.
// It should be idempotent - it will not error if the chain does not exist.
//
// We also need to be a bit clever about how we handle errors with initializing
// iptables. We may be on a system with no ip(6)tables, or no kernel support
// for that protocol. The ADD would be successful, since it only adds forwarding
// based on the addresses assigned to the container. However, at DELETE time we
// don't know which protocols were used.
// So, we first check that iptables is "generally OK" by doing a check. If
// not, we ignore the error, unless neither v4 nor v6 are OK.
func unforwardPorts(config *PortMapConf) error {
	dnatChain := genDnatChain(config.Name, config.ContainerID, nil)
	snatChain := genSnatChain(config.Name, config.ContainerID)

	ip4t := maybeGetIptables(false)
	ip6t := maybeGetIptables(true)
	if ip4t == nil && ip6t == nil {
		return fmt.Errorf("neither iptables nor ip6tables usable")
	}

	if ip4t != nil {
		if err := dnatChain.teardown(ip4t); err != nil {
			return fmt.Errorf("could not teardown ipv4 dnat: %v", err)
		}
		if err := snatChain.teardown(ip4t); err != nil {
			return fmt.Errorf("could not teardown ipv4 snat: %v", err)
		}
	}

	if ip6t != nil {
		if err := dnatChain.teardown(ip6t); err != nil {
			return fmt.Errorf("could not teardown ipv6 dnat: %v", err)
		}
		// no SNAT teardown because it doesn't work for v6
	}
	return nil
}

// maybeGetIptables implements the soft error swallowing. If iptables is
// usable for the given protocol, returns a handle, otherwise nil
func maybeGetIptables(isV6 bool) *iptables.IPTables {
	proto := iptables.ProtocolIPv4
	if isV6 {
		proto = iptables.ProtocolIPv6
	}

	ipt, err := iptables.NewWithProtocol(proto)
	if err != nil {
		return nil
	}

	_, err = ipt.List("nat", "OUTPUT")
	if err != nil {
		return nil
	}

	return ipt
}
