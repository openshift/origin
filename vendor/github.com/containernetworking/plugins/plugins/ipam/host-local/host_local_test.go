// Copyright 2016 CNI authors
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
	"io/ioutil"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/020"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/plugins/pkg/testutils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("host-local Operations", func() {
	It("allocates and releases addresses with ADD/DEL", func() {
		const ifname string = "eth0"
		const nspath string = "/some/where"

		tmpDir, err := ioutil.TempDir("", "host_local_artifacts")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		err = ioutil.WriteFile(filepath.Join(tmpDir, "resolv.conf"), []byte("nameserver 192.0.2.3"), 0644)
		Expect(err).NotTo(HaveOccurred())

		conf := fmt.Sprintf(`{
"cniVersion": "0.3.1",
"name": "mynet",
"type": "ipvlan",
"master": "foo0",
	"ipam": {
		"type": "host-local",
		"dataDir": "%s",
		"resolvConf": "%s/resolv.conf",
		"ranges": [
			[{ "subnet": "10.1.2.0/24" }, {"subnet": "10.2.2.0/24"}],
			[{ "subnet": "2001:db8:1::0/64" }]
		],
		"routes": [
			{"dst": "0.0.0.0/0"},
			{"dst": "::/0"},
			{"dst": "192.168.0.0/16", "gw": "1.1.1.1"},
			{"dst": "2001:db8:2::0/64", "gw": "2001:db8:3::1"}
		]
	}
}`, tmpDir, tmpDir)

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       nspath,
			IfName:      ifname,
			StdinData:   []byte(conf),
		}

		// Allocate the IP
		r, raw, err := testutils.CmdAddWithResult(nspath, ifname, []byte(conf), func() error {
			return cmdAdd(args)
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Index(string(raw), "\"version\":")).Should(BeNumerically(">", 0))

		result, err := current.GetResult(r)
		Expect(err).NotTo(HaveOccurred())

		// Gomega is cranky about slices with different caps
		Expect(*result.IPs[0]).To(Equal(
			current.IPConfig{
				Version: "4",
				Address: mustCIDR("10.1.2.2/24"),
				Gateway: net.ParseIP("10.1.2.1"),
			}))

		Expect(*result.IPs[1]).To(Equal(
			current.IPConfig{
				Version: "6",
				Address: mustCIDR("2001:db8:1::2/64"),
				Gateway: net.ParseIP("2001:db8:1::1"),
			},
		))
		Expect(len(result.IPs)).To(Equal(2))

		Expect(result.Routes).To(Equal([]*types.Route{
			{Dst: mustCIDR("0.0.0.0/0"), GW: nil},
			{Dst: mustCIDR("::/0"), GW: nil},
			{Dst: mustCIDR("192.168.0.0/16"), GW: net.ParseIP("1.1.1.1")},
			{Dst: mustCIDR("2001:db8:2::0/64"), GW: net.ParseIP("2001:db8:3::1")},
		}))

		ipFilePath1 := filepath.Join(tmpDir, "mynet", "10.1.2.2")
		contents, err := ioutil.ReadFile(ipFilePath1)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("dummy"))

		ipFilePath2 := filepath.Join(tmpDir, "mynet", "2001:db8:1::2")
		contents, err = ioutil.ReadFile(ipFilePath2)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("dummy"))

		lastFilePath1 := filepath.Join(tmpDir, "mynet", "last_reserved_ip.0")
		contents, err = ioutil.ReadFile(lastFilePath1)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("10.1.2.2"))

		lastFilePath2 := filepath.Join(tmpDir, "mynet", "last_reserved_ip.1")
		contents, err = ioutil.ReadFile(lastFilePath2)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("2001:db8:1::2"))
		// Release the IP
		err = testutils.CmdDelWithResult(nspath, ifname, func() error {
			return cmdDel(args)
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = os.Stat(ipFilePath1)
		Expect(err).To(HaveOccurred())
		_, err = os.Stat(ipFilePath2)
		Expect(err).To(HaveOccurred())
	})

	It("doesn't error when passed an unknown ID on DEL", func() {
		const ifname string = "eth0"
		const nspath string = "/some/where"

		tmpDir, err := ioutil.TempDir("", "host_local_artifacts")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		conf := fmt.Sprintf(`{
	"cniVersion": "0.3.0",
	"name": "mynet",
	"type": "ipvlan",
	"master": "foo0",
	"ipam": {
		"type": "host-local",
		"subnet": "10.1.2.0/24",
		"dataDir": "%s"
	}
}`, tmpDir)

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       nspath,
			IfName:      ifname,
			StdinData:   []byte(conf),
		}

		// Release the IP
		err = testutils.CmdDelWithResult(nspath, ifname, func() error {
			return cmdDel(args)
		})
		Expect(err).NotTo(HaveOccurred())
	})

	It("allocates and releases an address with ADD/DEL and 0.1.0 config", func() {
		const ifname string = "eth0"
		const nspath string = "/some/where"

		tmpDir, err := ioutil.TempDir("", "host_local_artifacts")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		err = ioutil.WriteFile(filepath.Join(tmpDir, "resolv.conf"), []byte("nameserver 192.0.2.3"), 0644)
		Expect(err).NotTo(HaveOccurred())

		conf := fmt.Sprintf(`{
	"cniVersion": "0.1.0",
	"name": "mynet",
	"type": "ipvlan",
	"master": "foo0",
	"ipam": {
		"type": "host-local",
		"subnet": "10.1.2.0/24",
		"dataDir": "%s",
		"resolvConf": "%s/resolv.conf"
	}
}`, tmpDir, tmpDir)

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       nspath,
			IfName:      ifname,
			StdinData:   []byte(conf),
		}

		// Allocate the IP
		r, raw, err := testutils.CmdAddWithResult(nspath, ifname, []byte(conf), func() error {
			return cmdAdd(args)
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Index(string(raw), "\"ip4\":")).Should(BeNumerically(">", 0))

		result, err := types020.GetResult(r)
		Expect(err).NotTo(HaveOccurred())

		expectedAddress, err := types.ParseCIDR("10.1.2.2/24")
		Expect(err).NotTo(HaveOccurred())
		expectedAddress.IP = expectedAddress.IP.To16()
		Expect(result.IP4.IP).To(Equal(*expectedAddress))
		Expect(result.IP4.Gateway).To(Equal(net.ParseIP("10.1.2.1")))

		ipFilePath := filepath.Join(tmpDir, "mynet", "10.1.2.2")
		contents, err := ioutil.ReadFile(ipFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("dummy"))

		lastFilePath := filepath.Join(tmpDir, "mynet", "last_reserved_ip.0")
		contents, err = ioutil.ReadFile(lastFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("10.1.2.2"))

		Expect(result.DNS).To(Equal(types.DNS{Nameservers: []string{"192.0.2.3"}}))

		// Release the IP
		err = testutils.CmdDelWithResult(nspath, ifname, func() error {
			return cmdDel(args)
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = os.Stat(ipFilePath)
		Expect(err).To(HaveOccurred())
	})

	It("ignores whitespace in disk files", func() {
		const ifname string = "eth0"
		const nspath string = "/some/where"

		tmpDir, err := ioutil.TempDir("", "host_local_artifacts")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		conf := fmt.Sprintf(`{
	"cniVersion": "0.3.1",
	"name": "mynet",
	"type": "ipvlan",
	"master": "foo0",
	"ipam": {
		"type": "host-local",
		"subnet": "10.1.2.0/24",
		"dataDir": "%s"
	}
}`, tmpDir)

		args := &skel.CmdArgs{
			ContainerID: "   dummy\n ",
			Netns:       nspath,
			IfName:      ifname,
			StdinData:   []byte(conf),
		}

		// Allocate the IP
		r, _, err := testutils.CmdAddWithResult(nspath, ifname, []byte(conf), func() error {
			return cmdAdd(args)
		})
		Expect(err).NotTo(HaveOccurred())

		result, err := current.GetResult(r)
		Expect(err).NotTo(HaveOccurred())

		ipFilePath := filepath.Join(tmpDir, "mynet", result.IPs[0].Address.IP.String())
		contents, err := ioutil.ReadFile(ipFilePath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("dummy"))

		// Release the IP
		err = testutils.CmdDelWithResult(nspath, ifname, func() error {
			return cmdDel(args)
		})
		Expect(err).NotTo(HaveOccurred())

		_, err = os.Stat(ipFilePath)
		Expect(err).To(HaveOccurred())
	})

	It("does not output an error message upon initial subnet creation", func() {
		const ifname string = "eth0"
		const nspath string = "/some/where"

		tmpDir, err := ioutil.TempDir("", "host_local_artifacts")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		conf := fmt.Sprintf(`{
	"cniVersion": "0.2.0",
	"name": "mynet",
	"type": "ipvlan",
	"master": "foo0",
	"ipam": {
		"type": "host-local",
		"subnet": "10.1.2.0/24",
		"dataDir": "%s"
	}
}`, tmpDir)

		args := &skel.CmdArgs{
			ContainerID: "testing",
			Netns:       nspath,
			IfName:      ifname,
			StdinData:   []byte(conf),
		}

		// Allocate the IP
		_, out, err := testutils.CmdAddWithResult(nspath, ifname, []byte(conf), func() error {
			return cmdAdd(args)
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Index(string(out), "Error retriving last reserved ip")).To(Equal(-1))
	})

	It("allocates a custom IP when requested by config args", func() {
		const ifname string = "eth0"
		const nspath string = "/some/where"

		tmpDir, err := ioutil.TempDir("", "host_local_artifacts")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		conf := fmt.Sprintf(`{
	"cniVersion": "0.3.1",
	"name": "mynet",
	"type": "ipvlan",
	"master": "foo0",
	"ipam": {
		"type": "host-local",
		"dataDir": "%s",
		"ranges": [
			[{ "subnet": "10.1.2.0/24" }]
		]
	},
	"args": {
		"cni": {
			"ips": ["10.1.2.88"]
		}
	}
}`, tmpDir)

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       nspath,
			IfName:      ifname,
			StdinData:   []byte(conf),
		}

		// Allocate the IP
		r, _, err := testutils.CmdAddWithResult(nspath, ifname, []byte(conf), func() error {
			return cmdAdd(args)
		})
		Expect(err).NotTo(HaveOccurred())
		result, err := current.GetResult(r)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IPs).To(HaveLen(1))
		Expect(result.IPs[0].Address.IP).To(Equal(net.ParseIP("10.1.2.88")))
	})

	It("allocates custom IPs from multiple ranges", func() {
		const ifname string = "eth0"
		const nspath string = "/some/where"

		tmpDir, err := ioutil.TempDir("", "host_local_artifacts")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		err = ioutil.WriteFile(filepath.Join(tmpDir, "resolv.conf"), []byte("nameserver 192.0.2.3"), 0644)
		Expect(err).NotTo(HaveOccurred())

		conf := fmt.Sprintf(`{
	"cniVersion": "0.3.1",
	"name": "mynet",
	"type": "ipvlan",
	"master": "foo0",
	"ipam": {
		"type": "host-local",
		"dataDir": "%s",
		"ranges": [
			[{ "subnet": "10.1.2.0/24" }],
			[{ "subnet": "10.1.3.0/24" }]
		]
	},
	"args": {
		"cni": {
			"ips": ["10.1.2.88", "10.1.3.77"]
		}
	}
}`, tmpDir)

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       nspath,
			IfName:      ifname,
			StdinData:   []byte(conf),
		}

		// Allocate the IP
		r, _, err := testutils.CmdAddWithResult(nspath, ifname, []byte(conf), func() error {
			return cmdAdd(args)
		})
		Expect(err).NotTo(HaveOccurred())
		result, err := current.GetResult(r)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IPs).To(HaveLen(2))
		Expect(result.IPs[0].Address.IP).To(Equal(net.ParseIP("10.1.2.88")))
		Expect(result.IPs[1].Address.IP).To(Equal(net.ParseIP("10.1.3.77")))
	})

	It("allocates custom IPs from multiple protocols", func() {
		const ifname string = "eth0"
		const nspath string = "/some/where"

		tmpDir, err := ioutil.TempDir("", "host_local_artifacts")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		err = ioutil.WriteFile(filepath.Join(tmpDir, "resolv.conf"), []byte("nameserver 192.0.2.3"), 0644)
		Expect(err).NotTo(HaveOccurred())

		conf := fmt.Sprintf(`{
	"cniVersion": "0.3.1",
	"name": "mynet",
	"type": "ipvlan",
	"master": "foo0",
	"ipam": {
		"type": "host-local",
		"dataDir": "%s",
		"ranges": [
			[{"subnet":"172.16.1.0/24"}, { "subnet": "10.1.2.0/24" }],
			[{ "subnet": "2001:db8:1::/24" }]
		]
	},
	"args": {
		"cni": {
			"ips": ["10.1.2.88", "2001:db8:1::999"]
		}
	}
}`, tmpDir)

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       nspath,
			IfName:      ifname,
			StdinData:   []byte(conf),
		}

		// Allocate the IP
		r, _, err := testutils.CmdAddWithResult(nspath, ifname, []byte(conf), func() error {
			return cmdAdd(args)
		})
		Expect(err).NotTo(HaveOccurred())
		result, err := current.GetResult(r)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IPs).To(HaveLen(2))
		Expect(result.IPs[0].Address.IP).To(Equal(net.ParseIP("10.1.2.88")))
		Expect(result.IPs[1].Address.IP).To(Equal(net.ParseIP("2001:db8:1::999")))
	})

	It("fails if a requested custom IP is not used", func() {
		const ifname string = "eth0"
		const nspath string = "/some/where"

		tmpDir, err := ioutil.TempDir("", "host_local_artifacts")
		Expect(err).NotTo(HaveOccurred())
		defer os.RemoveAll(tmpDir)

		conf := fmt.Sprintf(`{
	"cniVersion": "0.3.1",
	"name": "mynet",
	"type": "ipvlan",
	"master": "foo0",
	"ipam": {
		"type": "host-local",
		"dataDir": "%s",
		"ranges": [
			[{ "subnet": "10.1.2.0/24" }],
			[{ "subnet": "10.1.3.0/24" }]
		]
	},
	"args": {
		"cni": {
			"ips": ["10.1.2.88", "10.1.2.77"]
		}
	}
}`, tmpDir)

		args := &skel.CmdArgs{
			ContainerID: "dummy",
			Netns:       nspath,
			IfName:      ifname,
			StdinData:   []byte(conf),
		}

		// Allocate the IP
		_, _, err = testutils.CmdAddWithResult(nspath, ifname, []byte(conf), func() error {
			return cmdAdd(args)
		})
		Expect(err).To(HaveOccurred())
		// Need to match prefix, because ordering is not guaranteed
		Expect(err.Error()).To(HavePrefix("failed to allocate all requested IPs: 10.1.2."))
	})
})

func mustCIDR(s string) net.IPNet {
	ip, n, err := net.ParseCIDR(s)
	n.IP = ip
	if err != nil {
		Fail(err.Error())
	}

	return *n
}
