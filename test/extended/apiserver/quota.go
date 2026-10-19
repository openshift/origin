package apiserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// ResourceQuota tests for build-specific imagestream quota behavior.
var _ = g.Describe("[sig-api-machinery] API_Server", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("default")

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Build and imagestream quota tests are not supported on MicroShift")
		}
	})

	g.It("[OTP][OCP-11289] Check the imagestreams of quota in the project after build image [ConnectedOnly][Serial]", ote.Informing(), func() {
		if isBaselineCapsSet(oc) && !(isEnabledCapability(oc, "Build") && isEnabledCapability(oc, "DeploymentConfig")) {
			g.Skip("Skipping the test as baselinecaps have been set and some of API capabilities are not enabled!")
		}

		g.By("1) Create a new project")
		oc.SetupProject()
		namespace := oc.Namespace()

		// Helper to count imagestreams directly
		countImageStreams := func() int {
			output, err := oc.AsAdmin().Run("get").Args("imagestreams", "-n", namespace, "-o", "name").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if strings.TrimSpace(output) == "" {
				return 0
			}
			return len(strings.Split(strings.TrimSpace(output), "\n"))
		}

		// Helper to wait for build completion
		waitForBuildComplete := func(buildName string) {
			buildErr := wait.PollUntilContextTimeout(context.Background(), 5*time.Second, 90*time.Second, false, func(cxt context.Context) (bool, error) {
				phase, err := oc.AsAdmin().Run("get").Args("build", buildName, "-n", namespace, "-o", "jsonpath={.status.phase}").Output()
				if err == nil && strings.Contains(phase, "Complete") {
					e2e.Logf("Build %s completed", buildName)
					return true, nil
				}
				return false, nil
			})
			o.Expect(buildErr).NotTo(o.HaveOccurred(), fmt.Sprintf("Build %s did not complete", buildName))
		}

		g.By("2) Verify no imagestreams exist initially")
		o.Expect(countImageStreams()).To(o.Equal(0), "namespace should start with 0 imagestreams")

		g.By("3) Create S2I build (builder-image~git-url syntax)")
		err := oc.AsAdmin().Run("new-build").Args(
			`quay.io/openshifttest/ruby-27:1.2.0~https://github.com/sclorg/ruby-ex.git`,
			"-n", namespace,
			"--import-mode=PreserveOriginal",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBuildComplete("ruby-ex-1")

		g.By("4) Verify S2I build created exactly 2 imagestreams (builder + output)")
		count := countImageStreams()
		o.Expect(count).To(o.Equal(2), "S2I build should create 2 imagestreams: builder (ruby-27) and output (ruby-ex)")

		// List imagestreams to verify which ones were created
		imagestreams, err := oc.AsAdmin().Run("get").Args("imagestreams", "-n", namespace, "-o", "name").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("ImageStreams created by S2I build: %s", imagestreams)

		g.By("5) Start a second build using the same BuildConfig")
		err = oc.AsAdmin().Run("start-build").Args("ruby-ex", "-n", namespace).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		waitForBuildComplete("ruby-ex-2")

		g.By("6) Verify imagestream count remains 2 (new tags, not new imagestreams)")
		count = countImageStreams()
		o.Expect(count).To(o.Equal(2), "subsequent builds should reuse existing imagestreams, not create new ones")
	})
})
