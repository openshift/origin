package images

import (
	"context"
	"strings"

	"github.com/MakeNowJust/heredoc"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	"github.com/openshift/api/image/docker10"
	"github.com/openshift/library-go/pkg/image/imageutil"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

func cliPodWithPullSecret(cli *exutil.CLI, shell string) *kapiv1.Pod {
	sa, err := cli.KubeClient().CoreV1().ServiceAccounts(cli.Namespace()).Get(context.Background(), "builder", metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(sa.ImagePullSecrets).NotTo(o.BeEmpty())
	pullSecretName := sa.ImagePullSecrets[0].Name

	return &kapiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "append-test",
		},
		Spec: kapiv1.PodSpec{
			// so we have permission to push and pull to the registry
			ServiceAccountName: "builder",
			RestartPolicy:      kapiv1.RestartPolicyNever,
			Containers: []kapiv1.Container{
				{
					Name:    "test",
					Image:   image.ShellImage(),
					Command: []string{"/bin/bash", "-c", "set -euo pipefail; " + shell},
					Env: []kapiv1.EnvVar{
						{
							Name:  "HOME",
							Value: "/secret",
						},
					},
					VolumeMounts: []kapiv1.VolumeMount{
						{
							Name:      "pull-secret",
							MountPath: "/secret/.dockercfg",
							SubPath:   kapiv1.DockerConfigKey,
						},
					},
				},
			},
			Volumes: []kapiv1.Volume{
				{
					Name: "pull-secret",
					VolumeSource: kapiv1.VolumeSource{
						Secret: &kapiv1.SecretVolumeSource{
							SecretName: pullSecretName,
						},
					},
				},
			},
		},
	}
}

var _ = g.Describe("[sig-imageregistry][Feature:ImageAppend] Image append", func() {
	defer g.GinkgoRecover()

	var oc *exutil.CLI
	var ns string

	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() && len(ns) > 0 {
			exutil.DumpPodLogsStartingWithInNamespace("", ns, oc)
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("image-append", admissionapi.LevelBaseline)

	g.It("should create images by appending them [apigroup:image.openshift.io]", g.Label("Size:L"), func() {
		is, err := oc.ImageClient().ImageV1().ImageStreams("openshift").Get(context.Background(), "tools", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(is.Status.DockerImageRepository).NotTo(o.BeEmpty(), "registry not yet configured?")
		registry := strings.Split(is.Status.DockerImageRepository, "/")[0]

		ns = oc.Namespace()
		cli := e2epod.PodClientNS(oc.KubeFramework(), oc.Namespace())
		pod := cli.Create(context.TODO(), cliPodWithPullSecret(oc, heredoc.Docf(`
			set -x

			# create a copy of the single manifest image from the original image for layer count
			oc image append --insecure --from %[3]s --to %[2]s/%[1]s/tools:singlemanifest

			# create a scratch image with fixed date
			oc image append --insecure --to %[2]s/%[1]s/test:scratch1 --image='{"Cmd":["/bin/sleep"]}' --created-at=0

			# create a second scratch image with fixed date
			oc image append --insecure --to %[2]s/%[1]s/test:scratch2 --image='{"Cmd":["/bin/sleep"]}' --created-at=0

			# modify a shell image
			oc image append --insecure --from %[3]s --to %[2]s/%[1]s/test:busybox1 --image '{"Cmd":["/bin/sleep"]}'

			# verify mounting works
			oc create is test2
			oc image append --insecure --from %[2]s/%[1]s/test:scratch2 --to %[2]s/%[1]s/test2:scratch2 --force

			# add a simple layer to the image
			mkdir -p /tmp/test/dir
			touch /tmp/test/1
			touch /tmp/test/dir/2
			tar cvzf /tmp/layer.tar.gz -C /tmp/test/ .
			oc image append --insecure --from=%[2]s/%[1]s/test:busybox1 --to %[2]s/%[1]s/test:busybox2 /tmp/layer.tar.gz
		`, ns, registry, image.ShellImage())))
		cli.WaitForSuccess(context.TODO(), pod.Name, podStartupTimeout)

		baseTools, err := oc.ImageClient().ImageV1().ImageStreamTags(ns).Get(context.Background(), "tools:singlemanifest", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		// subsequent operations are expected to append to this image
		baseLayerCount := len(baseTools.Image.DockerImageLayers)
		o.Expect(baseLayerCount).To(o.BeNumerically(">=", 2))

		istag, err := oc.ImageClient().ImageV1().ImageStreamTags(ns).Get(context.Background(), "test:scratch1", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag.Image).NotTo(o.BeNil())

		imageutil.ImageWithMetadataOrDie(&istag.Image)

		o.Expect(istag.Image.DockerImageLayers).To(o.HaveLen(1))
		o.Expect(istag.Image.DockerImageLayers[0].Name).To(o.Equal(GzippedEmptyLayerDigest))
		err = imageutil.ImageWithMetadata(&istag.Image)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag.Image.DockerImageMetadata.Object.(*docker10.DockerImage).Config.Cmd).To(o.Equal([]string{"/bin/sleep"}))

		istag2, err := oc.ImageClient().ImageV1().ImageStreamTags(ns).Get(context.Background(), "test:scratch2", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag2.Image).NotTo(o.BeNil())
		o.Expect(istag2.Image.Name).To(o.Equal(istag.Image.Name))

		istag, err = oc.ImageClient().ImageV1().ImageStreamTags(ns).Get(context.Background(), "test:busybox1", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag.Image).NotTo(o.BeNil())
		imageutil.ImageWithMetadataOrDie(&istag.Image)
		o.Expect(istag.Image.DockerImageLayers).To(o.HaveLen(baseLayerCount))
		o.Expect(istag.Image.DockerImageLayers[0].Name).NotTo(o.Equal(GzippedEmptyLayerDigest))
		err = imageutil.ImageWithMetadata(&istag.Image)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag.Image.DockerImageMetadata.Object.(*docker10.DockerImage).Config.Cmd).To(o.Equal([]string{"/bin/sleep"}))
		busyboxLayer := istag.Image.DockerImageLayers[0].Name

		istag, err = oc.ImageClient().ImageV1().ImageStreamTags(ns).Get(context.Background(), "test:busybox2", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag.Image).NotTo(o.BeNil())
		imageutil.ImageWithMetadataOrDie(&istag.Image)
		o.Expect(istag.Image.DockerImageLayers).To(o.HaveLen(baseLayerCount + 1))
		o.Expect(istag.Image.DockerImageLayers[0].Name).To(o.Equal(busyboxLayer))
		err = imageutil.ImageWithMetadata(&istag.Image)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(istag.Image.DockerImageLayers[1].LayerSize).NotTo(o.Equal(0))
		o.Expect(istag.Image.DockerImageMetadata.Object.(*docker10.DockerImage).Config.Cmd).To(o.Equal([]string{"/bin/sleep"}))
	})
})

const (
	// GzippedEmptyLayerDigest is a digest of GzippedEmptyLayer
	GzippedEmptyLayerDigest = "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
	// EmptyLayerDiffID is the tarsum of the GzippedEmptyLayer
	EmptyLayerDiffID = "sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef"
	// DigestSha256EmptyTar is the canonical sha256 digest of empty data
	DigestSha256EmptyTar = "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)
