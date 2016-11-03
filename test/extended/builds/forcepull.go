package builds

import (
	"fmt"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	dockerClient "github.com/fsouza/go-dockerclient"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	buildPrefixFS = "ruby-sample-build-fs"
	buildPrefixTS = "ruby-sample-build-ts"
	buildPrefixFD = "ruby-sample-build-fd"
	buildPrefixTD = "ruby-sample-build-td"
	buildPrefixFC = "ruby-sample-build-fc"
	buildPrefixTC = "ruby-sample-build-tc"

	corruptor = "docker.io/openshift/origin-deployer"

	varSubSrc = "SERVICE_REGISTRY_IP"

	bldr       = "forcepull-extended-test-builder"
	bldrPrefix = "forcepull-bldr"
)

var (
	resetData     map[string]string
	authCfg       *dockerClient.AuthConfiguration
	fullImageName string
	tags          []string
	varSubDest    string
)

func doTest(bldPrefix, debugStr string, same bool, oc *exutil.CLI) {
	// corrupt the builder image
	exutil.CorruptImage(fullImageName, corruptor)

	if bldPrefix == buildPrefixFC || bldPrefix == buildPrefixTC {
		// grant access to the custom build strategy
		err := oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "system:build-strategy-custom", oc.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer func() {
			err = oc.AsAdmin().Run("adm").Args("policy", "remove-cluster-role-from-user", "system:build-strategy-custom", oc.Username()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()
	}

	// kick off the app/lang build and verify the builder image accordingly
	_, err := exutil.StartBuildAndWait(oc, bldPrefix)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())

	if same {
		exutil.VerifyImagesSame(fullImageName, corruptor, debugStr)
	} else {
		exutil.VerifyImagesDifferent(fullImageName, corruptor, debugStr)
	}

	// reset corrupted tagging for next test
	exutil.ResetImage(resetData)
	// dump tags/hexids for debug
	_, err = exutil.DumpAndReturnTagging(tags)
	o.ExpectWithOffset(1, err).NotTo(o.HaveOccurred())
}

/*
If docker.io is not responding to requests in a timely manner, this test suite will be adversely affected.

If you suspect such a situation, attempt pulling some openshift images other than ruby-22-centos7 or origin-custom-docker-builder
while this test is running and compare results.  Restarting your docker daemon, assuming you can ping docker.io quickly, could
be a quick fix.

Also, in order to build the test case specific builder images only once, we currently have to do all the testing within a single g.It block.
The project/namespace were being destroyed between tests, and that includes removal of the specific builder images
we built.  The credentials also are recycled between those points.

Dumping of the ImageStreams and Secrets JSON output at the various points proved this out.
*/

var _ = g.Describe("[LocalNode][builds] forcePull should affect pulling builder images", func() {
	defer g.GinkgoRecover()
	var oc = exutil.NewCLI("forcepull", exutil.KubeConfigPath())

	g.BeforeEach(func() {

		g.By("refresh corruptor, prep forcepull builder")
		exutil.PullImage(corruptor, dockerClient.AuthConfiguration{})

		exutil.DumpImage(corruptor)

		// create the image streams and build configs for a test case specific builders
		setupPath := exutil.FixturePath("testdata", "forcepull-setup.json")
		err := exutil.CreateResource(setupPath, oc)

		// kick off the build for the new builder image just for force pull so we can corrupt them without conflicting with
		// any other tests potentially running in parallel
		br, _ := exutil.StartBuildAndWait(oc, bldrPrefix)
		br.AssertSuccess()

		serviceIP, err := oc.Run("get").Args("svc", "docker-registry", "-n", "default", "--config", exutil.KubeConfigPath()).Template("{{.spec.clusterIP}}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		port, err := oc.Run("get").Args("svc", "docker-registry", "-n", "default", "--config", exutil.KubeConfigPath()).Template("{{ $x := index .spec.ports 0}}{{$x.port}}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		g.By(fmt.Sprintf("docker-registry service IP is %s and port %s ", serviceIP, port))

		// get the auth so we can pull the build image from the internal docker registry since the builder controller will  remove it
		// from the docker daemon cache when the docker build completes;
		authCfg, err = exutil.BuildAuthConfiguration(serviceIP+":"+port, oc)

		// now actually pull the image back in from the openshift internal docker registry
		fullImageName = authCfg.ServerAddress + "/" + oc.Namespace() + "/" + bldr
		err = exutil.PullImage(fullImageName, *authCfg)
		o.Expect(err).NotTo(o.HaveOccurred())
		exutil.DumpImage(fullImageName)

		//update the build configs in the json for the app/lang builds to point to the builder images in the internal docker registry
		// and then create the build config resources
		pre := exutil.FixturePath("testdata", "forcepull-test.json")
		post := exutil.ArtifactPath("forcepull-test.json")
		varSubDest = authCfg.ServerAddress + "/" + oc.Namespace()

		// grant access to the custom build strategy
		g.By("granting system:build-strategy-custom")
		err = oc.AsAdmin().Run("adm").Args("policy", "add-cluster-role-to-user", "system:build-strategy-custom", oc.Username()).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		defer func() {
			err = oc.AsAdmin().Run("adm").Args("policy", "remove-cluster-role-from-user", "system:build-strategy-custom", oc.Username()).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
		}()

		err = exutil.VarSubOnFile(pre, post, varSubSrc, varSubDest)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = exutil.CreateResource(post, oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		// dump the image textual tags and hex ids out for debug
		tags = []string{fullImageName + ":latest", corruptor + ":latest"}
		hexIDs, err := exutil.DumpAndReturnTagging(tags)
		o.Expect(err).NotTo(o.HaveOccurred())
		resetData = map[string]string{fullImageName: hexIDs[0], corruptor: hexIDs[1]}

	})

	g.Context("ForcePull test context  ", func() {

		g.JustBeforeEach(func() {
			g.By("waiting for builder service account")
			err := exutil.WaitForBuilderAccount(oc.AdminKubeREST().ServiceAccounts(oc.Namespace()))
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.It("ForcePull test case execution", func() {

			g.By("when s2i force pull is false")

			doTest(buildPrefixFS, "s2i false app/lang build", true, oc)

			g.By("when s2i force pull is true")

			doTest(buildPrefixTS, "s2i true app/lang build", false, oc)

			g.By("when docker force pull is false")

			doTest(buildPrefixFD, "dock false app/lang build", true, oc)

			g.By("docker when force pull is true")

			doTest(buildPrefixTD, "dock true app/lang build", false, oc)

			g.By("when custom force pull is false")

			doTest(buildPrefixFC, "cust false app/lang build", true, oc)

			g.By("when custom force pull is true")

			doTest(buildPrefixTC, "cust true app/lang build", false, oc)

		})

	})

})
