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
	"fmt"
	"net"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/020"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/containernetworking/plugins/pkg/utils/hwaddr"

	"github.com/vishvananda/netlink"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const (
	BRNAME = "bridge0"
	IFNAME = "eth0"
)

// testCase defines the CNI network configuration and the expected
// bridge addresses for a test case.
type testCase struct {
	cniVersion string      // CNI Version
	subnet     string      // Single subnet config: Subnet CIDR
	gateway    string      // Single subnet config: Gateway
	ranges     []rangeInfo // Ranges list (multiple subnets config)
	isGW       bool
	expGWCIDRs []string // Expected gateway addresses in CIDR form
}

// Range definition for each entry in the ranges list
type rangeInfo struct {
	subnet  string
	gateway string
}

// netConf() creates a NetConf structure for a test case.
func (tc testCase) netConf() *NetConf {
	return &NetConf{
		NetConf: types.NetConf{
			CNIVersion: tc.cniVersion,
			Name:       "testConfig",
			Type:       "bridge",
		},
		BrName: BRNAME,
		IsGW:   tc.isGW,
		IPMasq: false,
		MTU:    5000,
	}
}

// Snippets for generating a JSON network configuration string.
const (
	netConfStr = `
	"cniVersion": "%s",
	"name": "testConfig",
	"type": "bridge",
	"bridge": "%s",
	"isDefaultGateway": true,
	"ipMasq": false`

	ipamStartStr = `,
    "ipam": {
        "type":    "host-local"`

	// Single subnet configuration (legacy)
	subnetConfStr = `,
        "subnet":  "%s"`
	gatewayConfStr = `,
        "gateway": "%s"`

	// Ranges (multiple subnets) configuration
	rangesStartStr = `,
        "ranges": [`
	rangeSubnetConfStr = `
            [{
                "subnet":  "%s"
            }]`
	rangeSubnetGWConfStr = `
            [{
                "subnet":  "%s",
                "gateway": "%s"
            }]`
	rangesEndStr = `
        ]`

	ipamEndStr = `
    }`
)

// netConfJSON() generates a JSON network configuration string
// for a test case.
func (tc testCase) netConfJSON() string {
	conf := fmt.Sprintf(netConfStr, tc.cniVersion, BRNAME)
	if tc.subnet != "" || tc.ranges != nil {
		conf += ipamStartStr
		if tc.subnet != "" {
			conf += tc.subnetConfig()
		}
		if tc.ranges != nil {
			conf += tc.rangesConfig()
		}
		conf += ipamEndStr
	}
	return "{" + conf + "\n}"
}

func (tc testCase) subnetConfig() string {
	conf := fmt.Sprintf(subnetConfStr, tc.subnet)
	if tc.gateway != "" {
		conf += fmt.Sprintf(gatewayConfStr, tc.gateway)
	}
	return conf
}

func (tc testCase) rangesConfig() string {
	conf := rangesStartStr
	for i, tcRange := range tc.ranges {
		if i > 0 {
			conf += ","
		}
		if tcRange.gateway != "" {
			conf += fmt.Sprintf(rangeSubnetGWConfStr, tcRange.subnet, tcRange.gateway)
		} else {
			conf += fmt.Sprintf(rangeSubnetConfStr, tcRange.subnet)
		}
	}
	return conf + rangesEndStr
}

// createCmdArgs generates network configuration and creates command
// arguments for a test case.
func (tc testCase) createCmdArgs(targetNS ns.NetNS) *skel.CmdArgs {
	conf := tc.netConfJSON()
	return &skel.CmdArgs{
		ContainerID: "dummy",
		Netns:       targetNS.Path(),
		IfName:      IFNAME,
		StdinData:   []byte(conf),
	}
}

