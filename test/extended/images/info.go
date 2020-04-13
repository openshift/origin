package images

import (
	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageInfo] Image info", func() {
	defer g.GinkgoRecover()

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
			oc image info docker.io/library/mysql:latest

			# display info about an image in json format
			oc image info quay.io/coreos/etcd:latest -o json
		`)))
		cli.WaitForSuccess(pod.Name, podStartupTimeout)
	})
})
