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

package version_test

import (
	"github.com/containernetworking/cni/pkg/version"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Decoding versions reported by a plugin", func() {
	var (
		decoder       *version.PluginDecoder
		versionStdout []byte
	)

	BeforeEach(func() {
		decoder = &version.PluginDecoder{}
		versionStdout = []byte(`{
			"cniVersion": "some-library-version",
			"supportedVersions": [ "some-version", "some-other-version" ]
		}`)
	})

	It("returns a PluginInfo that represents the given json bytes", func() {
		pluginInfo, err := decoder.Decode(versionStdout)
		Expect(err).NotTo(HaveOccurred())
		Expect(pluginInfo).NotTo(BeNil())
		Expect(pluginInfo.SupportedVersions()).To(Equal([]string{
			"some-version",
			"some-other-version",
		}))
	})

	Context("when the bytes cannot be decoded as json", func() {
		BeforeEach(func() {
			versionStdout = []byte(`{{{`)
		})

		It("returns a meaningful error", func() {
			_, err := decoder.Decode(versionStdout)
			Expect(err).To(MatchError("decoding version info: invalid character '{' looking for beginning of object key string"))
		})
	})

	Context("when the json bytes are missing the required CNIVersion field", func() {
		BeforeEach(func() {
			versionStdout = []byte(`{ "supportedVersions": [ "foo" ] }`)
		})

		It("returns a meaningful error", func() {
			_, err := decoder.Decode(versionStdout)
			Expect(err).To(MatchError("decoding version info: missing field cniVersion"))
		})
	})

	Context("when there are no supported versions", func() {
		BeforeEach(func() {
			versionStdout = []byte(`{ "cniVersion": "0.2.0" }`)
		})

		It("assumes that the supported versions are 0.1.0 and 0.2.0", func() {
			pluginInfo, err := decoder.Decode(versionStdout)
			Expect(err).NotTo(HaveOccurred())
			Expect(pluginInfo).NotTo(BeNil())
			Expect(pluginInfo.SupportedVersions()).To(Equal([]string{
				"0.1.0",
				"0.2.0",
			}))
		})
	})

	Describe("ParseVersion", func() {
		It("parses a valid version correctly", func() {
			major, minor, micro, err := version.ParseVersion("1.2.3")
			Expect(err).NotTo(HaveOccurred())
			Expect(major).To(Equal(1))
			Expect(minor).To(Equal(2))
			Expect(micro).To(Equal(3))
		})

		It("returns an error for malformed versions", func() {
			badVersions := []string{"asdfasdf", "asdf.", ".asdfas", "asdf.adsf.", "0.", "..", "1.2.3.4.5", ""}
			for _, v := range badVersions {
				_, _, _, err := version.ParseVersion(v)
				Expect(err).To(HaveOccurred())
			}
		})
	})

	Describe("GreaterThanOrEqualTo", func() {
		It("correctly compares versions", func() {
			versions := [][2]string{
				{"1.2.34", "1.2.14"},
				{"2.5.4", "2.4.4"},
				{"1.2.3", "0.2.3"},
				{"0.4.0", "0.3.1"},
			}
			for _, v := range versions {
				// Make sure the first is greater than the second
				gt, err := version.GreaterThanOrEqualTo(v[0], v[1])
				Expect(err).NotTo(HaveOccurred())
				Expect(gt).To(Equal(true))

				// And the opposite
				gt, err = version.GreaterThanOrEqualTo(v[1], v[0])
				Expect(err).NotTo(HaveOccurred())
				Expect(gt).To(Equal(false))
			}
		})

		It("returns true when versions are the same", func() {
			gt, err := version.GreaterThanOrEqualTo("1.2.3", "1.2.3")
			Expect(err).NotTo(HaveOccurred())
			Expect(gt).To(Equal(true))
		})

		It("returns an error for malformed versions", func() {
			versions := [][2]string{
				{"1.2.34", "asdadf"},
				{"adsfad", "2.5.4"},
			}
			for _, v := range versions {
				_, err := version.GreaterThanOrEqualTo(v[0], v[1])
				Expect(err).To(HaveOccurred())
			}
		})
	})
})
