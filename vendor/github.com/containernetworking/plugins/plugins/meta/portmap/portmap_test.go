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
	"net"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("portmapping configuration", func() {
	netName := "testNetName"
	containerID := "icee6giejonei6sohng6ahngee7laquohquee9shiGo7fohferakah3Feiyoolu2pei7ciPhoh7shaoX6vai3vuf0ahfaeng8yohb9ceu0daez5hashee8ooYai5wa3y"

	mappings := []PortMapEntry{
		{80, 90, "tcp", ""},
		{1000, 2000, "udp", ""},
	}
	ipv4addr := net.ParseIP("192.2.0.1")
	ipv6addr := net.ParseIP("2001:db8::1")

	Context("config parsing", func() {
		It("Correctly parses an ADD config", func() {
			configBytes := []byte(`{
	"name": "test",
	"type": "portmap",
	"cniVersion": "0.3.1",
	"runtimeConfig": {
		"portMappings": [
			{ "hostPort": 8080, "containerPort": 80, "protocol": "tcp"},
			{ "hostPort": 8081, "containerPort": 81, "protocol": "udp"}
		]
	},
	"snat": false,
	"conditionsV4": ["a", "b"],
	"conditionsV6": ["c", "d"],
	"prevResult": {
		"interfaces": [
			{"name": "host"},
			{"name": "container", "sandbox":"netns"}
		],
		"ips": [
			{
				"version": "4",
				"address": "10.0.0.1/24",
				"gateway": "10.0.0.1",
				"interface": 0
			},
			{
				"version": "6",
				"address": "2001:db8:1::2/64",
				"gateway": "2001:db8:1::1",
				"interface": 1
			},
			{
				"version": "4",
				"address": "10.0.0.2/24",
				"gateway": "10.0.0.1",
				"interface": 1
			}
		]
	}
}`)
			c, err := parseConfig(configBytes, "container")
			Expect(err).NotTo(HaveOccurred())
			Expect(c.CNIVersion).To(Equal("0.3.1"))
			Expect(c.ConditionsV4).To(Equal(&[]string{"a", "b"}))
			Expect(c.ConditionsV6).To(Equal(&[]string{"c", "d"}))
			fvar := false
			Expect(c.SNAT).To(Equal(&fvar))
			Expect(c.Name).To(Equal("test"))

			Expect(c.ContIPv4).To(Equal(net.ParseIP("10.0.0.2")))
			Expect(c.ContIPv6).To(Equal(net.ParseIP("2001:db8:1::2")))
		})

		It("Correctly parses a DEL config", func() {
			// When called with DEL, neither runtimeConfig nor prevResult may be specified
			configBytes := []byte(`{
	"name": "test",
	"type": "portmap",
	"cniVersion": "0.3.1",
	"snat": false,
	"conditionsV4": ["a", "b"],
	"conditionsV6": ["c", "d"]
}`)
			c, err := parseConfig(configBytes, "container")
			Expect(err).NotTo(HaveOccurred())
			Expect(c.CNIVersion).To(Equal("0.3.1"))
			Expect(c.ConditionsV4).To(Equal(&[]string{"a", "b"}))
			Expect(c.ConditionsV6).To(Equal(&[]string{"c", "d"}))
			fvar := false
			Expect(c.SNAT).To(Equal(&fvar))
			Expect(c.Name).To(Equal("test"))
		})

		It("fails with invalid mappings", func() {
			configBytes := []byte(`{
	"name": "test",
	"type": "portmap",
	"cniVersion": "0.3.1",
	"snat": false,
	"conditionsV4": ["a", "b"],
	"conditionsV6": ["c", "d"],
	"runtimeConfig": {
		"portMappings": [
			{ "hostPort": 0, "containerPort": 80, "protocol": "tcp"}
		]
	}
}`)
			_, err := parseConfig(configBytes, "container")
			Expect(err).To(MatchError("Invalid host port number: 0"))
		})

		It("Does not fail on missing prevResult interface index", func() {
			configBytes := []byte(`{
	"name": "test",
	"type": "portmap",
	"cniVersion": "0.3.1",
	"runtimeConfig": {
		"portMappings": [
			{ "hostPort": 8080, "containerPort": 80, "protocol": "tcp"}
		]
	},
	"conditionsV4": ["a", "b"],
	"prevResult": {
		"interfaces": [
			{"name": "host"}
		],
		"ips": [
			{
				"version": "4",
				"address": "10.0.0.1/24",
				"gateway": "10.0.0.1"
			}
		]
	}
}`)
			_, err := parseConfig(configBytes, "container")
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("Generating chains", func() {
		Context("for DNAT", func() {
			It("generates a correct container chain", func() {
				ch := genDnatChain(netName, containerID, &[]string{"-m", "hello"})

				Expect(ch).To(Equal(chain{
					table: "nat",
					name:  "CNI-DN-bfd599665540dd91d5d28",
					entryRule: []string{
						"-m", "comment",
						"--comment", `dnat name: "testNetName" id: "` + containerID + `"`,
						"-m", "hello",
					},
					entryChains: []string{TopLevelDNATChainName},
				}))
			})

			It("generates a correct top-level chain", func() {
				ch := genToplevelDnatChain()

				Expect(ch).To(Equal(chain{
					table: "nat",
					name:  "CNI-HOSTPORT-DNAT",
					entryRule: []string{
						"-m", "addrtype",
						"--dst-type", "LOCAL",
					},
					entryChains: []string{"PREROUTING", "OUTPUT"},
				}))
			})
		})

		Context("for SNAT", func() {
			It("generates a correct container chain", func() {
				ch := genSnatChain(netName, containerID)

				Expect(ch).To(Equal(chain{
					table: "nat",
					name:  "CNI-SN-bfd599665540dd91d5d28",
					entryRule: []string{
						"-m", "comment",
						"--comment", `snat name: "testNetName" id: "` + containerID + `"`,
					},
					entryChains: []string{TopLevelSNATChainName},
				}))
			})

			It("generates a correct top-level chain", func() {
				Context("for ipv4", func() {
					ch := genToplevelSnatChain(false)
					Expect(ch).To(Equal(chain{
						table: "nat",
						name:  "CNI-HOSTPORT-SNAT",
						entryRule: []string{
							"-s", "127.0.0.1",
							"!", "-d", "127.0.0.1",
						},
						entryChains: []string{"POSTROUTING"},
					}))
				})
			})
		})
	})

	Describe("Forwarding rules", func() {
		Context("for DNAT", func() {
			It("generates correct ipv4 rules", func() {
				rules := dnatRules(mappings, ipv4addr)
				Expect(rules).To(Equal([][]string{
					{"-p", "tcp", "--dport", "80", "-j", "DNAT", "--to-destination", "192.2.0.1:90"},
					{"-p", "udp", "--dport", "1000", "-j", "DNAT", "--to-destination", "192.2.0.1:2000"},
				}))
			})
			It("generates correct ipv6 rules", func() {
				rules := dnatRules(mappings, ipv6addr)
				Expect(rules).To(Equal([][]string{
					{"-p", "tcp", "--dport", "80", "-j", "DNAT", "--to-destination", "[2001:db8::1]:90"},
					{"-p", "udp", "--dport", "1000", "-j", "DNAT", "--to-destination", "[2001:db8::1]:2000"},
				}))
			})
		})

		Context("for SNAT", func() {

			It("generates correct ipv4 rules", func() {
				rules := snatRules(mappings, ipv4addr)
				Expect(rules).To(Equal([][]string{
					{"-p", "tcp", "-s", "127.0.0.1", "-d", "192.2.0.1", "--dport", "90", "-j", "MASQUERADE"},
					{"-p", "udp", "-s", "127.0.0.1", "-d", "192.2.0.1", "--dport", "2000", "-j", "MASQUERADE"},
				}))
			})

			It("generates correct ipv6 rules", func() {
				rules := snatRules(mappings, ipv6addr)
				Expect(rules).To(Equal([][]string{
					{"-p", "tcp", "-s", "::1", "-d", "2001:db8::1", "--dport", "90", "-j", "MASQUERADE"},
					{"-p", "udp", "-s", "::1", "-d", "2001:db8::1", "--dport", "2000", "-j", "MASQUERADE"},
				}))
			})
		})
	})
})
