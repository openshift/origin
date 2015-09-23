package builds

/*
This particular builds test suite is not part of the "default" group,  because its testing
centers around manipulation of images tags to confirm whether the `docker pull` invocation occurs
correctly based on `forcePull` setting in the BuildConfig, and rather than spend time creating / pulling down separate, test only,
images for each test scenario, we reuse existing images and ensure that the tests do not run in parallel, and step on each
others toes by tagging the same, existing images in ways which conflict.
*/

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	"time"

	exutil "github.com/openshift/origin/test/extended/util"
)

var (
	resetData map[string]string
)

const (
	buildPrefix = "ruby-sample-build"
	buildName   = buildPrefix + "-1"
	s2iDockBldr = "docker.io/openshift/ruby-20-centos7"
	custBldr    = "docker.io/openshift/origin-custom-docker-builder"
)

/*
If docker.io is not responding to requests in a timely manner, this test suite will be adversely affected.

If you suspect such a situation, attempt pulling some openshift images other than ruby-20-centos7 or origin-custom-docker-builder
while this test is running and compare results.  Restarting your docker daemon, assuming you can ping docker.io quickly, could
be a quick fix.
*/

var _ = g.BeforeSuite(func() {
	// do a pull initially just to insure have the latest version
	exutil.PullImage(s2iDockBldr)
	exutil.PullImage(custBldr)
	// save hex image IDs for image reset after corruption
	tags := []string{s2iDockBldr + ":latest", custBldr + ":latest"}
	hexIDs, ierr := exutil.GetImageIDForTags(tags)
	o.Expect(ierr).NotTo(o.HaveOccurred())
	for _, hexID := range hexIDs {
		g.By(fmt.Sprintf("\n%s FORCE PULL TEST:  hex id %s ", time.Now().Format(time.RFC850), hexID))
	}
	o.Expect(len(hexIDs)).To(o.Equal(2))
	resetData = map[string]string{s2iDockBldr: hexIDs[0], custBldr: hexIDs[1]}
	g.By(fmt.Sprintf("\n%s FORCE PULL TEST:  hex id for s2i/docker %s and for custom %s ", time.Now().Format(time.RFC850), hexIDs[0], hexIDs[1]))
})