// expectedCIDRs determines the IPv4 and IPv6 CIDRs in which the resulting
// addresses are expected to be contained.
func (tc testCase) expectedCIDRs() ([]*net.IPNet, []*net.IPNet) {
	var cidrsV4, cidrsV6 []*net.IPNet
	appendSubnet := func(subnet string) {
		ip, cidr, err := net.ParseCIDR(subnet)
		Expect(err).NotTo(HaveOccurred())
		if ipVersion(ip) == "4" {
			cidrsV4 = append(cidrsV4, cidr)
		} else {
			cidrsV6 = append(cidrsV6, cidr)
		}
	}
	if tc.subnet != "" {
		appendSubnet(tc.subnet)
	}
	for _, r := range tc.ranges {
		appendSubnet(r.subnet)
	}
	return cidrsV4, cidrsV6
}

// delBridgeAddrs() deletes addresses from the bridge
func delBridgeAddrs(testNS ns.NetNS) {
	err := testNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		br, err := netlink.LinkByName(BRNAME)
		Expect(err).NotTo(HaveOccurred())
		addrs, err := netlink.AddrList(br, netlink.FAMILY_ALL)
		Expect(err).NotTo(HaveOccurred())
		for _, addr := range addrs {
			if !addr.IP.IsLinkLocalUnicast() {
				err = netlink.AddrDel(br, &addr)
				Expect(err).NotTo(HaveOccurred())
			}
		}

		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}

func ipVersion(ip net.IP) string {
	if ip.To4() != nil {
		return "4"
	}
	return "6"
}

type cmdAddDelTester interface {
	setNS(testNS ns.NetNS, targetNS ns.NetNS)
	cmdAddTest(tc testCase)
	cmdDelTest(tc testCase)
}

func testerByVersion(version string) cmdAddDelTester {
	switch {
	case strings.HasPrefix(version, "0.3."):
		return &testerV03x{}
	default:
		return &testerV01xOr02x{}
	}
}

type testerV03x struct {
	testNS   ns.NetNS
	targetNS ns.NetNS
	args     *skel.CmdArgs
	vethName string
}

func (tester *testerV03x) setNS(testNS ns.NetNS, targetNS ns.NetNS) {
	tester.testNS = testNS
	tester.targetNS = targetNS
}

