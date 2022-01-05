package images

import (
	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo"
	imageutil "github.com/openshift/origin/test/extended/util/image"
	"os"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageInfo] Image info", func() {
	defer g.GinkgoRecover()

	mirrorRegistryDefined := os.Getenv("TEST_IMAGE_MIRROR_REGISTRY") != ""
	mysqlImage := "docker.io/library/mysql:latest"
	if mirrorRegistryDefined {
		mysqlImage, _ = imageutil.GetE2eImageMappedToRegistry(mysqlImage, "library")
	}

	var oc *exutil.CLI
	var ns string

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLI("image-info")

	g.It("should display information about images", func() {
		ns = oc.Namespace()
		cli := oc.KubeFramework().PodClient()
		pod := cli.Create(cliPodWithPullSecret(oc, heredoc.Docf(`
			set -x

			# display info about an image on quay.io
			oc image info quay.io/coreos/etcd:latest

			# display info about an image on quay.io
			oc image info %[1]s

			# display info about an image in json format
			oc image info quay.io/coreos/etcd:latest -o json
		`, mysqlImage)))
		cli.WaitForSuccess(pod.Name, podStartupTimeout)
	})
})
