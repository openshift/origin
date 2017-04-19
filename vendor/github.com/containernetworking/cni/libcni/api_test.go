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

package libcni_test

import (
	"io/ioutil"
	"net"
	"path/filepath"

	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/skel"
	"github.com/containernetworking/cni/pkg/types"
	noop_debug "github.com/containernetworking/cni/plugins/test/noop/debug"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Invoking the plugin", func() {
	var (
		debugFilePath string
		debug         *noop_debug.Debug
		cniBinPath    string
		pluginConfig  string
		cniConfig     libcni.CNIConfig
		netConfig     *libcni.NetworkConfig
		runtimeConfig *libcni.RuntimeConf

		expectedCmdArgs skel.CmdArgs
	)

	BeforeEach(func() {
		debugFile, err := ioutil.TempFile("", "cni_debug")
		Expect(err).NotTo(HaveOccurred())
		Expect(debugFile.Close()).To(Succeed())
		debugFilePath = debugFile.Name()

		debug = &noop_debug.Debug{
			ReportResult: `{ "ip4": { "ip": "10.1.2.3/24" }, "dns": {} }`,
		}
		Expect(debug.WriteDebug(debugFilePath)).To(Succeed())

		cniBinPath = filepath.Dir(pathToPlugin)
		pluginConfig = `{ "type": "noop", "some-key": "some-value" }`
		cniConfig = libcni.CNIConfig{Path: []string{cniBinPath}}
		netConfig = &libcni.NetworkConfig{
			Network: &types.NetConf{
				Type: "noop",
			},
			Bytes: []byte(pluginConfig),
		}
		runtimeConfig = &libcni.RuntimeConf{
			ContainerID: "some-container-id",
			NetNS:       "/some/netns/path",
			IfName:      "some-eth0",
			Args:        [][2]string{[2]string{"DEBUG", debugFilePath}},
		}

		expectedCmdArgs = skel.CmdArgs{
			ContainerID: "some-container-id",
			Netns:       "/some/netns/path",
			IfName:      "some-eth0",
			Args:        "DEBUG=" + debugFilePath,
			Path:        cniBinPath,
			StdinData:   []byte(pluginConfig),
		}
	})

	Describe("AddNetwork", func() {
		It("executes the plugin with command ADD", func() {
			result, err := cniConfig.AddNetwork(netConfig, runtimeConfig)
			Expect(err).NotTo(HaveOccurred())

			Expect(result).To(Equal(&types.Result{
				IP4: &types.IPConfig{
					IP: net.IPNet{
						IP:   net.ParseIP("10.1.2.3"),
						Mask: net.IPv4Mask(255, 255, 255, 0),
					},
				},
			}))

			debug, err := noop_debug.ReadDebug(debugFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(debug.Command).To(Equal("ADD"))
			Expect(debug.CmdArgs).To(Equal(expectedCmdArgs))
		})

		Context("when finding the plugin fails", func() {
			BeforeEach(func() {
				netConfig.Network.Type = "does-not-exist"
			})

			It("returns the error", func() {
				_, err := cniConfig.AddNetwork(netConfig, runtimeConfig)
				Expect(err).To(MatchError(ContainSubstring(`failed to find plugin "does-not-exist"`)))
			})
		})

		Context("when the plugin errors", func() {
			BeforeEach(func() {
				debug.ReportError = "plugin error: banana"
				Expect(debug.WriteDebug(debugFilePath)).To(Succeed())
			})
			It("unmarshals and returns the error", func() {
				result, err := cniConfig.AddNetwork(netConfig, runtimeConfig)
				Expect(result).To(BeNil())
				Expect(err).To(MatchError("plugin error: banana"))
			})
		})
	})

	Describe("DelNetwork", func() {
		It("executes the plugin with command DEL", func() {
			err := cniConfig.DelNetwork(netConfig, runtimeConfig)
			Expect(err).NotTo(HaveOccurred())

			debug, err := noop_debug.ReadDebug(debugFilePath)
			Expect(err).NotTo(HaveOccurred())
			Expect(debug.Command).To(Equal("DEL"))
			Expect(debug.CmdArgs).To(Equal(expectedCmdArgs))
		})

		Context("when finding the plugin fails", func() {
			BeforeEach(func() {
				netConfig.Network.Type = "does-not-exist"
			})

			It("returns the error", func() {
				err := cniConfig.DelNetwork(netConfig, runtimeConfig)
				Expect(err).To(MatchError(ContainSubstring(`failed to find plugin "does-not-exist"`)))
			})
		})

		Context("when the plugin errors", func() {
			BeforeEach(func() {
				debug.ReportError = "plugin error: banana"
				Expect(debug.WriteDebug(debugFilePath)).To(Succeed())
			})
			It("unmarshals and returns the error", func() {
				err := cniConfig.DelNetwork(netConfig, runtimeConfig)
				Expect(err).To(MatchError("plugin error: banana"))
			})
		})
	})

	Describe("GetVersionInfo", func() {
		It("executes the plugin with the command VERSION", func() {
			versionInfo, err := cniConfig.GetVersionInfo("noop")
			Expect(err).NotTo(HaveOccurred())

			Expect(versionInfo).NotTo(BeNil())
			Expect(versionInfo.SupportedVersions()).To(Equal([]string{
				"0.-42.0", "0.1.0", "0.2.0",
			}))
		})

		Context("when finding the plugin fails", func() {
			It("returns the error", func() {
				_, err := cniConfig.GetVersionInfo("does-not-exist")
				Expect(err).To(MatchError(ContainSubstring(`failed to find plugin "does-not-exist"`)))
			})
		})
	})
})
