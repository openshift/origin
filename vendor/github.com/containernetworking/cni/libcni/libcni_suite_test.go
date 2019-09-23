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
	"encoding/json"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestLibcni(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Libcni Suite")
}

var pluginPackages = map[string]string{
	"noop":  "github.com/containernetworking/cni/plugins/test/noop",
	"sleep": "github.com/containernetworking/cni/plugins/test/sleep",
}

var pluginPaths map[string]string
var pluginDirs []string // array of plugin dirs

var _ = SynchronizedBeforeSuite(func() []byte {

	paths := map[string]string{}
	for name, packagePath := range pluginPackages {
		execPath, err := gexec.Build(packagePath)
		Expect(err).NotTo(HaveOccurred())
		paths[name] = execPath
	}
	crossNodeData, err := json.Marshal(paths)
	Expect(err).NotTo(HaveOccurred())

	return crossNodeData
}, func(crossNodeData []byte) {
	Expect(json.Unmarshal(crossNodeData, &pluginPaths)).To(Succeed())
	for _, pluginPath := range pluginPaths {
		pluginDirs = append(pluginDirs, filepath.Dir(pluginPath))
	}
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})
