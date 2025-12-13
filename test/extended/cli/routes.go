package cli

import (
	"fmt"
	"path/filepath"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-cli] oc", func() {
	defer g.GinkgoRecover()

	var (
		oc          = exutil.NewCLIWithPodSecurityLevel("oc-routes", admissionapi.LevelBaseline)
		cmdTestData = exutil.FixturePath("testdata", "cmd", "test", "cmd", "testdata")
		testRoute   = filepath.Join(cmdTestData, "test-route.json")
		testService = filepath.Join(cmdTestData, "test-service.json")
	)

	g.It("can route traffic to services [apigroup:route.openshift.io]", g.Label("Size:M"), func() {
		err := oc.Run("get").Args("routes").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("-f", testRoute).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err := oc.Run("get").Args("routes", "testroute", "--show-labels").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("rtlabel1"))

		err = oc.Run("delete").Args("routes", "testroute").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("-f", testService).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("route", "passthrough", "--service=svc/frontend").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("delete").Args("routes", "frontend").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("route", "edge", "--path", "/test", "--service=services/non-existent", "--port=80").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("delete").Args("routes", "non-existent").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("route", "edge", "test-route", "--service=frontend").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("delete").Args("routes", "test-route").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("route", "edge", "new-route").Execute()
		o.Expect(err).To(o.HaveOccurred())

		err = oc.Run("delete").Args("services", "frontend").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("create").Args("route", "edge", "--insecure-policy=Allow", "--service=foo", "--port=80").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("get").Args("route", "foo", "-o", "jsonpath=\"{.spec.tls.insecureEdgeTerminationPolicy}\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.Equal("\"Allow\""))

		err = oc.Run("delete").Args("routes", "foo").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("create").Args("route", "edge", "--service", "foo", "--port=8080").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("created"))

		out, err = oc.Run("create").Args("route", "edge", "--service", "bar", "--port=9090").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("created"))

		g.By("verify that reencrypt routes with no destination CA return the stub PEM block on the old API")
		projectName, err := oc.Run("project").Args("-q").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		out, err = oc.Run("create").Args("route", "reencrypt", "--service", "baz", "--port=9090").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("created"))

		out, err = oc.Run("get").Args("--raw", fmt.Sprintf("/apis/route.openshift.io/v1/namespaces/%s/routes/baz", projectName)).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).NotTo(o.ContainSubstring("This is an empty PEM file"))

		out, err = oc.Run("set").Args("route-backends", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("routes/foo"))

		out, err = oc.Run("set").Args("route-backends", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("Service"))

		out, err = oc.Run("set").Args("route-backends", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("100"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--zero", "--equal").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("error: --zero and --equal may not be specified together"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--zero", "--adjust").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("error: --adjust and --zero may not be specified together"))

		out, err = oc.Run("set").Args("route-backends", "foo", "a=").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("expected NAME=WEIGHT"))

		out, err = oc.Run("set").Args("route-backends", "foo", "=10").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("expected NAME=WEIGHT"))

		out, err = oc.Run("set").Args("route-backends", "foo", "a=a").Output()
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("WEIGHT must be a number"))

		out, err = oc.Run("set").Args("route-backends", "foo", "a=10").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("route-backends", "foo", "a=100").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("route-backends", "foo", "a=0").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("route-backends", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("0"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a"))

		out, err = oc.Run("set").Args("route-backends", "foo", "a1=0", "b2=0").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("route-backends", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1"))

		out, err = oc.Run("set").Args("route-backends", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("b2"))

		out, err = oc.Run("set").Args("route-backends", "foo", "a1=100", "b2=50", "c3=0").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1(66%),b2(33%),c3(0%)"))

		out, err = oc.Run("set").Args("route-backends", "foo", "a1=100", "b2=0", "c3=0").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--adjust", "b2=+10%").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1(90%),b2(10%),c3(0%)"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--adjust", "b2=+25%").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1(65%),b2(35%),c3(0%)"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--adjust", "b2=+99%").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1(0%),b2(100%),c3(0%)"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--adjust", "b2=-51%").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1(51%),b2(49%),c3(0%)"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--adjust", "a1=20%").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1(20%),b2(80%),c3(0%)"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--adjust", "c3=50%").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1(10%),b2(80%),c3(10%)"))

		out, err = oc.Run("describe").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("25 (10%)"))
		o.Expect(out).To(o.ContainSubstring("200 (80%)"))
		o.Expect(out).To(o.ContainSubstring("25 (10%)"))
		o.Expect(out).To(o.ContainSubstring("<error: endpoints \"c3\" not found>"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--adjust", "c3=1").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("describe").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("1 (0%)"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--equal").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1(33%),b2(33%),c3(33%)"))

		out, err = oc.Run("describe").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("100 (33%)"))

		out, err = oc.Run("set").Args("route-backends", "foo", "--zero").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("updated"))

		out, err = oc.Run("get").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("a1(0%),b2(0%),c3(0%)"))

		out, err = oc.Run("describe").Args("routes", "foo").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(out).To(o.ContainSubstring("0"))
	})
})
