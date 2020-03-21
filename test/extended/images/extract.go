package images

import (
	"strings"

	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imageapi "github.com/openshift/api/image/v1"
	imageclientset "github.com/openshift/client-go/image/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-imageregistry][Feature:ImageExtract] Image extract", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI
	var ns string

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLI("image-extract")

	g.It("should extract content from an image", func() {
		is, err := oc.ImageClient().ImageV1().ImageStreams("openshift").Get("php", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.DockerImageRepository).NotTo(o.BeEmpty(), "registry not yet configured?")
		registry := strings.Split(is.Status.DockerImageRepository, "/")[0]

		ns = oc.Namespace()
		cli := oc.KubeFramework().PodClient()
		client := imageclientset.NewForConfigOrDie(oc.UserConfig()).ImageV1()

		_, err = client.ImageStreamImports(ns).Create(&imageapi.ImageStreamImport{
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
			! oc image extract --insecure %[2]s/%[1]s/1:busybox --path=/:/tmp/doesnotexist
			# command exits if directory isn't empty
			! oc image extract --insecure %[2]s/%[1]s/1:busybox --path=/:/

			# extract busybox to a directory, verify the contents
			mkdir -p /tmp/test
			oc image extract --insecure %[2]s/%[1]s/1:busybox --path=/:/tmp/test
			[ -d /tmp/test/etc ] && [ -d /tmp/test/bin ]
			[ -f /tmp/test/etc/passwd ] && grep root /tmp/test/etc/passwd

			# extract multiple individual files
			mkdir -p /tmp/test2
			oc image extract --insecure %[2]s/%[1]s/1:busybox --path=/etc/shadow:/tmp/test2 --path=/etc/localtime:/tmp/test2
			[ -f /tmp/test2/shadow ] && [ -f /tmp/test2/localtime ]

			# extract a single file to the current directory
			mkdir -p /tmp/test3
			cd /tmp/test3
			oc image extract --insecure %[2]s/%[1]s/1:busybox --file=/etc/shadow
			[ -f /tmp/test3/shadow ]
		`, ns, registry)))
		cli.WaitForSuccess(pod.Name, podStartupTimeout)
	})
})