func (tester *testerV03x) cmdAddTest(tc testCase) {
	// Generate network config and command arguments
	tester.args = tc.createCmdArgs(tester.targetNS)

	// Execute cmdADD on the plugin
	var result *current.Result
	err := tester.testNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		r, raw, err := testutils.CmdAddWithResult(tester.targetNS.Path(), IFNAME, tester.args.StdinData, func() error {
			return cmdAdd(tester.args)
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Index(string(raw), "\"interfaces\":")).Should(BeNumerically(">", 0))

		result, err = current.GetResult(r)
		Expect(err).NotTo(HaveOccurred())

		Expect(len(result.Interfaces)).To(Equal(3))
		Expect(result.Interfaces[0].Name).To(Equal(BRNAME))
		Expect(result.Interfaces[2].Name).To(Equal(IFNAME))

		// Make sure bridge link exists
		link, err := netlink.LinkByName(result.Interfaces[0].Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(link.Attrs().Name).To(Equal(BRNAME))
		Expect(link).To(BeAssignableToTypeOf(&netlink.Bridge{}))
		Expect(link.Attrs().HardwareAddr.String()).To(Equal(result.Interfaces[0].Mac))

		// Ensure bridge has expected gateway address(es)
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(addrs)).To(BeNumerically(">", 0))
		for _, cidr := range tc.expGWCIDRs {
			ip, subnet, err := net.ParseCIDR(cidr)
			Expect(err).NotTo(HaveOccurred())
			if ip.To4() != nil {
				hwAddr := fmt.Sprintf("%s", link.Attrs().HardwareAddr)
				Expect(hwAddr).To(HavePrefix(hwaddr.PrivateMACPrefixString))
			}

			found := false
			subnetPrefix, subnetBits := subnet.Mask.Size()
			for _, a := range addrs {
				aPrefix, aBits := a.IPNet.Mask.Size()
				if a.IPNet.IP.Equal(ip) && aPrefix == subnetPrefix && aBits == subnetBits {
					found = true
					break
				}
			}
			Expect(found).To(Equal(true))
		}

		// Check for the veth link in the main namespace
		links, err := netlink.LinkList()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(links)).To(Equal(3)) // Bridge, veth, and loopback

		link, err = netlink.LinkByName(result.Interfaces[1].Name)
		Expect(err).NotTo(HaveOccurred())
		Expect(link).To(BeAssignableToTypeOf(&netlink.Veth{}))
		tester.vethName = result.Interfaces[1].Name
		return nil
	})
	Expect(err).NotTo(HaveOccurred())

	// Find the veth peer in the container namespace and the default route
	err = tester.targetNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		link, err := netlink.LinkByName(IFNAME)
		Expect(err).NotTo(HaveOccurred())
		Expect(link.Attrs().Name).To(Equal(IFNAME))
		Expect(link).To(BeAssignableToTypeOf(&netlink.Veth{}))

		expCIDRsV4, expCIDRsV6 := tc.expectedCIDRs()
		if expCIDRsV4 != nil {
			hwAddr := fmt.Sprintf("%s", link.Attrs().HardwareAddr)
			Expect(hwAddr).To(HavePrefix(hwaddr.PrivateMACPrefixString))
		}
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(addrs)).To(Equal(len(expCIDRsV4)))
		addrs, err = netlink.AddrList(link, netlink.FAMILY_V6)
		Expect(err).NotTo(HaveOccurred())
		// Ignore link local address which may or may not be
		// ready when we read addresses.
		var foundAddrs int
		for _, addr := range addrs {
			if !addr.IP.IsLinkLocalUnicast() {
				foundAddrs++
			}
		}
		Expect(foundAddrs).To(Equal(len(expCIDRsV6)))

		// Ensure the default route(s)
		routes, err := netlink.RouteList(link, 0)
		Expect(err).NotTo(HaveOccurred())

		var defaultRouteFound4, defaultRouteFound6 bool
		for _, cidr := range tc.expGWCIDRs {
			gwIP, _, err := net.ParseCIDR(cidr)
			Expect(err).NotTo(HaveOccurred())
			var found *bool
			if ipVersion(gwIP) == "4" {
				found = &defaultRouteFound4
			} else {
				found = &defaultRouteFound6
			}
			if *found == true {
				continue
			}
			for _, route := range routes {
				*found = (route.Dst == nil && route.Src == nil && route.Gw.Equal(gwIP))
				if *found {
					break
				}
			}
			Expect(*found).To(Equal(true))
		}

		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}

func (tester *testerV03x) cmdDelTest(tc testCase) {
	err := tester.testNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		err := testutils.CmdDelWithResult(tester.targetNS.Path(), IFNAME, func() error {
			return cmdDel(tester.args)
		})
		Expect(err).NotTo(HaveOccurred())
		return nil
	})
	Expect(err).NotTo(HaveOccurred())

	// Make sure the host veth has been deleted
	err = tester.targetNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		link, err := netlink.LinkByName(IFNAME)
		Expect(err).To(HaveOccurred())
		Expect(link).To(BeNil())
		return nil
	})
	Expect(err).NotTo(HaveOccurred())

	// Make sure the container veth has been deleted
	err = tester.testNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		link, err := netlink.LinkByName(tester.vethName)
		Expect(err).To(HaveOccurred())
		Expect(link).To(BeNil())
		return nil
	})
}

type testerV01xOr02x struct {
	testNS   ns.NetNS
	targetNS ns.NetNS
	args     *skel.CmdArgs
	vethName string
}

func (tester *testerV01xOr02x) setNS(testNS ns.NetNS, targetNS ns.NetNS) {
	tester.testNS = testNS
	tester.targetNS = targetNS
}

