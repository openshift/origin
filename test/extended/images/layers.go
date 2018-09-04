package images

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	buildv1 "github.com/openshift/api/build/v1"
	imagev1 "github.com/openshift/api/image/v1"
	buildv1client "github.com/openshift/client-go/build/clientset/versioned"
	imagev1client "github.com/openshift/client-go/image/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Feature:ImageLayers] Image layer subresource", func() {
	defer g.GinkgoRecover()
	var oc *exutil.CLI
	var ns []string

	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			for _, s := range ns {
				exutil.DumpPodLogsStartingWithInNamespace("", s, oc)
			}
		}
	})

	oc = exutil.NewCLI("image-layers", exutil.KubeConfigPath())

	g.It("should identify a deleted image as missing", func() {
		client := imagev1client.NewForConfigOrDie(oc.AdminConfig()).Image()
		_, err := client.ImageStreams(oc.Namespace()).Create(&imagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = client.ImageStreamMappings(oc.Namespace()).Create(&imagev1.ImageStreamMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test",
			},
			Image: imagev1.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "an_image_to_be_deleted",
				},
				DockerImageReference: "example.com/random/image:latest",
			},
			Tag: "missing",
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = client.Images().Delete("an_image_to_be_deleted", nil)
		o.Expect(err).NotTo(o.HaveOccurred())
		err = wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
			layers, err := client.ImageStreams(oc.Namespace()).Layers("test", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			ref, ok := layers.Images["an_image_to_be_deleted"]
			if !ok {
				return false, nil
			}
			return ref.ImageMissing, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should return layers from tagged images", func() {
		ns = []string{oc.Namespace()}
		client := imagev1client.NewForConfigOrDie(oc.UserConfig()).Image()
		isi, err := client.ImageStreamImports(oc.Namespace()).Create(&imagev1.ImageStreamImport{
			ObjectMeta: metav1.ObjectMeta{
				Name: "1",
			},
			Spec: imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{Kind: "DockerImage", Name: "busybox:latest"},
						To:   &corev1.LocalObjectReference{Name: "busybox"},
					},
					{
						From: corev1.ObjectReference{Kind: "DockerImage", Name: "mysql:latest"},
						To:   &corev1.LocalObjectReference{Name: "mysql"},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(isi.Status.Images).To(o.HaveLen(2))
		for _, image := range isi.Status.Images {
			o.Expect(image.Image).ToNot(o.BeNil(), fmt.Sprintf("image %s %#v", image.Tag, image.Status))
		}

		// TODO: we may race here with the cache, if this is a problem, loop
		g.By("verifying that layers for imported images are correct")
		var busyboxLayers []string
	Retry:
		for i := 0; ; i++ {
			layers, err := client.ImageStreams(oc.Namespace()).Layers("1", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			for i, image := range isi.Status.Images {
				l, ok := layers.Images[image.Image.Name]
				o.Expect(ok).To(o.BeTrue())
				if l.ImageMissing {
					e2e.Logf("Image %s is missing, retry", image.Image.Name)
					continue
				}
				o.Expect(len(l.Layers)).To(o.BeNumerically(">", 0))
				o.Expect(l.Config).ToNot(o.BeNil())
				o.Expect(layers.Blobs[*l.Config]).ToNot(o.BeNil())
				o.Expect(layers.Blobs[*l.Config].MediaType).To(o.Equal("application/vnd.docker.container.image.v1+json"))
				for _, layerID := range l.Layers {
					o.Expect(layers.Blobs).To(o.HaveKey(layerID))
					o.Expect(layers.Blobs[layerID].MediaType).NotTo(o.BeEmpty())
				}
				o.Expect(layers.Blobs).To(o.HaveKey(image.Image.Name))
				o.Expect(layers.Blobs[image.Image.Name].MediaType).To(o.Equal("application/vnd.docker.distribution.manifest.v2+json"))
				if i == 0 {
					busyboxLayers = l.Layers
					break Retry
				}
			}
			time.Sleep(time.Second)
			o.Expect(i).To(o.BeNumerically("<", 10), "Timed out waiting for layers to have expected data, got\n%#v\n%#v", layers, isi.Status.Images)
		}

		_, err = client.ImageStreams(oc.Namespace()).Create(&imagev1.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name: "output",
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		layers, err := client.ImageStreams(oc.Namespace()).Layers("output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(layers.Images).To(o.BeEmpty())
		o.Expect(layers.Blobs).To(o.BeEmpty())

		_, err = client.ImageStreams(oc.Namespace()).Layers("doesnotexist", metav1.GetOptions{})
		o.Expect(err).To(o.HaveOccurred())
		o.Expect(errors.IsNotFound(err)).To(o.BeTrue())

		dockerfile := `
FROM a
RUN mkdir -p /var/lib && echo "a" > /var/lib/file
`

		g.By("running a build based on our tagged layer")
		buildClient := buildv1client.NewForConfigOrDie(oc.UserConfig()).Build()
		_, err = buildClient.Builds(oc.Namespace()).Create(&buildv1.Build{
			ObjectMeta: metav1.ObjectMeta{
				Name: "output",
			},
			Spec: buildv1.BuildSpec{
				CommonSpec: buildv1.CommonSpec{
					Source: buildv1.BuildSource{
						Dockerfile: &dockerfile,
					},
					Strategy: buildv1.BuildStrategy{
						DockerStrategy: &buildv1.DockerBuildStrategy{
							From: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "1:busybox"},
						},
					},
					Output: buildv1.BuildOutput{
						To: &corev1.ObjectReference{Kind: "ImageStreamTag", Name: "output:latest"},
					},
				},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		newNamespace := oc.CreateProject()
		ns = append(ns)

		g.By("waiting for the build to finish")
		var lastBuild *buildv1.Build
		err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
			build, err := buildClient.Builds(oc.Namespace()).Get("output", metav1.GetOptions{})
			if err != nil {
				return false, err
			}
			o.Expect(build.Status.Phase).NotTo(o.Or(o.Equal(buildv1.BuildPhaseFailed), o.Equal(buildv1.BuildPhaseError), o.Equal(buildv1.BuildPhaseCancelled)))
			lastBuild = build
			return build.Status.Phase == buildv1.BuildPhaseComplete, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking the layers for the built image")
		layers, err = client.ImageStreams(oc.Namespace()).Layers("output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		to := lastBuild.Status.Output.To
		o.Expect(to).NotTo(o.BeNil())
		o.Expect(layers.Images).To(o.HaveKey(to.ImageDigest))
		builtImageLayers := layers.Images[to.ImageDigest]
		o.Expect(len(builtImageLayers.Layers)).To(o.Equal(len(busyboxLayers)+1), fmt.Sprintf("%#v", layers.Images))
		for i := range busyboxLayers {
			o.Expect(busyboxLayers[i]).To(o.Equal(builtImageLayers.Layers[i]))
		}

		g.By("tagging the built image into another namespace")
		_, err = client.ImageStreamTags(newNamespace).Create(&imagev1.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{
				Name: "output:latest",
			},
			Tag: &imagev1.TagReference{
				Name: "copied",
				From: &corev1.ObjectReference{Kind: "ImageStreamTag", Namespace: oc.Namespace(), Name: "output:latest"},
			},
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("checking that the image shows up in the other namespace")
		layers, err = client.ImageStreams(newNamespace).Layers("output", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(layers.Images).To(o.HaveKey(to.ImageDigest))
		o.Expect(layers.Images[to.ImageDigest]).To(o.Equal(builtImageLayers))
	})
})