// TODO this seems like a weird restriction with segregated namespaces.  provide a better explanation of why this doesn't work
// we don't run in parallel with this suite - do not want different tests tagging the same image in different ways at the same time
var _ = g.Describe("builds: serial: ForcePull from OpenShift induced builds (vs. sti)", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("force-pull-s2i", exutil.KubeConfigPath())

	g.JustBeforeEach(func() {
		g.By("waiting for builder service account")
		err := exutil.WaitForBuilderAccount(oc.AdminKubeREST().ServiceAccounts(oc.Namespace()))
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("\n FORCE PULL TEST:  Force pull and s2i builder", func() {
		// corrupt the s2i builder image
		g.BeforeEach(func() {
			exutil.CorruptImage(s2iDockBldr, custBldr, "s21")
		})

		g.AfterEach(func() {
			exutil.ResetImage(resetData)
		})

		g.Context("\n FORCE PULL TEST:  when s2i force pull is false and the image is bad", func() {

			g.It("\n FORCE PULL TEST s2i false", func() {
				fpFalseS2I := exutil.FixturePath("fixtures", "forcepull-false-s2i.json")
				g.By(fmt.Sprintf("\n%s FORCE PULL TEST s2i false:  calling create on %s", time.Now().Format(time.RFC850), fpFalseS2I))
				exutil.StartBuild(fpFalseS2I, buildPrefix, oc)

				exutil.WaitForBuild("FORCE PULL TEST s2i false:  ", buildName, oc)

				exutil.VerifyImagesSame(s2iDockBldr, custBldr, "FORCE PULL TEST s2i false:  ")

			})
		})

		g.Context("\n FORCE PULL TEST:  when s2i force pull is true and the image is bad", func() {
			g.It("\n FORCE PULL TEST s2i true", func() {
				fpTrueS2I := exutil.FixturePath("fixtures", "forcepull-true-s2i.json")
				g.By(fmt.Sprintf("\n%s FORCE PULL TEST s2i true:  calling create on %s", time.Now().Format(time.RFC850), fpTrueS2I))
				exutil.StartBuild(fpTrueS2I, buildPrefix, oc)

				exutil.WaitForBuild("FORCE PULL TEST s2i true: ", buildName, oc)

				exutil.VerifyImagesDifferent(s2iDockBldr, custBldr, "FORCE PULL TEST s2i true:  ")
			})
		})
	})

	g.Describe("\n FORCE PULL TEST:  Force pull and docker builder", func() {
		// corrupt the docker builder image
		g.BeforeEach(func() {
			exutil.CorruptImage(s2iDockBldr, custBldr, "docker")
		})

		g.AfterEach(func() {
			exutil.ResetImage(resetData)
		})

		g.Context("\n FORCE PULL TEST:  when docker force pull is false and the image is bad", func() {
			g.It("\n FORCE PULL TEST dock false", func() {
				fpFalseDock := exutil.FixturePath("fixtures", "forcepull-false-dock.json")
				g.By(fmt.Sprintf("\n%s FORCE PULL TEST dock false:  calling create on %s", time.Now().Format(time.RFC850), fpFalseDock))
				exutil.StartBuild(fpFalseDock, buildPrefix, oc)

				exutil.WaitForBuild("FORCE PULL TEST dock false", buildName, oc)

				exutil.VerifyImagesSame(s2iDockBldr, custBldr, "FORCE PULL TEST docker false:  ")

			})
		})

		g.Context("\n FORCE PULL TEST:  docker when force pull is true and the image is bad", func() {
			g.It("\n FORCE PULL TEST dock true", func() {
				fpTrueDock := exutil.FixturePath("fixtures", "forcepull-true-dock.json")
				g.By(fmt.Sprintf("\n%s FORCE PULL TEST dock true:  calling create on %s", time.Now().Format(time.RFC850), fpTrueDock))
				exutil.StartBuild(fpTrueDock, buildPrefix, oc)

				exutil.WaitForBuild("FORCE PULL TEST dock true", buildName, oc)

				exutil.VerifyImagesDifferent(s2iDockBldr, custBldr, "FORCE PULL TEST docker true:  ")

			})
		})
	})

	g.Describe("\n FORCE PULL TEST:  Force pull and custom builder", func() {
		// corrupt the custom builder image
		g.BeforeEach(func() {
			exutil.CorruptImage(custBldr, s2iDockBldr, "custom")
		})

		g.AfterEach(func() {
			exutil.ResetImage(resetData)
		})

		g.Context("\n FORCE PULL TEST:  when custom force pull is false and the image is bad", func() {
			g.It("\nFORCE PULL TEST cust false", func() {
				fpFalseCust := exutil.FixturePath("fixtures", "forcepull-false-cust.json")
				g.By(fmt.Sprintf("\n%s FORCE PULL TEST cust false:  calling create on %s", time.Now().Format(time.RFC850), fpFalseCust))
				exutil.StartBuild(fpFalseCust, buildPrefix, oc)

				g.By("\nFORCE PULL TEST cust false:  expecting the image is not refreshed")

				exutil.WaitForBuild("FORCE PULL TEST cust false", buildName, oc)

				exutil.VerifyImagesSame(s2iDockBldr, custBldr, "FORCE PULL TEST custom false:  ")
			})
		})

		g.Context("\n FORCE PULL TEST:  when custom force pull is true and the image is bad", func() {
			g.It("\n FORCE PULL TEST cust true", func() {
				fpTrueCust := exutil.FixturePath("fixtures", "forcepull-true-cust.json")
				g.By(fmt.Sprintf("\n%s FORCE PULL TEST cust true:  calling create on %s", time.Now().Format(time.RFC850), fpTrueCust))
				exutil.StartBuild(fpTrueCust, buildPrefix, oc)

				g.By("\n FORCE PULL TEST cust true:  expecting the image is refreshed")

				exutil.WaitForBuild("FORCE PULL TEST cust true", buildName, oc)

				exutil.VerifyImagesDifferent(s2iDockBldr, custBldr, "FORCE PULL TEST custom true:  ")

			})
		})

	})
})