func (tester *testerV01xOr02x) cmdAddTest(tc testCase) {
	// Generate network config and calculate gateway addresses
	tester.args = tc.createCmdArgs(tester.targetNS)

	// Execute cmdADD on the plugin
	var result *types020.Result
	err := tester.testNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		r, raw, err := testutils.CmdAddWithResult(tester.targetNS.Path(), IFNAME, tester.args.StdinData, func() error {
			return cmdAdd(tester.args)
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Index(string(raw), "\"ip\":")).Should(BeNumerically(">", 0))

		// We expect a version 0.1.0 result
		result, err = types020.GetResult(r)
		Expect(err).NotTo(HaveOccurred())

		// Make sure bridge link exists
		link, err := netlink.LinkByName(BRNAME)
		Expect(err).NotTo(HaveOccurred())
		Expect(link.Attrs().Name).To(Equal(BRNAME))
		Expect(link).To(BeAssignableToTypeOf(&netlink.Bridge{}))

		// Ensure bridge has expected gateway address(es)
		addrs, err := netlink.AddrList(link, netlink.FAMILY_ALL)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(addrs)).To(BeNumerically(">", 0))
		for _, cidr := range tc.expGWCIDRs {
			ip, subnet, err := net.ParseCIDR(cidr)
			Expect(err).NotTo(HaveOccurred())
			if ip.To4() != nil {
				hwAddr := fmt.Sprintf("%s", link.Attrs().HardwareAddr)
				Expect(hwAddr).To(HavePrefix(hwaddr.PrivateMACPrefixString))
			}

			found := false
			subnetPrefix, subnetBits := subnet.Mask.Size()
			for _, a := range addrs {
				aPrefix, aBits := a.IPNet.Mask.Size()
				if a.IPNet.IP.Equal(ip) && aPrefix == subnetPrefix && aBits == subnetBits {
					found = true
					break
				}
			}
			Expect(found).To(Equal(true))
		}

		// Check for the veth link in the main namespace; can't
		// check the for the specific link since version 0.1.0
		// doesn't report interfaces
		links, err := netlink.LinkList()
		Expect(err).NotTo(HaveOccurred())
		Expect(len(links)).To(Equal(3)) // Bridge, veth, and loopback
		return nil
	})
	Expect(err).NotTo(HaveOccurred())

	// Find the veth peer in the container namespace and the default route
	err = tester.targetNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		link, err := netlink.LinkByName(IFNAME)
		Expect(err).NotTo(HaveOccurred())
		Expect(link.Attrs().Name).To(Equal(IFNAME))
		Expect(link).To(BeAssignableToTypeOf(&netlink.Veth{}))

		expCIDRsV4, expCIDRsV6 := tc.expectedCIDRs()
		if expCIDRsV4 != nil {
			hwAddr := fmt.Sprintf("%s", link.Attrs().HardwareAddr)
			Expect(hwAddr).To(HavePrefix(hwaddr.PrivateMACPrefixString))
		}
		addrs, err := netlink.AddrList(link, netlink.FAMILY_V4)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(addrs)).To(Equal(len(expCIDRsV4)))
		addrs, err = netlink.AddrList(link, netlink.FAMILY_V6)
		Expect(err).NotTo(HaveOccurred())
		Expect(len(addrs)).To(Equal(len(expCIDRsV6) + 1)) // Link local address is automatic

		// Ensure the default route
		routes, err := netlink.RouteList(link, 0)
		Expect(err).NotTo(HaveOccurred())

		var defaultRouteFound bool
		for _, cidr := range tc.expGWCIDRs {
			gwIP, _, err := net.ParseCIDR(cidr)
			Expect(err).NotTo(HaveOccurred())
			for _, route := range routes {
				defaultRouteFound = (route.Dst == nil && route.Src == nil && route.Gw.Equal(gwIP))
				if defaultRouteFound {
					break
				}
			}
			Expect(defaultRouteFound).To(Equal(true))
		}

		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}

