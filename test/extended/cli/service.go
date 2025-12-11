package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc service", func() {
	defer g.GinkgoRecover()

	var oc = exutil.NewCLI("oc-service")

	g.It("creates and deletes services", g.Label("Size:M"), func() {
		err := oc.Run("get").Args("services").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		file, err := writeObjectToFile(newFrontendService())
		o.Expect(err).NotTo(o.HaveOccurred())
		defer os.Remove(file)

		err = oc.Run("create").Args("-f", file).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("delete").Args("service", "frontend").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("validate the 'create service nodeport' command and its --node-port and --tcp options")
		nodePort := 30000
		out, err := oc.Run("create").Args("service", "nodeport", "mynodeport", "--tcp=8080:7777", fmt.Sprintf("--node-port=%d", nodePort)).Output()
		if err != nil && strings.Contains(err.Error(), "provided port is already allocated") {
			// another service may use port number 30000 and instead of failing, we can try our chance
			// with another port to have a stable test.
			nodePort = 30001
			out, err = oc.Run("create").Args("service", "nodeport", "mynodeport", "--tcp=8080:7777", fmt.Sprintf("--node-port=%d", nodePort)).Output()
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("service/mynodeport created"))

		out, err = oc.Run("create").Args("service", "nodeport", "mynodeport", "--tcp=8080:7777", fmt.Sprintf("--node-port=%d", nodePort)).Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("provided port is already allocated"))

		out, err = oc.Run("create").Args("service", "nodeport", "mynodeport", "--tcp=8080:7777", "--node-port=300").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("provided port is not in the valid range. The range of valid ports is 30000-32767"))

		out, err = oc.Run("describe").Args("service", "mynodeport").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		npReg1 := fmt.Sprintf("NodePort:.*%d", nodePort)
		m1, err := regexp.MatchString(npReg1, out)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(m1).To(o.BeTrue())

		npReg2 := "NodePort:.*8080-7777"
		m2, err := regexp.MatchString(npReg2, out)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(m2).To(o.BeTrue())

		out, err = oc.Run("describe", "--v=8", "service", "mynodeport").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Response Body"))
	})
})
