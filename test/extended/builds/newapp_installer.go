package builds

import (
	"path/filepath"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	"io/ioutil"
	"time"
)

var _ = g.Describe("[builds][Slow] test new-app grant-install-rights", func() {
	defer g.GinkgoRecover()
	var (
		oc                  = exutil.NewCLI("install-rights", exutil.KubeConfigPath())
		testDataBaseDir     = exutil.FixturePath("testdata", "newapp_installer")
		installerDockerFile = filepath.Join(testDataBaseDir, "Dockerfile")
	)

	g.It("build with installer", func() {

		g.By("creating a valid installer imagestream")

		dockerfile, err := ioutil.ReadFile(installerDockerFile)
		o.Expect(err).NotTo(o.HaveOccurred())

		err = oc.Run("new-build").Args("-D", "-", "--to=myinstaller").InputString(string(dockerfile)).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for that imagestream to fulfill")
		err = exutil.TimedWaitForAnImageStreamTag(oc, oc.Namespace(), "myinstaller", "latest", 10*time.Minute)

		installerImageURI, err := oc.Run("get").Args("imagestreams/myinstaller", "-o=jsonpath=\"{.status.dockerImageRepository}\"").Output()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Running the installer")
		err = oc.Run("new-app").Args("--docker-image", installerImageURI, "--grant-install-rights").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for that installer to successfully affect runtime")
		err = exutil.TimedWaitForAnImageStreamTag(oc, oc.Namespace(), "ruby-hello-world", "latest", 10*time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())
	})

})