func (tester *testerV01xOr02x) cmdDelTest(tc testCase) {
	err := tester.testNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		err := testutils.CmdDelWithResult(tester.targetNS.Path(), IFNAME, func() error {
			return cmdDel(tester.args)
		})
		Expect(err).NotTo(HaveOccurred())
		return nil
	})
	Expect(err).NotTo(HaveOccurred())

	// Make sure the container veth has been deleted; cannot check
	// host veth as version 0.1.0 can't report its name
	err = tester.testNS.Do(func(ns.NetNS) error {
		defer GinkgoRecover()

		link, err := netlink.LinkByName(IFNAME)
		Expect(err).To(HaveOccurred())
		Expect(link).To(BeNil())
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
}

func cmdAddDelTest(testNS ns.NetNS, tc testCase) {
	// Get a Add/Del tester based on test case version
	tester := testerByVersion(tc.cniVersion)

	targetNS, err := ns.NewNS()
	Expect(err).NotTo(HaveOccurred())
	defer targetNS.Close()
	tester.setNS(testNS, targetNS)

	// Test IP allocation
	tester.cmdAddTest(tc)

	// Test IP Release
	tester.cmdDelTest(tc)

	// Clean up bridge addresses for next test case
	delBridgeAddrs(testNS)
}

var _ = Describe("bridge Operations", func() {
	var originalNS ns.NetNS

	BeforeEach(func() {
		// Create a new NetNS so we don't modify the host
		var err error
		originalNS, err = ns.NewNS()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		Expect(originalNS.Close()).To(Succeed())
	})

	It("creates a bridge", func() {
		conf := testCase{cniVersion: "0.3.1"}.netConf()
		err := originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			bridge, _, err := setupBridge(conf)
			Expect(err).NotTo(HaveOccurred())
			Expect(bridge.Attrs().Name).To(Equal(BRNAME))

			// Double check that the link was added
			link, err := netlink.LinkByName(BRNAME)
			Expect(err).NotTo(HaveOccurred())
			Expect(link.Attrs().Name).To(Equal(BRNAME))
			Expect(link.Attrs().Promisc).To(Equal(0))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("handles an existing bridge", func() {
		err := originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			err := netlink.LinkAdd(&netlink.Bridge{
				LinkAttrs: netlink.LinkAttrs{
					Name: BRNAME,
				},
			})
			Expect(err).NotTo(HaveOccurred())
			link, err := netlink.LinkByName(BRNAME)
			Expect(err).NotTo(HaveOccurred())
			Expect(link.Attrs().Name).To(Equal(BRNAME))
			ifindex := link.Attrs().Index

			tc := testCase{cniVersion: "0.3.1", isGW: false}
			conf := tc.netConf()

			bridge, _, err := setupBridge(conf)
			Expect(err).NotTo(HaveOccurred())
			Expect(bridge.Attrs().Name).To(Equal(BRNAME))
			Expect(bridge.Attrs().Index).To(Equal(ifindex))

			// Double check that the link has the same ifindex
			link, err = netlink.LinkByName(BRNAME)
			Expect(err).NotTo(HaveOccurred())
			Expect(link.Attrs().Name).To(Equal(BRNAME))
			Expect(link.Attrs().Index).To(Equal(ifindex))
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("configures and deconfigures a bridge and veth with default route with ADD/DEL for 0.3.0 config", func() {
		testCases := []testCase{
			{
				// IPv4 only
				subnet:     "10.1.2.0/24",
				expGWCIDRs: []string{"10.1.2.1/24"},
			},
			{
				// IPv6 only
				subnet:     "2001:db8::0/64",
				expGWCIDRs: []string{"2001:db8::1/64"},
			},
			{
				// Dual-Stack
				ranges: []rangeInfo{
					{subnet: "192.168.0.0/24"},
					{subnet: "fd00::0/64"},
				},
				expGWCIDRs: []string{
					"192.168.0.1/24",
					"fd00::1/64",
				},
			},
			{
				// 3 Subnets (1 IPv4 and 2 IPv6 subnets)
				ranges: []rangeInfo{
					{subnet: "192.168.0.0/24"},
					{subnet: "fd00::0/64"},
					{subnet: "2001:db8::0/64"},
				},
				expGWCIDRs: []string{
					"192.168.0.1/24",
					"fd00::1/64",
					"2001:db8::1/64",
				},
			},
		}
		for _, tc := range testCases {
			tc.cniVersion = "0.3.0"
			cmdAddDelTest(originalNS, tc)
		}
	})

	It("configures and deconfigures a bridge and veth with default route with ADD/DEL for 0.3.1 config", func() {
		testCases := []testCase{
			{
				// IPv4 only
				subnet:     "10.1.2.0/24",
				expGWCIDRs: []string{"10.1.2.1/24"},
			},
			{
				// IPv6 only
				subnet:     "2001:db8::0/64",
				expGWCIDRs: []string{"2001:db8::1/64"},
			},
			{
				// Dual-Stack
				ranges: []rangeInfo{
					{subnet: "192.168.0.0/24"},
					{subnet: "fd00::0/64"},
				},
				expGWCIDRs: []string{
					"192.168.0.1/24",
					"fd00::1/64",
				},
			},
		}
		for _, tc := range testCases {
			tc.cniVersion = "0.3.1"
			cmdAddDelTest(originalNS, tc)
		}
	})

	It("deconfigures an unconfigured bridge with DEL", func() {
		tc := testCase{
			cniVersion: "0.3.0",
			subnet:     "10.1.2.0/24",
			expGWCIDRs: []string{"10.1.2.1/24"},
		}

		tester := testerV03x{}
		targetNS, err := ns.NewNS()
		Expect(err).NotTo(HaveOccurred())
		defer targetNS.Close()
		tester.setNS(originalNS, targetNS)
		tester.args = tc.createCmdArgs(targetNS)

		// Execute cmdDEL on the plugin, expect no errors
		tester.cmdDelTest(tc)
	})

	It("configures and deconfigures a bridge and veth with default route with ADD/DEL for 0.1.0 config", func() {
		testCases := []testCase{
			{
				// IPv4 only
				subnet:     "10.1.2.0/24",
				expGWCIDRs: []string{"10.1.2.1/24"},
			},
			{
				// IPv6 only
				subnet:     "2001:db8::0/64",
				expGWCIDRs: []string{"2001:db8::1/64"},
			},
			{
				// Dual-Stack
				ranges: []rangeInfo{
					{subnet: "192.168.0.0/24"},
					{subnet: "fd00::0/64"},
				},
				expGWCIDRs: []string{
					"192.168.0.1/24",
					"fd00::1/64",
				},
			},
		}
		for _, tc := range testCases {
			tc.cniVersion = "0.1.0"
			cmdAddDelTest(originalNS, tc)
		}
	})

	It("ensure bridge address", func() {
		conf := testCase{cniVersion: "0.3.1", isGW: true}.netConf()

		testCases := []struct {
			gwCIDRFirst  string
			gwCIDRSecond string
		}{
			{
				// IPv4
				gwCIDRFirst:  "10.0.0.1/8",
				gwCIDRSecond: "10.1.2.3/16",
			},
			{
				// IPv6, overlapping subnets
				gwCIDRFirst:  "2001:db8:1::1/48",
				gwCIDRSecond: "2001:db8:1:2::1/64",
			},
			{
				// IPv6, non-overlapping subnets
				gwCIDRFirst:  "2001:db8:1:2::1/64",
				gwCIDRSecond: "fd00:1234::1/64",
			},
		}
		for _, tc := range testCases {

			gwIP, gwSubnet, err := net.ParseCIDR(tc.gwCIDRFirst)
			Expect(err).NotTo(HaveOccurred())
			gwnFirst := net.IPNet{IP: gwIP, Mask: gwSubnet.Mask}
			gwIP, gwSubnet, err = net.ParseCIDR(tc.gwCIDRSecond)
			Expect(err).NotTo(HaveOccurred())
			gwnSecond := net.IPNet{IP: gwIP, Mask: gwSubnet.Mask}

			var family, expNumAddrs int
			switch {
			case gwIP.To4() != nil:
				family = netlink.FAMILY_V4
				expNumAddrs = 1
			default:
				family = netlink.FAMILY_V6
				// Expect configured gw address plus link local
				expNumAddrs = 2
			}

			subnetsOverlap := gwnFirst.Contains(gwnSecond.IP) ||
				gwnSecond.Contains(gwnFirst.IP)

			err = originalNS.Do(func(ns.NetNS) error {
				defer GinkgoRecover()

				// Create the bridge
				bridge, _, err := setupBridge(conf)
				Expect(err).NotTo(HaveOccurred())

				// Function to check IP address(es) on bridge
				checkBridgeIPs := func(cidr0, cidr1 string) {
					addrs, err := netlink.AddrList(bridge, family)
					Expect(err).NotTo(HaveOccurred())
					Expect(len(addrs)).To(Equal(expNumAddrs))
					addr := addrs[0].IPNet.String()
					Expect(addr).To(Equal(cidr0))
					if cidr1 != "" {
						addr = addrs[1].IPNet.String()
						Expect(addr).To(Equal(cidr1))
					}
				}

				// Check if ForceAddress has default value
				Expect(conf.ForceAddress).To(Equal(false))

				// Set first address on bridge
				err = ensureBridgeAddr(bridge, family, &gwnFirst, conf.ForceAddress)
				Expect(err).NotTo(HaveOccurred())
				checkBridgeIPs(tc.gwCIDRFirst, "")

				// Attempt to set the second address on the bridge
				// with ForceAddress set to false.
				err = ensureBridgeAddr(bridge, family, &gwnSecond, false)
				if family == netlink.FAMILY_V4 || subnetsOverlap {
					// IPv4 or overlapping IPv6 subnets:
					// Expect an error, and address should remain the same
					Expect(err).To(HaveOccurred())
					checkBridgeIPs(tc.gwCIDRFirst, "")
				} else {
					// Non-overlapping IPv6 subnets:
					// There should be 2 addresses (in addition to link local)
					Expect(err).NotTo(HaveOccurred())
					expNumAddrs++
					checkBridgeIPs(tc.gwCIDRSecond, tc.gwCIDRFirst)
				}

				// Set the second address on the bridge
				// with ForceAddress set to true.
				err = ensureBridgeAddr(bridge, family, &gwnSecond, true)
				Expect(err).NotTo(HaveOccurred())
				if family == netlink.FAMILY_V4 || subnetsOverlap {
					// IPv4 or overlapping IPv6 subnets:
					// IP address should be reconfigured.
					checkBridgeIPs(tc.gwCIDRSecond, "")
				} else {
					// Non-overlapping IPv6 subnets:
					// There should be 2 addresses (in addition to link local)
					checkBridgeIPs(tc.gwCIDRSecond, tc.gwCIDRFirst)
				}

				return nil
			})
			Expect(err).NotTo(HaveOccurred())

			// Clean up bridge addresses for next test case
			delBridgeAddrs(originalNS)
		}
	})
	It("ensure promiscuous mode on bridge", func() {
		const IFNAME = "bridge0"
		const EXPECTED_IP = "10.0.0.0/8"
		const CHANGED_EXPECTED_IP = "10.1.2.3/16"

		conf := &NetConf{
			NetConf: types.NetConf{
				CNIVersion: "0.3.1",
				Name:       "testConfig",
				Type:       "bridge",
			},
			BrName:      IFNAME,
			IsGW:        true,
			IPMasq:      false,
			HairpinMode: false,
			PromiscMode: true,
			MTU:         5000,
		}

		err := originalNS.Do(func(ns.NetNS) error {
			defer GinkgoRecover()

			_, _, err := setupBridge(conf)
			Expect(err).NotTo(HaveOccurred())
			// Check if ForceAddress has default value
			Expect(conf.ForceAddress).To(Equal(false))

			//Check if promiscuous mode is set correctly
			link, err := netlink.LinkByName("bridge0")
			Expect(err).NotTo(HaveOccurred())

			Expect(link.Attrs().Promisc).To(Equal(1))

			return nil
		})
		Expect(err).NotTo(HaveOccurred())
	})
})
