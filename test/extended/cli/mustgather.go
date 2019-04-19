package cli

import (
	g "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"io/ioutil"
	"os"
	"path"

	"github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[cli] oc adm must-gather", func() {
	defer g.GinkgoRecover()
	oc := util.NewCLI("oc-adm-must-gather", util.KubeConfigPath()).AsAdmin()
	g.It("runs successfully", func() {
		tempDir, err := ioutil.TempDir("", "test.oc-adm-must-gather.")
		Ω(err).ShouldNot(HaveOccurred())
		defer os.RemoveAll(tempDir)
		Ω(oc.Run("adm", "must-gather").Args("--dest-dir", tempDir).Execute()).Should(Succeed())

		expectedDirectories := [][]string{
			{tempDir, "cluster-scoped-resources", "config.openshift.io"},
			{tempDir, "cluster-scoped-resources", "operator.openshift.io"},
			{tempDir, "cluster-scoped-resources", "core"},
			{tempDir, "cluster-scoped-resources", "apiregistration.k8s.io"},
			{tempDir, "namespaces", "openshift"},
			{tempDir, "namespaces", "openshift-kube-apiserver-operator"},
		}

		expectedFiles := [][]string{
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "apiservers.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "authentications.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "builds.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "clusteroperators.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "clusterversions.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "consoles.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "dnses.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "featuregates.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "images.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "infrastructures.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "ingresses.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "networks.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "oauths.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "projects.yaml"},
			{tempDir, "cluster-scoped-resources", "config.openshift.io", "schedulers.yaml"},
			{tempDir, "namespaces", "openshift-kube-apiserver", "core", "configmaps.yaml"},
			{tempDir, "namespaces", "openshift-kube-apiserver", "core", "secrets.yaml"},
		}

		for _, expectedDirectory := range expectedDirectories {
			Ω(path.Join(expectedDirectory...)).Should(BeADirectory())
		}

		for _, expectedFile := range expectedFiles {
			expectedFilePath := path.Join(expectedFile...)
			Ω(expectedFilePath).Should(BeAnExistingFile())
			stat, err := os.Stat(expectedFilePath)
			Ω(err).ShouldNot(HaveOccurred())
			Ω(stat.Size()).Should(BeNumerically(">=", 100))
		}

	})
})
