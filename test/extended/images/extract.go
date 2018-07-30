package images

import (
	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imageapi "github.com/openshift/api/image/v1"
	imageclientset "github.com/openshift/client-go/image/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:ImageExtract] Image extract", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI
	var ns string

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLI("image-extract", exutil.KubeConfigPath())

	g.It("should extract content from an image", func() {
		ns = oc.Namespace()
		cli := oc.KubeFramework().PodClient()
		client := imageclientset.NewForConfigOrDie(oc.UserConfig()).Image()

		_, err := client.ImageStreamImports(ns).Create(&imageapi.ImageStreamImport{
			ObjectMeta: metav1.ObjectMeta{
				Name: "1",
			},
			Spec: imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{
					{
						From: kapi.ObjectReference{Kind: "DockerImage", Name: "busybox:latest"},
						To:   &kapi.LocalObjectReference{Name: "busybox"},
					},
					{
						From: kapi.ObjectReference{Kind: "DockerImage", Name: "mysql:latest"},
						To:   &kapi.LocalObjectReference{Name: "mysql"},
					},
				},
			},
		})
		o.Expect(err).ToNot(o.HaveOccurred())

		// busyboxLayers := isi.Status.Images[0].Image.DockerImageLayers
		// busyboxLen := len(busyboxLayers)
		// mysqlLayers := isi.Status.Images[1].Image.DockerImageLayers
		// mysqlLen := len(mysqlLayers)

		pod := cli.Create(cliPodWithPullSecret(oc, heredoc.Docf(`
			set -x

			# command exits if directory doesn't exist
			! oc image extract --insecure docker-registry.default.svc:5000/%[1]s/1:busybox --path=/:/tmp/doesnotexist

			# extract busybox to a directory, verify the contents
			mkdir -p /tmp/test
			oc image extract --insecure docker-registry.default.svc:5000/%[1]s/1:busybox --path=/:/tmp/test
			[ -d /tmp/test/etc ] && [ -d /tmp/test/bin ]
			[ -f /tmp/test/bin/ls ] && /tmp/test/bin/ls /tmp/test
			oc image extract --insecure docker-registry.default.svc:5000/%[1]s/1:busybox --path=/etc/shadow:/tmp --path=/etc/localtime:/tmp
			[ -f /tmp/shadow ] && [ -f /tmp/localtime ]
		`, ns)))
		cli.WaitForSuccess(pod.Name, podStartupTimeout)
	})
})
