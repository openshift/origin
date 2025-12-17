package images

import (
	"context"

	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo/v2"

	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageInfo] Image info", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI
	var ns string

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("image-info", admissionapi.LevelBaseline)

	g.It("should display information about images [apigroup:image.openshift.io]", g.Label("Size:S"), func() {
		ns = oc.Namespace()
		cli := e2epod.PodClientNS(oc.KubeFramework(), ns)
		pod := cli.Create(context.TODO(), cliPodWithPullSecret(oc, heredoc.Docf(`
			set -x

			# display info about an image on quay.io
			oc image info quay.io/openshift-release-dev/ocp-release:4.18.3-x86_64

			# display info about an image in json format
			oc image info quay.io/openshift-release-dev/ocp-release:4.18.3-x86_64 -o json
		`)))
		cli.WaitForSuccess(context.TODO(), pod.Name, podStartupTimeout)
	})
})
