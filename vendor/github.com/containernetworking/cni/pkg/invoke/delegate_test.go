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

package invoke_test

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/pkg/invoke"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/plugins/test/noop/debug"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Delegate", func() {
	var (
		pluginName     string
		netConf        []byte
		debugFileName  string
		debugBehavior  *debug.Debug
		expectedResult *current.Result
		ctx            context.Context
	)

	BeforeEach(func() {
		netConf, _ = json.Marshal(map[string]string{
			"name":       "delegate-test",
			"cniVersion": "0.4.0",
		})

		expectedResult = &current.Result{
			CNIVersion: "0.4.0",
			IPs: []*current.IPConfig{
				{
					Version: "4",
					Address: net.IPNet{
						IP:   net.ParseIP("10.1.2.3"),
						Mask: net.CIDRMask(24, 32),
					},
				},
			},
		}
		expectedResultBytes, _ := json.Marshal(expectedResult)

		debugFile, err := ioutil.TempFile("", "cni_debug")
		Expect(err).NotTo(HaveOccurred())
		Expect(debugFile.Close()).To(Succeed())
		debugFileName = debugFile.Name()
		debugBehavior = &debug.Debug{
			ReportResult: string(expectedResultBytes),
		}
		Expect(debugBehavior.WriteDebug(debugFileName)).To(Succeed())
		pluginName = "noop"
		ctx = context.TODO()
		os.Setenv("CNI_ARGS", "DEBUG="+debugFileName)
		os.Setenv("CNI_PATH", filepath.Dir(pathToPlugin))
		os.Setenv("CNI_NETNS", "/tmp/some/netns/path")
		os.Setenv("CNI_IFNAME", "eth7")
		os.Setenv("CNI_CONTAINERID", "container")
	})

	AfterEach(func() {
		os.RemoveAll(debugFileName)

		for _, k := range []string{"CNI_COMMAND", "CNI_ARGS", "CNI_PATH", "CNI_NETNS", "CNI_IFNAME"} {
			os.Unsetenv(k)
		}
	})

	Describe("DelegateAdd", func() {
		BeforeEach(func() {
			os.Setenv("CNI_COMMAND", "ADD")
		})

		It("finds and execs the named plugin", func() {
			result, err := invoke.DelegateAdd(ctx, pluginName, netConf, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(result).To(Equal(expectedResult))

			pluginInvocation, err := debug.ReadDebug(debugFileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pluginInvocation.Command).To(Equal("ADD"))
			Expect(pluginInvocation.CmdArgs.IfName).To(Equal("eth7"))
		})

		Context("if the ADD delegation runs on an existing non-ADD command, ", func() {
			BeforeEach(func() {
				os.Setenv("CNI_COMMAND", "NOPE")
			})

			It("aborts and returns a useful error", func() {
				result, err := invoke.DelegateAdd(ctx, pluginName, netConf, nil)
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(expectedResult))

				pluginInvocation, err := debug.ReadDebug(debugFileName)
				Expect(err).NotTo(HaveOccurred())
				Expect(pluginInvocation.Command).To(Equal("ADD"))
				Expect(pluginInvocation.CmdArgs.IfName).To(Equal("eth7"))

				// check the original env
				Expect(os.Getenv("CNI_COMMAND")).To(Equal("NOPE"))
			})
		})

		Context("when the plugin cannot be found", func() {
			BeforeEach(func() {
				pluginName = "non-existent-plugin"
			})

			It("returns a useful error", func() {
				_, err := invoke.DelegateAdd(ctx, pluginName, netConf, nil)
				Expect(err).To(MatchError(HavePrefix("failed to find plugin")))
			})
		})
	})

	Describe("DelegateCheck", func() {
		BeforeEach(func() {
			os.Setenv("CNI_COMMAND", "CHECK")
		})

		It("finds and execs the named plugin", func() {
			err := invoke.DelegateCheck(ctx, pluginName, netConf, nil)
			Expect(err).NotTo(HaveOccurred())

			pluginInvocation, err := debug.ReadDebug(debugFileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pluginInvocation.Command).To(Equal("CHECK"))
			Expect(pluginInvocation.CmdArgs.IfName).To(Equal("eth7"))
		})

		Context("if the CHECK delegation runs on an existing non-CHECK command", func() {
			BeforeEach(func() {
				os.Setenv("CNI_COMMAND", "NOPE")
			})

			It("aborts and returns a useful error", func() {
				err := invoke.DelegateCheck(ctx, pluginName, netConf, nil)
				Expect(err).NotTo(HaveOccurred())

				pluginInvocation, err := debug.ReadDebug(debugFileName)
				Expect(err).NotTo(HaveOccurred())
				Expect(pluginInvocation.Command).To(Equal("CHECK"))
				Expect(pluginInvocation.CmdArgs.IfName).To(Equal("eth7"))

				// check the original env
				Expect(os.Getenv("CNI_COMMAND")).To(Equal("NOPE"))
			})
		})

		Context("when the plugin cannot be found", func() {
			BeforeEach(func() {
				pluginName = "non-existent-plugin"
			})

			It("returns a useful error", func() {
				err := invoke.DelegateCheck(ctx, pluginName, netConf, nil)
				Expect(err).To(MatchError(HavePrefix("failed to find plugin")))
			})
		})
	})

	Describe("DelegateDel", func() {
		BeforeEach(func() {
			os.Setenv("CNI_COMMAND", "DEL")
		})

		It("finds and execs the named plugin", func() {
			err := invoke.DelegateDel(ctx, pluginName, netConf, nil)
			Expect(err).NotTo(HaveOccurred())

			pluginInvocation, err := debug.ReadDebug(debugFileName)
			Expect(err).NotTo(HaveOccurred())
			Expect(pluginInvocation.Command).To(Equal("DEL"))
			Expect(pluginInvocation.CmdArgs.IfName).To(Equal("eth7"))
		})

		Context("if the DEL delegation runs on an existing non-DEL command", func() {
			BeforeEach(func() {
				os.Setenv("CNI_COMMAND", "NOPE")
			})

			It("aborts and returns a useful error", func() {
				err := invoke.DelegateDel(ctx, pluginName, netConf, nil)
				Expect(err).NotTo(HaveOccurred())

				pluginInvocation, err := debug.ReadDebug(debugFileName)
				Expect(err).NotTo(HaveOccurred())
				Expect(pluginInvocation.Command).To(Equal("DEL"))
				Expect(pluginInvocation.CmdArgs.IfName).To(Equal("eth7"))

				// check the original env
				Expect(os.Getenv("CNI_COMMAND")).To(Equal("NOPE"))
			})
		})

		Context("when the plugin cannot be found", func() {
			BeforeEach(func() {
				pluginName = "non-existent-plugin"
			})

			It("returns a useful error", func() {
				err := invoke.DelegateDel(ctx, pluginName, netConf, nil)
				Expect(err).To(MatchError(HavePrefix("failed to find plugin")))
			})
		})
	})
})
