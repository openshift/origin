package builds

import (
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	buildPrefixTS = "ruby-sample-build-ts"
	buildPrefixTD = "ruby-sample-build-td"
	buildPrefixTC = "ruby-sample-build-tc"
)

func scrapeLogs(bldPrefix string, oc *exutil.CLI) {
	// kick off the app/lang build and verify the builder image accordingly
	br, err := exutil.StartBuildAndWait(oc, bldPrefix)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	out, err := br.Logs()
	o.Expect(err).NotTo(o.HaveOccurred())
	lines := strings.Split(out, "\n")
	found := false
	for _, line := range lines {
		if strings.Contains(line, "Pulling image") && strings.Contains(line, "ruby") {
			fmt.Fprintf(g.GinkgoWriter, "\n\nfound pull image line %s\n\n", line)
			found = true
			break
		}
	}

	if !found {
		fmt.Fprintf(g.GinkgoWriter, "\n\n build log dump on failed test:  %s\n\n", out)
		o.Expect(found).To(o.BeTrue())
	}

}

func checkPodFlag(bldPrefix string, oc *exutil.CLI) {
	// kick off the app/lang build and verify the builder image accordingly
	_, err := exutil.StartBuildAndWait(oc, bldPrefix)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	out, err := oc.Run("get").Args("pods", bldPrefix+"-1-build", "-o", "jsonpath='{.spec.containers[0].imagePullPolicy}'").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(out).To(o.Equal("'Always'"))

}

/*
If docker.io is not responding to requests in a timely manner, this test suite will be adversely affected.

If you suspect such a situation, attempt pulling some openshift images other than ruby-22-centos7
while this test is running and compare results.  Restarting your docker daemon, assuming you can ping docker.io quickly, could
be a quick fix.
*/

var _ = g.Describe("[Feature:Builds] forcePull should affect pulling builder images", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("forcepull", exutil.KubeConfigPath())

	g.Context("", func() {

		g.BeforeEach(func() {
			exutil.DumpDockerInfo()

			g.By("granting system:build-strategy-custom")
			binding := fmt.Sprintf("custombuildaccess-%s", oc.Username())
			err := oc.AsAdmin().Run("create").Args("clusterrolebinding", binding, "--clusterrole", "system:build-strategy-custom", "--user", oc.Username()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for openshift/ruby:latest ImageStreamTag")
			err = exutil.WaitForAnImageStreamTag(oc, "openshift", "ruby", "latest")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("create application build configs for 3 strategies")
			apps := exutil.FixturePath("testdata", "forcepull-test.json")
			err = exutil.CreateResource(apps, oc)
			o.Expect(err).NotTo(o.HaveOccurred())

		})

		g.AfterEach(func() {

			binding := fmt.Sprintf("custombuildaccess-%s", oc.Username())
			err := oc.AsAdmin().Run("delete").Args("clusterrolebinding", binding).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			if g.CurrentGinkgoTestDescription().Failed {
				exutil.DumpPodStates(oc)
				exutil.DumpPodLogsStartingWith("", oc)
			}
		})

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.AdminKubeClient().Core().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("ForcePull test case execution s2i", func() {

			g.By("when s2i force pull is true")
			// run twice to ensure the builder image gets pulled even if it already exists on the node
			scrapeLogs(buildPrefixTS, oc)
			scrapeLogs(buildPrefixTS, oc)

		})

		g.It("ForcePull test case execution docker", func() {
			g.By("docker when force pull is true")
			// run twice to ensure the builder image gets pulled even if it already exists on the node
			scrapeLogs(buildPrefixTD, oc)
			scrapeLogs(buildPrefixTD, oc)
		})

		g.It("ForcePull test case execution custom", func() {
			g.By("when custom force pull is true")
			checkPodFlag(buildPrefixTC, oc)
		})
	})
})
