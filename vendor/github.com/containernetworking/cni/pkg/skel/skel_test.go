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

package skel

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/containernetworking/cni/pkg/types"
	"github.com/containernetworking/cni/pkg/types/current"
	"github.com/containernetworking/cni/pkg/version"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

type fakeCmd struct {
	CallCount int
	Returns   struct {
		Error error
	}
	Received struct {
		CmdArgs *CmdArgs
	}
}

func (c *fakeCmd) Func(args *CmdArgs) error {
	c.CallCount++
	c.Received.CmdArgs = args
	return c.Returns.Error
}

var _ = Describe("dispatching to the correct callback", func() {
	var (
		environment              map[string]string
		stdinData                string
		stdout, stderr           *bytes.Buffer
		cmdAdd, cmdCheck, cmdDel *fakeCmd
		dispatch                 *dispatcher
		expectedCmdArgs          *CmdArgs
		versionInfo              version.PluginInfo
	)

	BeforeEach(func() {
		environment = map[string]string{
			"CNI_COMMAND":     "ADD",
			"CNI_CONTAINERID": "some-container-id",
			"CNI_NETNS":       "/some/netns/path",
			"CNI_IFNAME":      "eth0",
			"CNI_ARGS":        "some;extra;args",
			"CNI_PATH":        "/some/cni/path",
		}

		stdinData = `{ "name":"skel-test", "some": "config", "cniVersion": "9.8.7" }`
		stdout = &bytes.Buffer{}
		stderr = &bytes.Buffer{}
		versionInfo = version.PluginSupports("9.8.7")
		dispatch = &dispatcher{
			Getenv: func(key string) string { return environment[key] },
			Stdin:  strings.NewReader(stdinData),
			Stdout: stdout,
			Stderr: stderr,
		}
		cmdAdd = &fakeCmd{}
		cmdCheck = &fakeCmd{}
		cmdDel = &fakeCmd{}
		expectedCmdArgs = &CmdArgs{
			ContainerID: "some-container-id",
			Netns:       "/some/netns/path",
			IfName:      "eth0",
			Args:        "some;extra;args",
			Path:        "/some/cni/path",
			StdinData:   []byte(stdinData),
		}
	})

	var envVarChecker = func(envVar string, isRequired bool) {
		delete(environment, envVar)

		err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
		if isRequired {
			Expect(err).To(Equal(&types.Error{
				Code: 100,
				Msg:  "required env variables [" + envVar + "] missing",
			}))
		} else {
			Expect(err).NotTo(HaveOccurred())
		}
	}

	Context("when the CNI_COMMAND is ADD", func() {
		It("extracts env vars and stdin data and calls cmdAdd", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(cmdAdd.CallCount).To(Equal(1))
			Expect(cmdCheck.CallCount).To(Equal(0))
			Expect(cmdDel.CallCount).To(Equal(0))
			Expect(cmdAdd.Received.CmdArgs).To(Equal(expectedCmdArgs))
		})

		It("does not call cmdCheck or cmdDel", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(cmdCheck.CallCount).To(Equal(0))
			Expect(cmdDel.CallCount).To(Equal(0))
		})

		DescribeTable("required / optional env vars", envVarChecker,
			Entry("command", "CNI_COMMAND", true),
			Entry("container id", "CNI_CONTAINERID", true),
			Entry("net ns", "CNI_NETNS", true),
			Entry("if name", "CNI_IFNAME", true),
			Entry("args", "CNI_ARGS", false),
			Entry("path", "CNI_PATH", true),
		)

		Context("when multiple required env vars are missing", func() {
			BeforeEach(func() {
				delete(environment, "CNI_NETNS")
				delete(environment, "CNI_IFNAME")
				delete(environment, "CNI_PATH")
			})

			It("reports that all of them are missing, not just the first", func() {
				err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
				Expect(err).To(HaveOccurred())

				Expect(err).To(Equal(&types.Error{
					Code: 100,
					Msg:  "required env variables [CNI_NETNS,CNI_IFNAME,CNI_PATH] missing",
				}))
			})
		})

		Context("when the stdin data is missing the required cniVersion config", func() {
			BeforeEach(func() {
				dispatch.Stdin = strings.NewReader(`{ "name": "skel-test", "some": "config" }`)
			})

			Context("when the plugin supports version 0.1.0", func() {
				BeforeEach(func() {
					versionInfo = version.PluginSupports("0.1.0")
					expectedCmdArgs.StdinData = []byte(`{ "name": "skel-test", "some": "config" }`)
				})

				It("infers the config is 0.1.0 and calls the cmdAdd callback", func() {

					err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
					Expect(err).NotTo(HaveOccurred())

					Expect(cmdAdd.CallCount).To(Equal(1))
					Expect(cmdAdd.Received.CmdArgs).To(Equal(expectedCmdArgs))
				})
			})

			Context("when the plugin does not support 0.1.0", func() {
				BeforeEach(func() {
					versionInfo = version.PluginSupports("4.3.2")
				})

				It("immediately returns a useful error", func() {
					err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
					Expect(err.Code).To(Equal(types.ErrIncompatibleCNIVersion)) // see https://github.com/containernetworking/cni/blob/master/SPEC.md#well-known-error-codes
					Expect(err.Msg).To(Equal("incompatible CNI versions"))
					Expect(err.Details).To(Equal(`config is "0.1.0", plugin supports ["4.3.2"]`))
				})

				It("does not call either callback", func() {
					dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
					Expect(cmdAdd.CallCount).To(Equal(0))
					Expect(cmdCheck.CallCount).To(Equal(0))
					Expect(cmdDel.CallCount).To(Equal(0))
				})
			})
		})
	})

	Context("when the CNI_COMMAND is CHECK", func() {
		BeforeEach(func() {
			environment["CNI_COMMAND"] = "CHECK"
		})

		It("extracts env vars and stdin data and calls cmdCheck", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(cmdAdd.CallCount).To(Equal(0))
			Expect(cmdCheck.CallCount).To(Equal(1))
			Expect(cmdDel.CallCount).To(Equal(0))
			Expect(cmdCheck.Received.CmdArgs).To(Equal(expectedCmdArgs))
		})

		It("does not call cmdAdd or cmdDel", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(cmdAdd.CallCount).To(Equal(0))
			Expect(cmdDel.CallCount).To(Equal(0))
		})

		DescribeTable("required / optional env vars", envVarChecker,
			Entry("command", "CNI_COMMAND", true),
			Entry("container id", "CNI_CONTAINERID", true),
			Entry("net ns", "CNI_NETNS", true),
			Entry("if name", "CNI_IFNAME", true),
			Entry("args", "CNI_ARGS", false),
			Entry("path", "CNI_PATH", true),
		)

		Context("when multiple required env vars are missing", func() {
			BeforeEach(func() {
				delete(environment, "CNI_NETNS")
				delete(environment, "CNI_IFNAME")
				delete(environment, "CNI_PATH")
			})

			It("reports that all of them are missing, not just the first", func() {
				err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
				Expect(err).To(HaveOccurred())

				Expect(err).To(Equal(&types.Error{
					Code: 100,
					Msg:  "required env variables [CNI_NETNS,CNI_IFNAME,CNI_PATH] missing",
				}))
			})
		})

		Context("when cniVersion is less than 0.4.0", func() {
			It("immediately returns a useful error", func() {
				dispatch.Stdin = strings.NewReader(`{ "name": "skel-test", "cniVersion": "0.3.0", "some": "config" }`)
				err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
				Expect(err.Code).To(Equal(types.ErrIncompatibleCNIVersion)) // see https://github.com/containernetworking/cni/blob/master/SPEC.md#well-known-error-codes
				Expect(err.Msg).To(Equal("config version does not allow CHECK"))
				Expect(cmdAdd.CallCount).To(Equal(0))
				Expect(cmdCheck.CallCount).To(Equal(0))
				Expect(cmdDel.CallCount).To(Equal(0))
			})
		})

		Context("when plugin does not support 0.4.0", func() {
			It("immediately returns a useful error", func() {
				dispatch.Stdin = strings.NewReader(`{ "name": "skel-test", "cniVersion": "0.4.0", "some": "config" }`)
				versionInfo = version.PluginSupports("0.1.0", "0.2.0", "0.3.0")
				err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
				Expect(err.Code).To(Equal(types.ErrIncompatibleCNIVersion)) // see https://github.com/containernetworking/cni/blob/master/SPEC.md#well-known-error-codes
				Expect(err.Msg).To(Equal("plugin version does not allow CHECK"))
				Expect(cmdAdd.CallCount).To(Equal(0))
				Expect(cmdCheck.CallCount).To(Equal(0))
				Expect(cmdDel.CallCount).To(Equal(0))
			})
		})

		Context("when the config has a bad version", func() {
			It("immediately returns a useful error", func() {
				dispatch.Stdin = strings.NewReader(`{ "cniVersion": "adsfsadf", "some": "config" }`)
				versionInfo = version.PluginSupports("0.1.0", "0.2.0", "0.3.0")
				err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
				Expect(err.Code).To(Equal(uint(100)))
				Expect(cmdAdd.CallCount).To(Equal(0))
				Expect(cmdCheck.CallCount).To(Equal(0))
				Expect(cmdDel.CallCount).To(Equal(0))
			})
		})

		Context("when the plugin has a bad version", func() {
			It("immediately returns a useful error", func() {
				dispatch.Stdin = strings.NewReader(`{ "cniVersion": "0.4.0", "some": "config" }`)
				versionInfo = version.PluginSupports("0.1.0", "0.2.0", "adsfasdf")
				err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
				Expect(err.Code).To(Equal(uint(100)))
				Expect(cmdAdd.CallCount).To(Equal(0))
				Expect(cmdCheck.CallCount).To(Equal(0))
				Expect(cmdDel.CallCount).To(Equal(0))
			})
		})
	})

	Context("when the CNI_COMMAND is DEL", func() {
		BeforeEach(func() {
			environment["CNI_COMMAND"] = "DEL"
		})

		It("calls cmdDel with the env vars and stdin data", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(cmdDel.CallCount).To(Equal(1))
			Expect(cmdDel.Received.CmdArgs).To(Equal(expectedCmdArgs))
		})

		It("does not call cmdAdd", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(cmdAdd.CallCount).To(Equal(0))
		})

		DescribeTable("required / optional env vars", envVarChecker,
			Entry("command", "CNI_COMMAND", true),
			Entry("container id", "CNI_CONTAINERID", true),
			Entry("net ns", "CNI_NETNS", false),
			Entry("if name", "CNI_IFNAME", true),
			Entry("args", "CNI_ARGS", false),
			Entry("path", "CNI_PATH", true),
		)
	})

	Context("when the CNI_COMMAND is VERSION", func() {
		BeforeEach(func() {
			environment["CNI_COMMAND"] = "VERSION"
		})

		It("prints the version to stdout", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(MatchJSON(fmt.Sprintf(`{
				"cniVersion": "%s",
				"supportedVersions": ["9.8.7"]
			}`, current.ImplementedSpecVersion)))
		})

		It("does not call cmdAdd or cmdDel", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(cmdAdd.CallCount).To(Equal(0))
			Expect(cmdDel.CallCount).To(Equal(0))
		})

		DescribeTable("VERSION does not need the usual env vars", envVarChecker,
			Entry("command", "CNI_COMMAND", true),
			Entry("container id", "CNI_CONTAINERID", false),
			Entry("net ns", "CNI_NETNS", false),
			Entry("if name", "CNI_IFNAME", false),
			Entry("args", "CNI_ARGS", false),
			Entry("path", "CNI_PATH", false),
		)

		It("does not read from Stdin", func() {
			r := &BadReader{}
			dispatch.Stdin = r

			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).NotTo(HaveOccurred())
			Expect(r.ReadCount).To(Equal(0))
			Expect(stdout).To(MatchJSON(fmt.Sprintf(`{
				"cniVersion": "%s",
				"supportedVersions": ["9.8.7"]
			}`, current.ImplementedSpecVersion)))
		})
	})

	Context("when the CNI_COMMAND is unrecognized", func() {
		BeforeEach(func() {
			environment["CNI_COMMAND"] = "NOPE"
		})

		It("does not call any cmd callback", func() {
			dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(cmdAdd.CallCount).To(Equal(0))
			Expect(cmdDel.CallCount).To(Equal(0))
		})

		It("returns an error", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).To(Equal(&types.Error{
				Code: 100,
				Msg:  "unknown CNI_COMMAND: NOPE",
			}))
		})

		It("prints the about string when the command is blank", func() {
			environment["CNI_COMMAND"] = ""
			dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "test framework v42")
			Expect(stderr.String()).To(ContainSubstring("test framework v42"))
		})
	})

	Context("when the CNI_COMMAND is missing", func() {
		It("prints the about string to stderr", func() {
			environment = map[string]string{}
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "AWESOME PLUGIN")
			Expect(err).NotTo(HaveOccurred())

			Expect(cmdAdd.CallCount).To(Equal(0))
			Expect(cmdDel.CallCount).To(Equal(0))
			log := stderr.String()
			Expect(log).To(Equal("AWESOME PLUGIN\n"))
		})

		It("fails if there is no about string", func() {
			environment = map[string]string{}
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")
			Expect(err).To(HaveOccurred())

			Expect(cmdAdd.CallCount).To(Equal(0))
			Expect(cmdDel.CallCount).To(Equal(0))
			Expect(err).To(Equal(&types.Error{
				Code: 100,
				Msg:  "required env variables [CNI_COMMAND] missing",
			}))
		})
	})

	Context("when stdin cannot be read", func() {
		BeforeEach(func() {
			dispatch.Stdin = &BadReader{}
		})

		It("does not call any cmd callback", func() {
			dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(cmdAdd.CallCount).To(Equal(0))
			Expect(cmdDel.CallCount).To(Equal(0))
		})

		It("wraps and returns the error", func() {
			err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

			Expect(err).To(Equal(&types.Error{
				Code: 100,
				Msg:  "error reading from stdin: banana",
			}))
		})
	})

	Context("when the callback returns an error", func() {
		Context("when it is a typed Error", func() {
			BeforeEach(func() {
				cmdAdd.Returns.Error = &types.Error{
					Code: 1234,
					Msg:  "insufficient something",
				}
			})

			It("returns the error as-is", func() {
				err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

				Expect(err).To(Equal(&types.Error{
					Code: 1234,
					Msg:  "insufficient something",
				}))
			})
		})

		Context("when it is an unknown error", func() {
			BeforeEach(func() {
				cmdAdd.Returns.Error = errors.New("potato")
			})

			It("wraps and returns the error", func() {
				err := dispatch.pluginMain(cmdAdd.Func, cmdCheck.Func, cmdDel.Func, versionInfo, "")

				Expect(err).To(Equal(&types.Error{
					Code: 100,
					Msg:  "potato",
				}))
			})
		})
	})
})

// BadReader is an io.Reader which always errors
type BadReader struct {
	Error     error
	ReadCount int
}

func (r *BadReader) Read(buffer []byte) (int, error) {
	r.ReadCount++
	if r.Error != nil {
		return 0, r.Error
	}
	return 0, errors.New("banana")
}

func (r *BadReader) Close() error {
	return nil
}
